package web

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
	"golang.org/x/sys/unix"
)

var detachedPattern = regexp.MustCompile(`rotari web listening at (http://\S+) in the background \(PID (\d+)`)

// detachedWeb is a server `rotari web --detach` reported.
type detachedWeb struct {
	url string
	pid int
}

// startDetached runs `rotari web --detach` with args and returns what it reports;
// the server is stopped when the test ends.
func startDetached(t *testing.T, e *support.Env, args ...string) (detachedWeb, support.Result) {
	t.Helper()
	result := e.Rotari(append([]string{"web", "--detach", "--port", "0"}, args...)...)
	match := detachedPattern.FindStringSubmatch(result.Stdout)
	if result.Code != 0 || match == nil {
		t.Fatalf("rotari web --detach: %s", result)
	}
	pid, _ := strconv.Atoi(match[2])
	server := detachedWeb{url: match[1], pid: pid}
	t.Cleanup(func() { stopDetached(t, e, server.pid, syscall.SIGKILL) })
	return server, result
}

// stopDetached signals pid only while it is a rotari of this test's master
// directory, so a PID reused by another process is never hit.
func stopDetached(t *testing.T, e *support.Env, pid int, signal syscall.Signal) {
	t.Helper()
	environment, err := support.ProcessEnvironment(pid)
	if err != nil || !slices.Contains(environment, "ROTARI_MASTERDIR="+e.Master) {
		return
	}
	_ = syscall.Kill(pid, signal)
}

func detachedFiles(t *testing.T, e *support.Env) (record, log string) {
	t.Helper()
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(e.Master, "web", host+".json"), filepath.Join(e.Master, "web", host+".log")
}

// sessionOf is the session ID of pid.
func sessionOf(t *testing.T, pid int) int {
	t.Helper()
	session, err := unix.Getsid(pid)
	if err != nil {
		t.Fatalf("session of %d: %v", pid, err)
	}
	return session
}

// TestWebDetachRunsInTheBackground checks that --detach returns once the server
// listens, in a session of its own, that running it again reports that server
// or refuses other options, and that its shells see neither the token nor the
// detach mark.
func TestWebDetachRunsInTheBackground(t *testing.T) {
	covers(t, "WEB-40")
	e := terminalEnv(t)
	server, first := startDetached(t, e, "--allow-terminal")
	record, log := detachedFiles(t, e)
	if !strings.Contains(first.Stdout, "Log: "+log) || !strings.Contains(first.Stdout, "rotari web --shutdown") {
		t.Errorf("--detach did not name its log and how to stop it:\n%s", first.Stdout)
	}
	if !strings.Contains(first.Stderr, "Shells end when the server stops or you sign out.") {
		t.Errorf("--detach did not say how long shells last:\n%s", first.Stderr)
	}
	if session := sessionOf(t, server.pid); session != server.pid {
		t.Errorf("the detached server is in session %d, not one of its own", session)
	}
	if status := e.HTTPDo(http.MethodGet, server.url+"/", map[string]string{"X-Rotari-Token": launchToken}, "").Status; status != http.StatusOK {
		t.Errorf("GET / after --detach returned: %d", status)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("no record: %v", err)
	}
	if strings.Contains(string(data), launchToken) {
		t.Errorf("the record holds the token:\n%s", data)
	}

	again := e.MustRotari("web", "--detach", "--allow-terminal", "--port", "0")
	if match := detachedPattern.FindStringSubmatch(again.Stdout); match == nil || match[1] != server.url || match[2] != strconv.Itoa(server.pid) || !strings.Contains(again.Stdout, "already running") {
		t.Errorf("a second --detach did not report the running server %v:\n%s", server, again)
	}
	other := e.Rotari("web", "--detach", "--port", "0")
	if other.Code == 0 || !strings.Contains(other.Stderr, "--allow-terminal") || !strings.Contains(other.Stderr, "PID "+strconv.Itoa(server.pid)) || !strings.Contains(other.Stderr, "--shutdown") {
		t.Errorf("--detach with other options did not fail naming them and the PID:\n%s", other)
	}

	page := openTerminalPage(t, server.url)
	envFile := filepath.Join(e.Root, "shell-env")
	page.typeLine(`env > ` + envFile + `; echo env-$((40+2))`)
	page.await("env-42")
	shellEnv, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ROTARI_WEB_DETACHED_CHILD=", "ROTARI_WEB_AUTH_TOKEN="} {
		if strings.Contains(string(shellEnv), name) {
			t.Errorf("a detached server's shell has %s in its environment", name)
		}
	}

	stopDetached(t, e, server.pid, syscall.SIGTERM)
	page.awaitClose(terminalStopping)
	awaitProcessGone(t, server.pid, "SIGTERM")
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Errorf("the record outlived its server: %v", err)
	}
}

