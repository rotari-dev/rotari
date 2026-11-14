package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	covers(t, "WEB-42", "WEB-34")
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
	files := map[string]string{
		filepath.Join(dir, "scores.csv"):     "name,score\nalpha,1\n",
		filepath.Join(dir, "two\nlines"):     "",
		filepath.Join(elsewhere, "far.csv"):  "name,score\nfaraway,2\n",
		filepath.Join(dir, "conf", "a.yaml"): "",
	}
	rawNames := support.NonUTF8NamesAllowed(t, dir)
	if rawNames {
		files[filepath.Join(dir, "raw\xffname")] = ""
	}
	for path, content := range files {
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
	if !rawNames {
		want.Total = 4
		want.Entries = slices.DeleteFunc(want.Entries, func(entry explorerEntry) bool { return entry.NotUTF8 })
	}
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
	// Open in a new terminal tab: a shell starts in the directory named, or not at all.
	status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80&dir="+url.QueryEscape(filepath.Join(real, "conf")), headers)
	var started struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &started); status != http.StatusCreated || err != nil {
		t.Fatalf("a shell in a directory: status %d, %v: %s", status, err, body)
	}
	if status, body := list("shell=" + url.QueryEscape(started.ID)); status != http.StatusOK || !strings.Contains(body, `"dir":"`+filepath.Join(real, "conf")+`"`) {
		t.Errorf("the new shell's directory: status %d: %s", status, body)
	}
	for name, dir := range map[string]string{"relative": "conf", "missing": filepath.Join(real, "gone"), "a file": filepath.Join(real, "scores.csv")} {
		if status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80&dir="+url.QueryEscape(dir), headers); status != http.StatusBadRequest {
			t.Errorf("a shell in %s: status %d: %s, want 400", name, status, body)
		}
	}
	if shells := listTerminalShells(t, base, headers); len(shells) != 2 {
		t.Errorf("refused starts: %d shells, want 2", len(shells))
	}

	plain := support.NewEnv(t).StartWeb()
	if got := e.HTTPDo(http.MethodGet, plain+"/_terminal/files?path=/", nil, ""); got.Status != http.StatusNotFound {
		t.Errorf("files without the terminal: (%d, %s), want 404", got.Status, got.Body)
	}
	if got := e.HTTPDo(http.MethodPost, plain+"/api/tableview-sources", nil, `{"path":"/"}`); got.Status != http.StatusForbidden || !strings.Contains(got.Body, "--allow-terminal") {
		t.Errorf("a path without the terminal: (%d, %s)", got.Status, got.Body)
	}
}

// The Explorer changes files as a terminal's file pane does, never replacing what exists,
// and only where the terminal is served.
func TestTerminalExplorerChangesFiles(t *testing.T) {
	covers(t, "WEB-43")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	dir := filepath.Join(e.Root, "changed here")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.csv"), []byte("a,b\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	at := func(name string) string { return filepath.Join(dir, name) }
	change := func(op, body string, headers map[string]string) support.HTTPResult {
		all := map[string]string{"X-Rotari-Token": launchToken, "Content-Type": "application/json"}
		for key, value := range headers {
			all[key] = value
		}
		return e.HTTPDo(http.MethodPost, base+"/_terminal/files/"+op, all, body)
	}
	quote := func(value string) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	for _, step := range []struct {
		op, body string
		status   int
		want     string
	}{
		{"mkdir", `{"path":` + quote(at("out")) + `}`, http.StatusOK, `"path":` + quote(at("out"))},
		{"create", `{"path":` + quote(at("out/notes.md")) + `}`, http.StatusOK, at("out/notes.md")},
		{"move", `{"from":` + quote(at("out/notes.md")) + `,"to":` + quote(at("readme.md")) + `}`, http.StatusOK, at("readme.md")},
		{"copy", `{"from":` + quote(at("data.csv")) + `,"to":` + quote(at("data.csv")) + `,"unique":true}`, http.StatusOK, at("data copy.csv")},
		{"copy", `{"from":` + quote(at("data.csv")) + `,"to":` + quote(at("out/data.csv")) + `}`, http.StatusOK, at("out/data.csv")},
		{"delete", `{"path":` + quote(at("readme.md")) + `}`, http.StatusOK, at("readme.md")},
		{"move", `{"from":` + quote(at("data.csv")) + `,"to":` + quote(at("out/data.csv")) + `}`, http.StatusConflict, "exists"},
		{"create", `{"path":` + quote(at("data copy.csv")) + `}`, http.StatusConflict, "exists"},
		{"move", `{"from":` + quote(at("out")) + `,"to":` + quote(at("out/in")) + `}`, http.StatusBadRequest, "into itself"},
		{"copy", `{"from":` + quote(at("out")) + `,"to":` + quote(at("out2")) + `}`, http.StatusBadRequest, "cp -r"},
		{"mkdir", `{"path":` + quote(at(`a\b`)) + `}`, http.StatusBadRequest, ""},
		{"delete", `{"path":"/"}`, http.StatusBadRequest, ""},
		{"delete", `{"path":` + quote(at("gone")) + `}`, http.StatusUnprocessableEntity, "no such file"},
	} {
		if got := change(step.op, step.body, nil); got.Status != step.status || !strings.Contains(got.Body, step.want) {
			t.Errorf("%s %s: (%d, %s), want %d saying %q", step.op, step.body, got.Status, got.Body, step.status, step.want)
		}
	}
	for path, want := range map[string]string{"data.csv": "a,b\n", "data copy.csv": "a,b\n", "out/data.csv": "a,b\n"} {
		content, err := os.ReadFile(at(path))
		if err != nil || string(content) != want {
			t.Errorf("%s: %q %v", path, content, err)
		}
	}
	// The source's permission bits, less what the umask takes, and never more.
	if info, err := os.Stat(at("out/data.csv")); err != nil || info.Mode().Perm()&^0o640 != 0 {
		t.Errorf("the copy's permissions: %v %v", info, err)
	}
	for _, gone := range []string{"readme.md", "out/notes.md", "out2"} {
		if _, err := os.Lstat(at(gone)); !os.IsNotExist(err) {
			t.Errorf("%s is there: %v", gone, err)
		}
	}
	// A JSON body, the terminal's connection checks, and the terminal itself.
	if got := change("mkdir", `{"path":`+quote(at("form"))+`}`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}); got.Status != http.StatusUnsupportedMediaType {
		t.Errorf("a form: (%d, %s)", got.Status, got.Body)
	}
	if got := change("mkdir", `{"path":`+quote(at("proxied"))+`}`, map[string]string{"X-Forwarded-For": "192.0.2.1"}); got.Status != http.StatusForbidden {
		t.Errorf("through a plain proxy: (%d, %s)", got.Status, got.Body)
	}
	if got := change("mkdir", `{"path":`+quote(at("elsewhere"))+`}`, map[string]string{"Origin": "http://attacker.example"}); got.Status != http.StatusForbidden {
		t.Errorf("from another site: (%d, %s)", got.Status, got.Body)
	}
	plain := support.NewEnv(t).StartWeb()
	if got := e.HTTPDo(http.MethodPost, plain+"/_terminal/files/mkdir", map[string]string{"Content-Type": "application/json"}, `{"path":`+quote(at("x"))+`}`); got.Status != http.StatusNotFound {
		t.Errorf("without the terminal: (%d, %s)", got.Status, got.Body)
	}
	for _, refused := range []string{"form", "proxied", "elsewhere", "x"} {
		if _, err := os.Lstat(at(refused)); !os.IsNotExist(err) {
			t.Errorf("a refused request made %s", refused)
		}
	}
}

