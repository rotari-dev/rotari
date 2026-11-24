package webui

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/launcher"
)

// /api/vscode-target says where an artifact is, for what its content requests serve and
// nothing else, without launching or the terminal, and on a read-only server.
func TestVSCodeTarget(t *testing.T) {
	f := newArtifactFixture(t)
	handler := Handler(Options{BaseDir: f.baseDir, Store: testStore()})
	target := func(entry, child string) (int, string) {
		query := url.Values{"project_name": {"default"}, "run_id": {f.runID}, "job_id": {"job-1"}, "entry": {entry}, "child": {child}}
		answer := serveJSON(t, handler, http.MethodGet, "/api/vscode-target?"+query.Encode(), "")
		return answer.Code, answer.Body.String()
	}
	entry := func(name string) string { return strconv.Itoa(f.entries[name]) }
	for name, test := range map[string]struct {
		entry, child string
		path         string
		kind         launcher.TargetKind
	}{
		"file":              {entry("data.csv"), "", filepath.Join(f.work, "data.csv"), launcher.TargetFile},
		"directory":         {entry("many"), "", filepath.Join(f.work, "many"), launcher.TargetDirectory},
		"child":             {entry("many"), "f001.txt", filepath.Join(f.work, "many", "f001.txt"), launcher.TargetFile},
		"working directory": {artifactWorkingDirectoryEntry, "", f.work, launcher.TargetDirectory},
	} {
		code, body := target(test.entry, test.child)
		if want := `{"path":"` + test.path + `","kind":"` + string(test.kind) + `"}`; code != http.StatusOK || strings.TrimSpace(body) != want {
			t.Errorf("%s = %d %s, want %s", name, code, body, want)
		}
	}
	for name, test := range map[string]struct {
		entry, child string
		code         int
	}{
		"outside the roots":    {entry("secret.txt"), "", http.StatusForbidden},
		"a link leaving them":  {entry("link.csv"), "", 0},
		"a child leaving them": {entry("many"), "../../x", http.StatusBadRequest},
		"a child link":         {entry("many"), "escape", 0},
		"missing":              {entry("gone.csv"), "", 0},
		"no such entry":        {"99", "", http.StatusBadRequest},
	} {
		code, body := target(test.entry, test.child)
		if code == http.StatusOK || test.code != 0 && code != test.code {
			t.Errorf("%s = %d %s, want refusal %d", name, code, body, test.code)
		}
	}
	if answer := serveJSON(t, handler, http.MethodPost, "/api/vscode-target", "{}"); answer.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d", answer.Code)
	}
}

// Live pages offer Open in VS Code, read-only ones too, and a static export does not.
func TestVSCodeIsOfferedOnLivePages(t *testing.T) {
	f := newArtifactFixture(t)
	handler := Handler(Options{BaseDir: f.baseDir, Store: testStore()})
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/jobs/"} {
		for _, session := range []string{"", "10.0.0.1 1 10.0.0.2 22"} {
			t.Setenv("SSH_CONNECTION", session)
			page := serveJSON(t, handler, http.MethodGet, path, "").Body.String()
			want := `id="vscode-connection" data-host="` + host + `" data-name="` + strings.Split(host, ".")[0] + `" data-remote="` + strconv.FormatBool(session != "") + `"`
			if !strings.Contains(page, want) || strings.Contains(page, "__ROTARI_VSCODE__") {
				t.Errorf("%s with SSH_CONNECTION=%q lacks %s", path, session, want)
			}
		}
	}
	for _, page := range []string{composeWebHTML(nil, false, ""), jobsHTML("/", nil, nil, jobsSelection{}, false, false)} {
		if strings.Contains(page, `id="vscode-connection"`) || strings.Contains(page, "__ROTARI_VSCODE__") {
			t.Error("a static page offers Open in VS Code")
		}
	}
	hostile := vscodeConnection{Host: `x"><script>`}.attributes()
	if strings.Contains(hostile, "<script>") || strings.Contains(hostile, `x">`) {
		t.Errorf("a host name is not escaped: %s", hostile)
	}
	if page := composeTerminalWindow("n", vscodeConnection{Host: "srv.example"}); !strings.Contains(page, `data-host="srv.example" data-name="srv"`) {
		t.Error("the terminal's window does not offer Open in VS Code")
	}
}

