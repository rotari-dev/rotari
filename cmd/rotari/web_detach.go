package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	stateinternal "github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/webdetach"
)

const (
	// webDetachedChildEnv marks the background process `rotari web --detach`
	// starts, which reports readiness on webDetachedReadyFD. It is removed
	// from the process's environment at once, so shells never see it.
	webDetachedChildEnv = "ROTARI_WEB_DETACHED_CHILD"
	webDetachedReadyFD  = 3
	// webDetachReadyTimeout bounds how long the starting command waits for
	// the server to listen.
	webDetachReadyTimeout = 30 * time.Second
	// webDetachLogRotation is how often a detached server checks its log.
	webDetachLogRotation = 30 * time.Second
	// webDetachFailureExcerpt bounds how much of the log a failed start shows.
	webDetachFailureExcerpt = 4096
)

// webDetachedReady is what a detached server reports once it listens.
type webDetachedReady struct {
	URL string `json:"url"`
	PID int    `json:"pid"`
}

// webDetachPaths are this host's detached-server files.
func webDetachPaths() (webdetach.Paths, error) {
	masterDir, err := stateinternal.ResolveMasterDir("")
	if err != nil {
		return webdetach.Paths{}, fmt.Errorf("failed to resolve the master directory: %w", err)
	}
	host, err := os.Hostname()
	if err != nil {
		return webdetach.Paths{}, fmt.Errorf("failed to read the host name: %w", err)
	}
	return webdetach.PathsFor(masterDir, host)
}

// webDetachSettings are the settings flags give a server of baseDir, with
// token as its token.
func webDetachSettings(flags webCommandFlags, baseDir, token string, port int) (webdetach.Settings, error) {
	absolute, err := filepath.Abs(baseDir)
	if err != nil {
		return webdetach.Settings{}, err
	}
	settings := webdetach.Settings{
		BaseDir: absolute, Host: flags.host, Port: port, AllowControl: flags.allowControl, Notifications: flags.notifications,
		AllowExec: flags.allowExec, AllowTerminal: flags.allowTerminal, AllowedHosts: flags.allowedHosts, ArtifactRoots: flags.artifactRoots,
		TokenSHA256: webdetach.TokenDigest(token),
	}
	if flags.allowTerminal {
		settings.TerminalIdleTimeout = flags.terminal.idleLimit.String()
		settings.TerminalMaxShells = flags.terminal.shellLimit
		settings.TerminalMaxShellsPerProject = flags.terminal.groupLimit
		settings.TerminalReplaySize = flags.terminal.ringSize
		settings.TerminalShell = flags.terminal.shell
	}
	return settings, nil
}

// startDetachedWeb starts the server of flags in the background, or reports
// the one already running on this host, and returns once it listens.
func startDetachedWeb(flags webCommandFlags, baseDir string) int {
	paths, err := webDetachPaths()
	if err != nil {
		printErrorf("cannot detach rotari web: %v", err)
		return 1
	}
	want, err := webDetachSettings(flags, baseDir, flags.authToken, flags.port)
	if err != nil {
		printErrorf("cannot detach rotari web: %v", err)
		return 1
	}
	running, found, err := webdetach.Find(paths)
	if err != nil {
		printErrorf("cannot detach rotari web: %v", err)
		return 1
	}
	if found {
		return reportDetachedWeb(running, want, flags, paths)
	}
	token, generated := flags.authToken, false
	if token == "" && (flags.allowExec || flags.allowTerminal) {
		if token, err = newWebAuthToken(); err != nil {
			printErrorf("cannot generate a Web token: %v", err)
			return 1
		}
		generated = true
	}
	warnUnauthenticatedHost(flags, token)
	ready, err := spawnDetachedWeb(paths, token, generated)
	if err != nil {
		printErrorf("%v", err)
		return 1
	}
	if flags.allowExec || flags.allowTerminal {
		printWebSignIn(flags, token, generated)
	}
	fmt.Printf("rotari web listening at %s in the background (PID %d)\n", ready.URL, ready.PID)
	printDetachedWebHelp(paths)
	return 0
}

