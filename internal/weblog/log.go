// Package weblog records bounded, allow-listed Web authentication and server
// startup evidence in the master directory.
package weblog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/webui"
)

const (
	FileName        = "web-events.jsonl"
	RotatedFileName = "web-events.jsonl.1"
	maxFileBytes    = 1 << 20
	rateWindow      = time.Minute
	rateLimit       = 20
)

// Start describes the allow-listed facts recorded once a listener is up.
type Start struct {
	Version                string
	ExecutableSHA256       string
	ExecutableModified     time.Time
	Host                   string
	Port                   int
	Auth                   string
	AllowExec              bool
	AllowTerminal          bool
	AllowControl           bool
	Notifications          bool
	ArtifactRootCount      int
	RegisteredBaseDirCount int
}

type record struct {
	Time                   string `json:"time"`
	Event                  string `json:"event"`
	PID                    int    `json:"pid"`
	Command                string `json:"command"`
	Outcome                string `json:"outcome,omitempty"`
	Transport              string `json:"transport,omitempty"`
	Cookie                 string `json:"cookie,omitempty"`
	CookieSecure           *bool  `json:"cookie_secure,omitempty"`
	UserAgent              string `json:"user_agent,omitempty"`
	Suppressed             int    `json:"suppressed,omitempty"`
	Version                string `json:"version,omitempty"`
	ExecutableSHA256       string `json:"executable_sha256,omitempty"`
	ExecutableModified     string `json:"executable_modified,omitempty"`
	Host                   string `json:"host,omitempty"`
	Port                   *int   `json:"port,omitempty"`
	Auth                   string `json:"auth,omitempty"`
	AllowExec              *bool  `json:"allow_exec,omitempty"`
	AllowTerminal          *bool  `json:"allow_terminal,omitempty"`
	AllowControl           *bool  `json:"allow_control,omitempty"`
	Notifications          *bool  `json:"notifications,omitempty"`
	ArtifactRootCount      *int   `json:"artifact_root_count,omitempty"`
	RegisteredBaseDirCount *int   `json:"registered_basedir_count,omitempty"`
}

type rateState struct {
	windowStart time.Time
	written     int
	suppressed  int
}

// Recorder queues records so filesystem latency and errors never change a Web
// response. Close flushes records already accepted into the queue.
type Recorder struct {
	path     string
	lockPath string
	command  string
	pid      int
	now      func() time.Time
	warning  io.Writer
	warnOnce sync.Once
	queue    chan record
	done     chan struct{}
	mu       sync.Mutex
	closed   bool
	rates    map[string]*rateState
}

// New starts a recorder for command under masterDir.
func New(masterDir, command string, warning io.Writer) *Recorder {
	recorder := &Recorder{
		path: filepath.Join(masterDir, FileName), lockPath: filepath.Join(masterDir, ".web-events.lock"),
		command: command, pid: os.Getpid(), now: time.Now, warning: warning,
		queue: make(chan record, 128), done: make(chan struct{}), rates: make(map[string]*rateState),
	}
	go recorder.writeLoop()
	return recorder
}

// Path returns the current event log path under masterDir.
func Path(masterDir string) string { return filepath.Join(masterDir, FileName) }

// RecordAuthEvent implements webui.AuthRecorder.
func (recorder *Recorder) RecordAuthEvent(event webui.AuthEvent) {
	if recorder == nil {
		return
	}
	userAgent := []byte(event.UserAgent)
	if len(userAgent) > 256 {
		userAgent = userAgent[:256]
	}
	recorder.enqueue(record{
		Event: event.Event, Outcome: event.Outcome, Transport: event.Transport,
		Cookie: event.Cookie, CookieSecure: event.CookieSecure, UserAgent: string(userAgent),
	}, true)
}

// RecordServerStart records a listener that is ready to serve.
func (recorder *Recorder) RecordServerStart(start Start) {
	if recorder == nil {
		return
	}
	modified := ""
	if !start.ExecutableModified.IsZero() {
		modified = start.ExecutableModified.UTC().Format(time.RFC3339Nano)
	}
	recorder.enqueue(record{
		Event: "server_start", Version: start.Version, ExecutableSHA256: start.ExecutableSHA256,
		ExecutableModified: modified, Host: start.Host, Port: pointer(start.Port), Auth: start.Auth,
		AllowExec: pointer(start.AllowExec), AllowTerminal: pointer(start.AllowTerminal), AllowControl: pointer(start.AllowControl),
		Notifications: pointer(start.Notifications), ArtifactRootCount: pointer(start.ArtifactRootCount),
		RegisteredBaseDirCount: pointer(start.RegisteredBaseDirCount),
	}, false)
}

func pointer[T any](value T) *T { return &value }

func (recorder *Recorder) enqueue(next record, limited bool) {
	now := recorder.now()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if recorder.closed {
		return
	}
	if limited {
		state := recorder.rates[next.Event]
		if state == nil || now.Sub(state.windowStart) >= rateWindow {
			if state == nil {
				state = &rateState{}
				recorder.rates[next.Event] = state
			}
			next.Suppressed = state.suppressed
			*state = rateState{windowStart: now, written: 1}
		} else if state.written >= rateLimit {
			state.suppressed++
			return
		} else {
			state.written++
		}
	}
	next.Time = now.UTC().Format(time.RFC3339Nano)
	next.PID = recorder.pid
	next.Command = recorder.command
	select {
	case recorder.queue <- next:
	default:
		if limited {
			recorder.rates[next.Event].suppressed++
		}
	}
}

// Close flushes queued records and stops the writer.
func (recorder *Recorder) Close() {
	recorder.mu.Lock()
	if !recorder.closed {
		recorder.closed = true
		close(recorder.queue)
	}
	recorder.mu.Unlock()
	<-recorder.done
}

func (recorder *Recorder) writeLoop() {
	defer close(recorder.done)
	for next := range recorder.queue {
		data, err := json.Marshal(next)
		if err == nil {
			data = append(data, '\n')
			err = recorder.append(data)
		}
		if err != nil {
			recorder.warnOnce.Do(func() {
				if recorder.warning != nil {
					fmt.Fprintf(recorder.warning, "WARNING: cannot record Web events: %v\n", err)
				}
			})
		}
	}
}

func (recorder *Recorder) append(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(recorder.path), state.DirectoryMode()); err != nil {
		return err
	}
	release, err := state.AcquireStateLock(recorder.lockPath)
	if err != nil {
		return err
	}
	defer release()
	if info, statErr := os.Stat(recorder.path); statErr == nil && info.Size()+int64(len(data)) > maxFileBytes {
		_ = os.Remove(filepath.Join(filepath.Dir(recorder.path), RotatedFileName))
		if err := os.Rename(recorder.path, filepath.Join(filepath.Dir(recorder.path), RotatedFileName)); err != nil {
			return err
		}
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	file, err := os.OpenFile(recorder.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	written, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return closeErr
}
