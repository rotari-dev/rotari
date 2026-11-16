package webui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The transcript page is served only with the terminal, behind its checks, under the page's
// nonce, with its own styles; it takes its transcript from its opener, so the server sends
// nothing of a shell with it.
func TestTerminalTranscriptPage(t *testing.T) {
	server, _ := terminalServer(t, "/bin/sh")
	request, err := http.NewRequest(http.MethodGet, server.URL+terminalTranscriptPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Rotari-Token", terminalTestToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(response.Body); err != nil {
		t.Fatal(err)
	}
	policy := response.Header.Get("Content-Security-Policy")
	if response.StatusCode != http.StatusOK || !strings.Contains(policy, "nonce-") || !strings.Contains(body.String(), "rotari-transcript-ask") || !strings.Contains(body.String(), `<script nonce="`) {
		t.Errorf("the page: status %d, policy %q:\n%s", response.StatusCode, policy, body.String())
	}
	if status, css := terminalRequest(t, server, http.MethodGet, terminalPrefix+"transcript.css"); status != http.StatusOK || !strings.Contains(css, ".transcript-card") {
		t.Errorf("its styles: status %d", status)
	}
	if status, _ := terminalRequest(t, server, http.MethodPost, terminalTranscriptPath); status != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d", status)
	}
	without := httptest.NewServer(WithAuthToken(Handler(Options{BaseDir: t.TempDir()}), terminalTestToken, AuthOptions{}))
	t.Cleanup(without.Close)
	if status, _ := terminalRequest(t, without, http.MethodGet, terminalTranscriptPath); status != http.StatusNotFound {
		t.Errorf("without the terminal: status %d", status)
	}
}

