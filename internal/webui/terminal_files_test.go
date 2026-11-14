package webui

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
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
	rawNames := nonUTF8NamesAllowed(t, dir)
	files := []string{"c.txt", "a\nline", "B.csv"}
	links := map[string]string{"to-dir": "beta", "to-file": "c.txt", "broken": "nowhere"}
	if rawNames {
		files = append(files, "bad\xffname")
		links["odd-target"] = "x\xfey"
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range links {
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
	if !rawNames {
		want = slices.DeleteFunc(want, func(entry terminalFileEntry) bool { return entry.NotUTF8 || entry.TargetNotUTF8 })
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

// terminalTransfer makes a download or upload request with the token.
func terminalTransfer(t *testing.T, server *httptest.Server, method, target, contentType, body string) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, server.URL+target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Rotari-Token", terminalTestToken)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, []byte(err.Error())
	}
	defer response.Body.Close()
	answer, err := io.ReadAll(response.Body)
	if err != nil {
		return response, []byte("read: " + err.Error())
	}
	return response, answer
}

func TestTerminalFilesDownloadAndUpload(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	dir := t.TempDir()
	at := func(name string) string { return filepath.Join(dir, name) }
	if err := os.MkdirAll(at("results/empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at("results/scores é.csv"), []byte("a,b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A file comes as it is, named, with ranges.
	response, body := terminalTransfer(t, server, http.MethodGet, terminalFilesPath+"/download?path="+url.QueryEscape(at("results/scores é.csv")), "", "")
	if response.StatusCode != http.StatusOK || string(body) != "a,b\n" || response.Header.Get("Content-Type") != "application/octet-stream" || !strings.Contains(response.Header.Get("Content-Disposition"), "attachment") || !strings.Contains(response.Header.Get("Content-Disposition"), "scores%20%C3%A9.csv") {
		t.Errorf("a file: %d %q %v", response.StatusCode, body, response.Header)
	}
	request, err := http.NewRequest(http.MethodGet, server.URL+terminalFilesPath+"/download?path="+url.QueryEscape(at("results/scores é.csv")), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Rotari-Token", terminalTestToken)
	request.Header.Set("Range", "bytes=2-")
	if ranged, err := http.DefaultClient.Do(request); err != nil || ranged.StatusCode != http.StatusPartialContent {
		t.Errorf("a range: %v %v", ranged, err)
	} else {
		ranged.Body.Close()
	}
	// A folder comes as a zip under its name.
	response, body = terminalTransfer(t, server, http.MethodGet, terminalFilesPath+"/download?path="+url.QueryEscape(at("results")), "", "")
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/zip" || !strings.Contains(response.Header.Get("Content-Disposition"), `filename=results.zip`) {
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
	if strings.Join(names, "|") != "results/|results/empty/|results/scores é.csv" {
		t.Errorf("zip entries %q", names)
	}
	for name, test := range map[string]struct {
		query  string
		status int
	}{
		"relative": {"path=results", http.StatusBadRequest},
		"missing":  {"path=" + url.QueryEscape(at("gone")), http.StatusUnprocessableEntity},
		"a device": {"path=/dev/null", http.StatusBadRequest},
	} {
		if response, body := terminalTransfer(t, server, http.MethodGet, terminalFilesPath+"/download?"+test.query, "", ""); response.StatusCode != test.status {
			t.Errorf("%s: %d %s, want %d", name, response.StatusCode, body, test.status)
		}
	}
	// A folder that cannot be read whole aborts the connection, after the status.
	if os.Geteuid() != 0 {
		if err := os.WriteFile(at("results/locked"), []byte("x"), 0); err != nil {
			t.Fatal(err)
		}
		response, body := terminalTransfer(t, server, http.MethodGet, terminalFilesPath+"/download?path="+url.QueryEscape(at("results")), "", "")
		if response != nil && response.StatusCode == http.StatusOK && !strings.HasPrefix(string(body), "read: ") {
			t.Errorf("an unreadable file: the download completed with %d bytes", len(body))
		}
		if err := os.Remove(at("results/locked")); err != nil {
			t.Fatal(err)
		}
	}
	// An upload writes a new file and replaces nothing.
	upload := func(query, contentType, content string) (int, string) {
		response, body := terminalTransfer(t, server, http.MethodPost, terminalFilesPath+"/upload?"+query, contentType, content)
		return response.StatusCode, string(body)
	}
	if status, body := upload("dir="+url.QueryEscape(at("results"))+"&name="+url.QueryEscape("new é.txt"), "application/octet-stream", "hello"); status != http.StatusOK || !strings.Contains(body, at("results/new é.txt")) {
		t.Errorf("an upload: %d %s", status, body)
	}
	if content, err := os.ReadFile(at("results/new é.txt")); err != nil || string(content) != "hello" {
		t.Errorf("uploaded %q %v", content, err)
	}
	for name, test := range map[string]struct {
		query, contentType string
		status             int
		want               string
	}{
		"a name taken":  {"dir=" + url.QueryEscape(at("results")) + "&name=new%20%C3%A9.txt", "application/octet-stream", http.StatusConflict, "already exists"},
		"a bad name":    {"dir=" + url.QueryEscape(at("results")) + "&name=a%2Fb", "application/octet-stream", http.StatusBadRequest, "holds / or"},
		"no name":       {"dir=" + url.QueryEscape(at("results")), "application/octet-stream", http.StatusBadRequest, "a name is needed"},
		"relative":      {"dir=results&name=x", "application/octet-stream", http.StatusBadRequest, "not absolute"},
		"a missing dir": {"dir=" + url.QueryEscape(at("gone")) + "&name=x", "application/octet-stream", http.StatusUnprocessableEntity, "no such file"},
		"a form":        {"dir=" + url.QueryEscape(at("results")) + "&name=x", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType, "octet-stream"},
	} {
		if status, body := upload(test.query, test.contentType, "other"); status != test.status || !strings.Contains(body, test.want) {
			t.Errorf("%s: %d %s, want %d saying %q", name, status, body, test.status, test.want)
		}
	}
	if content, err := os.ReadFile(at("results/new é.txt")); err != nil || string(content) != "hello" {
		t.Errorf("a refused upload changed the file: %q %v", content, err)
	}
	if response, _ := terminalTransfer(t, server, http.MethodGet, terminalFilesPath+"/upload?dir=/&name=x", "", ""); response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET upload: %d", response.StatusCode)
	}
	if response, _ := terminalTransfer(t, server, http.MethodPost, terminalFilesPath+"/download?path=/", "application/json", "{}"); response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST download: %d", response.StatusCode)
	}
}

// nonUTF8NamesAllowed reports whether the file system under dir holds a name
// that is not UTF-8. macOS (APFS) refuses one with EILSEQ; there the tests
// check everything else and leave such names out.
func nonUTF8NamesAllowed(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "probe\xff")
	err := os.WriteFile(probe, nil, 0o600)
	if errors.Is(err, syscall.EILSEQ) {
		t.Logf("this file system refuses names that are not UTF-8: %v", err)
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	return true
}
