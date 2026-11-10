package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
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
	terminalNoShell     = websocket.StatusCode(4005)
	terminalShellLimit  = 8
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

type terminalShell struct {
	ID      string `json:"id"`
	Number  int    `json:"number"`
	Project string `json:"project"`
	Link    string `json:"link"`
}

// terminalShellsRequest makes a request as a page does, returning the status and body.
func terminalShellsRequest(t *testing.T, method, address string, headers http.Header) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, address, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header = headers.Clone()
	response, err := (&http.Client{Timeout: terminalWait}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, body
}

func listTerminalShells(t *testing.T, base string, headers http.Header) []terminalShell {
	t.Helper()
	status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells", headers)
	if status != http.StatusOK {
		t.Fatalf("listing shells: status %d: %s", status, body)
	}
	var listed struct {
		Shells []terminalShell `json:"shells"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatal(err)
	}
	return listed.Shells
}

func startTerminalShell(t *testing.T, base string, headers http.Header) terminalShell {
	t.Helper()
	status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80", headers)
	if status != http.StatusCreated {
		t.Fatalf("starting a shell: status %d: %s", status, body)
	}
	var shell terminalShell
	if err := json.Unmarshal(body, &shell); err != nil {
		t.Fatal(err)
	}
	return shell
}

// aTerminalShell is the first running shell, started if none is.
func aTerminalShell(t *testing.T, base string, headers http.Header) string {
	t.Helper()
	if shells := listTerminalShells(t, base, headers); len(shells) > 0 {
		return shells[0].ID
	}
	return startTerminalShell(t, base, headers).ID
}

func dialTerminalPage(t *testing.T, base, query string, headers http.Header) (*terminalPage, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
	defer cancel()
	address := "ws" + strings.TrimPrefix(base, "http") + "/_terminal/ws?shell=" + aTerminalShell(t, base, headers) + "&rows=24&cols=80" + query
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
		{[]string{"--terminal-max-shells", "2"}, "--terminal-max-shells only applies with --allow-terminal"},
		{[]string{"--terminal-idle-timeout", "1h"}, "--terminal-idle-timeout only applies with --allow-terminal"},
		{[]string{"--terminal-shell", "/bin/sh"}, "--terminal-shell only applies with --allow-terminal"},
		{[]string{"--allow-terminal", "--terminal-idle-timeout", "1m"}, "must be at least 2m"},
		{[]string{"--allow-terminal", "--terminal-idle-timeout", "0"}, "must be at least 2m"},
		{[]string{"--allow-terminal", "--terminal-max-shells", "0"}, "must be at least 1"},
		{[]string{"--allow-terminal", "--terminal-max-shells-per-project", "0"}, "must be at least 1"},
		{[]string{"--allow-terminal", "--terminal-replay-size", "1GiB"}, "--terminal-replay-size"},
		{[]string{"--allow-terminal", "--terminal-replay-size", "99MiB"}, "from 1KiB to 64MiB"},
		{[]string{"--allow-terminal", "--terminal-shell", filepath.Join(e.Root, "missing")}, "--terminal-shell"},
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
	address := base + "/_terminal/ws?shell=1&rows=24&cols=80"
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
		"no shell":         {base + "/_terminal/ws?rows=24&cols=80", with(map[string]string{"X-Rotari-Token": launchToken}), http.StatusBadRequest},
		"no size":          {base + "/_terminal/ws?shell=1", with(map[string]string{"X-Rotari-Token": launchToken}), http.StatusBadRequest},
	} {
		if got := e.HTTPDo(http.MethodGet, test.address, test.headers, ""); got.Status != test.want {
			t.Errorf("%s: status %d, want %d: %s", name, got.Status, test.want, got.Body)
		}
	}
	// Starting and ending shells are guarded the same way, and none was started.
	for name, test := range map[string]struct {
		method, address string
		headers         map[string]string
		want            int
	}{
		"list, no credentials":    {http.MethodGet, base + "/_terminal/shells", map[string]string{}, http.StatusUnauthorized},
		"start, no credentials":   {http.MethodPost, base + "/_terminal/shells?rows=24&cols=80", map[string]string{}, http.StatusForbidden},
		"start, another site":     {http.MethodPost, base + "/_terminal/shells?rows=24&cols=80", map[string]string{"X-Rotari-Token": launchToken, "Origin": "http://attacker.example"}, http.StatusForbidden},
		"start, plain proxy":      {http.MethodPost, base + "/_terminal/shells?rows=24&cols=80", map[string]string{"X-Rotari-Token": launchToken, "X-Forwarded-For": "192.0.2.1"}, http.StatusForbidden},
		"end, no credentials":     {http.MethodDelete, base + "/_terminal/shells/1", map[string]string{}, http.StatusForbidden},
		"end, another site":       {http.MethodDelete, base + "/_terminal/shells/1", map[string]string{"X-Rotari-Token": launchToken, "Origin": "http://attacker.example"}, http.StatusForbidden},
		"start, no size":          {http.MethodPost, base + "/_terminal/shells", map[string]string{"X-Rotari-Token": launchToken}, http.StatusBadRequest},
		"alive, no credentials":   {http.MethodPost, base + "/_terminal/alive", map[string]string{}, http.StatusForbidden},
		"alive, another site":     {http.MethodPost, base + "/_terminal/alive", map[string]string{"X-Rotari-Token": launchToken, "Origin": "http://attacker.example"}, http.StatusForbidden},
		"alive":                   {http.MethodPost, base + "/_terminal/alive", map[string]string{"X-Rotari-Token": launchToken}, http.StatusNoContent},
		"order, another site":     {http.MethodPut, base + "/_terminal/order", map[string]string{"X-Rotari-Token": launchToken, "Origin": "http://attacker.example"}, http.StatusForbidden},
		"order, no credentials":   {http.MethodPut, base + "/_terminal/order", map[string]string{}, http.StatusForbidden},
		"name, another site":      {http.MethodPatch, base + "/_terminal/shells/1", map[string]string{"X-Rotari-Token": launchToken, "Origin": "http://attacker.example"}, http.StatusForbidden},
		"name, not running":       {http.MethodPatch, base + "/_terminal/shells/7", map[string]string{"X-Rotari-Token": launchToken}, http.StatusBadRequest},
		"window, no credentials":  {http.MethodGet, base + "/_terminal/window/project/p", map[string]string{}, http.StatusUnauthorized},
		"window, plain proxy":     {http.MethodGet, base + "/_terminal/window/project/p", map[string]string{"X-Rotari-Token": launchToken, "X-Forwarded-For": "192.0.2.1"}, http.StatusForbidden},
		"window, posted":          {http.MethodPost, base + "/_terminal/window/project/p", map[string]string{"X-Rotari-Token": launchToken}, http.StatusMethodNotAllowed},
		"window":                  {http.MethodGet, base + "/_terminal/window/project/p", map[string]string{"X-Rotari-Token": launchToken}, http.StatusOK},
		"end, not running":        {http.MethodDelete, base + "/_terminal/shells/7", map[string]string{"X-Rotari-Token": launchToken}, http.StatusNotFound},
		"preview, no credentials": {http.MethodGet, base + "/_terminal/shells/1/preview", map[string]string{}, http.StatusUnauthorized},
		"preview, plain proxy":    {http.MethodGet, base + "/_terminal/shells/1/preview", map[string]string{"X-Rotari-Token": launchToken, "X-Forwarded-For": "192.0.2.1"}, http.StatusForbidden},
		"preview, posted":         {http.MethodPost, base + "/_terminal/shells/1/preview", map[string]string{"X-Rotari-Token": launchToken}, http.StatusMethodNotAllowed},
		"preview, not running":    {http.MethodGet, base + "/_terminal/shells/7/preview", map[string]string{"X-Rotari-Token": launchToken}, http.StatusNotFound},
	} {
		if got := e.HTTPDo(test.method, test.address, test.headers, ""); got.Status != test.want {
			t.Errorf("%s: status %d, want %d: %s", name, got.Status, test.want, got.Body)
		}
	}
	if shells := listTerminalShells(t, base, http.Header{"X-Rotari-Token": {launchToken}}); len(shells) != 0 {
		t.Errorf("guarded requests left %v", shells)
	}
}

// Shells run side by side, each with its own state and connection, up to the limit,
// and a connection reaches only the shell it names.
func TestTerminalKeepsSeveralShells(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	token := http.Header{"X-Rotari-Token": {launchToken}}
	one, two := startTerminalShell(t, base, token), startTerminalShell(t, base, token)
	if one.Number != 1 || two.Number != 2 || one.ID == two.ID {
		t.Errorf("started %+v and %+v", one, two)
	}
	dial := func(id, query string) *terminalPage {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
		defer cancel()
		address := "ws" + strings.TrimPrefix(base, "http") + "/_terminal/ws?shell=" + id + "&rows=24&cols=80" + query
		connection, _, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: token})
		if err != nil {
			t.Fatal(err)
		}
		connection.SetReadLimit(1 << 24)
		t.Cleanup(func() { _ = connection.CloseNow() })
		return &terminalPage{t: t, connection: connection}
	}
	first, second := dial(one.ID, ""), dial(two.ID, "")
	first.typeLine(`VALUE=first; echo "one-$((1+1))"`)
	first.await("one-2")
	second.typeLine(`VALUE=second; echo "two-$((1+1))"`)
	second.await("two-2")
	first.typeLine(`echo "value-$VALUE"`)
	first.await("value-first")
	second.typeLine(`echo "value-$VALUE"`)
	second.await("value-second")
	if strings.Contains(second.shown.String(), "one-2") {
		t.Errorf("one shell's output reached the other:\n%s", second.shown.String())
	}

	// Take-over is per shell.
	third := dial(two.ID, "")
	second.awaitClose(terminalTakenOver)
	third.await("two-2")
	first.typeLine(`echo "still-$((1+1))"`)
	first.await("still-2")

	// Ending one closes its connection as an exit does, and leaves the other.
	status, body := terminalShellsRequest(t, http.MethodDelete, base+"/_terminal/shells/"+one.ID, token)
	if status != http.StatusNoContent {
		t.Fatalf("ending a shell: status %d: %s", status, body)
	}
	first.awaitClose(terminalShellExited)
	third.typeLine(`echo "alive-$((1+1))"`)
	third.await("alive-2")
	dial(one.ID, "").awaitClose(terminalNoShell)

	// Output with nothing connected is marked until something connects.
	hasUnseen := func() bool {
		t.Helper()
		_, listed := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells", token)
		return strings.Contains(string(listed), `"unseen_output":true`)
	}
	if hasUnseen() {
		t.Error("output on screen was marked as unseen")
	}
	third.typeLine(`(sleep 1; echo "later-$((1+1))") &`)
	_ = third.connection.Close(websocket.StatusNormalClosure, "")
	support.WaitUntil(t, terminalWait, func() (bool, string) { return hasUnseen(), "no shell has unseen output" })
	fourth := dial(two.ID, "")
	fourth.await("later-2")
	if hasUnseen() {
		t.Error("connecting left the output marked as unseen")
	}

	// An arrangement stays in the list, and a new shell goes after it.
	three := startTerminalShell(t, base, token)
	order := func() string {
		t.Helper()
		var ids []string
		for _, shell := range listTerminalShells(t, base, token) {
			ids = append(ids, shell.ID)
		}
		return strings.Join(ids, ",")
	}
	if got := e.HTTPDo(http.MethodPut, base+"/_terminal/order", map[string]string{"X-Rotari-Token": launchToken}, `{"ids":["`+three.ID+`","`+two.ID+`","gone"]}`); got.Status != http.StatusNoContent {
		t.Errorf("arranging: status %d: %s", got.Status, got.Body)
	}
	if got, want := order(), three.ID+","+two.ID; got != want {
		t.Errorf("arranged, the list is %s, want %s", got, want)
	}
	if status, body := terminalShellsRequest(t, http.MethodDelete, base+"/_terminal/shells/"+three.ID, token); status != http.StatusNoContent {
		t.Fatalf("ending a shell: status %d: %s", status, body)
	}

	// A name stays with its shell in the list, on the one line a tab has room for.
	got := e.HTTPDo(http.MethodPatch, base+"/_terminal/shells/"+two.ID, map[string]string{"X-Rotari-Token": launchToken}, `{"name":"build"}`)
	if got.Status != http.StatusOK || !strings.Contains(got.Body, `"name":"build"`) {
		t.Errorf("renaming: status %d: %s", got.Status, got.Body)
	}
	if got := e.HTTPDo(http.MethodPatch, base+"/_terminal/shells/"+two.ID, map[string]string{"X-Rotari-Token": launchToken}, `{"name":"a\nb"}`); got.Status != http.StatusBadRequest {
		t.Errorf("a name with a line break: status %d: %s", got.Status, got.Body)
	}
	if status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells", token); status != http.StatusOK || !strings.Contains(string(body), `"name":"build"`) {
		t.Errorf("the list lost the name: status %d: %s", status, body)
	}

	// A preview says where a running shell is.
	if got := e.HTTPDo(http.MethodGet, base+"/_terminal/shells/"+two.ID+"/preview", map[string]string{"X-Rotari-Token": launchToken}, ""); got.Status != http.StatusOK || !strings.Contains(got.Body, `"cwd":"/`) {
		t.Errorf("a preview: status %d: %s", got.Status, got.Body)
	}

	// A freed number comes back, under an ID that does not, up to the limit.
	replacement := startTerminalShell(t, base, token)
	if replacement.Number != 1 || replacement.ID == one.ID {
		t.Errorf("the replacement is %+v, after %+v", replacement, one)
	}
	for number := 3; number <= terminalShellLimit; number++ {
		if shell := startTerminalShell(t, base, token); shell.Number != number {
			t.Errorf("started %+v, want number %d", shell, number)
		}
	}
	if status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80", token); status != http.StatusConflict {
		t.Errorf("past the limit: status %d: %s", status, body)
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

	// Clearing forgets what would be replayed, and leaves the shell as it was.
	clearing := openTerminalPage(t, base)
	clearing.typeLine(`echo "gone-$((1+1))"`)
	clearing.await("gone-2")
	clearing.send(websocket.MessageText, `{"type":"clear"}`)
	clearing.typeLine(`echo "after-$((1+1))"`)
	clearing.await("after-2")
	cleared := openTerminalPage(t, base)
	clearing.awaitClose(terminalTakenOver)
	cleared.await("after-2")
	if strings.Contains(cleared.shown.String(), "gone-2") {
		t.Errorf("cleared output came back:\n%s", cleared.shown.String())
	}
	_ = cleared.connection.Close(websocket.StatusNormalClosure, "")

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

	// The shell's end says how, and it is gone: a page starts another.
	third.typeLine("exit 3")
	if reason := third.awaitClose(terminalShellExited); reason != "exit status 3" {
		t.Errorf("reason = %q", reason)
	}
	fresh := openTerminalPage(t, base)
	fresh.typeLine(`echo "value-[$VALUE]"`)
	fresh.await("value-[]")
}

