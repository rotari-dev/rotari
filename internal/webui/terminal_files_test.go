package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func terminalFiles(t *testing.T, server *httptest.Server, query string) terminalFilesResponse {
	t.Helper()
	status, body := terminalRequest(t, server, http.MethodGet, terminalFilesPath+"?"+query)
	var listing terminalFilesResponse
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	if err := json.Unmarshal([]byte(body), &listing); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return listing
}

func TestTerminalFilesListADirectory(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	shell := startTerminalShell(t, server)
	dir := terminalFiles(t, server, "shell="+shell.ID).Dir
	for _, sub := range []string{"beta", "Alpha", "beta/inner"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"c.txt", "a\nline", "bad\xffname", "B.csv"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range map[string]string{"to-dir": "beta", "to-file": "c.txt", "broken": "nowhere", "odd-target": "x\xfey"} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	want := []terminalFileEntry{
		{Name: "Alpha", Kind: "dir"},
		{Name: "beta", Kind: "dir"},
		{Name: "to-dir", Kind: "symlink", Target: "beta", TargetKind: "dir"},
		{Name: "a\nline", Kind: "file"},
		{Name: "B.csv", Kind: "file"},
		{Name: "bad�name", Kind: "file", NotUTF8: true},
		{Name: "broken", Kind: "symlink", Target: "nowhere", TargetKind: "missing"},
		{Name: "c.txt", Kind: "file"},
		{Name: "odd-target", Kind: "symlink", Target: "x�y", TargetNotUTF8: true, TargetKind: "missing"},
		{Name: "to-file", Kind: "symlink", Target: "c.txt", TargetKind: "file"},
	}
	// The shell's directory and the same directory by path list the same.
	for _, query := range []string{"shell=" + shell.ID, "path=" + url.QueryEscape(dir), "path=" + url.QueryEscape(dir+"/beta/..")} {
		listing := terminalFiles(t, server, query)
		if listing.Dir != dir || listing.Total != len(want) || !reflect.DeepEqual(listing.Entries, want) {
			t.Errorf("%s: %+v, want %s with %+v", query, listing, dir, want)
		}
	}
	inner := terminalFiles(t, server, "path="+url.QueryEscape(filepath.Join(dir, "to-dir")))
	if !reflect.DeepEqual(inner.Entries, []terminalFileEntry{{Name: "inner", Kind: "dir"}}) {
		t.Errorf("through a link: %+v", inner)
	}
}

func TestTerminalFilesSayHowManyTheyLeaveOut(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	dir := t.TempDir()
	for i := range terminalFilesLimit + 1 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%05d", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	listing := terminalFiles(t, server, "path="+url.QueryEscape(dir))
	if listing.Total != terminalFilesLimit+1 || len(listing.Entries) != terminalFilesLimit || listing.Entries[0].Name != "f00000" {
		t.Errorf("total %d, %d entries, first %+v", listing.Total, len(listing.Entries), listing.Entries[0])
	}
}

func TestTerminalFilesRefuseWithTheReason(t *testing.T) {
	server, keeper := terminalServer(t, "/bin/sh")
	dir := t.TempDir()
	file := filepath.Join(dir, "plain")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	ended := startTerminalShell(t, server)
	if err := keeper.End(ended.ID); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		query  string
		status int
		want   string
	}{
		"neither":                 {"", http.StatusBadRequest, "either a running shell"},
		"both":                    {"shell=1&path=/", http.StatusBadRequest, "either a running shell"},
		"relative":                {"path=tmp", http.StatusBadRequest, "not absolute"},
		"empty":                   {"path=", http.StatusBadRequest, "path is empty"},
		"not UTF-8":               {"path=/a%FF", http.StatusBadRequest, "not valid UTF-8"},
		"NUL":                     {"path=/a%00", http.StatusBadRequest, "NUL"},
		"missing":                 {"path=" + url.QueryEscape(filepath.Join(dir, "gone")), http.StatusUnprocessableEntity, "no such file"},
		"a file":                  {"path=" + url.QueryEscape(file), http.StatusUnprocessableEntity, "is not a directory"},
		"a shell gone":            {"shell=" + ended.ID, http.StatusNotFound, ""},
		"no such shell":           {"shell=nope", http.StatusNotFound, ""},
		"an unreadable directory": {"path=" + url.QueryEscape(locked), http.StatusUnprocessableEntity, "permission denied"},
	}
	if os.Geteuid() == 0 {
		delete(cases, "an unreadable directory")
	}
	for name, test := range cases {
		status, body := terminalRequest(t, server, http.MethodGet, terminalFilesPath+"?"+test.query)
		if status != test.status || !strings.Contains(body, test.want) {
			t.Errorf("%s: status %d: %s, want %d saying %q", name, status, body, test.status, test.want)
		}
	}
	if status, _ := terminalRequest(t, server, http.MethodPost, terminalFilesPath+"?path=/"); status != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d", status)
	}
}

