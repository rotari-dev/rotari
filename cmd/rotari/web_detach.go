package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

// webDetachedReady is what a background server reports on its readiness pipe: its
// address once it listens, and for an attached one, Event says which report this is
// and Reason why a detach was refused.
type webDetachedReady struct {
	Event  string `json:"event,omitempty"`
	URL    string `json:"url,omitempty"`
	PID    int    `json:"pid,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// The reports of an attached server, and the client's messages on its lifeline.
const (
	webEventReady         = "ready"
	webEventDetached      = "detached"
	webEventRefused       = "refused"
	webLifelineStop       = "stop"
	webLifelineDetach     = "detach"
	webLifelineDisconnect = "disconnect"
	webDisconnectStop     = "stop"
	webDisconnectDetach   = "detach"
)

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
	if flags.allowTerminal {
		fmt.Fprintln(os.Stderr, webIdleTimeoutLine(flags.terminal.idleLimit))
	}
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
	printDetachedWebBinaryNote(running)
	if running.Settings.TokenSHA256 != "" && flags.authToken == "" {
		fmt.Fprintf(os.Stderr, "Sign in with user name %q and the token printed when it started; set %s to keep one token across restarts.\n", "rotari", envWebAuthToken)
	}
	printDetachedWebHelp(paths)
	if running.Settings.AllowTerminal {
		if limit, err := time.ParseDuration(running.Settings.TerminalIdleTimeout); err == nil {
			fmt.Fprintln(os.Stderr, webIdleTimeoutLine(limit))
		}
	}
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
	paths webdetach.Paths
	ready *os.File
	// lifeline is the attached client's pipe, nil for a server started detached;
	// action is what losing it does: stop or detach.
	lifeline *os.File
	action   string

	mu sync.Mutex
	// holder is set once the server holds the host's lease; record is what it
	// publishes then, and shutdown stops the server.
	holder   *webdetach.Holder
	record   webdetach.Record
	shutdown func()
	refusal  string
}

// webChildMode is what webDetachedChildEnv says this process is: a server started
// detached, or one attached to a client, with the disconnect action after the prefix.
const (
	webChildDetached       = "1"
	webChildAttachedPrefix = "attached-"
	webAttachedLifelineFD  = 4
)

// takeWebChildMode returns webDetachedChildEnv's value, empty for a command run by
// hand, and clears it from the environment so shells never see it.
func takeWebChildMode() string {
	mode := os.Getenv(webDetachedChildEnv)
	_ = os.Unsetenv(webDetachedChildEnv)
	return mode
}

// inheritedFile opens a descriptor the starting command passed, closed on exec so no
// shell inherits it.
func inheritedFile(fd int, name string) (*os.File, error) {
	file := os.NewFile(uintptr(fd), name)
	if _, err := file.Stat(); err != nil {
		return nil, fmt.Errorf("the %s of a background rotari web is missing: %w", name, err)
	}
	syscall.CloseOnExec(fd)
	return file, nil
}

// holdWebDetached takes this host's lease for a server started detached.
func holdWebDetached() (*webDetachedChild, error) {
	ready, err := inheritedFile(webDetachedReadyFD, "readiness report")
	if err != nil {
		return nil, err
	}
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

// attachWebChild prepares a server attached to its client, which may detach it
// later. It holds no lease until then.
func attachWebChild(action string) (*webDetachedChild, error) {
	if action != webDisconnectStop && action != webDisconnectDetach {
		return nil, fmt.Errorf("unknown disconnect action %q", action)
	}
	ready, err := inheritedFile(webDetachedReadyFD, "readiness report")
	if err != nil {
		return nil, err
	}
	lifeline, err := inheritedFile(webAttachedLifelineFD, "client lifeline")
	if err != nil {
		_ = ready.Close()
		return nil, err
	}
	paths, err := webDetachPaths()
	if err != nil {
		_ = ready.Close()
		_ = lifeline.Close()
		return nil, err
	}
	// Its output goes to the client until it detaches; a client that is gone must not
	// kill it on its next line.
	signal.Ignore(syscall.SIGPIPE)
	return &webDetachedChild{paths: paths, ready: ready, lifeline: lifeline, action: action}, nil
}

// listening runs once the server listens. A server started detached records itself,
// reports its address, and starts keeping its log bounded. An attached one reports its
// address and follows its client from then on; shutdown stops it.
func (child *webDetachedChild) listening(flags webCommandFlags, baseDir string, address net.Addr, shutdown func()) error {
	port := 0
	if tcp, ok := address.(*net.TCPAddr); ok {
		port = tcp.Port
	}
	settings, err := webDetachSettings(flags, baseDir, flags.authToken, port)
	if err != nil {
		return err
	}
	executable, _ := os.Executable()
	url := "http://" + address.String() // NOSONAR: rotari web serves plain HTTP; this records the address it actually listens on.
	child.mu.Lock()
	child.record = webdetach.Record{Host: child.paths.Host, PID: os.Getpid(), URL: url, StartedAt: time.Now().UTC(), Version: versionString(), Executable: executable, Settings: settings, Binary: recordedBinary()}
	child.shutdown = shutdown
	child.mu.Unlock()
	if child.lifeline != nil {
		// announced reports readiness once the address line is out.
		return nil
	}
	defer child.ready.Close()
	if err := child.holder.Publish(child.record); err != nil {
		return fmt.Errorf("failed to record the detached rotari web: %w", err)
	}
	if err := child.send(webDetachedReady{URL: url, PID: os.Getpid()}); err != nil {
		return fmt.Errorf("failed to report readiness: %w", err)
	}
	go child.keepLogBounded()
	return nil
}

// announced tells an attached server's client that it listens, after the server has
// printed its address, so the client's hints follow that line, and starts following
// the client.
func (child *webDetachedChild) announced() error {
	if child.lifeline == nil {
		return nil
	}
	child.mu.Lock()
	record := child.record
	child.mu.Unlock()
	if err := child.send(webDetachedReady{Event: webEventReady, URL: record.URL, PID: record.PID}); err != nil {
		return fmt.Errorf("failed to report readiness: %w", err)
	}
	go child.followClient()
	return nil
}

func (child *webDetachedChild) send(event webDetachedReady) error {
	return json.NewEncoder(child.ready).Encode(event)
}

// followClient reads the client's lifeline: stop, detach, or disconnect, the last
// also being what its end means, however the client went.
func (child *webDetachedChild) followClient() {
	lines := bufio.NewScanner(child.lifeline)
	for lines.Scan() {
		switch lines.Text() {
		case webLifelineStop:
			child.shutdown()
			return
		case webLifelineDetach:
			if child.detach() {
				return
			}
		case webLifelineDisconnect:
			child.disconnected()
			return
		}
	}
	child.disconnected()
}

// disconnected follows the disconnect action: detach when asked and possible, stop
// otherwise, saying in the log why it did not detach.
func (child *webDetachedChild) disconnected() {
	if child.action == webDisconnectDetach {
		if child.detach() {
			return
		}
		child.noteInLog("rotari web: stopping, since its client went and it could not detach: " + child.lastRefusal())
	}
	child.shutdown()
}

// detach takes the host's lease, records the server, and moves its output to the log,
// telling the client; it refuses, telling the client why, when another server holds the
// lease or a step fails.
func (child *webDetachedChild) detach() bool {
	refuse := func(reason string) bool {
		child.mu.Lock()
		child.refusal = reason
		child.mu.Unlock()
		_ = child.send(webDetachedReady{Event: webEventRefused, Reason: reason})
		return false
	}
	holder, err := webdetach.Hold(child.paths, stateinternal.DirectoryMode())
	if errors.Is(err, webdetach.ErrRunning) {
		if running, found, findErr := webdetach.Find(child.paths); findErr == nil && found {
			return refuse(fmt.Sprintf("a detached rotari web is already running on this host at %s (PID %d); stop it with rotari web --shutdown first", running.URL, running.PID))
		}
		return refuse(err.Error())
	}
	if err != nil {
		return refuse(err.Error())
	}
	child.mu.Lock()
	record := child.record
	child.mu.Unlock()
	if err := holder.Publish(record); err != nil {
		_ = holder.Close()
		return refuse("failed to record the detached rotari web: " + err.Error())
	}
	logFile, _, err := webdetach.OpenLog(child.paths, stateinternal.DirectoryMode())
	if err == nil {
		err = redirectOutput(logFile)
		_ = logFile.Close()
	}
	if err != nil {
		_ = holder.Close()
		return refuse("failed to move its output to " + child.paths.Log + ": " + err.Error())
	}
	child.mu.Lock()
	child.holder = holder
	child.mu.Unlock()
	go child.keepLogBounded()
	_ = child.send(webDetachedReady{Event: webEventDetached, URL: record.URL, PID: record.PID})
	_ = child.ready.Close()
	_ = child.lifeline.Close()
	return true
}

func (child *webDetachedChild) lastRefusal() string {
	child.mu.Lock()
	defer child.mu.Unlock()
	return child.refusal
}

// noteInLog appends line to the host's log, for a server whose client is gone.
func (child *webDetachedChild) noteInLog(line string) {
	logFile, _, err := webdetach.OpenLog(child.paths, stateinternal.DirectoryMode())
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(logFile, time.Now().Format(time.RFC3339)+" "+line)
	_ = logFile.Close()
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

// close releases the lease and removes the record of a server that holds them.
func (child *webDetachedChild) close() {
	child.mu.Lock()
	holder := child.holder
	child.mu.Unlock()
	if holder == nil {
		return
	}
	if err := holder.Close(); err != nil {
		printWarningf("WARNING: %v", err)
	}
}

// printDetachedWebBinaryNote says, on stderr, when the detached server runs another
// rotari binary than this command.
func printDetachedWebBinaryNote(running webdetach.Record) {
	if note := binaryNote(fmt.Sprintf("the detached rotari web (PID %d) runs", running.PID), running.Binary); note != "" {
		fmt.Fprintln(os.Stderr, note)
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
			printDetachedWebBinaryNote(running)
		}
	}
	if err != nil && !errors.Is(err, webdetach.ErrStarting) {
		printWarningf("WARNING: cannot check for a detached rotari web: %v", err)
	}
}