// A project's shells are numbered apart from other projects', and start where its latest
// run started, or the named run. A page's place is looked up here, never taken from it.
func TestTerminalShellsFollowTheirProject(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	home := filepath.Join(e.Root, "shell-home")
	first, second := filepath.Join(e.Root, "first"), filepath.Join(e.Root, "second")
	for _, dir := range []string{home, first, second} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	e.Setenv("HOME", home)
	runIn := func(project, dir string) {
		t.Helper()
		e.MustRotari("add", "-p", project, "--", "true")
		command := e.Command("run", "-p", project, "--quiet")
		command.Dir = dir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("run in %s: %v\n%s", dir, err, output)
		}
	}
	runIn("p1", first)
	var shown struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(e.MustRotari("show", "-p", "p1", "--json").Stdout), &shown); err != nil || shown.RunID == "" {
		t.Fatalf("finding the first run: %v, %+v", err, shown)
	}
	firstRun := shown.RunID
	runIn("p1", second)

	base := e.StartWeb("--allow-terminal")
	token := http.Header{"X-Rotari-Token": {launchToken}}
	start := func(query string) terminalShell {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80"+query, token)
		if status != http.StatusCreated {
			t.Fatalf("starting a shell with %q: status %d: %s", query, status, body)
		}
		var shell terminalShell
		if err := json.Unmarshal(body, &shell); err != nil {
			t.Fatal(err)
		}
		return shell
	}
	open := func(shell terminalShell) *terminalPage {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
		defer cancel()
		address := "ws" + strings.TrimPrefix(base, "http") + "/_terminal/ws?shell=" + shell.ID + "&rows=24&cols=80"
		connection, _, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: token})
		if err != nil {
			t.Fatal(err)
		}
		connection.SetReadLimit(1 << 24)
		t.Cleanup(func() { _ = connection.CloseNow() })
		return &terminalPage{t: t, connection: connection}
	}
	real := func(dir string) string {
		t.Helper()
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatal(err)
		}
		return resolved
	}
	inDir := func(page *terminalPage, dir string) {
		t.Helper()
		page.typeLine(`echo "in-$(pwd -P)"`)
		page.await("in-" + real(dir))
	}

	latest := start("&project=p1")
	if latest.Number != 1 || latest.Project != "p1" || latest.Link != "/project/p1" {
		t.Errorf("the project's shell: %+v", latest)
	}
	latestPage := open(latest)
	inDir(latestPage, second)
	// Its commands mean its project with no -p, wherever it has moved to.
	latestPage.typeLine(`echo "project-$ROTARI_PROJECT_NAME"; echo "base-$ROTARI_BASEDIR"`)
	latestPage.await("project-p1")
	latestPage.await("base-" + e.Base)
	latestPage.typeLine(fmt.Sprintf(`cd / && %q show --json >/dev/null && echo "shown-$((1+1))"`, e.Command().Path))
	latestPage.await("shown-2")
	ofRun := start("&project=p1&run=" + firstRun)
	if ofRun.Number != 2 {
		t.Errorf("the project's second shell: %+v", ofRun)
	}
	inDir(open(ofRun), first)
	plain := start("")
	if plain.Number != 1 || plain.Project != "" || plain.Link != "/" {
		t.Errorf("a shell of no project: %+v", plain)
	}
	plainPage := open(plain)
	inDir(plainPage, home)
	plainPage.typeLine(`echo "project-[$ROTARI_PROJECT_NAME]"`)
	plainPage.await("project-[]")

	// What a page would be told before starting one.
	got := e.HTTPDo(http.MethodGet, base+"/_terminal/here?project=p1&run="+firstRun, map[string]string{"X-Rotari-Token": launchToken}, "")
	if got.Status != http.StatusOK || !strings.Contains(got.Body, `"dir":"`+first+`"`) || !strings.Contains(got.Body, `"project":"p1"`) {
		t.Errorf("here: status %d: %s", got.Status, got.Body)
	}

	// With nowhere to start, it opens at home and says why, safely and for good.
	for _, test := range []struct{ query, says string }{
		{"&project=empty", "empty"},
		{"&project=p1&run=none", "none"},
	} {
		page := open(start(test.query))
		page.await("so this shell opened in your home directory.")
		if !strings.Contains(page.shown.String(), test.says) {
			t.Errorf("%s: the shell was told %q", test.query, page.shown.String())
		}
		inDir(page, home)
	}
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	gone := start("&project=p1")
	page := open(gone)
	page.await("so this shell opened in your home directory.")
	if !strings.Contains(page.shown.String(), strconv.Quote(second)) {
		t.Errorf("the shell was not told what is missing: %q", page.shown.String())
	}
	inDir(page, home)
	_ = page.connection.Close(websocket.StatusNormalClosure, "")
	again := open(gone)
	again.await("so this shell opened in your home directory.")

	for name, test := range map[string]struct {
		query string
		want  int
	}{
		"a run alone":        {"&run=" + firstRun, http.StatusBadRequest},
		"a bad project":      {"&project=a%2Fb", http.StatusBadRequest},
		"a bad run":          {"&project=p1&run=..", http.StatusBadRequest},
		"an unknown basedir": {"&project=p1&basedir=nope", http.StatusNotFound},
	} {
		status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80"+test.query, token)
		if status != test.want {
			t.Errorf("%s: status %d, want %d: %s", name, status, test.want, body)
		}
	}
}

