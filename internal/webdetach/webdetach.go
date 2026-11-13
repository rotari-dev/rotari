// Package webdetach keeps track of a detached `rotari web`: at most one per
// user and host, found through a record and a lease under the master
// directory, with its output in a log bounded by rotation. It knows nothing of
// HTTP or of how the server is started.
package webdetach

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rotari-dev/rotari/internal/lease"
)

// MaxLogBytes is about how large the log grows before it is rotated, as the
// Web event log is. One rotated generation is kept beside it.
const MaxLogBytes = 1 << 20

// holdWait lets a probe's shared lock pass before a starting server gives up.
const holdWait = time.Second

// ErrRunning says a detached server already holds this host's lease.
var ErrRunning = errors.New("a detached rotari web is already running on this host")

// ErrStarting says a detached server holds the lease but has not recorded
// its address yet.
var ErrStarting = errors.New("a detached rotari web on this host is starting and has not recorded its address yet; try again in a moment")

// Paths are the files of a host's detached server under a master directory.
type Paths struct {
	Host, Dir, Record, Lease, Log, RotatedLog string
}

// PathsFor returns the files of host's detached server under masterDir. The
// host name becomes part of file names, so it may not hold a separator.
func PathsFor(masterDir, host string) (Paths, error) {
	if host == "" || host == "." || host == ".." || strings.ContainsAny(host, `/\`) || strings.ContainsRune(host, 0) {
		return Paths{}, fmt.Errorf("host name %q cannot name a file", host)
	}
	dir := filepath.Join(masterDir, "web")
	base := filepath.Join(dir, host)
	return Paths{Host: host, Dir: dir, Record: base + ".json", Lease: base + ".lock", Log: base + ".log", RotatedLog: base + ".log.1"}, nil
}

// Settings are the options a detached server runs with, compared when one is
// asked for again. Port is the port it listens on.
type Settings struct {
	BaseDir                     string   `json:"basedir"`
	Host                        string   `json:"host"`
	Port                        int      `json:"port"`
	AllowControl                bool     `json:"allow_control"`
	Notifications               bool     `json:"notifications"`
	AllowExec                   bool     `json:"allow_exec"`
	AllowTerminal               bool     `json:"allow_terminal"`
	AllowedHosts                []string `json:"allowed_hosts"`
	ArtifactRoots               []string `json:"artifact_roots"`
	TerminalIdleTimeout         string   `json:"terminal_idle_timeout,omitempty"`
	TerminalMaxShells           int      `json:"terminal_max_shells,omitempty"`
	TerminalMaxShellsPerProject int      `json:"terminal_max_shells_per_project,omitempty"`
	TerminalReplaySize          int      `json:"terminal_replay_size,omitempty"`
	TerminalShell               string   `json:"terminal_shell,omitempty"`
	// TokenSHA256 identifies the server's token without holding it.
	TokenSHA256 string `json:"token_sha256,omitempty"`
}

// TokenDigest is how a record names a token.
func TokenDigest(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Differences names, as their flags, the settings in which want differs from
// s, a running server's. The port counts only when portExplicit; a token only
// when want names one, since a server started without one has generated its
// own; the terminal's settings only when both have the terminal on.
func (s Settings) Differences(want Settings, portExplicit bool) []string {
	var names []string
	add := func(differs bool, name string) {
		if differs {
			names = append(names, name)
		}
	}
	add(s.BaseDir != want.BaseDir, "basedir")
	add(s.Host != want.Host, "host")
	add(portExplicit && want.Port != 0 && s.Port != want.Port, "port")
	add(s.AllowControl != want.AllowControl, "allow-control")
	add(s.Notifications != want.Notifications, "notifications")
	add(s.AllowExec != want.AllowExec, "allow-exec")
	add(s.AllowTerminal != want.AllowTerminal, "allow-terminal")
	add(!sameSet(s.AllowedHosts, want.AllowedHosts), "allowed-host")
	add(!sameSet(s.ArtifactRoots, want.ArtifactRoots), "artifact-root")
	// The terminal's own settings matter only when both have a terminal.
	terminal := s.AllowTerminal && want.AllowTerminal
	add(terminal && s.TerminalIdleTimeout != want.TerminalIdleTimeout, "terminal-idle-timeout")
	add(terminal && s.TerminalMaxShells != want.TerminalMaxShells, "terminal-max-shells")
	add(terminal && s.TerminalMaxShellsPerProject != want.TerminalMaxShellsPerProject, "terminal-max-shells-per-project")
	add(terminal && s.TerminalReplaySize != want.TerminalReplaySize, "terminal-replay-size")
	add(terminal && s.TerminalShell != want.TerminalShell, "terminal-shell")
	add(want.TokenSHA256 != "" && s.TokenSHA256 != want.TokenSHA256, "auth-token")
	return names
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), slices.Compact(b))
}

// Record is what a detached server says of itself. It never holds the token.
type Record struct {
	Host       string    `json:"host"`
	PID        int       `json:"pid"`
	URL        string    `json:"url"`
	StartedAt  time.Time `json:"started_at"`
	Version    string    `json:"version"`
	Executable string    `json:"executable,omitempty"`
	Settings   Settings  `json:"settings"`
}

// Find returns the detached server of paths when one is running. A record
// left by a server that is gone is removed, under the shared lock so no new
// server can record itself meanwhile. A lease that cannot be probed, or a
// running server's record that cannot be read, is an error, never "not
// running".
func Find(paths Paths) (Record, bool, error) {
	held, err := lease.Probe(paths.Lease, func() error {
		if err := os.Remove(paths.Record); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove the record of a detached rotari web that stopped: %w", err)
		}
		return nil
	})
	if err != nil || !held {
		return Record{}, false, err
	}
	data, err := os.ReadFile(paths.Record)
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, false, ErrStarting
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("failed to read the record of the detached rotari web: %w", err)
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, false, fmt.Errorf("the record of the detached rotari web, %s, is not valid: %w", paths.Record, err)
	}
	return record, true, nil
}

// Holder is a detached server's hold on its host's lease.
type Holder struct {
	paths Paths
	lease *os.File
}

// Hold takes the host's lease for this process, or returns ErrRunning while
// another server holds it.
func Hold(paths Paths, dirMode os.FileMode) (*Holder, error) {
	if err := os.MkdirAll(paths.Dir, dirMode); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", paths.Dir, err)
	}
	file, err := lease.Hold(paths.Lease, 0o600, holdWait)
	if errors.Is(err, lease.ErrHeld) {
		return nil, ErrRunning
	}
	if err != nil {
		return nil, err
	}
	return &Holder{paths: paths, lease: file}, nil
}

// Publish writes record, readable only by its owner, replacing it whole.
func (holder *Holder) Publish(record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writeReplacing(holder.paths.Record, append(data, '\n'))
}

// Close removes the record and releases the lease.
func (holder *Holder) Close() error {
	err := os.Remove(holder.paths.Record)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if closeErr := holder.lease.Close(); err == nil {
		err = closeErr
	}
	return err
}

// OpenLog opens the log for a server about to start, first rotating one that
// has reached MaxLogBytes, and returns it with its size, from which the new
// server's output starts.
func OpenLog(paths Paths, dirMode os.FileMode) (*os.File, int64, error) {
	if err := os.MkdirAll(paths.Dir, dirMode); err != nil {
		return nil, 0, fmt.Errorf("failed to create %s: %w", paths.Dir, err)
	}
	info, err := os.Stat(paths.Log)
	switch {
	case err == nil && info.Size() >= MaxLogBytes:
		if err := os.Rename(paths.Log, paths.RotatedLog); err != nil {
			return nil, 0, fmt.Errorf("failed to rotate %s: %w", paths.Log, err)
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return nil, 0, err
	}
	file, err := os.OpenFile(paths.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open %s: %w", paths.Log, err)
	}
	info, err = file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, err
	}
	return file, info.Size(), nil
}

// KeepLogBounded rotates the log a running server writes through output,
// which it cannot reopen: past MaxLogBytes it copies the log to the rotated
// file and truncates it in place. Lines written between the copy and the
// truncation are lost.
func KeepLogBounded(paths Paths, output *os.File) error {
	info, err := output.Stat()
	if err != nil || info.Size() < MaxLogBytes {
		return err
	}
	source, err := os.Open(paths.Log)
	if err != nil {
		return err
	}
	defer source.Close()
	temporary, err := os.CreateTemp(paths.Dir, filepath.Base(paths.RotatedLog)+".*")
	if err != nil {
		return err
	}
	_, err = io.Copy(temporary, source)
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporary.Name(), paths.RotatedLog)
	}
	if err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	return output.Truncate(0)
}

// ReadLogFrom returns up to limit bytes of the log's end from offset on, as
// the reason a server that failed to start gives.
func ReadLogFrom(paths Paths, offset, limit int64) (string, error) {
	file, err := os.Open(paths.Log)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if info.Size()-offset > limit {
		offset = info.Size() - limit
	}
	if offset < 0 || offset > info.Size() {
		offset = 0
	}
	data, err := io.ReadAll(io.NewSectionReader(file, offset, info.Size()-offset))
	return string(data), err
}

func writeReplacing(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, err = temporary.Write(data)
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporary.Name(), path)
	}
	if err != nil {
		_ = os.Remove(temporary.Name())
	}
	return err
}
