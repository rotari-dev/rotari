package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/terminal"
)

// movingTerminalServer is terminalServer with bash and rotari's integration, so that its
// shells follow moves, one shell allowed per group, and projects p1, p2, and p3.
func movingTerminalServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))
	baseDir := t.TempDir()
	for _, project := range []string{"p1", "p2", "p3"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "projects", project), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	keeper, err := terminal.NewKeeper(terminal.KeeperOptions{
		Shell:     terminal.Options{Shell: bash, Dir: home, Env: []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "PS1=$ "}},
		IdleLimit: time.Hour, RingSize: 1 << 16, ShellLimit: terminalTestShellLimit, GroupLimit: 1, ShellIntegration: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	server := httptest.NewServer(WithExecGuard(WithAuthToken(Handler(Options{BaseDir: baseDir, Terminal: keeper}), terminalTestToken, AuthOptions{}), nil))
	t.Cleanup(server.Close)
	return server, baseDir
}

// A shell of a project or of a basedir's own moves to another project, or to the basedir's
// own shells, named as a new shell's place is; the list shows it there, movable. What
// cannot be is refused, and nothing moves.
func TestTerminalMovesAShell(t *testing.T) {
	server, baseDir := movingTerminalServer(t)
	start := func(query string) terminalShellResponse {
		t.Helper()
		status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80"+query)
		if status != http.StatusCreated {
			t.Fatalf("starting a shell with %q: status %d: %s", query, status, body)
		}
		var shell terminalShellResponse
		if err := json.Unmarshal([]byte(body), &shell); err != nil {
			t.Fatal(err)
		}
		return shell
	}
	move := func(id, query string) (int, string) {
		t.Helper()
		return terminalRequest(t, server, http.MethodPost, terminalShellsPath+"/"+id+"/"+terminalPlaceName+query)
	}
	listed := func(id string) terminalShellResponse {
		t.Helper()
		status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath)
		var list struct {
			Shells []terminalShellResponse `json:"shells"`
		}
		if err := json.Unmarshal([]byte(body), &list); status != http.StatusOK || err != nil {
			t.Fatalf("listing: status %d, %v: %s", status, err, body)
		}
		for _, shell := range list.Shells {
			if shell.ID == id {
				return shell
			}
		}
		t.Fatalf("shell %s is not listed: %s", id, body)
		return terminalShellResponse{}
	}
	p1, p3, none := start("&project=p1"), start("&project=p3"), start("")
	if !p1.Movable || none.Movable {
		t.Errorf("movable: a project's %v, no project's %v", p1.Movable, none.Movable)
	}
	status, body := move(p1.ID, "?project=p2")
	var moved terminalShellResponse
	if err := json.Unmarshal([]byte(body), &moved); status != http.StatusOK || err != nil || moved.Project != "p2" || moved.Number != 1 || moved.Link != "/project/p2" || !moved.Movable {
		t.Errorf("moving to p2: status %d: %s", status, body)
	}
	if got := listed(p1.ID); got.Project != "p2" || got.BaseDir != baseDir {
		t.Errorf("listed after the move: %+v", got)
	}
	if status, body := move(p1.ID, "?basedir="); status != http.StatusOK || listed(p1.ID).Project != "" || listed(p1.ID).BaseDir != baseDir {
		t.Errorf("moving to the basedir's own: status %d: %s", status, body)
	}
	for name, test := range map[string]struct {
		id, query string
		want      int
	}{
		"no place":         {p1.ID, "", http.StatusBadRequest},
		"a run":            {p1.ID, "?project=p2&run=x", http.StatusBadRequest},
		"a directory":      {p1.ID, "?project=p2&dir=/", http.StatusBadRequest},
		"a bad project":    {p1.ID, "?project=a/b", http.StatusBadRequest},
		"no such project":  {p1.ID, "?project=p9", http.StatusNotFound},
		"no such basedir":  {p1.ID, "?basedir=zz&project=p2", http.StatusNotFound},
		"no such shell":    {"99", "?project=p2", http.StatusNotFound},
		"no project's":     {none.ID, "?project=p2", http.StatusConflict},
		"a full project":   {p1.ID, "?project=p3", http.StatusConflict},
		"its own is fine":  {p3.ID, "?project=p3", http.StatusOK},
		"back to its last": {p1.ID, "?project=p1", http.StatusOK},
	} {
		status, body := move(test.id, test.query)
		if status != test.want {
			t.Errorf("%s: status %d, want %d: %s", name, status, test.want, body)
		}
	}
	if got := listed(none.ID); got.BaseDir != "" || got.Project != "" {
		t.Errorf("a refused move moved the shell of no project: %+v", got)
	}
	if status, body := move(p3.ID, "?basedir="); status != http.StatusOK {
		t.Errorf("into the basedir's own, now free: status %d: %s", status, body)
	}
	if status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath+"/"+p1.ID+"/"+terminalPlaceName+"?project=p2"); status != http.StatusMethodNotAllowed {
		t.Errorf("GET: status %d: %s", status, body)
	}
}

// A shell that would not take its new place's variables does not move.
func TestTerminalDoesNotMoveAShellWithoutTheIntegration(t *testing.T) {
	baseDir := t.TempDir()
	for _, project := range []string{"p1", "p2"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "projects", project), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	server, _ := terminalServerAt(t, "/bin/sh", baseDir)
	status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80&project=p1")
	var shell terminalShellResponse
	if err := json.Unmarshal([]byte(body), &shell); status != http.StatusCreated || err != nil || shell.Movable {
		t.Fatalf("starting sh: status %d: %s", status, body)
	}
	status, body = terminalRequest(t, server, http.MethodPost, terminalShellsPath+"/"+shell.ID+"/"+terminalPlaceName+"?project=p2")
	if status != http.StatusConflict || !strings.Contains(body, "integration") {
		t.Errorf("moving sh: status %d: %s", status, body)
	}
}