// reportDetachedWeb answers a second --detach with the server already
// running, unless the options given differ from its own.
func reportDetachedWeb(running webdetach.Record, want webdetach.Settings, flags webCommandFlags, paths webdetach.Paths) int {
	if names := running.Settings.Differences(want, flags.portExplicit); len(names) != 0 {
		printErrorf("a detached rotari web (PID %d) is already running on this host with different --%s; stop it with rotari web --shutdown first, or run rotari web without --detach",
			running.PID, strings.Join(names, ", --"))
		return 1
	}
	fmt.Printf("rotari web listening at %s in the background (PID %d, already running since %s)\n", running.URL, running.PID, running.StartedAt.Local().Format(time.DateTime))
	if running.Settings.TokenSHA256 != "" && flags.authToken == "" {
		fmt.Fprintf(os.Stderr, "Sign in with user name %q and the token printed when it started; set %s to keep one token across restarts.\n", "rotari", envWebAuthToken)
	}
	printDetachedWebHelp(paths)
	return 0
}

func printDetachedWebHelp(paths webdetach.Paths) {
	fmt.Printf("Log: %s\nStop it with: rotari web --shutdown\n", paths.Log)
}

// printWebSignIn says how to sign in to a server that requires a token.
func printWebSignIn(flags webCommandFlags, token string, generated bool) {
	option := "--allow-exec"
	if !flags.allowExec {
		option = "--allow-terminal"
	}
	if generated {
		fmt.Fprintf(os.Stderr, "%s requires authentication. Sign in with user name %q and this token as the password:\n  %s\nSet %s to keep one token across restarts.\n", option, "rotari", token, envWebAuthToken)
	} else {
		fmt.Fprintf(os.Stderr, "%s requires authentication. Sign in with user name %q and the configured token as the password.\n", option, "rotari")
	}
}

// spawnDetachedWeb starts this command again in a session of its own, with
// its output in the host's log, and waits for it to report that it listens.
// A generated token reaches it in the environment, never in its arguments.
func spawnDetachedWeb(paths webdetach.Paths, token string, generated bool) (webDetachedReady, error) {
	executable, err := os.Executable()
	if err != nil {
		return webDetachedReady{}, fmt.Errorf("cannot detach rotari web: %w", err)
	}
	logFile, offset, err := webdetach.OpenLog(paths, stateinternal.DirectoryMode())
	if err != nil {
		return webDetachedReady{}, fmt.Errorf("cannot detach rotari web: %w", err)
	}
	defer logFile.Close()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return webDetachedReady{}, fmt.Errorf("cannot detach rotari web: %w", err)
	}
	defer devNull.Close()
	readyReader, readyWriter, err := os.Pipe()
	if err != nil {
		return webDetachedReady{}, fmt.Errorf("cannot detach rotari web: %w", err)
	}
	defer readyReader.Close()
	child := exec.Command(executable, os.Args[1:]...)
	child.Env = append(os.Environ(), webDetachedChildEnv+"=1")
	if generated {
		child.Env = append(child.Env, envWebAuthToken+"="+token)
	}
	child.Stdin, child.Stdout, child.Stderr = devNull, logFile, logFile
	child.ExtraFiles = []*os.File{readyWriter} // webDetachedReadyFD
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err = child.Start()
	_ = readyWriter.Close() // The child has its own copy; keeping this would hide its exit.
	if err != nil {
		return webDetachedReady{}, fmt.Errorf("cannot detach rotari web: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	reported := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(io.LimitReader(readyReader, 64<<10))
		reported <- data
	}()
	failed := func(reason string) error {
		excerpt, readErr := webdetach.ReadLogFrom(paths, offset, webDetachFailureExcerpt)
		message := fmt.Sprintf("rotari web did not start in the background (PID %d): %s; see %s", child.Process.Pid, reason, paths.Log)
		if readErr == nil && strings.TrimSpace(excerpt) != "" {
			message += "\n" + strings.TrimRight(excerpt, "\n")
		}
		return errors.New(message)
	}
	select {
	case data := <-reported:
		var ready webDetachedReady
		if len(data) != 0 && json.Unmarshal(data, &ready) == nil && ready.URL != "" {
			return ready, nil
		}
		select {
		case err := <-exited:
			return webDetachedReady{}, failed(describeExit(err))
		case <-time.After(5 * time.Second):
			_ = child.Process.Kill()
			return webDetachedReady{}, failed("it closed its readiness report without an address, and was stopped")
		}
	case <-time.After(webDetachReadyTimeout):
		_ = child.Process.Kill()
		return webDetachedReady{}, failed(fmt.Sprintf("it did not listen within %s, and was stopped", webDetachReadyTimeout))
	}
}

