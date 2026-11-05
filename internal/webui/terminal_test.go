package webui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/rotari-dev/rotari/internal/terminal"
)

const terminalTestToken = "secret"

// terminalServer is a Web server with the terminal enabled, wrapped as rotari web
// wraps it.
func terminalServer(t *testing.T, shell string) (*httptest.Server, *terminal.Keeper) {
	t.Helper()
	home := t.TempDir()
	keeper, err := terminal.NewKeeper(terminal.KeeperOptions{
		Shell:     terminal.Options{Shell: shell, Dir: home, Env: []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "PS1=$ "}},
		IdleLimit: time.Hour,
		RingSize:  1 << 16,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	handler := Handler(Options{BaseDir: t.TempDir(), Terminal: keeper})
	server := httptest.NewServer(WithExecGuard(WithAuthToken(handler, terminalTestToken, keeper.EndShell), nil))
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

func dialTerminal(t *testing.T, server *httptest.Server) *terminalClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, terminalSocketURL(server, "rows=24&cols=80"), &websocket.DialOptions{
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
	connection, _, err := websocket.Dial(ctx, terminalSocketURL(server, "rows=24&cols=80&takeover=0"), &websocket.DialOptions{
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

func TestTerminalEndMessageEndsTheShell(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	client := dialTerminal(t, server)
	client.typeLine(`VALUE=first; echo "one-$((1+1))"`)
	client.await("one-2")
	client.control(`{"type":"end"}`)
	client.awaitClose(terminalCloseExited)
	next := dialTerminal(t, server)
	next.typeLine(`echo "value-[$VALUE]"`)
	next.await("value-[]")
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

func TestTerminalBadResizeKeepsTheShell(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	client := dialTerminal(t, server)
	client.control(`{"type":"resize","rows":0,"cols":100}`)
	client.typeLine(`echo "still-$((1+1))"`)
	client.await("still-2")
}

func TestTerminalReportsAFailedStart(t *testing.T) {
	server, _ := terminalServer(t, filepath.Join(t.TempDir(), "missing"))
	client := dialTerminal(t, server)
	if reason := client.awaitClose(terminalCloseFailed); !strings.Contains(reason, "missing") {
		t.Errorf("reason = %q, want the shell named", reason)
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
		{"no credentials", terminalSocketPath + "?rows=24&cols=80", nil, http.StatusForbidden},
		{"wrong token", terminalSocketPath + "?rows=24&cols=80", map[string]string{"X-Rotari-Token": "wrong"}, http.StatusUnauthorized},
		{"another site", terminalSocketPath + "?rows=24&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken, "Origin": "http://attacker.example"}, http.StatusForbidden},
		{"rebinding host", terminalSocketPath + "?rows=24&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken, "Host": "attacker.example"}, http.StatusMisdirectedRequest},
		{"through a plain HTTP proxy, with a token", terminalSocketPath + "?rows=24&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken, "X-Forwarded-For": "192.0.2.1"}, http.StatusForbidden},
		{"no size", terminalSocketPath, map[string]string{"X-Rotari-Token": terminalTestToken}, http.StatusBadRequest},
		{"no rows", terminalSocketPath + "?rows=0&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken}, http.StatusBadRequest},
		{"size too large", terminalSocketPath + "?rows=70000&cols=80", map[string]string{"X-Rotari-Token": terminalTestToken}, http.StatusBadRequest},
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

// A browser signing in reaches the shell by its session, and signing out ends it.
func TestTerminalSessionAndSignOut(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
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
	connection, _, err := websocket.Dial(ctx, terminalSocketURL(server, "rows=24&cols=80"), &websocket.DialOptions{
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
}

func TestTerminalIsAbsentWithoutTheOption(t *testing.T) {
	server := httptest.NewServer(Handler(Options{BaseDir: t.TempDir()}))
	defer server.Close()
	response, err := http.Get(server.URL + terminalSocketPath + "?rows=24&cols=80")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("status %d, want 404", response.StatusCode)
	}
}
