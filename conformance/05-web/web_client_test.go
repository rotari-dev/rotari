package web

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/rotari-dev/rotari/conformance/support"
)

const webClientWait = 15 * time.Second

var (
	attachedListening = regexp.MustCompile(`rotari web listening at (http://\S+)`)
	attachedDetached  = regexp.MustCompile(`rotari web keeps running in the background at (http://\S+) \(PID (\d+)\)`)
)

// attachedWeb is rotari web run at a terminal, as its client.
type attachedWeb struct {
	t       *testing.T
	command *exec.Cmd
	pty     *os.File
	mu      sync.Mutex
	shown   bytes.Buffer
	exited  chan error
	url     string
}

// startAttached runs rotari web with args on a terminal and waits for its address. When
// the test ends the client is killed and any detached server it left is stopped.
func startAttached(t *testing.T, e *support.Env, args ...string) *attachedWeb {
	t.Helper()
	command := e.Command(append([]string{"web", "--port", "0"}, args...)...)
	terminal, err := pty.Start(command)
	if err != nil {
		t.Fatal(err)
	}
	web := &attachedWeb{t: t, command: command, pty: terminal, exited: make(chan error, 1)}
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := terminal.Read(buffer)
			web.mu.Lock()
			web.shown.Write(buffer[:n])
			web.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	go func() { web.exited <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = terminal.Close()
		_ = e.Command("web", "--shutdown", "--yes").Run()
	})
	web.url = web.await(attachedListening)[1]
	return web
}

func (web *attachedWeb) text() string {
	web.mu.Lock()
	defer web.mu.Unlock()
	return strings.ReplaceAll(web.shown.String(), "\r", "")
}

// await waits until the output matches pattern and returns the match.
func (web *attachedWeb) await(pattern *regexp.Regexp) []string {
	web.t.Helper()
	for deadline := time.Now().Add(webClientWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if match := pattern.FindStringSubmatch(web.text()); match != nil {
			return match
		}
	}
	web.t.Fatalf("no %q in:\n%s", pattern, web.text())
	return nil
}

func (web *attachedWeb) press(keys string) {
	web.t.Helper()
	if _, err := web.pty.WriteString(keys); err != nil {
		web.t.Fatal(err)
	}
}

// exitCode waits for the client to exit.
func (web *attachedWeb) exitCode() int {
	web.t.Helper()
	select {
	case err := <-web.exited:
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		if err != nil {
			web.t.Fatal(err)
		}
		return 0
	case <-time.After(webClientWait):
		web.t.Fatalf("the client did not exit:\n%s", web.text())
		return -1
	}
}

// serving says whether url answers.
func serving(url string) bool {
	request, _ := http.NewRequest(http.MethodGet, url+"/", nil)
	request.Header.Set("X-Rotari-Token", launchToken)
	response, err := (&http.Client{Timeout: 2 * time.Second}).Do(request)
	if err != nil {
		return false
	}
	_ = response.Body.Close()
	return true
}

func awaitServing(t *testing.T, url string, want bool, what string) {
	t.Helper()
	for deadline := time.Now().Add(webClientWait); serving(url) != want; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s: serving is %v, want %v", what, !want, want)
		}
	}
}

// TestWebClientDetachesOnCtrlD checks that rotari web at a terminal says how to detach
// and how long shells last, and that Ctrl-D leaves the server running detached, found
// and stopped as any detached server is.
func TestWebClientDetachesOnCtrlD(t *testing.T) {
	covers(t, "WEB-47")
	e := terminalEnv(t)
	web := startAttached(t, e, "--allow-terminal", "--terminal-idle-timeout", "90m")
	web.await(regexp.MustCompile(`Press Ctrl-D to detach and keep the server running; Ctrl-C to stop it\.`))
	web.await(regexp.MustCompile(`Shells end 1h30m after the last page closes \(--terminal-idle-timeout\), and when the server stops\.`))
	page := openTerminalPage(t, web.url)
	page.typeLine(`echo before-$((1+1))`)
	page.await("before-2")
	web.press("\x04")
	match := web.await(attachedDetached)
	if match[1] != web.url {
		t.Errorf("detached at %s, attached at %s", match[1], web.url)
	}
	if code := web.exitCode(); code != 0 {
		t.Errorf("the client exited with %d after Ctrl-D", code)
	}
	if !strings.Contains(web.text(), "rotari web --shutdown") {
		t.Errorf("the client did not say how to stop it:\n%s", web.text())
	}
	page.typeLine(`echo after-$((2+2))`)
	page.await("after-4")
	again := e.MustRotari("web", "--detach", "--allow-terminal", "--terminal-idle-timeout", "90m", "--port", "0")
	if !strings.Contains(again.Stdout, "PID "+match[2]) || !strings.Contains(again.Stderr, "Shells end 1h30m") {
		t.Errorf("--detach did not report the server Ctrl-D detached, or its idle limit: %s", again)
	}
	e.MustRotari("web", "--shutdown", "--yes")
	awaitServing(t, web.url, false, "after --shutdown")
}

// TestWebClientStopsOnCtrlC checks that Ctrl-C stops the server and the client exits 0.
func TestWebClientStopsOnCtrlC(t *testing.T) {
	covers(t, "WEB-47")
	e := terminalEnv(t)
	web := startAttached(t, e)
	if strings.Contains(web.text(), "Shells end") {
		t.Errorf("the idle-limit line showed without the terminal:\n%s", web.text())
	}
	web.press("\x03")
	if code := web.exitCode(); code != 0 {
		t.Errorf("the client exited with %d after Ctrl-C", code)
	}
	awaitServing(t, web.url, false, "after Ctrl-C")
	record, _ := detachedFiles(t, e)
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Errorf("Ctrl-C left a detached record: %v", err)
	}
}

