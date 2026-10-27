package web

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

func TestTableViewCommandServesOnePath(t *testing.T) {
	covers(t, "WEB-20")
	e := support.NewEnv(t)
	data := filepath.Join(e.Root, "data")
	outside := filepath.Join(e.Root, "outside")
	for path, content := range map[string]string{
		filepath.Join(data, "list.csv"):     "audio\nwav/a.wav\n" + filepath.Join(outside, "b.wav") + "\n",
		filepath.Join(data, "wav", "a.wav"): "RIFF",
		filepath.Join(outside, "b.wav"):     "RIFF",
		filepath.Join(outside, "other.csv"): "x\n1\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTableViewTree(t, data)

	// A file: its directory is the only root and the base of its paths.
	base := e.StartTableView(filepath.Join(data, "list.csv"))
	if r := e.HTTPGet(base + "/"); r.Status != 200 || !strings.Contains(r.Body, "table_view.js") {
		t.Fatalf("page = (%d, %.200s)", r.Status, r.Body)
	}
	for _, asset := range []string{"/table_view.js", "/table_view_panes.js", "/table_view.css", "/web_tokens.css", "/web_fonts.css"} {
		if r := e.HTTPGet(base + asset); r.Status != 200 {
			t.Fatalf("%s = %d", asset, r.Status)
		}
	}
	if r := e.HTTPGet(base + "/api/info"); !strings.Contains(r.Body, `"kind":"file"`) || !strings.Contains(r.Body, `"bases":[{"id":"file"`) {
		t.Fatalf("info = (%d, %s)", r.Status, r.Body)
	}
	r := e.HTTPGet(base + "/api/rows?sep=,")
	if !strings.Contains(r.Body, `"ref":"wav/a.wav"`) || !strings.Contains(r.Body, `"error":"Not shown: outside the allowed roots"`) {
		t.Fatalf("rows = (%d, %s)", r.Status, r.Body)
	}
	if r := e.HTTPGet(base + "/api/media?" + url.Values{"ref": {"wav/a.wav"}}.Encode()); r.Status != 200 || r.Body != "RIFF" {
		t.Fatalf("media = (%d, %q)", r.Status, r.Body)
	}
	if r := e.HTTPGet(base + "/api/info?child=list.csv"); r.Status != 400 || !strings.Contains(r.Body, "child applies only to a directory") {
		t.Fatalf("a file's child = (%d, %s)", r.Status, r.Body)
	}
	for _, ref := range []string{filepath.Join(outside, "b.wav"), "../outside/b.wav"} {
		if r := e.HTTPGet(base + "/api/media?" + url.Values{"ref": {ref}}.Encode()); r.Status != 403 {
			t.Fatalf("%s = (%d, %q)", ref, r.Status, r.Body)
		}
	}

	// A directory, with a token required.
	base = e.StartTableView(data, "--auth-token", "secret-token")
	if r := e.HTTPGet(base + "/api/info"); r.Status != 401 {
		t.Fatalf("without the token = %d", r.Status)
	}
	if r := e.HTTPDo("GET", base+"/api/rows?pattern=**/*.wav&expanded=wav", map[string]string{"Authorization": "Bearer secret-token"}, ""); r.Status != 200 || !strings.Contains(r.Body, `"cells":["wav/a.wav"`) || strings.Contains(r.Body, "b.wav") {
		t.Fatalf("directory rows = (%d, %s)", r.Status, r.Body)
	}
	checkTableViewExpansion(t, func(route string, extra url.Values) support.HTTPResult {
		return e.HTTPDo("GET", base+"/api/"+route+"?"+extra.Encode(), map[string]string{"Authorization": "Bearer secret-token"}, "")
	})
	// A table file in it is marked, and opens as its child, read under the
	// same root; a child cannot leave it.
	authorized := func(route string, query url.Values) support.HTTPResult {
		return e.HTTPDo("GET", base+"/api/"+route+"?"+query.Encode(), map[string]string{"Authorization": "Bearer secret-token"}, "")
	}
	if r := authorized("rows", url.Values{"pattern": {"*.csv"}}); !strings.Contains(r.Body, `"cells":["list.csv"`) || !strings.Contains(r.Body, `"tables":[0]`) {
		t.Fatalf("table files = (%d, %s)", r.Status, r.Body)
	}
	if r := authorized("info", url.Values{"child": {"list.csv"}}); r.Status != 200 || !strings.Contains(r.Body, `"kind":"file"`) || !strings.Contains(r.Body, `/list.csv"`) {
		t.Fatalf("child info = (%d, %s)", r.Status, r.Body)
	}
	if r := authorized("rows", url.Values{"child": {"list.csv"}, "sep": {","}}); !strings.Contains(r.Body, `"ref":"wav/a.wav","properties"`) || !strings.Contains(r.Body, `"error":"Not shown: outside the allowed roots"`) {
		t.Fatalf("child rows = (%d, %s)", r.Status, r.Body)
	}
	if r := authorized("media", url.Values{"child": {"list.csv"}, "ref": {"wav/a.wav"}}); r.Status != 200 || r.Body != "RIFF" {
		t.Fatalf("child media = (%d, %q)", r.Status, r.Body)
	}
	if err := os.Symlink(filepath.Join(outside, "other.csv"), filepath.Join(data, "link.csv")); err != nil {
		t.Fatal(err)
	}
	for child, status := range map[string]int{"../outside/other.csv": 400, filepath.Join(outside, "other.csv"): 400, "wav/../list.csv": 400, "link.csv": 403, "missing.csv": 404} {
		if r := authorized("info", url.Values{"child": {child}}); r.Status != status {
			t.Fatalf("child %s = (%d, %s), want %d", child, r.Status, r.Body, status)
		}
	}

	if r := e.Rotari("tableview", "--port", "0", filepath.Join(e.Root, "missing.csv")); r.Code != 1 || !strings.Contains(r.Stderr, "cannot view") {
		t.Fatalf("missing path = %s", r)
	}
	if r := e.Rotari("tableview", "--port", "0"); r.Code != 1 || !strings.Contains(r.Stderr, "usage: rotari tableview") {
		t.Fatalf("no path = %s", r)
	}
}

func TestTableViewCommandWritesAStaticSnapshot(t *testing.T) {
	covers(t, "WEB-22")
	e := support.NewEnv(t)
	data := filepath.Join(e.Root, "data")
	for path, content := range map[string]string{
		filepath.Join(data, "list.csv"):     "id,audio\n2,wav/a.wav\n10,wav/a.wav\n1,wav/gone.wav\n",
		filepath.Join(data, "wav", "a.wav"): "RIFF",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(e.Root, "site", "tableview")
	r := e.MustRotari("tableview", "--static-dir", output, filepath.Join(data, "list.csv"))
	if !strings.Contains(r.Stdout, ": 3 rows and 1 media files (4 bytes)") {
		t.Fatalf("stdout = %q", r.Stdout)
	}
	page, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	// The page embeds every row, each column's orders, and Describe; it is
	// named by the file alone and links the one media file that exists.
	for _, want := range []string{`"name":"list.csv"`, `"static":`, `"ascending":[[2,0,1]`, `"descending":[[1,0,2]`, `"describe":`, `"media":{"wav/a.wav":"media/0.wav"}`, "table_view_static.js"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("page lacks %s", want)
		}
	}
	if strings.Contains(string(page), data) {
		t.Errorf("page names the source directory %s", data)
	}
	for _, name := range []string{"table_view.js", "table_view_panes.js", "table_view_static.js", "table_view.css", "web_tokens.css", "web_fonts.css", "fonts/LICENSE.txt"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if copied, err := os.ReadFile(filepath.Join(output, "media", "0.wav")); err != nil || string(copied) != "RIFF" {
		t.Errorf("media copy = (%q, %v)", copied, err)
	}

	for _, test := range []struct {
		env  *support.Env
		args []string
		want string
	}{
		{e, []string{"--port", "0"}, "--port cannot be combined with --static-dir"},
		{e, []string{"--host", "0.0.0.0", "--auth-token", "x"}, "--auth-token, --host cannot be combined with --static-dir"},
		{e.WithVar("ROTARI_WEB_AUTH_TOKEN", "x"), nil, "--auth-token cannot be combined with --static-dir"},
	} {
		args := append(append([]string{"tableview"}, test.args...), "--static-dir", output, filepath.Join(data, "list.csv"))
		if r := test.env.Rotari(args...); r.Code != 1 || !strings.Contains(r.Stderr, test.want) {
			t.Errorf("%v = %s", test.args, r)
		}
	}
	for _, test := range []struct{ path, output, want string }{
		{data, filepath.Join(e.Root, "dir-out"), "a static snapshot takes a delimited file"},
		{filepath.Join(data, "list.csv"), data, "which it would replace"},
	} {
		if r := e.Rotari("tableview", "--static-dir", test.output, test.path); r.Code != 1 || !strings.Contains(r.Stderr, test.want) {
			t.Errorf("%s into %s = %s", test.path, test.output, r)
		}
	}
	if _, err := os.Stat(filepath.Join(data, "list.csv")); err != nil {
		t.Fatalf("source after refusal: %v", err)
	}
}
