package webui

import (
	"encoding/json"
	"fmt"
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