const transcriptCheck = `
const { JSDOM, VirtualConsole } = require('jsdom');
const assert = require('assert');
const fs = require('fs');
const html = fs.readFileSync(process.argv[1], 'utf8');
const url = 'http://127.0.0.1:9/_terminal/transcript';
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
(async () => {
  // Opened by hand, the page has nothing to ask.
  const lone = new JSDOM(html, { runScripts: 'dangerously', url, virtualConsole: new VirtualConsole() });
  assert.match(lone.window.document.querySelector('.transcript-status').textContent, /no terminal to ask/);
  const asked = [];
  const downloads = [];
  const copied = [];
  const opener = { closed: false, postMessage(message, origin) { asked.push([JSON.parse(JSON.stringify(message)), origin]); } };
  // The server: the shell's preview, and uploads, the first to a taken name refused.
  const requests = [];
  const taken = new Set(['/w/p1/build-box-20261116-102030.sh']);
  let confirmed = true;
  const confirms = [];
  const dom = new JSDOM(html, {
    runScripts: 'dangerously', url, virtualConsole: new VirtualConsole(),
    beforeParse(window) {
      Object.defineProperty(window, 'opener', { configurable: true, value: opener });
      window.Blob = class { constructor(parts, options) { this.content = parts.join(''); this.type = options.type; } };
      window.URL.createObjectURL = (blob) => { downloads.push({ content: blob.content, type: blob.type }); return 'blob:x'; };
      window.URL.revokeObjectURL = () => {};
      window.HTMLAnchorElement.prototype.click = function () { downloads.at(-1).name = this.download; };
      Object.defineProperty(window.navigator, 'clipboard', { configurable: true, value: { writeText: async (text) => copied.push(text) } });
      window.confirm = (text) => { confirms.push(text); return confirmed; };
      window.fetch = async (address, init = {}) => {
        const body = init.body ? new TextDecoder().decode(init.body) : '';
        requests.push([init.method || 'GET', address, init.headers?.['Content-Type'] || '', body]);
        const answer = (status, value) => ({ ok: status < 300, status, json: async () => value, text: async () => (typeof value === 'string' ? value : JSON.stringify(value)) });
        if (address === '/_terminal/shells/7/preview') return answer(200, { cwd: '/w/p1', processes: [] });
        const upload = /^\/_terminal\/files\/upload\?dir=([^&]*)&name=([^&]*)(&replace=1)?$/.exec(address);
        if (upload) {
          const path = decodeURIComponent(upload[1]) + '/' + decodeURIComponent(upload[2]);
          if (taken.has(path) && !upload[3]) return answer(409, path + ' already exists');
          taken.add(path);
          return answer(200, { path });
        }
        return answer(404, 'no such route');
      };
    },
  });
  const { window } = dom;
  const { document } = window;
  const status = () => document.querySelector('.transcript-status').textContent;
  assert.deepEqual(asked, [[{ type: 'rotari-transcript-ask' }, 'http://127.0.0.1:9']]);
  assert.match(status(), /Asking the terminal/);
  const deliver = (data, source = opener, origin = 'http://127.0.0.1:9') => {
    const event = new window.MessageEvent('message', { data, origin });
    Object.defineProperty(event, 'source', { value: source });
    window.dispatchEvent(event);
  };
  // Only its opener, of this origin, is heard.
  deliver({ type: 'rotari-transcript', transcript: null, reason: 'from elsewhere' }, opener, 'http://elsewhere.test');
  deliver({ type: 'rotari-transcript', transcript: null, reason: 'from another window' }, {});
  assert.match(status(), /Asking the terminal/);
  deliver({ type: 'rotari-transcript', transcript: null, reason: 'The terminal pane it came from has closed.' });
  assert.equal(status(), 'The terminal pane it came from has closed.');
  const numbered = [];
  for (let i = 1; i <= 40; i++) numbered.push({ t: 'line ' + i, s: {} }, { t: '\n', s: {} });
  numbered.pop();
  const transcript = {
    shell: { id: '7', dir: '/w/start', name: 'build box', project: 'p1', basedir: '/b' },
    takenAt: '2026-11-16T10:20:30.000Z',
    partial: false,
    cards: [
      { command: null, state: 'output', runs: [{ t: 'Welcome', s: {} }] },
      { command: 'make', state: 'ok', exit: 0, dir: '/w/p1', startedAt: 1000, endedAt: 2200, runs: [{ t: 'ok', s: { b: 1, fg: 2 } }, { t: ' <b>done</b>', s: {} }, { t: '\n', s: {} }, { t: 'warm', s: { fg: 208 } }] },
      { command: 'for i in 1 2\ndo echo $i; done', state: 'ok', exit: 0, dir: '/w/p1', startedAt: null, endedAt: null, runs: [{ t: '1', s: {} }, { t: '\n', s: {} }, { t: '2', s: {} }] },
      { command: "echo '` + "```" + `'", state: 'failed', exit: 1, dir: "/w/it's", startedAt: null, endedAt: null, runs: [{ t: '` + "```" + `', s: {} }] },
      { command: 'seq 40', state: 'ok', exit: 0, dir: "/w/it's", startedAt: null, endedAt: null, runs: numbered },
      { command: 'true', state: 'unknown', exit: null, dir: null, startedAt: null, endedAt: null, runs: [] },
      { command: 'sleep 9', state: 'running', exit: null, dir: '/w/p1', startedAt: null, endedAt: null, runs: [{ t: 'zz', s: {} }] },
    ],
  };
  deliver({ type: 'rotari-transcript', transcript });
  await wait(10);
  assert.equal(status(), '');
  assert.equal(document.title, 'Transcript of build box - rotari');
  assert.equal(document.querySelector('h1').textContent, 'Transcript of build box');
  assert.match(document.querySelector('.transcript-about').textContent, /^p1 · taken /);
  const cards = () => [...document.querySelectorAll('.transcript-card')];
  assert.deepEqual(cards().map((card) => [card.querySelector('.transcript-command').textContent, card.querySelector('.transcript-command').tagName, card.querySelector('.transcript-dot').dataset.state, card.querySelector('.transcript-meta').textContent]), [
    ['Output before the first prompt', 'CODE', 'output', 'before the first prompt'],
    ['make', 'CODE', 'ok', 'exit 0 · /w/p1 · 1.2 s'],
    ['for i in 1 2\ndo echo $i; done', 'PRE', 'ok', 'exit 0 · /w/p1'],
    ["echo '` + "```" + `'", 'CODE', 'failed', "failed, exit 1 · /w/it's"],
    ['seq 40', 'CODE', 'ok', "exit 0 · /w/it's"],
    ['true', 'CODE', 'unknown', 'no exit code'],
    ['sleep 9', 'CODE', 'running', 'still running · /w/p1'],
  ]);
  // Output is styled text: colours of the theme, 256 colours, bold; markup stays text.
  const make = cards()[1].querySelector('.transcript-output');
  const ok = make.querySelector('span');
  assert.equal(ok.textContent, 'ok');
  assert.equal(ok.style.getPropertyValue('color'), 'var(--terminal-green)');
  assert.equal(ok.style.getPropertyValue('font-weight'), 'bold');
  assert.equal(make.textContent, 'ok <b>done</b>\nwarm');
  assert.equal(make.querySelectorAll('b').length, 0, 'output became markup');
  assert.equal([...make.querySelectorAll('span')].find((span) => span.textContent === 'warm').style.getPropertyValue('color'), 'rgb(255, 135, 0)');
  assert.equal(cards()[5].querySelector('.transcript-empty').textContent, 'No output.');
  // A long output opens folded, until Show all.
  const long = cards()[4];
  assert.ok(long.querySelector('.transcript-output').classList.contains('transcript-output-folded'));
  long.querySelector('.transcript-more').click();
  assert.ok(!long.querySelector('.transcript-output').classList.contains('transcript-output-folded'));
  assert.equal(long.querySelector('.transcript-more'), null);
  // Copies.
  cards()[1].querySelector('.transcript-actions button').click();
  await wait(10);
  assert.deepEqual(copied, ['make']);
  // The filter and Failed only.
  const filter = document.querySelector('input[type="search"]');
  filter.value = 'ECHO';
  filter.dispatchEvent(new window.Event('input'));
  assert.deepEqual(cards().filter((card) => !card.hidden).map((card) => card.querySelector('.transcript-command').textContent), ['for i in 1 2\ndo echo $i; done', "echo '` + "```" + `'"]);
  const failed = document.querySelector('.transcript-failed input');
  failed.checked = true;
  failed.dispatchEvent(new window.Event('change'));
  assert.equal(cards().filter((card) => !card.hidden).length, 1);
  failed.checked = false;
  failed.dispatchEvent(new window.Event('change'));
  filter.value = '';
  filter.dispatchEvent(new window.Event('input'));
  const tool = (label) => [...document.querySelectorAll('.transcript-tools button')].find((button) => button.textContent === label);
  // A format is chosen once, remembered, and saved by Download or on the server.
  const choose = (kind) => {
    const radio = document.querySelector('.transcript-format input[value="' + kind + '"]');
    radio.checked = true;
    radio.dispatchEvent(new window.Event('change'));
  };
  assert.deepEqual([...document.querySelectorAll('.transcript-format input')].map((radio) => [radio.value, radio.checked]), [['md', true], ['html', false], ['sh', false]]);
  // The shell script: every command checked, cd where the directory changes, comments for
  // a failure and one still running, no output.
  choose('sh');
  assert.equal(window.localStorage.getItem('rotari-transcript-format'), 'sh');
  tool('Download').click();
  assert.equal(downloads.at(-1).name, 'build-box-20261116-102030.sh');
  assert.equal(downloads.at(-1).content, [
    '#!/usr/bin/env bash',
    "# Commands of build box (p1) from rotari's Web terminal, taken 2026-11-16T10:20:30.000Z.",
    '# Their output is left out.',
    '',
    "cd '/w/p1'",
    'make',
    '',
    'for i in 1 2',
    'do echo $i; done',
    '',
    "cd '/w/it'\\''s'",
    '# Failed with exit 1:',
    "echo '` + "```" + `'",
    '',
    'seq 40',
    '',
    'true',
    '',
    "cd '/w/p1'",
    '# Still running when the transcript was taken:',
    'sleep 9',
    '',
  ].join('\n'));
  // Markdown of the cards checked: fences longer than the backquotes inside.
  tool('Check none').click();
  for (const n of [1, 3]) {
    const box = cards()[n].querySelector('input[type="checkbox"]');
    box.checked = true;
    box.dispatchEvent(new window.Event('change'));
  }
  choose('md');
  tool('Download').click();
  assert.equal(downloads.at(-1).name, 'build-box-20261116-102030.md');
  assert.equal(downloads.at(-1).content, [
    '# Transcript of build box',
    '',
    "p1, taken 2026-11-16T10:20:30.000Z, from rotari's Web terminal.",
    '',
    '## ` + "`make`" + `',
    '',
    '` + "`/w/p1`" + ` · exit 0 · 1.2 s',
    '',
    '` + "```" + `text',
    'ok <b>done</b>',
    'warm',
    '` + "```" + `',
    '',
    "## ` + "````" + `echo '` + "```" + `'` + "````" + `",
    '',
    "` + "`/w/it's`" + ` · failed, exit 1",
    '',
    '` + "````" + `text',
    '` + "```" + `',
    '` + "````" + `',
    '',
  ].join('\n'));
  // HTML: one file, styles inline, output escaped and coloured, no script.
  choose('html');
  tool('Download').click();
  const saved = downloads.at(-1).content;
  assert.equal(downloads.at(-1).name, 'build-box-20261116-102030.html');
  assert.ok(saved.startsWith('<!doctype html>'));
  assert.ok(saved.includes('<span style="color:#7ee787;font-weight:bold">ok</span> &lt;b&gt;done&lt;/b&gt;'), saved);
  assert.ok(saved.includes('<span style="color:#ff8700">warm</span>'), saved);
  assert.ok(!saved.includes('<script'), 'the saved page has a script');
  assert.ok(!saved.includes('seq 40'), 'an unchecked card was saved');
  // Nothing checked: nothing saved, and it says so.
  // Save on the server: the shell's directory now and the name, the name taken asked
  // about and replaced when the viewer says so, kept when not; then shown in the Explorer.
  tool('Check all').click();
  choose('sh');
  const saveBox = document.querySelector('.transcript-save');
  const pathField = saveBox.querySelector('input[type="text"]');
  tool('Save on the server\u2026').click();
  await wait(10);
  assert.equal(saveBox.hidden, false);
  assert.equal(pathField.value, '/w/p1/build-box-20261116-102030.sh');
  const script = downloads.find((file) => file.name.endsWith('.sh')).content;
  saveBox.querySelector('button[type="submit"]').click();
  await wait(10);
  assert.deepEqual(confirms, ['/w/p1/build-box-20261116-102030.sh exists. Replace it?']);
  assert.deepEqual(requests.filter(([method]) => method === 'POST').map(([method, address, type, body]) => [address, type, body === script]), [
    ['/_terminal/files/upload?dir=%2Fw%2Fp1&name=build-box-20261116-102030.sh', 'application/octet-stream', true],
    ['/_terminal/files/upload?dir=%2Fw%2Fp1&name=build-box-20261116-102030.sh&replace=1', 'application/octet-stream', true],
  ]);
  assert.equal(saveBox.hidden, true);
  assert.match(status(), /^Saved 7 cards to \/w\/p1\/build-box-20261116-102030\.sh\. Show in the Explorer$/);
  document.querySelector('.transcript-status button').click();
  assert.deepEqual(asked.at(-1), [{ type: 'rotari-transcript-show', path: '/w/p1/build-box-20261116-102030.sh' }, 'http://127.0.0.1:9']);
  // Not replaced when the viewer says no, and a path that is not absolute is refused.
  confirmed = false;
  tool('Save on the server\u2026').click();
  await wait(10);
  saveBox.querySelector('button[type="submit"]').click();
  await wait(10);
  assert.equal(saveBox.querySelector('.transcript-save-note').textContent, 'Not saved: /w/p1/build-box-20261116-102030.sh exists, and was kept.');
  pathField.value = 'notes.sh';
  saveBox.querySelector('button[type="submit"]').click();
  await wait(10);
  assert.match(saveBox.querySelector('.transcript-save-note').textContent, /absolute path/);
  saveBox.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.equal(saveBox.hidden, true);
  // Where the shell is now cannot be told: where it started, and why.
  transcript.shell.id = '8';
  deliver({ type: 'rotari-transcript', transcript });
  await wait(10);
  tool('Save on the server\u2026').click();
  await wait(10);
  assert.equal(pathField.value, '/w/start/build-box-20261116-102030.sh');
  assert.match(saveBox.querySelector('.transcript-save-note').textContent, /^Where the shell is now could not be told \(no such route\); this is where it started\.$/);
  saveBox.querySelector('button:not([type="submit"])').click();
  tool('Check none').click();
  const before = downloads.length;
  tool('Download').click();
  assert.equal(downloads.length, before);
  assert.match(status(), /Check at least one card/);
  // Refresh asks again.
  const asks = asked.filter(([message]) => message.type === 'rotari-transcript-ask').length;
  tool('Refresh').click();
  assert.equal(asked.filter(([message]) => message.type === 'rotari-transcript-ask').length, asks + 1);
  // The page's timers, such as the one that frees a download's URL, are not waited for.
  process.exit(0);
})().catch((error) => { console.error(error); process.exit(1); });
`

// The transcript page asks its opener, hears only it, and shows the cards: styled output,
// folded long outputs, copies, the filter, and the three files, made in the page.
func TestTerminalTranscript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	dir := t.TempDir()
	page := filepath.Join(dir, "transcript.html")
	if err := os.WriteFile(page, []byte(composeTerminalTranscript("n")), 0o600); err != nil {
		t.Fatal(err)
	}
	check := exec.Command("node", "-e", transcriptCheck, page)
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("transcript check failed: %v\n%s", err, output)
	}
}
