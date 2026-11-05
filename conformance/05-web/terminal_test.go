package web

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/rotari-dev/rotari/conformance/support"
)

const (
	terminalWait        = 15 * time.Second
	terminalTakenOver   = websocket.StatusCode(4001)
	terminalShellExited = websocket.StatusCode(4002)
	terminalStopping    = websocket.StatusCode(4004)
)

// terminalEnv is a web server's environment with a known token and a plain shell,
// whose home keeps its profile out of the test.
func terminalEnv(t *testing.T) *support.Env {
	t.Helper()
	e := support.NewEnv(t)
	e.Setenv("ROTARI_WEB_AUTH_TOKEN", launchToken)
	e.Setenv("SHELL", "/bin/sh")
	e.Setenv("HOME", e.Root)
	return e
}

type terminalPage struct {
	t          *testing.T
	connection *websocket.Conn
	shown      strings.Builder
}

func dialTerminalPage(t *testing.T, base, query string, headers http.Header) (*terminalPage, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
	defer cancel()
	address := "ws" + strings.TrimPrefix(base, "http") + "/_terminal/ws?rows=24&cols=80" + query
	connection, _, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		return nil, err
	}
	connection.SetReadLimit(1 << 24)
	t.Cleanup(func() { _ = connection.CloseNow() })
	return &terminalPage{t: t, connection: connection}, nil
}

func openTerminalPage(t *testing.T, base string) *terminalPage {
	t.Helper()
	page, err := dialTerminalPage(t, base, "", http.Header{"X-Rotari-Token": {launchToken}})
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func (page *terminalPage) send(kind websocket.MessageType, message string) {
	page.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
	defer cancel()
	if err := page.connection.Write(ctx, kind, []byte(message)); err != nil {
		page.t.Fatal(err)
	}
}

func (page *terminalPage) typeLine(line string) {
	page.t.Helper()
	page.send(websocket.MessageBinary, line+"\n")
}

// await reads until output has want on a line's end. The typed line's echo cannot
// satisfy it, since commands compute what they print.
func (page *terminalPage) await(want string) {
	page.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
	defer cancel()
	for !strings.Contains(page.shown.String(), want+"\r\n") {
		_, message, err := page.connection.Read(ctx)
		if err != nil {
			page.t.Fatalf("waiting for %q: %v\n%s", want, err, page.shown.String())
		}
		page.shown.Write(message)
	}
}

// awaitClose reads until the connection closes with the code and returns the reason.
func (page *terminalPage) awaitClose(want websocket.StatusCode) string {
	page.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
	defer cancel()
	for {
		_, _, err := page.connection.Read(ctx)
		if err == nil {
			continue
		}
		var closed websocket.CloseError
		if !errors.As(err, &closed) || closed.Code != want {
			page.t.Fatalf("closed with %v, want code %d", err, want)
		}
		return closed.Reason
	}
}

func readTerminalPID(t *testing.T, path string) int {
	t.Helper()
	for deadline := time.Now().Add(terminalWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if data, err := os.ReadFile(path); err == nil && strings.HasSuffix(string(data), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatalf("%s holds %q", path, data)
			}
			return pid
		}
	}
	t.Fatalf("nothing wrote %s", path)
	return 0
}

func awaitProcessGone(t *testing.T, pid int, what string) {
	t.Helper()
	for deadline := time.Now().Add(terminalWait); syscall.Kill(pid, 0) == nil; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("process %d outlived %s", pid, what)
		}
	}
}

func TestTerminalNeedsItsFlag(t *testing.T) {
	covers(t, "WEB-34")
	e := support.NewEnv(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--allow-terminal", "--allow-control=false"}, "--allow-terminal requires job control"},
		{[]string{"--allow-terminal", "--static-dir", filepath.Join(e.Root, "export")}, "--allow-terminal cannot be combined with --static-dir"},
	} {
		if r := e.Rotari(append([]string{"web"}, test.args...)...); r.Code == 0 || !strings.Contains(r.Stderr, test.want) {
			t.Errorf("web %v: %s, want an error with %q", test.args, r, test.want)
		}
	}
	base := e.StartWeb()
	for _, path := range []string{"/_terminal/ws?rows=24&cols=80", "/_terminal/xterm.js"} {
		if got := e.HTTPGet(base + path); got.Status != http.StatusNotFound {
			t.Errorf("GET %s without the flag: status %d, want 404", path, got.Status)
		}
	}
	for _, path := range []string{"/", "/jobs/"} {
		if got := e.HTTPGet(base + path); strings.Contains(got.Body, `id="terminal-button"`) {
			t.Errorf("GET %s without the flag offers the terminal", path)
		}
	}
}