// The dialog, what it keeps, and the URLs.
func TestVSCodeInPage(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	path := filepath.Join(t.TempDir(), "page.html")
	// The script alone, as the page frame has it; a whole page per case would be slow.
	page := "<!doctype html><script>" + webVSCodeJS + "</script><body>" + livePage{vscode: &vscodeConnection{Host: "ks000.example.com", Remote: true}}.vscodeControl()
	if err := os.WriteFile(path, []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
const fs = require('fs');
const assert = require('node:assert/strict');
const { JSDOM, VirtualConsole } = require('jsdom');
const load = ({ stored = {}, broken = false, host } = {}) => {
  let html = fs.readFileSync(process.argv[1], 'utf8');
  if (host !== undefined) html = html.replace('data-host="ks000.example.com"', 'data-host="' + host + '"');
  const dom = new JSDOM(html, { runScripts: 'dangerously', url: 'http://127.0.0.1:8787/', virtualConsole: new VirtualConsole(), beforeParse(window) {
    window.fetch = () => new Promise(() => {});
    window.setInterval = () => 1;
    for (const [key, value] of Object.entries(stored)) window.localStorage.setItem(key, value);
    if (broken) {
      window.Storage.prototype.getItem = () => { throw new Error('storage is off'); };
      window.Storage.prototype.setItem = () => { throw new Error('storage is off'); };
    }
  } });
  const { window } = dom, { document } = window;
  const opened = [];
  document.addEventListener('click', (event) => {
    if (event.target.href?.startsWith('vscode:')) { opened.push(event.target.getAttribute('href')); event.preventDefault(); }
  }, true);
  return {
    window, document, opened,
    open: (target) => window.openInVSCode(target),
    dialog: () => document.querySelector('.vscode-modal'),
    note: () => document.querySelector('.vscode-note:not([hidden])')?.textContent || '',
    radios: () => [...document.querySelectorAll('.vscode-form input[type=radio]')],
    name: () => document.querySelector('.vscode-form input[type=text]'),
    problem: () => document.querySelector('.vscode-form [role=alert]').textContent,
    submit: () => document.querySelector('.vscode-form button[type=submit]').click(),
    cancel: () => [...document.querySelectorAll('.vscode-form button')].find((each) => each.textContent === 'Cancel').click(),
    kept: () => window.localStorage.getItem('rotari-vscode-ks000.example.com'),
  };
};
const remote = (name) => ({ 'rotari-vscode-ks000.example.com': JSON.stringify({ remote: true, name }) });
const local = { 'rotari-vscode-ks000.example.com': JSON.stringify({ remote: false }) };
const prefix = 'vscode://vscode-remote/ssh-remote+ks000';

(async () => {
// The first open asks, with the server's suggestions; Cancel and Escape open and keep nothing.
let p = load();
p.open({ path: '/w/a.py' });
assert.equal(p.radios()[1].checked, true);
assert.equal(p.name().value, 'ks000');
assert.match(p.dialog().textContent, /as in ssh ks000, or USER@HOST\. VS Code asks for a password/);
p.cancel();
assert.equal(p.dialog(), null);
p.open({ path: '/w/a.py' });
p.dialog().dispatchEvent(new p.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
assert.equal(p.dialog(), null);
assert.deepEqual([p.opened, p.kept()], [[], null]);
// Open keeps the choice and opens; the next one does not ask.
p.open({ path: '/w/a.py' });
p.submit();
assert.deepEqual(p.opened, [prefix + '/w/a.py:1']);
assert.deepEqual(JSON.parse(p.kept()), { remote: true, name: 'ks000' });
assert.match(p.note(), /^Opening \/w\/a\.py in VS Code through SSH host ks000\./);
p.open({ path: '/w/a.py', line: 12, column: 3 });
assert.equal(p.dialog(), null);
assert.equal(p.opened[1], prefix + '/w/a.py:12:3');
// Change, in the note, and Settings ask again, and open nothing.
[...p.document.querySelectorAll('.vscode-note button')].find((each) => each.textContent === 'Change').click();
p.radios()[0].click();
assert.equal(p.name().disabled, true);
p.submit();
assert.equal(p.opened.length, 2);
assert.deepEqual(JSON.parse(p.kept()), { remote: false });
p.open({ path: '/w/a.py' });
assert.equal(p.opened[2], 'vscode://file/w/a.py');
await new Promise((resolve) => setTimeout(resolve, 50)); // DOMContentLoaded
p.document.getElementById('vscode-connection-button').click();
assert.ok(p.dialog());
assert.equal(p.radios()[0].checked, true);
p.cancel();

// A name ssh would read as something else is refused where it is typed and where it was kept.
for (const bad of ['', '-oProxyCommand=x', 'u@-h', 'a b', 'a/b', 'a\nb', 'a@b@c', '日本', 'h:22', 'u:secret@h']) {
  p = load();
  p.open({ path: '/w/a.py' });
  p.name().value = bad.replace('\n', ' ');
  p.submit();
  assert.ok(p.dialog() && p.problem(), 'typed ' + JSON.stringify(bad));
  assert.deepEqual([p.opened, p.kept()], [[], null]);
  p = load({ stored: remote(bad) });
  p.open({ path: '/w/a.py' });
  assert.ok(p.dialog() && /kept SSH host cannot be used/.test(p.problem()), 'kept ' + JSON.stringify(bad));
  assert.deepEqual(p.opened, []);
}
for (const good of ['ks000', 'kamo@ks000.example.com', 'my_host-2', '10.0.0.1']) {
  p = load({ stored: remote(good) });
  p.open({ path: '/w/a.py' });
  assert.equal(p.opened[0], 'vscode://vscode-remote/ssh-remote+' + good + '/w/a.py:1');
}
for (const unreadable of ['', 'x', '{}', 'null', '[]']) {
  p = load({ stored: { 'rotari-vscode-ks000.example.com': unreadable } });
  p.open({ path: '/w/a.py' });
  assert.match(p.problem(), /could not be read/);
}
// Another server's choice, at the same address, is not used.
p = load({ stored: { 'rotari-vscode-other': JSON.stringify({ remote: true, name: 'other' }) } });
p.open({ path: '/w/a.py' });
assert.ok(p.dialog());
assert.deepEqual(p.opened, []);
// Without storage, or the server's name, each open asks, says why, and still opens.
for (const options of [{ broken: true }, { host: '' }]) {
  p = load(options);
  for (const count of [1, 2]) {
    p.open({ path: '/w/a.py' });
    assert.ok(p.problem());
    p.submit();
    assert.equal(p.opened.length, count);
    assert.match(p.note(), /The choice was not kept/);
  }
}

// Directories never take a window's place, and every other character is encoded.
for (const [target, remoteURL, localURL] of [
  [{ path: '/w', folder: true }, prefix + '/w?windowId=_blank', 'vscode://file/w?windowId=_blank'],
  [{ path: '/w/a b#c?d%e/日本.py' }, prefix + '/w/a%20b%23c%3Fd%25e/%E6%97%A5%E6%9C%AC.py:1', 'vscode://file/w/a%20b%23c%3Fd%25e/%E6%97%A5%E6%9C%AC.py'],
  [{ path: '/w/a:b.py', line: 2 }, prefix + '/w/a%3Ab.py:2', 'vscode://file/w/a%3Ab.py:2'],
]) {
  for (const [stored, url] of [[remote('ks000'), remoteURL], [local, localURL]]) {
    p = load({ stored });
    p.open(target);
    assert.deepEqual(p.opened, [url]);
  }
}
// What cannot be named is refused with the reason, before asking anything.
for (const [target, reason] of [
  [{ path: 'w/a.py' }, /not absolute/],
  [{ path: '/w/a\0' }, /NUL/],
  [{ path: '/w/a:12' }, /number after the colon/],
  [{ path: '/w/a:12:b' }, /number after the colon/],
  [{ path: '/w/a:' }, /number after the colon/],
  [{ path: '/w', folder: true, line: 1 }, /no lines/],
  [{ path: '/w/a', line: 0 }, /positive/],
  [{ path: '/w/a', line: 1.5 }, /positive/],
  [{ path: '/w/a', line: 1e300 }, /positive/],
  [{ path: '/w/a', column: 2 }, /positive/],
  [{ path: '/w/a', line: 1, column: 0 }, /positive/],
  [{ path: '/w/\ud800' }, /Cannot open/],
]) {
  p = load({ stored: remote('ks000') });
  p.open(target);
  assert.match(p.note(), reason, JSON.stringify(target));
  assert.deepEqual([p.opened, p.dialog()], [[], null]);
}
process.exit(0);
})().catch((error) => { console.error(error.stack); process.exit(1); });
`
	command := exec.Command("node", "-e", script, path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("page check failed: %v\n%s", err, output)
	}
}

// Artifacts rows and the preview's title line open what the server says the artifact is.
func TestVSCodeFromArtifactsInBrowser(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	f := newArtifactFixture(t)
	runLaunchBrowserScript(t, Options{BaseDir: f.baseDir, Store: testStore(), AllowControl: true}, f, `
  const asked = [];
  window.openInVSCode = (target) => asked.push(target);
  await window.showArtifacts('default', runID, 'job-1', '');
  document.getElementById('artifact-preview').scrollIntoView = () => {};
  const rows = () => [...document.querySelectorAll('.artifact-table tbody tr')];
  const rowOf = (name) => rows().find(row => row.children[2].textContent === name);
  await settle(() => rowOf('data.csv')?.querySelector('.artifact-vscode'));
  for (const name of ['data.csv', 'many']) rowOf(name).querySelector('.artifact-vscode').click();
  await settle(() => asked.length === 2);
  const sorted = asked.map(target => JSON.stringify(target)).sort();
  if (sorted[0] !== JSON.stringify({ path: work + '/data.csv', folder: false }) || sorted[1] !== JSON.stringify({ path: work + '/many', folder: true })) fail('rows', JSON.stringify(asked));
  if (rows().some(row => /secret|gone/.test(row.textContent) && row.querySelector('.artifact-vscode'))) fail('a row that is not served offers VS Code');
  await window.openArtifact(entries['many'], 'f001.txt', 'file');
  document.querySelector('.artifact-preview-title .artifact-vscode').click();
  await settle(() => asked.length === 3);
  if (asked[2].path !== work + '/many/f001.txt') fail('title', JSON.stringify(asked));
  // A refusal is shown, and opens nothing.
  await window.openArtifactInVSCode(entries['many'], 'escape');
  if (asked.length !== 3 || !document.querySelector('.vscode-note').textContent.includes('Cannot open')) fail('refusal', document.querySelector('.vscode-note')?.textContent);
  process.exit(0); // The note's timer would hold the script.
`)
}
