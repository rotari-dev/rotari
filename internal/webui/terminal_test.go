package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/rotari-dev/rotari/internal/terminal"
)

const (
	terminalTestToken      = "secret"
	terminalTestShellLimit = 3
)

// terminalServer is a Web server with the terminal enabled, wrapped as rotari web
// wraps it.
func terminalServer(t *testing.T, shell string) (*httptest.Server, *terminal.Keeper) {
	t.Helper()
	home := t.TempDir()
	keeper, err := terminal.NewKeeper(terminal.KeeperOptions{
		Shell:      terminal.Options{Shell: shell, Dir: home, Env: []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "PS1=$ "}},
		IdleLimit:  time.Hour,
		RingSize:   1 << 16,
		ShellLimit: terminalTestShellLimit,
		GroupLimit: terminalTestShellLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	handler := Handler(Options{BaseDir: t.TempDir(), Terminal: keeper})
	server := httptest.NewServer(WithExecGuard(WithAuthToken(handler, terminalTestToken, AuthOptions{OnSignOut: []func(){keeper.EndAll}}), nil))
	t.Cleanup(server.Close)
	return server, keeper
}

func terminalSocketURL(server *httptest.Server, query string) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + terminalSocketPath + "?" + query
}

// terminalClient is a connected page.
type terminalClient struct {
	t          *testing.T
	connection *websocket.Conn
	outputs    chan []byte
	closed     chan error
	text       strings.Builder
}

// terminalRequest makes a request with the token and returns the status and body.
func terminalRequest(t *testing.T, server *httptest.Server, method, path string) (int, string) {
	t.Helper()
	request, err := http.NewRequest(method, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Rotari-Token", terminalTestToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(body)
}

func startTerminalShell(t *testing.T, server *httptest.Server) terminal.ShellInfo {
	t.Helper()
	status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80")
	if status != http.StatusCreated {
		t.Fatalf("starting a shell: status %d: %s", status, body)
	}
	var shell terminal.ShellInfo
	if err := json.Unmarshal([]byte(body), &shell); err != nil {
		t.Fatal(err)
	}
	return shell
}

func listTerminalShells(t *testing.T, server *httptest.Server) []terminal.ShellInfo {
	t.Helper()
	status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath)
	if status != http.StatusOK {
		t.Fatalf("listing shells: status %d: %s", status, body)
	}
	var listed struct {
		Shells []terminal.ShellInfo `json:"shells"`
		Limit  int                  `json:"limit"`
	}
	if err := json.Unmarshal([]byte(body), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Limit != terminalTestShellLimit {
		t.Errorf("limit = %d, want %d", listed.Limit, terminalTestShellLimit)
	}
	return listed.Shells
}

// dialTerminal connects to the first running shell, starting one if none is.
func dialTerminal(t *testing.T, server *httptest.Server) *terminalClient {
	t.Helper()
	shells := listTerminalShells(t, server)
	if len(shells) == 0 {
		shells = []terminal.ShellInfo{startTerminalShell(t, server)}
	}
	return dialTerminalShell(t, server, shells[0].ID)
}

func dialTerminalShell(t *testing.T, server *httptest.Server, id string) *terminalClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, terminalSocketURL(server, "shell="+id+"&rows=24&cols=80"), &websocket.DialOptions{
		HTTPHeader: http.Header{"X-Rotari-Token": {terminalTestToken}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return readTerminal(t, connection)
}

// readTerminal collects what a connection receives until it closes.
func readTerminal(t *testing.T, connection *websocket.Conn) *terminalClient {
	t.Helper()
	connection.SetReadLimit(1 << 24)
	client := &terminalClient{t: t, connection: connection, outputs: make(chan []byte, 1024), closed: make(chan error, 1)}
	t.Cleanup(func() { _ = connection.CloseNow() })
	go func() {
		for {
			_, message, err := connection.Read(context.Background())
			if err != nil {
				client.closed <- err
				return
			}
			client.outputs <- message
		}
	}()
	return client
}

func (client *terminalClient) typeLine(line string) {
	client.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.connection.Write(ctx, websocket.MessageBinary, []byte(line+"\n")); err != nil {
		client.t.Fatal(err)
	}
}

func (client *terminalClient) control(text string) {
	client.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.connection.Write(ctx, websocket.MessageText, []byte(text)); err != nil {
		client.t.Fatal(err)
	}
}

// await reads until a line of output ends with want.
func (client *terminalClient) await(want string) {
	client.t.Helper()
	timeout := time.After(15 * time.Second)
	for !strings.Contains(client.text.String(), want+"\r\n") {
		select {
		case output := <-client.outputs:
			client.text.Write(output)
		case err := <-client.closed:
			client.t.Fatalf("closed waiting for %q: %v; output:\n%s", want, err, client.text.String())
		case <-timeout:
			client.t.Fatalf("output never showed %q:\n%s", want, client.text.String())
		}
	}
}

// awaitClose waits for the connection to close with the code.
func (client *terminalClient) awaitClose(want websocket.StatusCode) string {
	client.t.Helper()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case output := <-client.outputs:
			client.text.Write(output)
		case err := <-client.closed:
			var closed websocket.CloseError
			if !errors.As(err, &closed) || closed.Code != want {
				client.t.Fatalf("closed with %v, want code %d", err, want)
			}
			return closed.Reason
		case <-timeout:
			client.t.Fatalf("the connection stayed open, want close code %d", want)
		}
	}
}

