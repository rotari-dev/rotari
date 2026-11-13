package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

type explorerEntry struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	NotUTF8    bool   `json:"not_utf8"`
	Target     string `json:"target"`
	TargetKind string `json:"target_kind"`
}

type explorerListing struct {
	Dir     string          `json:"dir"`
	Entries []explorerEntry `json:"entries"`
	Total   int             `json:"total"`
}

// The terminal's Explorer lists where a shell is now, or any directory, and opens a file
// in the Table viewer, only where the terminal is served.
func TestTerminalExplorerListsAndOpensFiles(t *testing.T) {
	covers(t, "WEB-42")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	dir := filepath.Join(e.Root, "explored here")
	elsewhere := filepath.Join(e.Root, "elsewhere")
	for _, sub := range []string{filepath.Join(dir, "conf"), elsewhere} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(dir, "scores.csv"):     "name,score\nalpha,1\n",
		filepath.Join(dir, "two\nlines"):     "",
		filepath.Join(dir, "raw\xffname"):    "",
		filepath.Join(elsewhere, "far.csv"):  "name,score\nfaraway,2\n",
		filepath.Join(dir, "conf", "a.yaml"): "",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(elsewhere, "far.csv"), filepath.Join(dir, "far.csv")); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	realElsewhere, err := filepath.EvalSymlinks(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	page := openTerminalPage(t, base)
	page.typeLine(`cd '` + dir + `' && echo "moved-$((1+1))"`)
	page.await("moved-2")
	shell := listTerminalShells(t, base, headers)[0].ID
	list := func(query string) (int, string) {
		status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/files?"+query, headers)
		return status, string(body)
	}
	want := explorerListing{Dir: real, Total: 5, Entries: []explorerEntry{
		{Name: "conf", Kind: "dir"},
		{Name: "far.csv", Kind: "symlink", Target: filepath.Join(elsewhere, "far.csv"), TargetKind: "file"},
		{Name: "raw�name", Kind: "file", NotUTF8: true},
		{Name: "scores.csv", Kind: "file"},
		{Name: "two\nlines", Kind: "file"},
	}}
	for _, query := range []string{"shell=" + url.QueryEscape(shell), "path=" + url.QueryEscape(real)} {
		status, body := list(query)
		var listing explorerListing
		if err := json.Unmarshal([]byte(body), &listing); status != http.StatusOK || err != nil || !reflect.DeepEqual(listing, want) {
			t.Errorf("%s: status %d, %v: %s\nwant %+v", query, status, err, body, want)
		}
	}
	for name, test := range map[string]struct {
		query  string
		status int
		want   string
	}{
		"a relative path":     {"path=conf", http.StatusBadRequest, "not absolute"},
		"a file":              {"path=" + url.QueryEscape(filepath.Join(real, "scores.csv")), http.StatusUnprocessableEntity, "not a directory"},
		"a missing directory": {"path=" + url.QueryEscape(filepath.Join(real, "gone")), http.StatusUnprocessableEntity, "no such file"},
		"a shell not running": {"shell=77", http.StatusNotFound, ""},
	} {
		if status, body := list(test.query); status != test.status || !strings.Contains(body, test.want) {
			t.Errorf("%s: status %d: %s, want %d saying %q", name, status, body, test.status, test.want)
		}
	}
	// Only where the terminal is served: behind its token and its connection checks.
	if status, _ := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/files?path=/", http.Header{"X-Rotari-Token": {launchToken}, "X-Forwarded-For": {"192.0.2.1"}}); status != http.StatusForbidden {
		t.Errorf("through a plain HTTP proxy: status %d, want 403", status)
	}
	if status, _ := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/files?path=/", http.Header{"X-Rotari-Token": {"wrong"}}); status != http.StatusUnauthorized {
		t.Errorf("with a wrong token: status %d, want 401", status)
	}

	// A file opens in the Table viewer pinned in its directory, following a link first.
	for file, expect := range map[string]struct{ dir, cell string }{"scores.csv": {real, "alpha"}, "far.csv": {realElsewhere, "faraway"}} {
		issued := api(e, http.MethodPost, base+"/api/tableview-sources", `{"path":"`+filepath.Join(real, file)+`"}`)
		var source struct{ Source, Dir, Child string }
		if err := json.Unmarshal([]byte(issued.Body), &source); err != nil || issued.Status != http.StatusOK || source.Dir != expect.dir || source.Child != filepath.Base(file) {
			t.Errorf("%s: (%d, %s) %v", file, issued.Status, issued.Body, err)
			continue
		}
		rows := api(e, http.MethodGet, base+"/tableview/api/rows?source="+url.QueryEscape(source.Source)+"&child="+url.QueryEscape(source.Child)+"&sep=,&header=1", "")
		if rows.Status != http.StatusOK || !strings.Contains(rows.Body, expect.cell) {
			t.Errorf("%s rows: (%d, %s)", file, rows.Status, rows.Body)
		}
	}
	folder := api(e, http.MethodPost, base+"/api/tableview-sources", `{"path":"`+filepath.Join(real, "conf")+`"}`)
	if folder.Status != http.StatusOK || !strings.Contains(folder.Body, `"dir":"`+filepath.Join(real, "conf")+`"`) || strings.Contains(folder.Body, `"child"`) {
		t.Errorf("a folder: (%d, %s)", folder.Status, folder.Body)
	}
	for name, test := range map[string]struct {
		body    string
		headers map[string]string
		status  int
	}{
		"a relative path":       {`{"path":"conf"}`, nil, http.StatusBadRequest},
		"a missing path":        {`{"path":"` + filepath.Join(real, "gone") + `"}`, nil, http.StatusBadRequest},
		"a shell and a path":    {`{"shell":"` + shell + `","path":"` + real + `"}`, nil, http.StatusBadRequest},
		"through a plain proxy": {`{"path":"` + real + `"}`, map[string]string{"X-Forwarded-For": "192.0.2.1"}, http.StatusForbidden},
	} {
		headers := map[string]string{"X-Rotari-Token": launchToken}
		for key, value := range test.headers {
			headers[key] = value
		}
		if got := e.HTTPDo(http.MethodPost, base+"/api/tableview-sources", headers, test.body); got.Status != test.status {
			t.Errorf("%s: (%d, %s), want %d", name, got.Status, got.Body, test.status)
		}
	}
	plain := support.NewEnv(t).StartWeb()
	if got := e.HTTPDo(http.MethodGet, plain+"/_terminal/files?path=/", nil, ""); got.Status != http.StatusNotFound {
		t.Errorf("files without the terminal: (%d, %s), want 404", got.Status, got.Body)
	}
	if got := e.HTTPDo(http.MethodPost, plain+"/api/tableview-sources", nil, `{"path":"/"}`); got.Status != http.StatusForbidden || !strings.Contains(got.Body, "--allow-terminal") {
		t.Errorf("a path without the terminal: (%d, %s)", got.Status, got.Body)
	}
}