// TestWebClientCannotDetachBesideAnother checks that Ctrl-D, while another server is
// detached on the host, says so and leaves this one attached.
func TestWebClientCannotDetachBesideAnother(t *testing.T) {
	covers(t, "WEB-47")
	e := terminalEnv(t)
	other, _ := startDetached(t, e)
	web := startAttached(t, e)
	web.await(regexp.MustCompile(`Press Ctrl-D`))
	web.press("\x04")
	web.await(regexp.MustCompile(`cannot detach: a detached rotari web is already running on this host at ` + regexp.QuoteMeta(other.url) + ` \(PID ` + strconv.Itoa(other.pid) + `\)`))
	if !serving(web.url) {
		t.Fatal("a refused Ctrl-D stopped the server")
	}
	web.press("\x03")
	if code := web.exitCode(); code != 0 {
		t.Errorf("the client exited with %d after Ctrl-C", code)
	}
	awaitServing(t, web.url, false, "after Ctrl-C")
	if !serving(other.url) {
		t.Error("stopping the attached server stopped the detached one")
	}
}

// TestWebClientDisconnectActions checks what SIGHUP and SIGKILL of the client do with
// each --disconnect-action: stop ends the server, detach leaves it running detached.
func TestWebClientDisconnectActions(t *testing.T) {
	covers(t, "WEB-47")
	for _, signal := range []syscall.Signal{syscall.SIGHUP, syscall.SIGKILL} {
		for _, action := range []string{"stop", "detach"} {
			t.Run(signal.String()+"/"+action, func(t *testing.T) {
				e := terminalEnv(t)
				args := []string{}
				if action == "detach" {
					args = append(args, "--disconnect-action", "detach")
				}
				web := startAttached(t, e, args...)
				web.await(regexp.MustCompile(`Press Ctrl-D`))
				if err := web.command.Process.Signal(signal); err != nil {
					t.Fatal(err)
				}
				web.exitCode()
				record, _ := detachedFiles(t, e)
				if action == "stop" {
					awaitServing(t, web.url, false, "after "+signal.String())
					return
				}
				for deadline := time.Now().Add(webClientWait); ; time.Sleep(50 * time.Millisecond) {
					if _, err := os.Stat(record); err == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("no detached record after %s", signal)
					}
				}
				if !serving(web.url) {
					t.Fatalf("the server did not keep running after %s", signal)
				}
				e.MustRotari("web", "--shutdown", "--yes")
				awaitServing(t, web.url, false, "after --shutdown")
			})
		}
	}
}

// TestWebWithoutATerminalServesInItsOwnProcess checks that rotari web started without a
// terminal, as from a script, starts no background server and offers no Ctrl-D.
func TestWebWithoutATerminalServesInItsOwnProcess(t *testing.T) {
	covers(t, "WEB-47")
	e := terminalEnv(t)
	command := e.Command("web", "--port", "0", "--allow-terminal")
	var stderr bytes.Buffer
	command.Stderr = &stderr
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
	for deadline := time.Now().Add(webClientWait); !attachedListening.MatchString(shown.String()) && time.Now().Before(deadline); {
		n, err := stdout.Read(buffer)
		shown.Write(buffer[:n])
		if err != nil {
			break
		}
	}
	match := attachedListening.FindStringSubmatch(shown.String())
	if match == nil {
		t.Fatalf("no address:\n%s", shown.String())
	}
	if children := childrenOf(t, command.Process.Pid); len(children) != 0 {
		t.Errorf("rotari web without a terminal started processes %v", children)
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	// Read once the process is gone, since exec copies stderr meanwhile.
	if strings.Contains(stderr.String(), "Ctrl-D") {
		t.Errorf("rotari web without a terminal offered Ctrl-D:\n%s", stderr.String())
	}
	awaitServing(t, match[1], false, "after killing it")
}

// childrenOf lists the PIDs whose parent is pid.
func childrenOf(t *testing.T, pid int) []int {
	t.Helper()
	var children []int
	for _, process := range support.MustProcesses(t) {
		if process.PPID == pid {
			children = append(children, process.PID)
		}
	}
	return children
}

// TestWebDisconnectActionIsItsOwn checks that rotari web reads its disconnect action
// from [web] and ROTARI_WEB_DISCONNECT_ACTION only: run's variable and a top-level
// disconnect-action, whose values are a run's, do not reach it, and a bad value of its
// own is an error naming where it came from.
func TestWebDisconnectActionIsItsOwn(t *testing.T) {
	covers(t, "WEB-47")
	e := terminalEnv(t)
	e.Setenv("ROTARI_DISCONNECT_ACTION", "cancel")
	config := filepath.Join(e.Root, "config", "rotari", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("disconnect-action = \"cancel\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if url := e.StartWeb(); !serving(url) {
		t.Fatal("run's disconnect action stopped rotari web from serving")
	}
	bad := e.WithVar("ROTARI_WEB_DISCONNECT_ACTION", "cancel").Rotari("web", "--port", "0")
	if bad.Code == 0 || !strings.Contains(bad.Stderr, "ROTARI_WEB_DISCONNECT_ACTION") {
		t.Errorf("a bad ROTARI_WEB_DISCONNECT_ACTION was not an error naming it: %s", bad)
	}
	if err := os.WriteFile(config, []byte("[web]\ndisconnect-action = \"later\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if bad := e.Rotari("web", "--port", "0"); bad.Code == 0 || !strings.Contains(bad.Stderr, config) {
		t.Errorf("a bad [web] disconnect-action was not an error naming its file: %s", bad)
	}
}