func TestTerminalRunsCommandsAndResizes(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	client := dialTerminal(t, server)
	client.typeLine(`echo "typed-$((1+1))"; stty size`)
	client.await("typed-2")
	client.await("24 80")
	client.control(`{"type":"resize","rows":40,"cols":100}`)
	client.typeLine(`stty size`)
	client.await("40 100")
}

func TestTerminalShellOutlivesItsConnection(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	first := dialTerminal(t, server)
	first.typeLine(`VALUE=kept; echo "one-$((1+1))"`)
	first.await("one-2")
	_ = first.connection.Close(websocket.StatusNormalClosure, "")

	second := dialTerminal(t, server)
	second.await("one-2")
	second.typeLine(`echo "value-$VALUE"`)
	second.await("value-kept")
}

func TestTerminalNewConnectionTakesOver(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	first := dialTerminal(t, server)
	first.typeLine(`VALUE=kept; echo "one-$((1+1))"`)
	first.await("one-2")
	second := dialTerminal(t, server)
	first.awaitClose(terminalCloseTakenOver)
	second.typeLine(`echo "value-$VALUE"`)
	second.await("value-kept")
}

// A page reconnecting by itself must not take the shell from the tab in use.
func TestTerminalReconnectingDoesNotTakeOver(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	first := dialTerminal(t, server)
	first.typeLine(`echo "one-$((1+1))"`)
	first.await("one-2")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	id := listTerminalShells(t, server)[0].ID
	connection, _, err := websocket.Dial(ctx, terminalSocketURL(server, "shell="+id+"&rows=24&cols=80&takeover=0"), &websocket.DialOptions{
		HTTPHeader: http.Header{"X-Rotari-Token": {terminalTestToken}},
	})
	if err != nil {
		t.Fatal(err)
	}
	reconnecting := readTerminal(t, connection)
	reconnecting.awaitClose(terminalCloseTakenOver)
	first.typeLine(`echo "still-$((1+1))"`)
	first.await("still-2")
}

func TestTerminalReportsHowTheShellEnded(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	client := dialTerminal(t, server)
	client.typeLine("exit 3")
	if reason := client.awaitClose(terminalCloseExited); reason != "exit status 3" {
		t.Errorf("reason = %q", reason)
	}
}

