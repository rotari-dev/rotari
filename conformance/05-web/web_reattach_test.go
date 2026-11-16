package web

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

var reattachedListening = regexp.MustCompile(`rotari web listening at (http://\S+) in the background \(PID (\d+), running since [^)]+\); showing its log`)

// reattach runs rotari web with args at a terminal while server is detached with the
// same options, and checks that it attached to that server rather than starting one.
func reattach(t *testing.T, server detachedWeb, web *attachedWeb) {
	t.Helper()
	match := web.await(reattachedListening)
	if match[1] != server.url || match[2] != strconv.Itoa(server.pid) {
		t.Fatalf("attached to %s (PID %s), want the detached server at %s (PID %d):\n%s", match[1], match[2], server.url, server.pid, web.text())
	}
	web.await(regexp.MustCompile(`Press Ctrl-D to detach and keep the server running; Ctrl-C to stop it\.`))
	if children := childrenOf(t, web.command.Process.Pid); len(children) != 0 {
		t.Errorf("rotari web started processes %v beside a detached server with its options", children)
	}
}

// TestWebClientReattachesToTheDetachedServer checks that rotari web at a terminal,
// while a server with the options given is detached on the host, attaches to it: it
// shows what the server logs from then on, Ctrl-D, SIGHUP, and SIGTERM leave it
// running, Ctrl-C stops it, and the client exits once the server stops however it was
// stopped, failing when it went without cleaning up.
func TestWebClientReattachesToTheDetachedServer(t *testing.T) {
	covers(t, "WEB-61")
	args := []string{"--allow-terminal", "--terminal-idle-timeout", "90m"}
	e := terminalEnv(t)
	server, _ := startDetached(t, e, args...)
	_, log := detachedFiles(t, e)

	appendLog := func(line string) {
		t.Helper()
		file, err := os.OpenFile(log, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if _, err := file.WriteString(line + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	appendLog("logged-" + "before attaching")
	web := startAttached(t, e, args...)
	reattach(t, server, web)
	web.await(regexp.MustCompile(`Shells end 1h30m after the last page closes`))
	appendLog("logged-" + "after attaching")
	web.await(regexp.MustCompile(`logged-after attaching`))
	if strings.Contains(web.text(), "logged-before attaching") {
		t.Errorf("the client showed what the server logged before it attached:\n%s", web.text())
	}
	web.press("\x04")
	if match := web.await(attachedDetached); match[1] != server.url || match[2] != strconv.Itoa(server.pid) {
		t.Errorf("Ctrl-D named %s (PID %s)", match[1], match[2])
	}
	if code := web.exitCode(); code != 0 {
		t.Errorf("the client exited with %d after Ctrl-D", code)
	}
	if !serving(server.url) {
		t.Fatal("Ctrl-D of a reattached client stopped the server")
	}

	for _, signal := range []syscall.Signal{syscall.SIGHUP, syscall.SIGTERM} {
		web := startAttached(t, e, args...)
		reattach(t, server, web)
		if err := web.command.Process.Signal(signal); err != nil {
			t.Fatal(err)
		}
		web.exitCode()
		if !serving(server.url) {
			t.Fatalf("%s to a reattached client stopped the server", signal)
		}
	}

	web = startAttached(t, e, args...)
	reattach(t, server, web)
	e.MustRotari("web", "--shutdown", "--yes")
	web.await(regexp.MustCompile(`Stopped rotari web at ` + regexp.QuoteMeta(server.url) + ` \(PID ` + strconv.Itoa(server.pid) + `\)\.`))
	if code := web.exitCode(); code != 0 {
		t.Errorf("the client exited with %d after rotari web --shutdown", code)
	}

	server, _ = startDetached(t, e, args...)
	web = startAttached(t, e, args...)
	reattach(t, server, web)
	web.press("\x03")
	web.await(regexp.MustCompile(`Stopped rotari web at ` + regexp.QuoteMeta(server.url)))
	if code := web.exitCode(); code != 0 {
		t.Errorf("the client exited with %d after Ctrl-C", code)
	}
	awaitServing(t, server.url, false, "after Ctrl-C")
	if record, _ := detachedFiles(t, e); fileExists(record) {
		t.Error("Ctrl-C of a reattached client left the detached record")
	}

	server, _ = startDetached(t, e, args...)
	web = startAttached(t, e, args...)
	reattach(t, server, web)
	stopDetached(t, e, server.pid, syscall.SIGKILL)
	web.await(regexp.MustCompile(`rotari web \(PID ` + strconv.Itoa(server.pid) + `\) ended without removing its record`))
	if code := web.exitCode(); code == 0 {
		t.Error("the client exited 0 after the server was killed")
	}
}

// TestWebClientReattachesOnlyWithTheSameOptions checks that rotari web with options
// other than the detached server's starts a server of its own at a terminal, as WEB-47
// says, and fails at start with --disconnect-action detach, naming the options; with
// the same options and --disconnect-action detach it attaches, even without a
// terminal.
func TestWebClientReattachesOnlyWithTheSameOptions(t *testing.T) {
	covers(t, "WEB-61")
	e := terminalEnv(t)
	server, _ := startDetached(t, e)

	other := startAttached(t, e, "--allow-control=false")
	if other.url == server.url || reattachedListening.MatchString(other.text()) {
		t.Fatalf("rotari web with other options attached to the detached server:\n%s", other.text())
	}
	other.await(regexp.MustCompile(`Note: a detached rotari web is also running on this host at ` + regexp.QuoteMeta(server.url)))
	other.press("\x03")
	other.exitCode()

	refused := e.Rotari("web", "--port", "0", "--disconnect-action", "detach", "--allow-control=false")
	if refused.Code == 0 || !strings.Contains(refused.Stderr, "with different --allow-control") || !strings.Contains(refused.Stderr, "PID "+strconv.Itoa(server.pid)) {
		t.Errorf("--disconnect-action detach with other options: %s", refused)
	}

	command := e.Command("web", "--port", "0", "--disconnect-action", "detach")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	buffer := make([]byte, 4096)
	var shown strings.Builder
	for !reattachedListening.MatchString(shown.String()) {
		n, err := stdout.Read(buffer)
		shown.Write(buffer[:n])
		if err != nil {
			t.Fatalf("no reattachment without a terminal:\n%s", shown.String())
		}
	}
	if children := childrenOf(t, command.Process.Pid); len(children) != 0 {
		t.Errorf("rotari web --disconnect-action detach started processes %v beside a detached server with its options", children)
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	if !serving(server.url) {
		t.Fatal("SIGTERM to a reattached client without a terminal stopped the server")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
