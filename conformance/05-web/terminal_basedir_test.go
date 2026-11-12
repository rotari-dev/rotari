package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

// A basedir's own pages have shells of their own, apart from its projects' and those of no
// project. One starts in the workspace the basedir belongs to, which is its parent when the
// basedir is named .rotari-state or is the one the parent's .rotari.toml names, and at home
// otherwise; it knows its basedir and no project, whatever project rotari web was given.
func TestTerminalBasedirShellsStartInTheirWorkspace(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	home := filepath.Join(e.Root, "shell-home")
	initialized := filepath.Join(e.Root, "initialized")
	configured := filepath.Join(e.Root, "configured")
	elsewhere := filepath.Join(e.Root, "elsewhere")
	gone := filepath.Join(e.Root, "gone")
	for _, dir := range []string{home, initialized, configured, elsewhere, gone} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	e.Setenv("HOME", home)
	writeFile := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// configured's workspace names its basedir; elsewhere's names another one.
	writeFile(filepath.Join(configured, ".rotari.toml"), "basedir = \"records\"\n")
	writeFile(filepath.Join(elsewhere, ".rotari.toml"), "basedir = \"other\"\n")
	bases := map[string]string{
		"initialized": filepath.Join(initialized, ".rotari-state"),
		"configured":  filepath.Join(configured, "records"),
		"elsewhere":   filepath.Join(elsewhere, "state"),
		"gone":        filepath.Join(gone, ".rotari-state"),
	}
	for _, baseDir := range bases {
		e.MustRotari("add", "-b", baseDir, "-p", "p", "--", "true")
	}
	e.MustRotari("add", "-p", "p", "--", "true")
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	idOf := func(baseDir string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(filepath.Clean(baseDir)))
	}

	e.Setenv("ROTARI_PROJECT_NAME", "from-the-server")
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

	project := start("&project=p")
	for _, test := range []struct {
		name, query, baseDir, link, dir string
		// says is what the shell's first line says, if it says why it opened at home.
		says string
	}{
		{"the server's basedir", "&basedir=", e.Base, "/", home, ""},
		{"a basedir of rotari init", "&basedir=" + idOf(bases["initialized"]), bases["initialized"], "/_basedir/" + idOf(bases["initialized"]) + "/", initialized, ""},
		{"a basedir its workspace names", "&basedir=" + idOf(bases["configured"]), bases["configured"], "/_basedir/" + idOf(bases["configured"]) + "/", configured, ""},
		{"a basedir its parent does not name", "&basedir=" + idOf(bases["elsewhere"]), bases["elsewhere"], "/_basedir/" + idOf(bases["elsewhere"]) + "/", home, ""},
		{"a basedir whose workspace is gone", "&basedir=" + idOf(bases["gone"]), bases["gone"], "/_basedir/" + idOf(bases["gone"]) + "/", home, strconv.Quote(gone)},
	} {
		shell := start(test.query)
		if shell.Number != 1 || shell.Project != "" || shell.Link != test.link {
			t.Errorf("%s: %+v, want shell 1 of no project linking to %s", test.name, shell, test.link)
		}
		page := open(shell)
		if test.says != "" {
			page.await("so this shell opened in your home directory.")
			if !strings.Contains(page.shown.String(), test.says) {
				t.Errorf("%s: the shell was told %q, want %s", test.name, page.shown.String(), test.says)
			}
		}
		page.typeLine(`echo "in-$(pwd -P) base-$ROTARI_BASEDIR project-[$ROTARI_PROJECT_NAME]"`)
		page.await("in-" + real(test.dir) + " base-" + test.baseDir + " project-[]")
		if test.says == "" && strings.Contains(page.shown.String(), "rotari:") {
			t.Errorf("%s: the shell said why it opened where it should: %q", test.name, page.shown.String())
		}
	}
	// The server's basedir has its project's shell, its own, and one of no project, each the first of its group.
	if plain := start(""); project.Number != 1 || plain.Number != 1 || plain.Link != "/activity" {
		t.Errorf("the project's shell %+v and the one of no project %+v", project, plain)
	}

	for name, test := range map[string]struct {
		query string
		want  int
	}{
		"a run of a basedir":           {"&basedir=&run=r", http.StatusBadRequest},
		"an unknown basedir":           {"&basedir=nope", http.StatusNotFound},
		"an unknown basedir's project": {"&project=p&basedir=nope", http.StatusNotFound},
	} {
		status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80"+test.query, token)
		if status != test.want {
			t.Errorf("%s: status %d, want %d: %s", name, status, test.want, body)
		}
	}
}