func TestTerminalEndingAShellClosesItsConnection(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	client := dialTerminal(t, server)
	client.typeLine(`VALUE=first; echo "one-$((1+1))"`)
	client.await("one-2")
	id := listTerminalShells(t, server)[0].ID
	if status, body := terminalRequest(t, server, http.MethodDelete, terminalShellsPath+"/"+id); status != http.StatusNoContent {
		t.Fatalf("ending the shell: status %d: %s", status, body)
	}
	client.awaitClose(terminalCloseExited)
	if status, _ := terminalRequest(t, server, http.MethodDelete, terminalShellsPath+"/"+id); status != http.StatusNotFound {
		t.Errorf("ending it again: status %d, want 404", status)
	}
	next := dialTerminal(t, server)
	next.typeLine(`echo "value-[$VALUE]"`)
	next.await("value-[]")
}

// Shells run side by side, each with its own connection, up to the limit.
func TestTerminalKeepsSeveralShells(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	if shells := listTerminalShells(t, server); len(shells) != 0 {
		t.Fatalf("shells before any was asked for: %v", shells)
	}
	one, two := startTerminalShell(t, server), startTerminalShell(t, server)
	if one.Number != 1 || two.Number != 2 || one.ID == two.ID {
		t.Errorf("started %+v and %+v", one, two)
	}
	first, second := dialTerminalShell(t, server, one.ID), dialTerminalShell(t, server, two.ID)
	first.typeLine(`VALUE=first; echo "one-$((1+1))"`)
	first.await("one-2")
	second.typeLine(`VALUE=second; echo "two-$((1+1))"`)
	second.await("two-2")
	first.typeLine(`echo "value-$VALUE"`)
	first.await("value-first")
	second.typeLine(`echo "value-$VALUE"`)
	second.await("value-second")

	startTerminalShell(t, server)
	if status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80"); status != http.StatusConflict || !strings.Contains(body, "3") {
		t.Errorf("past the limit: status %d: %s", status, body)
	}

	if status, body := terminalRequest(t, server, http.MethodDelete, terminalShellsPath+"/"+one.ID); status != http.StatusNoContent {
		t.Fatalf("ending a shell: status %d: %s", status, body)
	}
	first.awaitClose(terminalCloseExited)
	second.typeLine(`echo "still-$((1+1))"`)
	second.await("still-2")
}

// A page holding a shell that has ended hears so rather than getting a new one.
func TestTerminalConnectionToAShellNotRunning(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, terminalSocketURL(server, "shell=7&rows=24&cols=80"), &websocket.DialOptions{
		HTTPHeader: http.Header{"X-Rotari-Token": {terminalTestToken}},
	})
	if err != nil {
		t.Fatal(err)
	}
	readTerminal(t, connection).awaitClose(terminalCloseNoShell)
	if shells := listTerminalShells(t, server); len(shells) != 0 {
		t.Errorf("a connection started %v", shells)
	}
}