// TestWebDetachGeneratesATokenOutsideItsArguments checks that a token --detach
// generates is printed once and reaches the server without appearing in its
// command line or record.
func TestWebDetachGeneratesATokenOutsideItsArguments(t *testing.T) {
	covers(t, "WEB-40")
	e := support.NewEnv(t)
	e.Setenv("SHELL", "/bin/sh")
	server, result := startDetached(t, e, "--allow-terminal")
	token := regexp.MustCompile(`(?m)^  ([0-9a-f]{32,})$`).FindStringSubmatch(result.Stderr)
	if token == nil {
		t.Fatalf("no generated token printed:\n%s", result)
	}
	args, running, err := support.ProcessArgs(server.pid)
	if err != nil || !running {
		t.Fatalf("cannot read the server's command line: running %v, %v", running, err)
	}
	if commandLine := strings.Join(args, " "); strings.Contains(commandLine, token[1]) {
		t.Errorf("the token is in the server's command line: %q", commandLine)
	}
	record, log := detachedFiles(t, e)
	for _, path := range []string{record, log} {
		if data, err := os.ReadFile(path); err != nil || strings.Contains(string(data), token[1]) {
			t.Errorf("%s holds the token or cannot be read: %v", path, err)
		}
	}
	if status := e.HTTPDo(http.MethodGet, server.url+"/", map[string]string{"X-Rotari-Token": token[1]}, "").Status; status != http.StatusOK {
		t.Errorf("the printed token was refused: %d", status)
	}
	again := e.MustRotari("web", "--detach", "--allow-terminal", "--port", "0")
	if strings.Contains(again.Stdout+again.Stderr, token[1]) || !strings.Contains(again.Stderr, "token printed when it started") {
		t.Errorf("a second --detach must not show the token again, and must say which to use:\n%s", again)
	}
}

// TestWebDetachReportsAFailedStart checks that a server that cannot listen makes
// --detach fail with the reason and its log, and leaves no record.
func TestWebDetachReportsAFailedStart(t *testing.T) {
	covers(t, "WEB-40")
	e := support.NewEnv(t)
	busy := e.StartWeb()
	port := busy[strings.LastIndex(busy, ":")+1:]
	result := e.Rotari("web", "--detach", "--port", port)
	_, log := detachedFiles(t, e)
	if result.Code == 0 || !strings.Contains(result.Stderr, log) || !strings.Contains(result.Stderr, "address already in use") {
		t.Errorf("--detach on a busy port did not fail with the reason and the log:\n%s", result)
	}
	record, _ := detachedFiles(t, e)
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Errorf("a failed start left a record: %v", err)
	}
}

// TestWebDetachStartsAgainAfterACrash checks that a detached server killed
// without a chance to clean up does not count as running.
func TestWebDetachStartsAgainAfterACrash(t *testing.T) {
	covers(t, "WEB-40")
	e := support.NewEnv(t)
	first, _ := startDetached(t, e)
	stopDetached(t, e, first.pid, syscall.SIGKILL)
	awaitProcessGone(t, first.pid, "SIGKILL")
	second, result := startDetached(t, e)
	if second.pid == first.pid || strings.Contains(result.Stdout, "already running") {
		t.Errorf("--detach after a crash reported the dead server:\n%s", result)
	}
	if status := e.HTTPGet(second.url + "/").Status; status != http.StatusOK {
		t.Errorf("GET / of the new server returned: %d", status)
	}
}

// TestWebNotesADetachedServer checks that a foreground rotari web says when a
// detached one runs on the host, and still starts.
func TestWebNotesADetachedServer(t *testing.T) {
	covers(t, "WEB-40")
	e := support.NewEnv(t)
	detached, _ := startDetached(t, e)
	command := e.Command("web", "--port", "0")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	buffer := make([]byte, 4096)
	deadline := time.Now().Add(10 * time.Second)
	var shown strings.Builder
	for !strings.Contains(shown.String(), "listening at") && time.Now().Before(deadline) {
		n, err := stdout.Read(buffer)
		shown.Write(buffer[:n])
		if err != nil {
			break
		}
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	if !strings.Contains(shown.String(), "listening at") {
		t.Fatalf("the foreground server did not start:\n%s\n%s", shown.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), detached.url) || !strings.Contains(stderr.String(), strconv.Itoa(detached.pid)) {
		t.Errorf("the foreground server did not name the detached one:\n%s", stderr.String())
	}
}
