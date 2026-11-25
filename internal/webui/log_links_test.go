package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	stateinternal "github.com/rotari-dev/rotari/internal/state"
)

// postLogLinks asks handler, from an address that is not loopback.
func postLogLinks(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "http://far.example/api/log-links", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// Candidates in a job's log are links when they name a file: relative ones in the attempt's
// recorded working directory, and only under the roots its artifacts are served from unless
// the asker may use the terminal.
func TestLogLinks(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "a b.py"), "x")
	write(t, filepath.Join(f.outside, "lib.py"), "x")
	if err := syscall.Mkfifo(filepath.Join(f.work, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.work, "data.csv"), filepath.Join(f.work, "inside.csv")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("data.csv", filepath.Join(f.work, "near.csv")); err != nil {
		t.Fatal(err)
	}
	data, outside := filepath.Join(f.work, "data.csv"), filepath.Join(f.outside, "lib.py")
	parent := filepath.Join(filepath.Dir(f.baseDir), "beside.py")
	write(t, parent, "x")
	candidates := []string{
		"data.csv:3", "./data.csv", "data.csv:3:7", data, data + ":9", "a b.py:2", "near.csv:1", "data.csv:0", "data.csv:3",
		parent + ":4", outside + ":5", "../" + filepath.Base(f.outside) + "/lib.py:1", "link.csv:1", "./link.csv", "./many/escape/secret.txt",
		"many/", "many:1", "./pipe", "data.csv", "gone.csv:1", "12:30:45", "", "/", "/proc/self/environ:1",
	}
	inRoots := []logLink{
		{Text: "data.csv:3", Path: data, Line: 3}, {Text: "./data.csv", Path: data}, {Text: "data.csv:3:7", Path: data, Line: 3, Column: 7},
		{Text: data, Path: data}, {Text: data + ":9", Path: data, Line: 9}, {Text: "a b.py:2", Path: filepath.Join(f.work, "a b.py"), Line: 2},
		{Text: "near.csv:1", Path: filepath.Join(f.work, "near.csv"), Line: 1}, {Text: "data.csv:0", Path: data},
		{Text: parent + ":4", Path: parent, Line: 4},
	}
	body := func(fields map[string]any) string {
		asked := map[string]any{"project_name": "default", "run_id": f.runID, "job_id": "job-1", "candidates": candidates}
		for key, value := range fields {
			asked[key] = value
		}
		text, err := json.Marshal(asked)
		if err != nil {
			t.Fatal(err)
		}
		return string(text)
	}
	attempt := stateinternal.MakeAttemptID(f.runID, "job-1", 0)
	for name, options := range map[string]Options{
		"read-only": {BaseDir: f.baseDir, Store: testStore()},
		"control":   {BaseDir: f.baseDir, Store: testStore(), AllowControl: true},
	} {
		for _, id := range []string{"", attempt} {
			answer := postLogLinks(t, Handler(options), body(map[string]any{"attempt_id": id}))
			got := decodeBody[logLinksResponse](t, answer)
			if answer.Code != http.StatusOK || !reflect.DeepEqual(got.Links, inRoots) || got.DirError != "" {
				t.Errorf("%s, attempt %q = %d:\n got %+v\nwant %+v", name, id, answer.Code, got, inRoots)
			}
		}
	}

	// An allowed root adds what is under it.
	rooted := postLogLinks(t, Handler(Options{BaseDir: f.baseDir, Store: testStore(), ArtifactRoots: []string{f.outside}}), body(map[string]any{"candidates": []string{outside + ":5"}}))
	if got := decodeBody[logLinksResponse](t, rooted).Links; !reflect.DeepEqual(got, []logLink{{Text: outside + ":5", Path: outside, Line: 5}}) {
		t.Errorf("under --artifact-root = %+v", got)
	}

	// Whoever may use the terminal may ask about any path.
	server, _ := terminalServerWith(t, "/bin/sh", Options{BaseDir: f.baseDir, Store: testStore()})
	post := func(contentType, text string) (int, string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, server.URL+"/api/log-links", strings.NewReader(text))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Rotari-Token", terminalTestToken)
		request.Header.Set("Content-Type", contentType)
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
	anywhere := body(map[string]any{"candidates": []string{outside + ":5", "./link.csv", "data.csv:1", "many/", "./pipe"}})
	var got logLinksResponse
	if status, text := post("application/json", anywhere); status != http.StatusOK || json.Unmarshal([]byte(text), &got) != nil ||
		!reflect.DeepEqual(got.Links, []logLink{{Text: outside + ":5", Path: outside, Line: 5}, {Text: "./link.csv", Path: filepath.Join(f.work, "link.csv")}, {Text: "data.csv:1", Path: data, Line: 1}}) {
		t.Errorf("with the terminal = %d %s", status, text)
	}
	// But not over a connection the terminal refuses.
	keeperOptions := Options{BaseDir: f.baseDir, Store: testStore()}
	_, keeper := terminalServerWith(t, "/bin/sh", keeperOptions)
	keeperOptions.Terminal = keeper
	recorder := postLogLinks(t, Handler(keeperOptions), anywhere)
	if got := decodeBody[logLinksResponse](t, recorder).Links; recorder.Code != http.StatusOK || !reflect.DeepEqual(got, []logLink{{Text: "data.csv:1", Path: data, Line: 1}}) {
		t.Errorf("with the terminal over plain HTTP = %d %+v", recorder.Code, got)
	}

	// An attempt without a record resolves no relative path, and says so.
	bare := decodeBody[logLinksResponse](t, postLogLinks(t, Handler(Options{BaseDir: f.baseDir, Store: testStore()}), body(map[string]any{"job_id": "job-2"})))
	if !reflect.DeepEqual(bare.Links, []logLink{{Text: parent + ":4", Path: parent, Line: 4}}) || bare.DirError != "the attempt recorded no working directory" {
		t.Errorf("an attempt without a record = %+v", bare)
	}

	// Records another user could write name no roots.
	if err := os.Chmod(f.baseDir, 0o777); err != nil {
		t.Fatal(err)
	}
	if got := decodeBody[logLinksResponse](t, postLogLinks(t, Handler(Options{BaseDir: f.baseDir, Store: testStore()}), body(nil))).Links; len(got) != 0 {
		t.Errorf("records another user could write = %+v", got)
	}
	if err := os.Chmod(f.baseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	many := make([]string, linkCandidatesLimit+1)
	for name, test := range map[string]struct {
		contentType, body string
		want              int
	}{
		"too many":        {"application/json", body(map[string]any{"candidates": many}), http.StatusBadRequest},
		"too long":        {"application/json", body(map[string]any{"candidates": []string{strings.Repeat("a", linkCandidateLimit+1)}}), http.StatusBadRequest},
		"unknown fields":  {"application/json", body(map[string]any{"dir": "/"}), http.StatusBadRequest},
		"not JSON":        {"text/plain", body(nil), http.StatusUnsupportedMediaType},
		"unknown run":     {"application/json", body(map[string]any{"run_id": "20990101-000000-00000000"}), http.StatusBadRequest},
		"unknown job":     {"application/json", body(map[string]any{"job_id": "job-9"}), http.StatusBadRequest},
		"a path as run":   {"application/json", body(map[string]any{"run_id": "../x"}), http.StatusBadRequest},
		"another attempt": {"application/json", body(map[string]any{"attempt_id": stateinternal.MakeAttemptID(f.runID, "job-2", 0)}), http.StatusBadRequest},
	} {
		if status, text := post(test.contentType, test.body); status != test.want {
			t.Errorf("%s = %d %s, want %d", name, status, text, test.want)
		}
	}
	if answer := serveJSON(t, Handler(Options{BaseDir: f.baseDir, Store: testStore()}), http.MethodGet, "/api/log-links", ""); answer.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET = %d", answer.Code)
	}
}