func TestTerminalShellRoutes(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	shell := startTerminalShell(t, server)
	for name, test := range map[string]struct {
		method, path string
		want         int
	}{
		"start with no size":  {http.MethodPost, terminalShellsPath, http.StatusBadRequest},
		"start with no rows":  {http.MethodPost, terminalShellsPath + "?rows=0&cols=80", http.StatusBadRequest},
		"replace the list":    {http.MethodPut, terminalShellsPath, http.StatusMethodNotAllowed},
		"read one shell":      {http.MethodGet, terminalShellsPath + "/" + shell.ID, http.StatusMethodNotAllowed},
		"end an unknown one":  {http.MethodDelete, terminalShellsPath + "/7", http.StatusNotFound},
		"end a path in there": {http.MethodDelete, terminalShellsPath + "/" + shell.ID + "/more", http.StatusNotFound},
		"a run alone":         {http.MethodPost, terminalShellsPath + "?rows=24&cols=80&run=r", http.StatusBadRequest},
		"a bad project":       {http.MethodPost, terminalShellsPath + "?rows=24&cols=80&project=a%2Fb", http.StatusBadRequest},
		"a bad run":           {http.MethodPost, terminalShellsPath + "?rows=24&cols=80&project=p&run=..", http.StatusBadRequest},
		"an unknown basedir":  {http.MethodPost, terminalShellsPath + "?rows=24&cols=80&project=p&basedir=nope", http.StatusNotFound},
		"here, a bad project": {http.MethodGet, terminalHerePath + "?project=a%2Fb", http.StatusBadRequest},
		"change here":         {http.MethodPost, terminalHerePath, http.StatusMethodNotAllowed},
		"say alive":           {http.MethodPost, terminalAlivePath, http.StatusNoContent},
		"order, no body":      {http.MethodPut, terminalOrderPath, http.StatusBadRequest},
		"read the order":      {http.MethodGet, terminalOrderPath, http.StatusMethodNotAllowed},
		"rename, no body":     {http.MethodPatch, terminalShellsPath + "/" + shell.ID, http.StatusBadRequest},
		"rename an unknown":   {http.MethodPatch, terminalShellsPath + "/7", http.StatusBadRequest},
		"read alive":          {http.MethodGet, terminalAlivePath, http.StatusMethodNotAllowed},
		"the window":          {http.MethodGet, terminalWindowPath + "/project/p", http.StatusOK},
		"the window, bare":    {http.MethodGet, terminalWindowPath, http.StatusOK},
		"post to the window":  {http.MethodPost, terminalWindowPath + "/project/p", http.StatusMethodNotAllowed},
		"a window lookalike":  {http.MethodGet, terminalWindowPath + "s", http.StatusNotFound},
		"preview an unknown":  {http.MethodGet, terminalShellsPath + "/7/" + terminalPreviewName, http.StatusNotFound},
		"post to a preview":   {http.MethodPost, terminalShellsPath + "/" + shell.ID + "/" + terminalPreviewName, http.StatusMethodNotAllowed},
		"end a preview":       {http.MethodDelete, terminalShellsPath + "/" + shell.ID + "/" + terminalPreviewName, http.StatusMethodNotAllowed},
	} {
		if status, body := terminalRequest(t, server, test.method, test.path); status != test.want {
			t.Errorf("%s: status %d, want %d: %s", name, status, test.want, body)
		}
	}
	if shells := listTerminalShells(t, server); len(shells) != 1 {
		t.Errorf("the requests left %v", shells)
	}
	// The list says whether a command finished out of sight, for the tabs' bell, and whether
	// one runs now, for signing out. Their values are another test's: a login shell may still
	// be running its profile's commands here.
	status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath)
	var listed struct {
		Shells []map[string]json.RawMessage `json:"shells"`
	}
	if err := json.Unmarshal([]byte(body), &listed); status != http.StatusOK || err != nil || len(listed.Shells) != 1 {
		t.Fatalf("listing: status %d, %v: %s", status, err, body)
	}
	for _, field := range []string{"command_finished", "running"} {
		if value := string(listed.Shells[0][field]); value != "true" && value != "false" {
			t.Errorf("the list's %s is %q, want true or false: %s", field, value, body)
		}
	}
	// And when the shell started and was last active, with the server's time to date them by.
	var times struct {
		Now    time.Time `json:"now"`
		Shells []struct {
			StartedAt time.Time `json:"started_at"`
			ActiveAt  time.Time `json:"active_at"`
		} `json:"shells"`
	}
	if err := json.Unmarshal([]byte(body), &times); err != nil || times.Now.IsZero() || times.Shells[0].StartedAt.IsZero() ||
		times.Shells[0].ActiveAt.Before(times.Shells[0].StartedAt) || times.Now.Before(times.Shells[0].ActiveAt) {
		t.Errorf("the list's times do not hold together: %v: %s", err, body)
	}
}