// The flags set the limits, the replay, and the shell.
func TestTerminalFollowsItsFlags(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	marker := filepath.Join(e.Root, "my-shell")
	if err := os.WriteFile(marker, []byte("#!/bin/sh\nMY_SHELL=marked exec /bin/sh \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	base := e.StartWeb("--allow-terminal", "--terminal-max-shells", "3", "--terminal-max-shells-per-project", "2", "--terminal-replay-size", "1KiB", "--terminal-idle-timeout", "5m", "--terminal-shell", marker)
	token := http.Header{"X-Rotari-Token": {launchToken}}
	start := func(query string, want int) {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80"+query, token)
		if status != want {
			t.Errorf("starting with %q: status %d, want %d: %s", query, status, want, body)
		}
	}
	start("", http.StatusCreated)
	start("", http.StatusCreated)
	start("", http.StatusConflict) // The project-less group's limit.
	start("&project=a", http.StatusCreated)
	start("&project=b", http.StatusConflict) // The limit on all of them.
	if status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells", token); status != http.StatusOK || !strings.Contains(string(body), `"limit":3`) || !strings.Contains(string(body), `"group_limit":2`) {
		t.Errorf("the list's limits: status %d: %s", status, body)
	}

	page := openTerminalPage(t, base)
	page.typeLine(`echo "shell-$MY_SHELL"; yes old | head -c 8192; echo; echo "end-$((1+1))"`)
	page.await("shell-marked")
	page.await("end-2")
	replayed := openTerminalPage(t, base)
	page.awaitClose(terminalTakenOver)
	replayed.await("end-2")
	// Two limits' worth at most, since the ring keeps up to twice before cutting.
	if size := replayed.shown.Len(); size > 2<<10+512 {
		t.Errorf("replayed %d bytes with a 1 KiB replay", size)
	}
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
	token := http.Header{"X-Rotari-Token": {launchToken}}
	if status, body := terminalShellsRequest(t, http.MethodDelete, base+"/_terminal/shells/"+aTerminalShell(t, base, token), token); status != http.StatusNoContent {
		t.Fatalf("ending the shell: status %d: %s", status, body)
	}
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
	browserHeaders := http.Header{"Cookie": {session}, "Origin": {base}}
	page, err := dialTerminalPage(t, base, "", browserHeaders)
	if err != nil {
		t.Fatalf("a signed-in browser could not connect: %v", err)
	}
	startTerminalShell(t, base, browserHeaders)
	page.typeLine(`echo "in-$((1+1))"`)
	page.await("in-2")
	post("/_auth/logout", session, url.Values{})
	page.awaitClose(terminalShellExited)
	if shells := listTerminalShells(t, base, http.Header{"X-Rotari-Token": {launchToken}}); len(shells) != 0 {
		t.Errorf("signing out left %v", shells)
	}
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

// TestTerminalMarksACommandFinishedOutOfSight checks that the shell list says when a
// command ended while no connection showed its shell, until one does again.
func TestTerminalMarksACommandFinishedOutOfSight(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	running := false
	finished := func() bool {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells", headers)
		var listed struct {
			Shells []struct {
				CommandFinished bool `json:"command_finished"`
				Running         bool `json:"running"`
			} `json:"shells"`
		}
		if err := json.Unmarshal(body, &listed); status != http.StatusOK || err != nil || len(listed.Shells) != 1 {
			t.Fatalf("listing: status %d, %v: %s", status, err, body)
		}
		running = listed.Shells[0].Running
		return listed.Shells[0].CommandFinished
	}
	page := openTerminalPage(t, base)
	page.typeLine(`echo "ready-$((1+1))"`)
	page.await("ready-2")
	id := listTerminalShells(t, base, headers)[0].ID
	page.typeLine(`sleep 1`)
	for deadline := time.Now().Add(terminalWait); ; time.Sleep(20 * time.Millisecond) {
		if _, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells/"+id+"/preview", headers); strings.Contains(string(body), `"command":"sleep 1"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sleep never ran")
		}
	}
	if finished() || !running {
		t.Errorf("a command still running counted as finished, or not as running (running %v)", running)
	}
	_ = page.connection.Close(websocket.StatusNormalClosure, "")
	for deadline := time.Now().Add(terminalWait); !finished(); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the command that ended out of sight never counted as finished")
		}
	}
	if running {
		t.Error("a shell back at its prompt counted as running a command")
	}
	again := openTerminalPage(t, base)
	again.await("ready-2")
	if finished() {
		t.Error("showing the shell again left its command finished")
	}
}

