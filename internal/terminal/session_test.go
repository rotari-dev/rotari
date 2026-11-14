package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const testWait = 15 * time.Second

// startedShell is a shell whose output a goroutine collects.
type startedShell struct {
	*Session
	mu     sync.Mutex
	output bytes.Buffer
	ended  chan error
}

func startShell(t *testing.T, env ...string) *startedShell {
	t.Helper()
	home := t.TempDir()
	return startShellWith(t, Options{Shell: "/bin/sh", Dir: home, Env: append([]string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "PS1=$ "}, env...), Rows: 24, Cols: 80})
}

// startShellWith starts a shell as options say and collects its output.
func startShellWith(t *testing.T, options Options) *startedShell {
	t.Helper()
	session, err := Start(options)
	if err != nil {
		t.Fatal(err)
	}
	shell := &startedShell{Session: session, ended: make(chan error, 1)}
	t.Cleanup(session.Close)
	go func() {
		buffer := make([]byte, 4096)
		for {
			count, err := session.Read(buffer)
			shell.mu.Lock()
			shell.output.Write(buffer[:count])
			shell.mu.Unlock()
			if err != nil {
				shell.ended <- err
				return
			}
		}
	}()
	return shell
}

func (shell *startedShell) text() string {
	shell.mu.Lock()
	defer shell.mu.Unlock()
	return shell.output.String()
}