// A preview says where the shell is now, without taking it from the page showing it.
func TestTerminalPreviewSaysWhereTheShellIs(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	client := dialTerminal(t, server)
	id := listTerminalShells(t, server)[0].ID
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client.typeLine(`cd '` + dir + `' && echo "moved-$((1+1))"`)
	client.await("moved-2")
	status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath+"/"+id+"/"+terminalPreviewName)
	var preview terminalPreviewResponse
	if err := json.Unmarshal([]byte(body), &preview); status != http.StatusOK || err != nil || !reflect.DeepEqual(preview, terminalPreviewResponse{Cwd: dir, Processes: []terminalProcessResponse{}}) {
		t.Errorf("preview: status %d, %v: %s; want the directory %q", status, err, body, dir)
	}
	client.typeLine(`echo "still-$((1+1))"`)
	client.await("still-2")
	// What it runs, while it runs.
	client.typeLine(`sleep 30`)
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		status, body = terminalRequest(t, server, http.MethodGet, terminalShellsPath+"/"+id+"/"+terminalPreviewName)
		preview = terminalPreviewResponse{}
		if err := json.Unmarshal([]byte(body), &preview); status == http.StatusOK && err == nil && preview.Cwd == dir && preview.Command == "sleep 30" &&
			len(preview.Processes) == 1 && preview.Processes[0].Command == "sleep 30" && preview.Processes[0].Depth == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("preview while sleep runs: status %d: %s", status, body)
		}
	}
}

func TestTerminalRenamesAShell(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	shell := startTerminalShell(t, server)
	rename := func(id, body string) (int, string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPatch, server.URL+terminalShellsPath+"/"+id, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Rotari-Token", terminalTestToken)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		text, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, string(text)
	}
	if status, body := rename(shell.ID, `{"name":"  ビルド <b>  "}`); status != http.StatusOK || !strings.Contains(body, "ビルド \\u003cb\\u003e") {
		t.Errorf("rename: status %d: %s", status, body)
	}
	status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath)
	if status != http.StatusOK || !strings.Contains(body, `"name":"ビルド \u003cb\u003e"`) {
		t.Errorf("the list lost the name: status %d: %s", status, body)
	}
	for name, test := range map[string]struct {
		id, body string
		want     int
	}{
		"a line break": {shell.ID, `{"name":"a\nb"}`, http.StatusBadRequest},
		"an escape":    {shell.ID, `{"name":"a\u001bb"}`, http.StatusBadRequest},
		"too long":     {shell.ID, `{"name":"` + strings.Repeat("あ", terminalNameLimit+1) + `"}`, http.StatusBadRequest},
		"not JSON":     {shell.ID, `name`, http.StatusBadRequest},
		"unknown":      {"7", `{"name":"x"}`, http.StatusNotFound},
		"longest":      {shell.ID, `{"name":"` + strings.Repeat("あ", terminalNameLimit) + `"}`, http.StatusOK},
		"cleared":      {shell.ID, `{"name":""}`, http.StatusOK},
	} {
		if status, body := rename(test.id, test.body); status != test.want {
			t.Errorf("%s: status %d, want %d: %s", name, status, test.want, body)
		}
	}
}