// The log dialog asks about the lines in view, makes the answers links, keeps the text as it
// is, and opens VS Code with a click or Enter but not at the end of a selection.
func TestLogLinksInPage(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	dir := t.TempDir()
	live, static := filepath.Join(dir, "live.html"), filepath.Join(dir, "static.html")
	if err := os.WriteFile(live, []byte(composeWebPage(livePage{nonce: "n", editor: &editorConnection{Host: "srv"}}, nil, false, "", "")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(static, []byte(composeWebHTML(nil, false, "")), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
const fs = require('fs');
const assert = require('node:assert/strict');
const { JSDOM, VirtualConsole } = require('jsdom');
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const load = (file) => {
  const state = { text: '', asked: [], links: {}, status: 200, observers: [], polls: [] };
  const dom = new JSDOM(fs.readFileSync(file, 'utf8'), { runScripts: 'dangerously', url: 'http://127.0.0.1/project/p/run/r', virtualConsole: new VirtualConsole(), beforeParse(window) {
    window.fetch = async (url, options) => {
      if (url.startsWith('/api/log?')) return { ok: true, status: 200, text: async () => url.endsWith('before=0') ? state.text : state.older || '' };
      if (url === '/api/log-links') {
        const body = JSON.parse(options.body);
        state.asked.push(body);
        if (state.status !== 200) return { ok: false, status: state.status, text: async () => 'refused\n' };
        return { ok: true, status: 200, json: async () => ({ links: body.candidates.filter((text) => state.links[text]).map((text) => ({ text, ...state.links[text] })) }) };
      }
      return new Promise(() => {});
    };
    window.setInterval = (poll) => { state.polls.push(poll); return 1; };
    window.IntersectionObserver = class {
      constructor(callback) { this.callback = callback; this.targets = new Set(); state.observers.push(this); }
      observe(target) { this.targets.add(target); }
      unobserve(target) { this.targets.delete(target); }
      disconnect() { this.targets.clear(); }
    };
  } });
  state.window = dom.window;
  state.output = () => dom.window.document.getElementById('modal-log');
  state.paths = () => [...state.output().querySelectorAll('.log-path')];
  // show brings lines into view, by what they hold.
  state.show = async (...held) => {
    const observer = state.observers.at(-1);
    observer.callback([...observer.targets].map((target) => ({ target, isIntersecting: held.some((text) => target.textContent.includes(text)) })), observer);
    await wait(10);
  };
  return state;
};
(async () => {
  let p = load(process.argv[1]);
  p.links = {
    'train.py:12': { path: '/w/train.py', line: 12 },
    '/w/a b.py:41': { path: '/w/a b.py', line: 41 },
    'src/x.py:3:7': { path: '/w/src/x.py', line: 3, column: 7 },
    'out/<b>.csv': { path: '/w/never.csv' },
  };
  p.text = ['start 20261114-000700-aaaaaaaa job aaaaaaaa1', 'train.py:12: error, see src/x.py:3:7 and gone/y.py:1', '  File "/w/a b.py", line 41, in f', '<img src=x> plain </b>', 'later/z.py:9', '', ''].join('\n');
  await p.window.showLog('p', 'r', 'j', 'att', 'stdout', 'merge');
  assert.equal(p.output().textContent, p.text);
  assert.equal(p.output().querySelector('img'), null, 'a log became markup');
  assert.deepEqual(p.asked, [], 'lines out of view were asked about');
  await p.show('train.py:12', 'File');
  assert.deepEqual(p.asked, [{ project_name: 'p', run_id: 'r', job_id: 'j', attempt_id: 'att', candidates: p.asked[0].candidates }]);
  assert.ok(p.asked[0].candidates.includes('/w/a b.py:41') && p.asked[0].candidates.includes('gone/y.py:1'), p.asked[0].candidates.join(' '));
  assert.ok(!p.asked[0].candidates.some((text) => /later|aaaaaaaa/.test(text)), 'asked about a line out of view, a run, or a job: ' + p.asked[0].candidates);
  assert.deepEqual(p.paths().map((path) => [path.textContent, path.title]), [
    ['train.py:12', 'Open /w/train.py in VS Code at line 12'], ['src/x.py:3:7', 'Open /w/src/x.py in VS Code at line 3'],
    ['/w/a b.py", line 41', 'Open /w/a b.py in VS Code at line 41'],
  ]);
  assert.equal(p.output().textContent, p.text, 'links changed the text');
  // A click and Enter open it, and a click that ends a selection does not.
  const opened = [];
  p.window.openInEditor = (target) => opened.push([target.path, target.line, target.column].join(' '));
  p.paths()[1].click();
  p.paths()[2].dispatchEvent(new p.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  p.paths()[0].dispatchEvent(new p.window.KeyboardEvent('keydown', { key: 'a', bubbles: true }));
  assert.deepEqual(opened, ['/w/src/x.py 3 7', '/w/a b.py 41 ']);
  p.window.getSelection().selectAllChildren(p.paths()[0]);
  p.paths()[0].click();
  assert.equal(opened.length, 2, 'selecting a path opened it');
  p.window.getSelection().removeAllRanges();
  // The same text is not drawn again; new text is, and what was answered is not asked again.
  const kept = p.paths()[0];
  await p.polls.at(-1)();
  assert.equal(p.paths()[0], kept);
  p.text += 'more train.py:12';
  await p.polls.at(-1)();
  assert.equal(p.output().textContent, p.text);
  const before = p.asked.length;
  await p.show('more', 'File');
  assert.equal(p.asked.length, before, 'an answered path was asked about again');
  assert.deepEqual(p.paths().map((path) => path.textContent), ['/w/a b.py", line 41', 'train.py:12']);
  // Older lines go before, whole.
  p.older = 'older src/x.py:3:7\n';
  p.output().scrollTop = 0;
  await p.output().onscroll();
  assert.equal(p.output().textContent, p.older + p.text);
  // More candidates than one request may have are asked in several, a very long line not at all.
  p.text = Array.from({ length: 100 }, (_, index) => 'd/f' + index).join(' ') + '\nlong/' + 'x'.repeat(5000);
  await p.window.showLog('p', 'r', 'j2', '', 'stdout', 'merge');
  p.asked.length = 0;
  await p.show('d/f', 'long/');
  assert.deepEqual(p.asked.map((body) => body.candidates.length), [64, 36]);
  assert.equal(p.asked[0].attempt_id, '');
  // A refusal says so once and leaves the text.
  p.status = 403;
  p.text = 'q/a.py:1\nq/b.py:1';
  await p.window.showLog('p', 'r', 'j3', '', 'stdout', 'merge');
  await p.show('a.py');
  const note = () => p.window.document.querySelector('.editor-note').textContent;
  assert.equal(note(), 'Paths in this log are not links: refused');
  p.window.document.querySelector('.editor-note').replaceChildren();
  await p.show('b.py');
  assert.equal(note(), '');
  assert.equal(p.output().textContent, p.text);
  // An answer for a log no longer open changes nothing.
  p.status = 200;
  p.links['q/a.py:1'] = { path: '/w/q/a.py', line: 1 };
  await p.window.showLog('p', 'r', 'j4', '', 'stdout', 'merge');
  const showing = p.show('a.py');
  p.window.closeOutputModal();
  await showing;
  assert.equal(p.paths().length, 0);

  // A static export asks nothing.
  p = load(process.argv[2]);
  p.text = 'train.py:12';
  await p.window.showLog('p', 'r', 'j', '', 'stdout', 'merge');
  assert.deepEqual([p.output().textContent, p.observers.length, p.asked.length], ['train.py:12', 0, 0]);
  process.exit(0);
})().catch((error) => { console.error(error.stack); process.exit(1); });
`
	if output, err := exec.Command("node", "-e", script, live, static).CombinedOutput(); err != nil {
		t.Fatalf("page check failed: %v\n%s", err, output)
	}
}