// TestTerminalListsWhenShellsStartedAndWereActive checks that the shell list dates each
// shell's start and last activity by the server's clock, and that output moves the latter.
func TestTerminalListsWhenShellsStartedAndWereActive(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	type listing struct {
		Now    time.Time `json:"now"`
		Shells []struct {
			StartedAt time.Time `json:"started_at"`
			ActiveAt  time.Time `json:"active_at"`
		} `json:"shells"`
	}
	list := func() listing {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells", headers)
		var listed listing
		if err := json.Unmarshal(body, &listed); status != http.StatusOK || err != nil || len(listed.Shells) != 1 {
			t.Fatalf("listing: status %d, %v: %s", status, err, body)
		}
		return listed
	}
	page := openTerminalPage(t, base)
	page.typeLine(`echo "ready-$((1+1))"`)
	page.await("ready-2")
	first := list()
	shell := first.Shells[0]
	if shell.StartedAt.IsZero() || shell.ActiveAt.Before(shell.StartedAt) || first.Now.Before(shell.ActiveAt) {
		t.Fatalf("the times do not hold together: %+v", first)
	}
	time.Sleep(50 * time.Millisecond)
	page.typeLine(`echo "later-$((1+1))"`)
	page.await("later-2")
	if second := list(); !second.Shells[0].ActiveAt.After(shell.ActiveAt) || !second.Shells[0].StartedAt.Equal(shell.StartedAt) {
		t.Errorf("activity did not move, or the start did: %+v then %+v", first, second)
	}
}