func describeExit(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return "it exited with status " + strconv.Itoa(exitErr.ExitCode())
	}
	if err != nil {
		return err.Error()
	}
	return "it exited with status 0 before listening"
}

// webDetachedChild is the background side of --detach: it holds the host's
// lease while it runs, records itself once it listens, and keeps its log
// bounded.
type webDetachedChild struct {
	paths  webdetach.Paths
	holder *webdetach.Holder
	ready  *os.File
}

// takeWebDetachedChild reports whether this process is a detached server, and
// clears the mark from its environment.
func takeWebDetachedChild() bool {
	if os.Getenv(webDetachedChildEnv) == "" {
		return false
	}
	_ = os.Unsetenv(webDetachedChildEnv)
	return true
}

// holdWebDetached takes this host's lease for a detached server.
func holdWebDetached() (*webDetachedChild, error) {
	ready := os.NewFile(webDetachedReadyFD, "rotari-web-ready")
	if _, err := ready.Stat(); err != nil {
		return nil, fmt.Errorf("the readiness report of a detached rotari web is missing: %w", err)
	}
	syscall.CloseOnExec(webDetachedReadyFD)
	paths, err := webDetachPaths()
	if err != nil {
		_ = ready.Close()
		return nil, err
	}
	holder, err := webdetach.Hold(paths, stateinternal.DirectoryMode())
	if err != nil {
		_ = ready.Close()
		return nil, err
	}
	return &webDetachedChild{paths: paths, holder: holder, ready: ready}, nil
}

// listening records the server, reports its address to the command that
// started it, and starts keeping the log bounded.
func (child *webDetachedChild) listening(flags webCommandFlags, baseDir string, address net.Addr) error {
	defer child.ready.Close()
	port := 0
	if tcp, ok := address.(*net.TCPAddr); ok {
		port = tcp.Port
	}
	settings, err := webDetachSettings(flags, baseDir, flags.authToken, port)
	if err != nil {
		return err
	}
	executable, _ := os.Executable()
	url := "http://" + address.String()
	record := webdetach.Record{Host: child.paths.Host, PID: os.Getpid(), URL: url, StartedAt: time.Now().UTC(), Version: versionString(), Executable: executable, Settings: settings}
	if err := child.holder.Publish(record); err != nil {
		return fmt.Errorf("failed to record the detached rotari web: %w", err)
	}
	if err := json.NewEncoder(child.ready).Encode(webDetachedReady{URL: url, PID: os.Getpid()}); err != nil {
		return fmt.Errorf("failed to report readiness: %w", err)
	}
	go child.keepLogBounded()
	return nil
}

func (child *webDetachedChild) keepLogBounded() {
	lastProblem := ""
	for range time.Tick(webDetachLogRotation) {
		problem := ""
		if err := webdetach.KeepLogBounded(child.paths, os.Stdout); err != nil {
			problem = err.Error()
		}
		if problem != "" && problem != lastProblem {
			printWarningf("WARNING: cannot rotate %s: %s", child.paths.Log, problem)
		}
		lastProblem = problem
	}
}

func (child *webDetachedChild) close() {
	if err := child.holder.Close(); err != nil {
		printWarningf("WARNING: %v", err)
	}
}

// noteDetachedWeb tells a foreground rotari web about a detached one on this
// host, whose shells it does not share.
func noteDetachedWeb() {
	paths, err := webDetachPaths()
	if err == nil {
		var running webdetach.Record
		var found bool
		running, found, err = webdetach.Find(paths)
		if found {
			fmt.Fprintf(os.Stderr, "Note: a detached rotari web is also running on this host at %s (PID %d); its shells are there, not here.\n", running.URL, running.PID)
		}
	}
	if err != nil && !errors.Is(err, webdetach.ErrStarting) {
		printWarningf("WARNING: cannot check for a detached rotari web: %v", err)
	}
}