// run types a command and returns what it printed between two markers, which
// the typed command's own echo does not contain as one piece.
func (shell *startedShell) run(t *testing.T, command string) string {
	t.Helper()
	const left, right = "<<", ">>"
	if _, err := fmt.Fprintf(shell, "printf '%%s%%s' '%s' \"$(%s)\"; printf '%s\\n'\n", left, command, right); err != nil {
		t.Fatal(err)
	}
	var found string
	deadline := time.Now().Add(testWait)
	for time.Now().Before(deadline) {
		text := shell.text()
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimRight(line, "\r")
			if start := strings.Index(line, left); start >= 0 && strings.HasSuffix(line, right) && !strings.Contains(line, "printf") {
				found = line[start+len(left) : len(line)-len(right)]
			}
		}
		if found != "" {
			shell.mu.Lock()
			shell.output.Reset()
			shell.mu.Unlock()
			return found
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%q printed nothing in time; the terminal shows:\n%s", command, shell.text())
	return ""
}

func TestShellRunsWhatIsTyped(t *testing.T) {
	shell := startShell(t)
	if got := shell.run(t, "echo typed"); got != "typed" {
		t.Errorf("output = %q", got)
	}
	// A program can tell it has a terminal.
	if got := shell.run(t, "test -t 0 && echo terminal"); got != "terminal" {
		t.Errorf("standard input = %q, want a terminal", got)
	}
}

func TestShellGetsOnlyTheGivenEnvironment(t *testing.T) {
	t.Setenv("ROTARI_TERMINAL_TEST_SECRET", "secret")
	shell := startShell(t, "GIVEN=yes", "TERM=dumb")
	if got := shell.run(t, `echo "${GIVEN}-${TERM}-${ROTARI_TERMINAL_TEST_SECRET:-absent}"`); got != "yes-"+termType+"-absent" {
		t.Errorf("environment = %q", got)
	}
}

func TestResizeReachesPrograms(t *testing.T) {
	shell := startShell(t)
	if got := shell.run(t, "stty size"); got != "24 80" {
		t.Fatalf("first size = %q", got)
	}
	if err := shell.Resize(40, 120); err != nil {
		t.Fatal(err)
	}
	if got := shell.run(t, "stty size"); got != "40 120" {
		t.Errorf("size after Resize = %q", got)
	}
}

func TestStartAndResizeRejectBadInput(t *testing.T) {
	dir := t.TempDir()
	for name, options := range map[string]Options{
		"no shell":      {Dir: dir, Rows: 24, Cols: 80},
		"missing shell": {Shell: filepath.Join(dir, "missing"), Dir: dir, Rows: 24, Cols: 80},
		"missing dir":   {Shell: "/bin/sh", Dir: filepath.Join(dir, "missing"), Rows: 24, Cols: 80},
		"no rows":       {Shell: "/bin/sh", Dir: dir, Cols: 80},
		"no cols":       {Shell: "/bin/sh", Dir: dir, Rows: 24},
	} {
		if session, err := Start(options); err == nil {
			session.Close()
			t.Errorf("%s: Start succeeded", name)
		}
	}
	shell := startShell(t)
	if err := shell.Resize(0, 80); err == nil {
		t.Error("Resize to no rows succeeded")
	}
}

func TestShellExitIsReported(t *testing.T) {
	shell := startShell(t)
	if _, err := io.WriteString(shell, "exit 7\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-shell.Done():
	case <-time.After(testWait):
		t.Fatal("the shell did not exit")
	}
	if result := shell.Result(); result.ExitCode != 7 || result.Signal != 0 {
		t.Errorf("Result() = %+v, want exit 7", result)
	}
	if dir, err := shell.Dir(); err == nil {
		t.Errorf("Dir() of an exited shell = %q, want an error", dir)
	}
	if command, err := shell.Foreground(); err == nil {
		t.Errorf("Foreground() of an exited shell = %q, want an error", command)
	}
	select {
	case err := <-shell.ended:
		if !errors.Is(err, io.EOF) {
			t.Errorf("Read ended with %v, want io.EOF", err)
		}
	case <-time.After(testWait):
		t.Fatal("Read did not end after the shell exited")
	}
}

// awaitRead fails unless the goroutine reading the terminal ended with io.EOF.
func (shell *startedShell) awaitRead(t *testing.T) {
	t.Helper()
	select {
	case err := <-shell.ended:
		if !errors.Is(err, io.EOF) {
			t.Errorf("Read ended with %v, want io.EOF", err)
		}
	case <-time.After(testWait):
		t.Fatal("Read did not end after Close")
	}
}

// A Read is in progress here, as it always is in use: Close must still hang up
// at once, rather than waiting for the grace period and killing the shell.
func TestCloseHangsUpTheShellAndIsRepeatable(t *testing.T) {
	shell := startShell(t)
	shell.run(t, "echo ready")
	start := time.Now()
	shell.Close()
	shell.Close()
	if took := time.Since(start); took >= closeGrace {
		t.Errorf("Close took %v, the whole grace period", took)
	}
	select {
	case <-shell.Done():
	default:
		t.Fatal("Close returned before the shell exited")
	}
	// The hangup usually ends it, and a shell that finds its terminal gone first exits by
	// itself, as dash does with status 2. Either way, nothing had to kill it.
	if result := shell.Result(); result.Signal == syscall.SIGKILL {
		t.Errorf("Result() = %+v, want it to end by itself on the hangup", result)
	}
	shell.awaitRead(t)
}

func TestCloseKillsAShellThatIgnoresTheHangup(t *testing.T) {
	grace := closeGrace
	closeGrace = 300 * time.Millisecond
	t.Cleanup(func() { closeGrace = grace })
	shell := startShell(t)
	// A loop, since a shell reading its input ends by itself once input ends. It runs no
	// command, because dash takes the terminal back after each one, and with the terminal
	// gone that makes it exit with status 2 before anything has to kill it.
	if _, err := io.WriteString(shell, "trap '' HUP; echo trapped; while :; do :; done\n"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(testWait); !strings.Contains(shell.text(), "trapped\r\n"); {
		if time.Now().After(deadline) {
			t.Fatalf("the trap was not set; the terminal shows:\n%s", shell.text())
		}
		time.Sleep(10 * time.Millisecond)
	}
	shell.Close()
	if result := shell.Result(); result.Signal != syscall.SIGKILL {
		t.Errorf("Result() = %+v, want SIGKILL", result)
	}
}

// readPID waits for a process to write its PID to path.
func readPID(t *testing.T, path string) int {
	t.Helper()
	for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		data, err := os.ReadFile(path)
		if err == nil && bytes.HasSuffix(data, []byte("\n")) {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatalf("%s holds %q", path, data)
			}
			return pid
		}
	}
	t.Fatalf("no process wrote %s", path)
	return 0
}

// checkAlive fails unless pid is still running shortly after a Close.
func checkAlive(t *testing.T, pid int) {
	t.Helper()
	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err != nil {
		t.Errorf("process %d did not survive Close: %v", pid, err)
	}
}

// What ignores the hangup keeps running and keeps the terminal's other end open;
// Close must still end the Read, or the connection's goroutine would never end. The
// command writes its PID only once nohup has started it: closing before nohup ignores
// the hangup would rightly end it.
func TestCloseLeavesProcessesThatIgnoreTheHangup(t *testing.T) {
	shell := startShell(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	shell.run(t, `nohup sh -c 'echo $$ > `+pidFile+`; exec sleep 300' >/dev/null 2>&1 & echo started`)
	pid := readPID(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	shell.Close()
	shell.awaitRead(t)
	checkAlive(t, pid)
}

// What leaves the shell's session keeps running, so a run started in a terminal
// outlives it.
func TestCloseLeavesDetachedProcessesRunning(t *testing.T) {
	shell := startShell(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	shell.run(t, `setsid sh -c 'echo $$ > `+pidFile+`; exec sleep 300' >/dev/null 2>&1 < /dev/null & echo started`)
	pid := readPID(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	shell.Close()
	checkAlive(t, pid)
}