// Starting and ending shells are guarded like the connection.
func TestTerminalShellRoutesAreGuarded(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	shell := startTerminalShell(t, server)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, terminalShellsPath},
		{http.MethodPost, terminalShellsPath + "?rows=24&cols=80"},
		{http.MethodDelete, terminalShellsPath + "/" + shell.ID},
		{http.MethodPost, terminalAlivePath},
		{http.MethodPatch, terminalShellsPath + "/" + shell.ID},
		{http.MethodPut, terminalOrderPath},
		{http.MethodGet, terminalWindowPath + "/project/p"},
		{http.MethodGet, terminalShellsPath + "/" + shell.ID + "/" + terminalPreviewName},
		{http.MethodGet, terminalFilesPath + "?path=/"},
	} {
		mutates := route.method != http.MethodGet
		for _, test := range []struct {
			name    string
			headers map[string]string
			want    int
		}{
			{"wrong token", map[string]string{"X-Rotari-Token": "wrong"}, http.StatusUnauthorized},
			{"rebinding host", map[string]string{"X-Rotari-Token": terminalTestToken, "Host": "attacker.example"}, http.StatusMisdirectedRequest},
			{"plain HTTP proxy", map[string]string{"X-Rotari-Token": terminalTestToken, "X-Forwarded-For": "192.0.2.1"}, http.StatusForbidden},
			{"another site", map[string]string{"X-Rotari-Token": terminalTestToken, "Origin": "http://attacker.example"}, http.StatusForbidden},
		} {
			if test.name == "another site" && !mutates {
				continue
			}
			request, err := http.NewRequest(route.method, server.URL+route.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			for name, value := range test.headers {
				if name == "Host" {
					request.Host = value
				} else {
					request.Header.Set(name, value)
				}
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.want {
				t.Errorf("%s %s, %s: status %d, want %d", route.method, route.path, test.name, response.StatusCode, test.want)
			}
		}
	}
	if shells := listTerminalShells(t, server); len(shells) != 1 || shells[0].ID != shell.ID {
		t.Errorf("a refused request changed the shells: %v", shells)
	}
}

func TestTerminalRejectsUnknownControlMessages(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	for message, want := range map[string]websocket.StatusCode{
		`{"type":"other"}`: websocket.StatusPolicyViolation,
		`not json`:         websocket.StatusInvalidFramePayloadData,
	} {
		client := dialTerminal(t, server)
		client.control(message)
		client.awaitClose(want)
	}
}

func TestTerminalClearForgetsTheReplay(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	first := dialTerminal(t, server)
	first.typeLine(`VALUE=kept; echo "old-$((1+1))"`)
	first.await("old-2")
	first.control(`{"type":"clear"}`)
	first.typeLine(`echo "new-$((1+1))"`)
	first.await("new-2")
	_ = first.connection.Close(websocket.StatusNormalClosure, "")
	second := dialTerminal(t, server)
	second.await("new-2")
	second.typeLine(`echo "value-$VALUE"`)
	second.await("value-kept")
	if strings.Contains(second.text.String(), "old-2") {
		t.Errorf("cleared output came back:\n%s", second.text.String())
	}
}

func TestTerminalBadResizeKeepsTheShell(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	client := dialTerminal(t, server)
	client.control(`{"type":"resize","rows":0,"cols":100}`)
	client.typeLine(`echo "still-$((1+1))"`)
	client.await("still-2")
}

func TestTerminalReportsAFailedStart(t *testing.T) {
	server, _ := terminalServer(t, filepath.Join(t.TempDir(), "missing"))
	status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80")
	if status != http.StatusInternalServerError || !strings.Contains(body, "missing") {
		t.Errorf("status %d, body %q, want the shell named", status, body)
	}
}

func TestTerminalClosesWhenTheServerStops(t *testing.T) {
	server, keeper := terminalServer(t, "/bin/sh")
	client := dialTerminal(t, server)
	client.typeLine(`echo "one-$((1+1))"`)
	client.await("one-2")
	keeper.Close()
	client.awaitClose(terminalCloseStopping)
}

func TestTerminalHandshakeIsGuarded(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	hostName := strings.TrimPrefix(server.URL, "http://")
	for _, test := range []struct {
		name    string
		path    string
		headers map[string]string
		want    int
	}{
		{"no credentials", terminalSocketPath + "?shell=1&rows=24&cols=80", nil, http.StatusForbidden},
		{"wrong token", terminalSocketPath + "?shell=1&rows=24&cols=80", map[string]string{"X-Rotari-Token": "wrong"}, http.StatusUnauthorized},
		{"another site", terminalSocketPath + "?shell=1&rows=24&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken, "Origin": "http://attacker.example"}, http.StatusForbidden},
		{"rebinding host", terminalSocketPath + "?shell=1&rows=24&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken, "Host": "attacker.example"}, http.StatusMisdirectedRequest},
		{"through a plain HTTP proxy, with a token", terminalSocketPath + "?shell=1&rows=24&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken, "X-Forwarded-For": "192.0.2.1"}, http.StatusForbidden},
		{"no shell", terminalSocketPath + "?rows=24&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken}, http.StatusBadRequest},
		{"no size", terminalSocketPath + "?shell=1", map[string]string{"X-Rotari-Token": terminalTestToken}, http.StatusBadRequest},
		{"no rows", terminalSocketPath + "?shell=1&rows=0&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken}, http.StatusBadRequest},
		{"size too large", terminalSocketPath + "?shell=1&rows=70000&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken}, http.StatusBadRequest},
		{"another path", terminalPrefix + "other", map[string]string{"X-Rotari-Token": terminalTestToken}, http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, server.URL+test.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			for name, value := range map[string]string{"Connection": "Upgrade", "Upgrade": "websocket", "Sec-WebSocket-Version": "13", "Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ=="} {
				request.Header.Set(name, value)
			}
			for name, value := range test.headers {
				if name == "Host" {
					request.Host = value
				} else {
					request.Header.Set(name, value)
				}
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.want {
				t.Errorf("%s on %s: status %d, want %d", test.name, hostName, response.StatusCode, test.want)
			}
		})
	}
}

// A browser signing in reaches a shell by its session, and signing out ends them all.
func TestTerminalSessionAndSignOut(t *testing.T) {
	server, keeper := terminalServer(t, "/bin/sh")
	browser := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var session string
	post := func(path string, form url.Values) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, server.URL+path, strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", server.URL)
		if session != "" {
			request.Header.Set("Cookie", session)
		}
		response, err := browser.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response
	}
	response := post(webLoginPath, url.Values{"username": {"rotari"}, "password": {terminalTestToken}, "next": {"/"}})
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("sign in: status %d", response.StatusCode)
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == webSessionCookie {
			session = cookie.Name + "=" + cookie.Value
		}
	}
	if session == "" {
		t.Fatal("signing in set no session cookie")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := keeper.Start(terminal.StartOptions{Rows: 24, Cols: 80}); err != nil {
		t.Fatal(err)
	}
	shell, err := keeper.Start(terminal.StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	connection, _, err := websocket.Dial(ctx, terminalSocketURL(server, "shell="+shell.ID+"&rows=24&cols=80"), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {server.URL}, "Cookie": {session}},
	})
	if err != nil {
		t.Fatalf("a signed-in browser could not connect: %v", err)
	}
	client := readTerminal(t, connection)
	client.typeLine(`echo "in-$((1+1))"`)
	client.await("in-2")
	if response := post(webLogoutPath, url.Values{}); response.StatusCode >= http.StatusBadRequest {
		t.Fatalf("sign out: status %d", response.StatusCode)
	}
	client.awaitClose(terminalCloseExited)
	if shells := keeper.Shells(); len(shells) != 0 {
		t.Errorf("signing out left %v", shells)
	}
}

func TestTerminalIsAbsentWithoutTheOption(t *testing.T) {
	server := httptest.NewServer(Handler(Options{BaseDir: t.TempDir()}))
	defer server.Close()
	for _, path := range []string{terminalSocketPath + "?shell=1&rows=24&cols=80", terminalShellsPath, terminalWindowPath + "/project/p", terminalFilesPath + "?path=/"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, response.StatusCode)
		}
	}
}

// The window's page runs only its own script, under the nonce its policy names.
func TestTerminalWindowPage(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	request, err := http.NewRequest(http.MethodGet, server.URL+terminalWindowPath+"/project/p", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Rotari-Token", terminalTestToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	nonce := regexp.MustCompile(`script-src 'nonce-([^']+)'`).FindStringSubmatch(response.Header.Get("Content-Security-Policy"))
	if nonce == nil {
		t.Fatalf("no script nonce in the policy %q", response.Header.Get("Content-Security-Policy"))
	}
	page := string(body)
	if !strings.Contains(page, `<script nonce="`+nonce[1]+`">`) || strings.Count(page, "<script") != 1 {
		t.Errorf("the page's script does not carry the policy's nonce:\n%s", page[:min(len(page), 2000)])
	}
	if !strings.Contains(page, "data-terminal-window") || strings.Contains(page, "__ROTARI_") {
		t.Errorf("the page is not the window's, or keeps a placeholder")
	}
}