// The Explorer downloads a file, or a folder as a zip, and uploads files without
// replacing any, only where the terminal is served.
func TestTerminalExplorerDownloadsAndUploads(t *testing.T) {
	covers(t, "WEB-44")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	dir := filepath.Join(e.Root, "results")
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scores.csv"), []byte("a,b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(e.Root, "elsewhere"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	get := func(path string) (*http.Response, []byte) {
		request, err := http.NewRequest(http.MethodGet, base+"/_terminal/files/download?path="+url.QueryEscape(path), nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Rotari-Token", launchToken)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response, body
	}
	response, body := get(filepath.Join(dir, "scores.csv"))
	if response.StatusCode != http.StatusOK || string(body) != "a,b\n" || response.Header.Get("Content-Disposition") != "attachment; filename=scores.csv" {
		t.Errorf("a file: %d %q %v", response.StatusCode, body, response.Header)
	}
	response, body = get(dir)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/zip" || response.Header.Get("Content-Disposition") != "attachment; filename=results.zip" {
		t.Fatalf("a folder: %d %v", response.StatusCode, response.Header)
	}
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	if strings.Join(names, "|") != "results/|results/empty/|results/scores.csv" {
		t.Errorf("zip entries %q, want the folders and the file without the link", names)
	}
	upload := func(name, contentType, content string, headers map[string]string) support.HTTPResult {
		all := map[string]string{"X-Rotari-Token": launchToken, "Content-Type": contentType}
		for key, value := range headers {
			all[key] = value
		}
		return e.HTTPDo(http.MethodPost, base+"/_terminal/files/upload?dir="+url.QueryEscape(dir)+"&name="+url.QueryEscape(name), all, content)
	}
	if got := upload("new.txt", "application/octet-stream", "hello", nil); got.Status != http.StatusOK || !strings.Contains(got.Body, filepath.Join(dir, "new.txt")) {
		t.Errorf("an upload: (%d, %s)", got.Status, got.Body)
	}
	for name, test := range map[string]struct {
		file, contentType string
		headers           map[string]string
		status            int
	}{
		"a name taken":          {"new.txt", "application/octet-stream", nil, http.StatusConflict},
		"a bad name":            {"a/b", "application/octet-stream", nil, http.StatusBadRequest},
		"a form":                {"form.txt", "application/x-www-form-urlencoded", nil, http.StatusUnsupportedMediaType},
		"through a plain proxy": {"proxied.txt", "application/octet-stream", map[string]string{"X-Forwarded-For": "192.0.2.1"}, http.StatusForbidden},
		"from another site":     {"elsewhere.txt", "application/octet-stream", map[string]string{"Origin": "http://attacker.example"}, http.StatusForbidden},
	} {
		if got := upload(test.file, test.contentType, "other", test.headers); got.Status != test.status {
			t.Errorf("%s: (%d, %s), want %d", name, got.Status, got.Body, test.status)
		}
	}
	if content, err := os.ReadFile(filepath.Join(dir, "new.txt")); err != nil || string(content) != "hello" {
		t.Errorf("the upload after refusals: %q %v", content, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, entry := range entries {
		left = append(left, entry.Name())
	}
	if strings.Join(left, ",") != "empty,link,new.txt,scores.csv" {
		t.Errorf("left %v, want no refused or temporary files", left)
	}
	plain := support.NewEnv(t).StartWeb()
	if got := e.HTTPDo(http.MethodGet, plain+"/_terminal/files/download?path="+url.QueryEscape(dir), nil, ""); got.Status != http.StatusNotFound {
		t.Errorf("a download without the terminal: (%d, %s)", got.Status, got.Body)
	}
}