// TestTerminalPreviewListsTheShellsProcesses checks that a shell's preview lists the
// processes it started, a background one included, each with its depth below the shell.
func TestTerminalPreviewListsTheShellsProcesses(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	page := openTerminalPage(t, base)
	page.typeLine(`sleep 26 & echo "started-$((1+1))"`)
	page.await("started-2")
	id := listTerminalShells(t, base, headers)[0].ID
	var preview struct {
		Processes []struct {
			PID     int    `json:"pid"`
			Depth   int    `json:"depth"`
			Command string `json:"command"`
		} `json:"processes"`
	}
	for deadline := time.Now().Add(terminalWait); ; time.Sleep(20 * time.Millisecond) {
		status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells/"+id+"/preview", headers)
		if err := json.Unmarshal(body, &preview); status != http.StatusOK || err != nil {
			t.Fatalf("preview: status %d, %v: %s", status, err, body)
		}
		if len(preview.Processes) == 1 && preview.Processes[0].Command == "sleep 26" && preview.Processes[0].Depth == 1 && preview.Processes[0].PID > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the preview lists %+v, want sleep 26 alone", preview.Processes)
		}
	}
}

// /_terminal/projects says what each project with shells last ran: a project whose
// second run fixed the first's failure counts none, one whose last run failed counts
// its failure, and one with no runs says so.
func TestTerminalProjectsSayWhatTheirLastRunDid(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	// Each project's job fails or succeeds by whether its marker exists, and its first
	// run makes the marker, so its second run does the other.
	for project, script := range map[string]string{
		"fixed": `test -e "$1" || { touch "$1"; exit 1; }`,
		"broke": `test -e "$1" && exit 1; touch "$1"`,
	} {
		for range 2 {
			e.MustRotari("add", "-p", project, "--", "sh", "-c", script, "sh", filepath.Join(e.Root, project+"-marker"))
			e.Rotari("run", "-p", project, "--quiet")
		}
	}
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	for _, project := range []string{"fixed", "broke", "fresh"} {
		if status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80&project="+project, headers); status != http.StatusCreated {
			t.Fatalf("starting a shell in %s: status %d: %s", project, status, body)
		}
	}
	status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/projects", headers)
	var listed struct {
		Projects []struct {
			Project string `json:"project"`
			Runs    int    `json:"runs"`
			LastRun *struct {
				Lifecycle string         `json:"lifecycle"`
				StartedAt string         `json:"started_at"`
				Jobs      map[string]int `json:"jobs"`
			} `json:"last_run"`
			StatusError string `json:"status_error"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(body, &listed); status != http.StatusOK || err != nil {
		t.Fatalf("projects: status %d, %v: %s", status, err, body)
	}
	got := map[string]string{}
	for _, project := range listed.Projects {
		summary := fmt.Sprintf("runs %d", project.Runs)
		if project.LastRun != nil {
			summary += fmt.Sprintf(", %s, %v, started %t", project.LastRun.Lifecycle, project.LastRun.Jobs, project.LastRun.StartedAt != "")
		}
		if project.StatusError != "" {
			summary += ", error " + project.StatusError
		}
		got[project.Project] = summary
	}
	want := map[string]string{
		"fixed": "runs 2, finished, map[ok:1], started true",
		"broke": "runs 2, failed, map[bad:1], started true",
		"fresh": "runs 0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("projects:\n got %v\nwant %v\n%s", got, want, body)
	}
}