// Without a configured token one is generated, as for --allow-exec, and printed.
func TestTerminalAlwaysHasAToken(t *testing.T) {
	covers(t, "WEB-34")
	e := support.NewEnv(t)
	command := e.Command("web", "--allow-terminal", "--port", "0")
	stderr, err := command.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	scanner := bufio.NewScanner(stderr)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) == 3 {
			break
		}
	}
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "--allow-terminal requires authentication") || !regexp.MustCompile(`(?m)^\s+\S{20,}$`).MatchString(text) {
		t.Errorf("stderr does not give a generated token:\n%s", text)
	}
}

func TestTerminalIsOfferedAndGuarded(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	token := map[string]string{"X-Rotari-Token": launchToken}
	for _, path := range []string{"/", "/jobs/"} {
		if got := e.HTTPDo(http.MethodGet, base+path, token, ""); got.Status != http.StatusOK || !strings.Contains(got.Body, `id="terminal-button"`) || strings.Contains(got.Body, "xterm.js authors") {
			t.Errorf("GET %s: status %d, want a Terminal button and no xterm.js in the page", path, got.Status)
		}
	}
	if got := e.HTTPDo(http.MethodGet, base+"/_terminal/xterm.js", token, ""); got.Status != http.StatusOK || !strings.Contains(got.Body, "Terminal") {
		t.Errorf("GET xterm.js: status %d", got.Status)
	}
	address := base + "/_terminal/ws?rows=24&cols=80"
	upgrade := map[string]string{"Connection": "Upgrade", "Upgrade": "websocket", "Sec-WebSocket-Version": "13", "Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ=="}
	with := func(extra map[string]string) map[string]string {
		headers := map[string]string{}
		for _, set := range []map[string]string{upgrade, extra} {
			for name, value := range set {
				headers[name] = value
			}
		}
		return headers
	}
	for name, test := range map[string]struct {
		address string
		headers map[string]string
		want    int
	}{
		"no credentials":   {address, with(nil), http.StatusForbidden},
		"wrong token":      {address, with(map[string]string{"X-Rotari-Token": "wrong"}), http.StatusUnauthorized},
		"another site":     {address, with(map[string]string{"X-Rotari-Token": launchToken, "Origin": "http://attacker.example"}), http.StatusForbidden},
		"rebinding host":   {address, with(map[string]string{"X-Rotari-Token": launchToken, "Host": "attacker.example"}), http.StatusMisdirectedRequest},
		"plain HTTP proxy": {address, with(map[string]string{"X-Rotari-Token": launchToken, "X-Forwarded-For": "192.0.2.1"}), http.StatusForbidden},
		"no size":          {base + "/_terminal/ws", with(map[string]string{"X-Rotari-Token": launchToken}), http.StatusBadRequest},
	} {
		if got := e.HTTPDo(http.MethodGet, test.address, test.headers, ""); got.Status != test.want {
			t.Errorf("%s: status %d, want %d: %s", name, got.Status, test.want, got.Body)
		}
	}
}

func TestTerminalShellOutlivesItsConnections(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")

	first := openTerminalPage(t, base)
	first.typeLine(`VALUE=kept; echo "one-$((1+1))"; stty size; echo "token-${ROTARI_WEB_AUTH_TOKEN:-absent}-end"`)
	first.await("one-2")
	first.await("24 80")
	first.await("token-absent-end")
	first.send(websocket.MessageText, `{"type":"resize","rows":40,"cols":100}`)
	first.typeLine(`stty size`)
	first.await("40 100")

	// Reconnecting finds the same shell, with recent output replayed, and a
	// connection reconnecting by itself does not disturb one in use.
	second := openTerminalPage(t, base)
	first.awaitClose(terminalTakenOver)
	second.await("one-2")
	polite, err := dialTerminalPage(t, base, "&takeover=0", http.Header{"X-Rotari-Token": {launchToken}})
	if err != nil {
		t.Fatal(err)
	}
	polite.awaitClose(terminalTakenOver)
	second.typeLine(`echo "value-$VALUE"`)
	second.await("value-kept")
	_ = second.connection.Close(websocket.StatusNormalClosure, "")

	// With nothing attached, reconnecting by itself may attach.
	third := func() *terminalPage {
		for deadline := time.Now().Add(terminalWait); ; time.Sleep(20 * time.Millisecond) {
			page, err := dialTerminalPage(t, base, "&takeover=0", http.Header{"X-Rotari-Token": {launchToken}})
			if err != nil {
				t.Fatal(err)
			}
			page.typeLine(`echo "again-$VALUE"`)
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			_, message, err := page.connection.Read(ctx)
			cancel()
			if err == nil {
				page.shown.Write(message)
				return page
			}
			if time.Now().After(deadline) {
				t.Fatalf("could not attach: %v", err)
			}
		}
	}()
	third.await("again-kept")

	// The shell's end says how, and a new connection starts another.
	third.typeLine("exit 3")
	if reason := third.awaitClose(terminalShellExited); reason != "exit status 3" {
		t.Errorf("reason = %q", reason)
	}
	fresh := openTerminalPage(t, base)
	fresh.typeLine(`echo "value-[$VALUE]"`)
	fresh.await("value-[]")
}