// The Explorer's Open in a new terminal tab starts a shell in the directory it names, and
// in no other place when that directory will not do.
func TestTerminalShellStartsInANamedDirectory(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "plain")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80&dir="+url.QueryEscape(dir+"/."))
	var shell struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &shell); status != http.StatusCreated || err != nil {
		t.Fatalf("start: status %d, %v: %s", status, err, body)
	}
	if listing := terminalFiles(t, server, "shell="+shell.ID); listing.Dir != dir {
		t.Errorf("the shell is in %q, want %q", listing.Dir, dir)
	}
	for name, test := range map[string]struct{ query, want string }{
		"relative":       {"dir=tmp", "not absolute"},
		"missing":        {"dir=" + url.QueryEscape(filepath.Join(dir, "gone")), "no such file"},
		"a file":         {"dir=" + url.QueryEscape(file), "is not a directory"},
		"empty":          {"dir=", "path is empty"},
		"beside its run": {"project=p&run=r&dir=" + url.QueryEscape(dir), "not both"},
	} {
		status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80&"+test.query)
		if status != http.StatusBadRequest || !strings.Contains(body, test.want) {
			t.Errorf("%s: status %d: %s, want 400 saying %q", name, status, body, test.want)
		}
	}
	if shells := listTerminalShells(t, server); len(shells) != 1 {
		t.Errorf("refused starts left %d shells, want 1", len(shells))
	}
}

// terminalChange posts a change to files as the Explorer does.
func terminalChange(t *testing.T, server *httptest.Server, op string, body any) (int, string) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+terminalFilesPath+"/"+op, strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Rotari-Token", terminalTestToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	answer, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(answer)
}

func TestTerminalFilesChange(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	dir := t.TempDir()
	at := func(name string) string { return filepath.Join(dir, name) }
	if err := os.WriteFile(at("data.csv"), []byte("a,b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		op     string
		body   map[string]any
		status int
		want   string
	}{
		{"mkdir", map[string]any{"path": at("out")}, http.StatusOK, `{"path":"` + at("out") + `"}`},
		{"create", map[string]any{"path": at("out/notes.md")}, http.StatusOK, `"path":"` + at("out/notes.md") + `"`},
		{"move", map[string]any{"from": at("out/notes.md"), "to": at("out/readme.md")}, http.StatusOK, at("out/readme.md")},
		{"copy", map[string]any{"from": at("data.csv"), "to": at("data.csv"), "unique": true}, http.StatusOK, at("data copy.csv")},
		{"copy", map[string]any{"from": at("data.csv"), "to": at("out/data.csv")}, http.StatusOK, at("out/data.csv")},
		{"delete", map[string]any{"path": at("data copy.csv")}, http.StatusOK, at("data copy.csv")},
		// Refusals, each with its reason.
		{"mkdir", map[string]any{"path": at("out")}, http.StatusConflict, "file exists"},
		{"move", map[string]any{"from": at("data.csv"), "to": at("out/data.csv")}, http.StatusConflict, "file exists"},
		{"move", map[string]any{"from": at("out"), "to": at("out/inner")}, http.StatusBadRequest, "into itself"},
		{"move", map[string]any{"from": at("data.csv"), "to": at("data.csv")}, http.StatusConflict, "already there"},
		{"copy", map[string]any{"from": at("out"), "to": at("out2")}, http.StatusBadRequest, "cp -r"},
		{"copy", map[string]any{"from": at("data.csv"), "to": at("out/data.csv")}, http.StatusConflict, "file exists"},
		{"create", map[string]any{"path": at(`a\b`)}, http.StatusBadRequest, "holds / or"},
		{"create", map[string]any{"path": "relative"}, http.StatusBadRequest, "not absolute"},
		{"delete", map[string]any{"path": at("gone")}, http.StatusUnprocessableEntity, "no such file"},
		{"delete", map[string]any{"path": "/"}, http.StatusBadRequest, "is not removed"},
		{"delete", map[string]any{"path": at("x"), "recursive": true}, http.StatusBadRequest, "unknown field"},
		{"chmod", map[string]any{"path": at("x")}, http.StatusNotFound, ""},
	}
	for _, step := range steps {
		status, body := terminalChange(t, server, step.op, step.body)
		if status != step.status || !strings.Contains(body, step.want) {
			t.Errorf("%s %v: status %d: %s, want %d saying %q", step.op, step.body, status, body, step.status, step.want)
		}
	}
	listing := terminalFiles(t, server, "path="+url.QueryEscape(dir))
	var names []string
	for _, entry := range listing.Entries {
		names = append(names, entry.Name)
	}
	if strings.Join(names, ",") != "out,data.csv" {
		t.Errorf("left %v", names)
	}
	inner := terminalFiles(t, server, "path="+url.QueryEscape(at("out")))
	if len(inner.Entries) != 2 || inner.Entries[0].Name != "data.csv" || inner.Entries[1].Name != "readme.md" {
		t.Errorf("out holds %+v", inner.Entries)
	}
	// JSON only, POST only.
	request, err := http.NewRequest(http.MethodPost, server.URL+terminalFilesPath+"/mkdir", strings.NewReader("path="+at("form")))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Rotari-Token", terminalTestToken)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("a form: status %d", response.StatusCode)
	}
	if status, _ := terminalRequest(t, server, http.MethodGet, terminalFilesPath+"/mkdir"); status != http.StatusMethodNotAllowed {
		t.Errorf("GET: status %d", status)
	}
}
