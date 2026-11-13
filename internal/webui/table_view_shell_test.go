package webui

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/terminal"
)

func TestTableViewOnAShellsDirectory(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "scores.csv"), []byte("name,score\nalpha,1\nbeta,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	options := testOptions(t.TempDir(), true)
	keeper, err := terminal.NewKeeper(terminal.KeeperOptions{
		Shell:     terminal.Options{Shell: "/bin/sh", Dir: dir, Env: []string{"HOME=" + dir, "PATH=" + os.Getenv("PATH"), "PS1=$ "}},
		IdleLimit: time.Hour, RingSize: 1 << 16, ShellLimit: 3, GroupLimit: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	shell, err := keeper.Start(terminal.StartOptions{Rows: 24, Cols: 80, Group: "/b\x00p", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	options.Terminal = keeper
	handler := Handler(options)

	issued := serveJSON(t, handler, http.MethodPost, "/api/tableview-sources", `{"shell":"`+shell.ID+`"}`)
	source := decodeBody[shellTableViewSource](t, issued)
	if issued.Code != http.StatusOK || source.Source == "" || source.Dir != dir {
		t.Fatalf("issue = %d %s", issued.Code, issued.Body.String())
	}
	// The directory is pinned: the shell ending does not move or end the view.
	if err := keeper.End(shell.ID); err != nil {
		t.Fatal(err)
	}
	info := serveJSON(t, handler, http.MethodGet, "/tableview/api/info?source="+source.Source, "")
	shown := decodeBody[tableViewInfo](t, info)
	if info.Code != http.StatusOK || shown.Kind != "directory" || shown.Name != dir || strings.Join(shown.Trail, "|") != "p|shell 1" {
		t.Fatalf("info = %d %s", info.Code, info.Body.String())
	}
	if page := serveJSON(t, handler, http.MethodGet, "/tableview/?source="+source.Source, ""); page.Code != http.StatusOK {
		t.Fatalf("page = %d %s", page.Code, page.Body.String())
	}
	rows := serveJSON(t, handler, http.MethodGet, "/tableview/api/rows?source="+source.Source+"&child=sub/scores.csv&sep=,&header=1", "")
	if rows.Code != http.StatusOK || !strings.Contains(rows.Body.String(), "alpha") {
		t.Fatalf("child rows = %d %s", rows.Code, rows.Body.String())
	}

	for name, test := range map[string]struct {
		target string
		status int
		want   string
	}{
		"an unknown source":        {"/tableview/api/info?source=nope", http.StatusNotFound, "no longer held"},
		"an empty source":          {"/tableview/api/info?source=", http.StatusBadRequest, "source is empty"},
		"an artifact as well":      {"/tableview/api/info?source=" + source.Source + "&project_name=p", http.StatusBadRequest, "takes no project_name"},
		"a child leaving the root": {"/tableview/api/info?source=" + source.Source + "&child=../x", http.StatusBadRequest, ""},
	} {
		got := serveJSON(t, handler, http.MethodGet, test.target, "")
		if got.Code != test.status || !strings.Contains(got.Body.String(), test.want) {
			t.Errorf("%s: %d %s, want %d saying %q", name, got.Code, got.Body.String(), test.status, test.want)
		}
	}
	for name, test := range map[string]struct {
		body   string
		status int
		want   string
	}{
		"a shell not running": {`{"shell":"` + shell.ID + `"}`, http.StatusBadRequest, "is not running"},
		"no shell":            {`{}`, http.StatusBadRequest, "shell is required"},
		"a path":              {`{"shell":"1","dir":"/"}`, http.StatusBadRequest, "names only a running shell"},
	} {
		got := serveJSON(t, handler, http.MethodPost, "/api/tableview-sources", test.body)
		if got.Code != test.status || !strings.Contains(got.Body.String(), test.want) {
			t.Errorf("%s: %d %s, want %d saying %q", name, got.Code, got.Body.String(), test.status, test.want)
		}
	}
	if got := serveJSON(t, handler, http.MethodGet, "/api/tableview-sources", ""); got.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", got.Code)
	}
	options.Terminal = nil
	if got := serveJSON(t, Handler(options), http.MethodPost, "/api/tableview-sources", `{"shell":"1"}`); got.Code != http.StatusForbidden || !strings.Contains(got.Body.String(), "--allow-terminal") {
		t.Errorf("without the terminal: %d %s", got.Code, got.Body.String())
	}
}

func TestShellTableViewsDropTheLeastRecentlyUsed(t *testing.T) {
	views := newShellTableViews()
	first := views.add(tableViewSource{name: "first"})
	second := views.add(tableViewSource{name: "second"})
	for range maxShellTableViews - 2 {
		views.add(tableViewSource{})
	}
	if _, err := views.get(first); err != nil {
		t.Fatal(err)
	}
	views.add(tableViewSource{})
	if _, err := views.get(second); err != errShellTableViewGone {
		t.Errorf("second: %v, want it dropped", err)
	}
	if got, err := views.get(first); err != nil || got.name != "first" {
		t.Errorf("first: %v %v, want it kept as recently used", got, err)
	}
}