// A run started in the terminal outlives it, as after closing any terminal.
func TestTerminalRunOutlivesItsShell(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	done := filepath.Join(e.Root, "done")
	e.MustRotari("add", "-p", "panel", "--", "sh", "-c", `sleep 2; touch "$0"`, done)
	base := e.StartWeb("--allow-terminal")
	page := openTerminalPage(t, base)
	page.typeLine(fmt.Sprintf(`%q run -p panel`, e.Command().Path))
	// The job's run has started once the project reports it.
	support.WaitUntil(t, terminalWait, func() (bool, string) {
		state := e.CheckState("panel")
		return state == "running", "state " + state
	})
	page.send(websocket.MessageText, `{"type":"end"}`)
	page.awaitClose(terminalShellExited)
	for deadline := time.Now().Add(terminalWait); ; time.Sleep(50 * time.Millisecond) {
		if _, err := os.Stat(done); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the run did not finish after its shell ended")
		}
	}
}

func TestTerminalSignOutEndsTheShell(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	browser := &http.Client{Timeout: terminalWait, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	post := func(path, cookie string, form url.Values) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", base)
		if cookie != "" {
			request.Header.Set("Cookie", cookie)
		}
		response, err := browser.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response
	}
	cookies := post("/_auth/login", "", url.Values{"username": {"rotari"}, "password": {launchToken}, "next": {"/"}}).Cookies()
	if len(cookies) != 1 {
		t.Fatalf("sign-in cookies = %v", cookies)
	}
	session := cookies[0].Name + "=" + cookies[0].Value
	page, err := dialTerminalPage(t, base, "", http.Header{"Cookie": {session}, "Origin": {base}})
	if err != nil {
		t.Fatalf("a signed-in browser could not connect: %v", err)
	}
	page.typeLine(`echo "in-$((1+1))"`)
	page.await("in-2")
	post("/_auth/logout", session, url.Values{})
	page.awaitClose(terminalShellExited)
}

// The shell ends with the server, however the server ends, while a process that left
// the shell's session goes on.
func TestTerminalShellEndsWithTheServer(t *testing.T) {
	covers(t, "WEB-34")
	for _, signal := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL} {
		t.Run(signal.String(), func(t *testing.T) {
			e := terminalEnv(t)
			command := e.Command("web", "--allow-terminal", "--port", "0")
			stdout, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = command.Process.Kill() }()
			scanner := bufio.NewScanner(stdout)
			scanner.Scan()
			base := regexp.MustCompile(`http://\S+`).FindString(scanner.Text())
			if base == "" {
				t.Fatalf("no address in %q", scanner.Text())
			}
			page := openTerminalPage(t, base)
			shellFile, detachedFile := filepath.Join(e.Root, "shell"), filepath.Join(e.Root, "detached")
			page.typeLine(`echo $$ > ` + shellFile + `; setsid sh -c 'echo $$ > ` + detachedFile + `; exec sleep 300' >/dev/null 2>&1 </dev/null &`)
			shell, detached := readTerminalPID(t, shellFile), readTerminalPID(t, detachedFile)
			t.Cleanup(func() { _ = syscall.Kill(detached, syscall.SIGKILL) })
			if err := command.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			if signal != syscall.SIGKILL {
				page.awaitClose(terminalStopping)
			}
			_ = command.Wait()
			awaitProcessGone(t, shell, "the server")
			if err := syscall.Kill(detached, 0); err != nil {
				t.Errorf("the detached process did not survive: %v", err)
			}
		})
	}
}
