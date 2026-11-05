package webui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/joblist"
)

// The panel's own logic, with stand-ins for xterm.js and the socket. That it works
// with the real ones is TestTerminalPanelInBrowser.
func TestTerminalPanel(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	dir := t.TempDir()
	withTerminal := filepath.Join(dir, "with.html")
	without := filepath.Join(dir, "without.html")
	if err := os.WriteFile(withTerminal, []byte(composeWebPage(livePage{nonce: "page-nonce", terminal: true}, nil, false, "", "")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(without, []byte(composeWebPage(livePage{nonce: "page-nonce"}, nil, false, "", "")), 0o600); err != nil {
		t.Fatal(err)
	}
	jobsWithTerminal := filepath.Join(dir, "jobs.html")
	jobRows := []joblist.Row{{State: "success", Project: "p", RunID: "r", JobName: "decode", Command: "sh -c echo…", FullCommand: "sh -c 'echo hi'", AttemptID: "attempt-1"}}
	if err := os.WriteFile(jobsWithTerminal, []byte(jobsPage(livePage{nonce: "page-nonce", terminal: true}, "/", nil, jobRows, jobsSelection{}, true, false, "")), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
const fs = require('fs');
const assert = require('node:assert/strict');
const { JSDOM, VirtualConsole } = require('jsdom');
const webShellCSS = fs.readFileSync(process.argv[3], 'utf8');
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const home = '/home/u';
const plain = (id, number) => ({ id, number, basedir: '', project: '', dir: home, link: '/' });
const inProject = (id, number, project, basedir = '/b') => ({ id, number, basedir, project, dir: '/w/' + project, link: '/project/' + encodeURIComponent(project) });
const load = (file, { stubs = true, stored = '', height = '', hidden = false, status = () => 200, shells = [], remembered = '', limit = 8, path = '/', maximized = false, fontSize = '' } = {}) => {
  const sockets = [], written = [], terminals = [], requests = [], polls = [], alive = [], aliveTimers = [], pasted = [], fits = [];
  const server = { shells: [...shells], lastID: 100, limit, startError: '' };
  const fakeShells = (url, method) => {
    requests.push(method + ' ' + url);
    const refused = (code, text) => Promise.resolve({ ok: false, status: code, text: async () => text });
    if (status() !== 200) return refused(status(), 'refused');
    if (method === 'GET') return Promise.resolve({ ok: true, status: 200, json: async () => ({ shells: [...server.shells], limit: 32, group_limit: server.limit }) });
    if (method === 'POST') {
      if (server.startError) return refused(500, server.startError);
      const project = decodeURIComponent(/project=([^&]*)/.exec(url)?.[1] || '');
      const inGroup = server.shells.filter((shell) => shell.project === project);
      if (inGroup.length >= server.limit) return refused(409, 'at most ' + server.limit);
      let number = 1;
      while (inGroup.some((shell) => shell.number === number)) number++;
      const id = String(++server.lastID);
      const shell = project ? inProject(id, number, project) : plain(id, number);
      server.shells = [...server.shells, shell].sort((a, b) => a.number - b.number);
      return Promise.resolve({ ok: true, status: 201, json: async () => shell });
    }
    server.shells = server.shells.filter((shell) => !url.endsWith('/' + shell.id));
    return Promise.resolve({ ok: true, status: 204 });
  };
  const dom = new JSDOM(fs.readFileSync(file, 'utf8'), {
    runScripts: 'dangerously',
    pretendToBeVisual: true,
    url: 'http://127.0.0.1:9' + path,
    virtualConsole: new VirtualConsole(),
    beforeParse(window) {
      if (stored) window.localStorage.setItem('rotari-terminal-open', stored);
      if (maximized) window.localStorage.setItem('rotari-terminal-maximized', '1');
      if (fontSize) window.localStorage.setItem('rotari-terminal-font-size', fontSize);
      if (remembered) window.sessionStorage.setItem(remembered[0], remembered[1]);
      window.fetch = (url, init = {}) => {
        const address = String(url);
        if (address.includes('xterm-license')) return Promise.resolve({ ok: status() === 200, status: status() });
        if (address === '/_terminal/alive') {
          alive.push(init.method);
          return Promise.resolve({ ok: true, status: 204 });
        }
        if (address.startsWith('/_terminal/shells')) return fakeShells(address, init.method || 'GET');
        if (address.startsWith('/_terminal/here')) {
          requests.push('GET ' + address);
          if (status() !== 200) return Promise.resolve({ ok: false, status: status(), text: async () => 'refused' });
          const project = decodeURIComponent(/project=([^&]*)/.exec(address)?.[1] || '');
          return Promise.resolve({ ok: true, status: 200, json: async () => ({ basedir: project ? '/b' : '', project, dir: project ? '/w/' + project : home }) });
        }
        return new Promise(() => {});
      };
      window.setInterval = (callback, delay) => { if (delay === 5000) polls.push(callback); if (delay === 60000) aliveTimers.push(callback); return 1; };
      window.confirm = () => true;
      window.ResizeObserver = class { observe() {} };
      window.matchMedia = window.matchMedia || (() => ({ matches: false, addEventListener() {} }));
      if (hidden) Object.defineProperty(window.Document.prototype, 'hidden', { configurable: true, get: () => window.__hidden !== false });
      if (height) window.localStorage.setItem('rotari-terminal-height', height);
      window.WebSocket = class {
        static OPEN = 1;
        constructor(url) { this.url = url; this.sent = []; this.readyState = 1; sockets.push(this); }
        send(data) { this.sent.push(data); }
        close() { this.closed = true; }
      };
      if (!stubs) return;
      window.Terminal = class {
        rows = 24; cols = 80;
        buffer = { active: { type: 'normal' } };
        modes = { bracketedPasteMode: false };
        options = {};
        constructor(options) { Object.assign(this.options, options); terminals.push(this); }
        loadAddon() {} open() {} focus() {}
        reset() { written.length = 0; }
        write(data, done) { written.push(Buffer.from(data).toString()); done?.(); }
        paste(text) { pasted.push(text); }
        clear() { this.cleared = (this.cleared || 0) + 1; }
        onData(callback) { this.data = callback; }
        onBinary() {}
        onResize(callback) { this.resize = callback; }
      };
      window.FitAddon = { FitAddon: class { fit() { fits.push(1); } } };
    },
  });
  return { dom, window: dom.window, document: dom.window.document, sockets, written, terminals, requests, polls, server, alive, aliveTimers, pasted, fits };
};
(async () => {
  // Any open page says so, with its panel closed and in the background too, and a page
  // with no terminal says nothing.
  const looking = load(process.argv[1], { hidden: true });
  await wait(30);
  assert.deepEqual(looking.alive, ['POST'], 'an open page did not say so');
  assert.equal(looking.aliveTimers.length, 1);
  looking.aliveTimers[0]();
  assert.equal(looking.alive.length, 2);
  assert.deepEqual(looking.requests, [], 'saying so opened the terminal');

  // Without the option there is nothing to open.
  const off = load(process.argv[2], { stored: '1' });
  await wait(50);
  assert.equal(off.document.getElementById('terminal-button'), null);
  assert.equal(off.sockets.length, 0, 'connected with no terminal on offer');
  assert.deepEqual(off.alive, [], 'a page with no terminal kept shells alive');

  const { window, document, sockets, written, terminals, requests, polls, server, fits } = load(process.argv[1], { limit: 2 });
  await wait(50);
  assert.equal(sockets.length, 0, 'connected before the panel opened');
  assert.equal(document.getElementById('terminal-panel'), null);
  const button = document.getElementById('terminal-button');
  assert.equal(button.getAttribute('aria-pressed'), 'false');

  button.click();
  await wait(20);
  assert.equal(button.getAttribute('aria-pressed'), 'true');
  assert.equal(window.localStorage.getItem('rotari-terminal-open'), '1');
  assert.equal(sockets.length, 1);
  assert.equal(sockets[0].url, 'ws://127.0.0.1:9/_terminal/ws?shell=101&rows=24&cols=80');
  assert.deepEqual(requests, ['GET /_terminal/here?', 'GET /_terminal/shells', 'POST /_terminal/shells?rows=24&cols=80'], 'the first shell was not started on opening');

  // Each wait says what it is waiting for, until output arrives.
  assert.match(document.querySelector('.terminal-status').textContent, /Connecting/);
  sockets[0].onopen();
  assert.match(document.querySelector('.terminal-status').textContent, /Waiting for the shell/);

  // Output, input, and resize.
  sockets[0].onmessage({ data: new TextEncoder().encode('hello').buffer });
  assert.equal(document.querySelector('.terminal-status').hidden, true, 'the wait stayed on screen');
  assert.deepEqual(written, ['hello']);
  terminals[0].data('typed');
  assert.equal(Buffer.from(sockets[0].sent[0]).toString(), 'typed');
  terminals[0].resize({ rows: 30, cols: 100 });
  assert.deepEqual(JSON.parse(sockets[0].sent[1]), { type: 'resize', rows: 30, cols: 100 });

  const status = () => document.querySelector('.terminal-status');
  const action = () => document.querySelector('.terminal-action');
  const add = () => document.querySelector('.terminal-add');
  const tabs = () => [...document.querySelectorAll('.terminal-tab [role=tab]')];
  const tabNames = () => tabs().map((tab) => tab.textContent).join(',');
  const selectedTab = () => tabs().find((tab) => tab.getAttribute('aria-selected') === 'true')?.textContent;
  assert.equal(tabNames(), '1');
  assert.equal(selectedTab(), '1');

  // Another tab took over: it says so, stays quiet, and can take it back.
  sockets[0].onclose({ code: 4001, reason: '' });
  await wait(1200);
  assert.equal(sockets.length, 1, 'fought the other tab for the shell');
  assert.match(status().textContent, /Another tab/);
  action().click();
  assert.equal(sockets.length, 2);
  assert.doesNotMatch(sockets[1].url, /takeover/);
  assert.match(status().textContent, /Connecting/);
  assert.deepEqual(written, [], 'the old screen stayed under the replay');

  // A new tab starts a shell and moves to it, leaving the first one's connection.
  add().click();
  await wait(20);
  assert.equal(tabNames(), '1,2');
  assert.equal(selectedTab(), '2');
  assert.equal(sockets.length, 3);
  assert.match(sockets[2].url, /shell=102&/);
  assert.equal(sockets[1].closed, true, 'stayed connected to the shell no longer shown');
  assert.equal(window.sessionStorage.getItem('rotari-terminal-shell::'), '102');
  assert.equal(add().disabled, true, 'offered a shell past the limit');

  // Choosing a tab connects to its shell and shows its replay alone.
  sockets[2].onmessage({ data: new TextEncoder().encode('second').buffer });
  tabs()[0].click();
  assert.equal(selectedTab(), '1');
  assert.equal(sockets.length, 4);
  assert.match(sockets[3].url, /shell=101&/);
  assert.deepEqual(written, [], 'the other shell stayed on screen');
  assert.equal(window.sessionStorage.getItem('rotari-terminal-shell::'), '101');
  // The connection left behind closing later changes nothing.
  sockets[2].onclose({ code: 1006, reason: '' });
  await wait(1200);
  assert.equal(sockets.length, 4, 'an old connection closing reconnected');

  // Ending a shell in the background leaves the one in use alone.
  document.querySelectorAll('.terminal-tab-close')[1].click();
  await wait(20);
  assert.equal(requests.at(-2), 'DELETE /_terminal/shells/102');
  assert.equal(tabNames(), '1');
  assert.equal(sockets.length, 4);
  assert.notEqual(sockets[3].closed, true);
  assert.equal(add().disabled, false);

  // A shell ending moves to a neighbour, without taking it from a tab using it.
  add().click();
  await wait(20);
  assert.match(sockets[4].url, /shell=103&/);
  server.shells = server.shells.filter((shell) => shell.id !== '103');
  sockets[4].onclose({ code: 4002, reason: 'exit status 3' });
  await wait(20);
  assert.equal(tabNames(), '1');
  assert.match(sockets[5].url, /shell=101&.*takeover=0/);

  // The last shell ending says how, and a new one is a choice.
  server.shells = [];
  sockets[5].onclose({ code: 4002, reason: 'exit status 3' });
  await wait(1200);
  assert.equal(sockets.length, 6, 'started a new shell unasked');
  assert.match(status().textContent, /exit status 3/);
  assert.equal(tabNames(), '');
  assert.equal(action().textContent, 'New shell');
  action().click();
  await wait(20);
  assert.equal(sockets.length, 7);
  assert.equal(tabNames(), '1');

  // A tab's own close asks first, ends it on the server, and waits to hear it ended.
  window.confirm = () => false;
  document.querySelector('.terminal-tab-close').click();
  await wait(20);
  assert.equal(server.shells.length, 1, 'ended a shell without asking');
  window.confirm = () => true;
  document.querySelector('.terminal-tab-close').click();
  await wait(20);
  assert.equal(server.shells.length, 0);
  assert.equal(sockets.length, 7);
  sockets[6].onclose({ code: 4002, reason: 'hangup' });
  await wait(20);
  assert.match(status().textContent, /hangup/);
  action().click();
  await wait(20);
  assert.equal(sockets.length, 8);

  // A shell found gone says so; a failure to show one can be retried.
  server.shells = [];
  sockets[7].onclose({ code: 4005, reason: '' });
  await wait(20);
  assert.match(status().textContent, /has ended/);
  action().click();
  await wait(20);
  sockets[8].onclose({ code: 4000, reason: 'bad size' });
  assert.match(status().textContent, /bad size/);
  action().click();
  assert.equal(sockets.length, 10);

  // A failed start says why, and the shell in use stays.
  server.startError = 'no such shell';
  add().click();
  await wait(20);
  assert.match(status().textContent, /did not start: no such shell/);
  assert.equal(sockets.length, 10);
  server.startError = '';
  action().click();
  await wait(20);
  assert.equal(sockets.length, 11);
  assert.equal(tabNames(), '1,2');

  // Shells started or ended from elsewhere appear.
  server.limit = 3;
  server.shells = [...server.shells, plain('200', 3)];
  polls[0]();
  await wait(20);
  assert.equal(tabNames(), '1,2,3');
  assert.equal(sockets.length, 11, 'noticing a shell moved to it');

  // A lost connection comes back by itself, without taking the shell from a tab.
  const shownNow = /shell=\d+/.exec(sockets[10].url)[0];
  sockets[10].onclose({ code: 1006, reason: '' });
  assert.match(status().textContent, /Reconnecting/);
  await wait(1200);
  assert.equal(sockets.length, 12, 'did not reconnect');
  assert.match(sockets[11].url, /takeover=0/);
  assert.equal(/shell=\d+/.exec(sockets[11].url)[0], shownNow, 'reconnected to another shell');
  assert.doesNotMatch(sockets[0].url, /takeover/, 'opening the panel did not take the shell');

  // Clear empties the screen and the server's replay, and does neither unconnected.
  const clearButton = [...document.querySelectorAll('.terminal-bar button')].find((item) => item.textContent === 'Clear');
  clearButton.click();
  assert.equal(terminals[0].cleared, 1);
  assert.deepEqual(JSON.parse(sockets[11].sent.at(-1)), { type: 'clear' });
  sockets[11].readyState = 0;
  clearButton.click();
  assert.equal(terminals[0].cleared, 1, 'cleared a screen the server would bring back');
  sockets[11].readyState = 1;
  // A full-screen program keeps its screen, and the replay that restores it.
  const sentBefore = sockets[11].sent.length;
  terminals[0].buffer.active.type = 'alternate';
  clearButton.click();
  assert.equal(terminals[0].cleared, 1, 'cleared a full-screen program\'s screen');
  assert.equal(sockets[11].sent.length, sentBefore, 'forgot the replay of a full-screen program');
  assert.match(status().textContent, /full-screen program/);
  terminals[0].buffer.active.type = 'normal';

  // Maximize covers the page, comes back with the next page, and changes no connection.
  const maximizeButton = [...document.querySelectorAll('.terminal-bar button')].find((item) => item.textContent === 'Maximize');
  assert.equal(maximizeButton.getAttribute('aria-pressed'), 'false');
  maximizeButton.click();
  assert.equal(document.documentElement.classList.contains('terminal-maximized'), true);
  assert.equal(maximizeButton.textContent, 'Restore');
  assert.equal(maximizeButton.getAttribute('aria-pressed'), 'true');
  assert.equal(window.localStorage.getItem('rotari-terminal-maximized'), '1');
  assert.equal(sockets.length, 12, 'maximizing reconnected');
  maximizeButton.click();
  assert.equal(document.documentElement.classList.contains('terminal-maximized'), false);
  assert.equal(maximizeButton.textContent, 'Maximize');
  assert.equal(window.localStorage.getItem('rotari-terminal-maximized'), '0');
  const big = load(process.argv[1], { stored: '1', maximized: true });
  await wait(50);
  assert.equal(big.document.documentElement.classList.contains('terminal-maximized'), true, 'the next page lost it');
  assert.equal([...big.document.querySelectorAll('.terminal-bar button')].some((item) => item.textContent === 'Restore'), true);
  assert.match(webShellCSS, /html\.terminal-open\.terminal-maximized #terminal-panel \{\s*top: 0;\s*z-index: 50;/);

  // The text size changes the terminal and fits it again, and stays within limits.
  const fontButton = (text) => [...document.querySelectorAll('.terminal-bar button')].find((item) => item.textContent === text);
  assert.equal(terminals[0].options.fontSize, 13);
  const fitsBefore = fits.length;
  fontButton('A+').click();
  assert.equal(terminals[0].options.fontSize, 14);
  assert.equal(window.localStorage.getItem('rotari-terminal-font-size'), '14');
  assert.equal(fits.length, fitsBefore + 1, 'the terminal did not fit its new text size');
  fontButton('A-').click();
  fontButton('A-').click();
  assert.equal(terminals[0].options.fontSize, 12);
  for (let press = 0; press < 40; press++) fontButton('A-').click();
  assert.equal(terminals[0].options.fontSize, 8, 'the text went under the minimum');
  for (let press = 0; press < 40; press++) fontButton('A+').click();
  assert.equal(terminals[0].options.fontSize, 32, 'the text went over the maximum');
  assert.equal(sockets.length, 12, 'changing the text size reconnected');
  // It comes back with the next page, unless what is saved makes no sense.
  for (const [saved, want] of [['20', 20], ['9000', 13], ['1', 13], ['nope', 13], ['12.5', 13]]) {
    const sized = load(process.argv[1], { stored: '1', fontSize: saved, shells: [plain('1', 1)] });
    await wait(40);
    assert.equal(sized.terminals[0].options.fontSize, want, 'saved text size ' + saved);
  }

  // Hide leaves the shell and the connection alone.
  [...document.querySelectorAll('.terminal-bar button')].find((item) => item.textContent === 'Hide').click();
  assert.equal(document.getElementById('terminal-panel').hidden, true);
  assert.equal(window.localStorage.getItem('rotari-terminal-open'), '0');
  assert.equal(button.getAttribute('aria-pressed'), 'false');
  assert.equal(sockets.length, 12);
  button.click();
  assert.equal(document.getElementById('terminal-panel').hidden, false);
  assert.equal(sockets.length, 12, 'showing the panel again reconnected');

  // A panel left open comes back with the next page.
  const next = load(process.argv[1], { stored: '1' });
  await wait(50);
  assert.equal(next.sockets.length, 0, 'a page opened with the panel left open started a shell');
  assert.deepEqual(next.requests, ['GET /_terminal/here?', 'GET /_terminal/shells']);
  assert.match(next.document.querySelector('.terminal-status').textContent, /No shell here yet/);
  assert.equal(next.document.getElementById('terminal-panel').classList.contains('terminal-empty'), true, 'an empty terminal showed with no shell');
  next.document.querySelector('.terminal-action').click();
  await wait(20);
  assert.equal(next.sockets.length, 1, 'the offer to start a shell did nothing');
  assert.equal(next.document.getElementById('terminal-panel').classList.contains('terminal-empty'), false, 'the terminal stayed out of sight with a shell');

  // The next page lists the shells, and goes back to the tab this browser tab showed.
  const two = [plain('5', 1), plain('6', 2)];
  const returning = load(process.argv[1], { stored: '1', shells: two, remembered: ['rotari-terminal-shell::', '6'] });
  await wait(50);
  assert.equal(returning.sockets.length, 1);
  assert.match(returning.sockets[0].url, /shell=6&/);
  assert.doesNotMatch(returning.sockets[0].url, /takeover/);
  assert.deepEqual(returning.requests, ['GET /_terminal/here?', 'GET /_terminal/shells'], 'started a shell with some running');
  const ended = load(process.argv[1], { stored: '1', shells: two, remembered: ['rotari-terminal-shell::', '99'] });
  await wait(50);
  assert.match(ended.sockets[0].url, /shell=5&/, 'a shell that has gone was not replaced by the first');

  // A saved height comes back, within what leaves the page and the panel usable.
  const tall = load(process.argv[1], { stored: '1', height: '400' });
  await wait(50);
  const heightOf = (page) => page.document.documentElement.style.getPropertyValue('--terminal-saved-height');
  assert.equal(heightOf(tall), '400px');
  assert.equal(heightOf(load(process.argv[1], { height: '50' })), '120px', 'a saved height went under the minimum');
  assert.equal(heightOf(load(process.argv[1], { height: '99999' })), (tall.window.innerHeight - 120) + 'px', 'a saved height left no page');

  // A saved height takes no room while the panel is closed: only an open panel sets the
  // height the page keeps clear of.
  const closedTall = load(process.argv[1], { height: '400' });
  assert.equal(closedTall.document.documentElement.style.getPropertyValue('--terminal-height'), '', 'a closed panel took room');
  assert.match(webShellCSS, /html\.terminal-open \{\s*--terminal-height: var\(--terminal-saved-height, 320px\);/);
  // The top bar's menus open over the panel, and a dialog fits above it.
  const stacking = (selector) => Number(new RegExp(selector + ' \\{[^}]*?z-index: (\\d+)').exec(webShellCSS)[1]);
  assert.ok(stacking('#terminal-panel') < stacking('\\.content > \\.topbar'), 'the panel covers the top bar');
  assert.match(webShellCSS, /html\.terminal-open \.output-panel \{\s*height: min\(760px, 100%\);\s*max-height: 100%;/);

  // The handle moves by pointer and by keyboard, and the result is kept.
  const resizer = tall.document.querySelector('.terminal-resizer');
  assert.equal(resizer.getAttribute('role'), 'separator');
  tall.document.getElementById('terminal-panel').getBoundingClientRect = () => ({ height: 400 });
  resizer.onpointerdown({ preventDefault() {}, clientY: 500, pointerId: 1 });
  resizer.onpointermove({ clientY: 450 });
  assert.equal(heightOf(tall), '450px');
  tall.document.getElementById('terminal-panel').getBoundingClientRect = () => ({ height: 450 });
  resizer.onpointerup();
  assert.equal(tall.window.localStorage.getItem('rotari-terminal-height'), '450');
  assert.equal(resizer.onpointermove, null, 'the drag went on after the button came up');
  resizer.onkeydown({ key: 'ArrowUp', preventDefault() {} });
  assert.equal(heightOf(tall), '474px');
  assert.equal(tall.window.localStorage.getItem('rotari-terminal-height'), '450', 'read the old panel size');

  // Colours come from the tokens, and follow the theme.
  const themed = load(process.argv[1]);
  await wait(30);
  const root = themed.document.documentElement;
  root.style.setProperty('--terminal-background', '#111111');
  root.style.setProperty('--terminal-bright-red', '#222222');
  themed.document.getElementById('terminal-button').click();
  await wait(20);
  assert.equal(themed.terminals[0].options.theme.background, '#111111');
  assert.equal(themed.terminals[0].options.theme.brightRed, '#222222');
  root.style.setProperty('--terminal-background', '#333333');
  root.dataset.theme = 'dark';
  await wait(20);
  assert.equal(themed.terminals[0].options.theme.background, '#333333', 'the theme did not follow');

  // A page with no terminal keeps none of the room it would take.
  await wait(0);
  assert.equal(off.document.documentElement.classList.contains('terminal-open'), false);

  // A tab nobody is looking at neither connects nor retries, so old tabs cannot
  // hold up the tab in use. It acts when shown, and an opened page may take over.
  const background = load(process.argv[1], { stored: '1', hidden: true, shells: [plain('1', 1)] });
  await wait(50);
  assert.equal(background.sockets.length, 0, 'a background tab took the shell');
  assert.deepEqual(background.requests, [], 'a background tab started a shell');
  background.window.__hidden = false;
  background.document.dispatchEvent(new background.window.Event('visibilitychange'));
  await wait(20);
  assert.equal(background.sockets.length, 1);
  assert.doesNotMatch(background.sockets[0].url, /takeover/);
  background.window.__hidden = true;
  background.sockets[0].onclose({ code: 1006, reason: '' });
  await wait(1200);
  assert.equal(background.sockets.length, 1, 'a background tab kept retrying');
  background.window.__hidden = false;
  background.document.dispatchEvent(new background.window.Event('visibilitychange'));
  await wait(1200);
  assert.equal(background.sockets.length, 2, 'a tab did not reconnect when shown');
  assert.match(background.sockets[1].url, /takeover=0/);

  // A sign-in that has gone cannot be retried away: it says so and stops.
  let answer = 200;
  const signedOut = load(process.argv[1], { stored: '1', status: () => answer, shells: [plain('1', 1)] });
  await wait(50);
  answer = 401;
  signedOut.sockets[0].onclose({ code: 1006, reason: '' });
  await wait(1200);
  assert.equal(signedOut.sockets.length, 1, 'kept retrying with no sign-in');
  assert.match(signedOut.document.querySelector('.terminal-status').textContent, /Sign in again/);
  const notSignedIn = load(process.argv[1], { stored: '1', status: () => 401 });
  await wait(50);
  assert.equal(notSignedIn.sockets.length, 0);
  assert.match(notSignedIn.document.querySelector('.terminal-status').textContent, /Sign in again/);

  // A list that cannot be read says why, and can be tried again.
  let listing = 500;
  const unlisted = load(process.argv[1], { stored: '1', status: () => listing, shells: [plain('1', 1)] });
  await wait(50);
  assert.match(unlisted.document.querySelector('.terminal-status').textContent, /could not be listed: refused/);
  listing = 200;
  unlisted.document.querySelector('.terminal-action').click();
  await wait(20);
  assert.equal(unlisted.sockets.length, 1);

  // A project page shows that project's shells, with the others a choice away.
  const hostile = '<b>p2</b>';
  const mixed = () => [inProject('1', 1, 'p1'), inProject('2', 2, 'p1'), inProject('3', 1, hostile), plain('4', 1), inProject('5', 1, 'p1', '/other')];
  const tabsOf = (page) => [...page.document.querySelectorAll('.terminal-tab [role=tab]')].map((tab) => tab.textContent).join(',');
  const options = (page) => [...page.document.querySelectorAll('.terminal-groups option')];
  const project = load(process.argv[1], { stored: '1', path: '/project/p1', shells: mixed(), remembered: ['rotari-terminal-shell:/b:p1', '2'] });
  await wait(50);
  assert.equal(tabsOf(project), '1,2');
  assert.match(project.sockets[0].url, /shell=2&/, 'the project lost its last tab');
  assert.equal(options(project)[0].textContent, 'p1 in /b (2)', 'this page is not first, or a twin is not told apart');
  assert.deepEqual(options(project).slice(1).map((option) => option.textContent), [hostile + ' (1)', 'No project (1)', 'p1 in /other (1)']);
  assert.equal(options(project)[1].value, '/project/' + encodeURIComponent(hostile));
  assert.equal(project.document.querySelector('.terminal-groups b'), null, 'a project name ran as markup');
  assert.equal(project.document.querySelector('.terminal-add').title, 'New shell in /w/p1');
  project.document.querySelector('.terminal-add').click();
  await wait(20);
  assert.match(project.requests.at(-1), /^POST .*rows=24&cols=80&project=p1$/);
  assert.equal(tabsOf(project), '1,2,3');
  assert.equal(project.window.sessionStorage.getItem('rotari-terminal-shell:/b:p1'), '101');

  // Moving to a project with no shell starts none, even with the panel open.
  const quiet = load(process.argv[1], { stored: '1', path: '/project/p3', shells: mixed() });
  await wait(50);
  assert.equal(quiet.sockets.length, 0, 'visiting a project connected to another project\'s shell');
  assert.equal(quiet.requests.some((request) => request.startsWith('POST')), false, 'visiting a project started a shell');
  assert.equal(tabsOf(quiet), '');
  assert.match(quiet.document.querySelector('.terminal-status').textContent, /No shell for p3 yet/);
  // Opening the panel there does start one, in the project.
  const closed = load(process.argv[1], { path: '/project/p3', shells: mixed() });
  await wait(30);
  closed.document.getElementById('terminal-button').click();
  await wait(30);
  assert.match(closed.requests.at(-1), /^POST .*project=p3$/);
  assert.equal(closed.sockets.length, 1);

  // A run page asks for its run, and a mounted basedir for its basedir. Opening the
  // panel with a shell running shows it; the run's own button starts a new one.
  const runPage = load(process.argv[1], { path: '/_basedir/abc/project/p1/run/r%201', shells: mixed() });
  await wait(50);
  runPage.document.getElementById('terminal-button').click();
  await wait(60);
  assert.equal(runPage.requests[0], 'GET /_terminal/here?basedir=abc&project=p1&run=r%201');
  assert.equal(runPage.requests.some((request) => request.startsWith('POST')), false, 'opening the panel started a shell with some running');
  assert.equal(runPage.sockets.length, 1);
  const here = runPage.window.terminalHereIcon();
  assert.match(here, /data-action="openTerminalHere"/);
  const holder = runPage.document.createElement('div');
  holder.innerHTML = here;
  runPage.document.body.append(holder);
  holder.querySelector('button').click();
  await wait(30);
  assert.match(runPage.requests.filter((request) => request.startsWith('POST'))[0], /basedir=abc&project=p1&run=r%201$/);
  assert.equal(runPage.sockets.length, 2);
  // The same button with the panel closed, before anything has loaded.
  const fresh = load(process.argv[1], { path: '/project/p1/run/r', shells: mixed() });
  await wait(30);
  fresh.window.openTerminalHere();
  await wait(30);
  assert.match(fresh.requests.at(-1), /^POST .*project=p1&run=r$/);
  assert.equal(fresh.document.documentElement.classList.contains('terminal-open'), true);
  assert.equal(runPage.requests.every((request) => !/\/_basedir\//.test(request)), true, 'a request took the mounted basedir\'s prefix');
  // A page with no terminal, such as a static export, has no such button.
  assert.equal(off.window.terminalHereIcon(), '');

  // A guide's send button types into the project's shown shell, as one line and with no
  // Enter. With no shell it says so, and starts none.
  const output = (page, socket) => page.sockets[socket].onmessage({ data: new TextEncoder().encode('prompt').buffer });
  const runState = (cwd) => ({ project_name: 'p1', queue: { commands: [] }, runs: [
    { run_id: 'new', status: 'failed', lifecycle: 'finished', started_at: '2026-10-11 05:52:22', jobs: [] },
    { run_id: 'old', status: 'failed', lifecycle: 'finished', started_at: '2026-10-10 05:52:22', cwd, jobs: [] },
  ] });
  const sending = load(process.argv[1], { path: '/project/p1/run/old', stored: '1', shells: mixed() });
  await wait(50);
  sending.window.eval("state = { base_dir: '/b', projects: [] }");
  sending.window.renderRun(runState('/w/o d'), 'old');
  const guide = sending.document.querySelector('#app .execution-guide');
  assert.equal(guide.querySelector('.command-guide-send').dataset.sendText, "cd '/w/o d' && rotari retry -r 'old'", 'the guide sends more than its one command');
  assert.match(guide.dataset.copyText, /JOB_ID/, 'the copied text lost its alternative');
  assert.equal(sending.sockets.length, 1);
  guide.querySelector('.command-guide-send').click();
  await wait(30);
  assert.deepEqual(sending.pasted, ["cd '/w/o d' && rotari retry -r 'old'"]);
  const note = (page) => page.document.querySelector('.terminal-send-note');
  assert.match(note(sending).textContent, /press Enter/);
  assert.equal(note(sending).hidden, false);
  assert.equal(sending.sockets[0].sent.some((data) => String(data).includes('\n')), false);
  const copyOnly = load(process.argv[2]);
  await wait(30);
  copyOnly.window.eval("state = { base_dir: '/b', projects: [] }");
  copyOnly.window.renderRun(runState('/w'), 'old');
  assert.equal(copyOnly.window.copyIconForValue('v', 'x').includes('terminal-send'), false, 'a value offered to send with no terminal');
  assert.equal(copyOnly.window.copyButtonHTML('v', 'x').includes('terminal-send'), false);
  assert.equal(copyOnly.document.querySelector('#app .command-guide-send'), null, 'a page with no terminal offers to send');
  assert.notEqual(copyOnly.document.querySelector('#app .command-guide-copy'), null);

  // Every one-line copy button has one beside it, which sends the value as one word.
  const press = async (page, html) => {
    const holder = page.document.createElement('div');
    holder.innerHTML = html;
    page.document.body.append(holder);
    holder.querySelector('.terminal-send').click();
    await wait(20);
    return page.pasted.at(-1);
  };
  assert.equal(await press(sending, sending.window.copyIconForValue('run-1', 'run ID')), 'run-1');
  assert.equal(await press(sending, sending.window.copyButtonHTML("sh -c 'echo hi'", 'command')), "sh -c 'echo hi'", 'a command was quoted as a word');
  assert.equal(await press(sending, sending.window.copyButtonHTML('/w/o d', 'path')), "'/w/o d'");
  assert.equal(await press(sending, sending.window.copyIconForValue("it's", 'name')), "'it'\\''s'");
  assert.equal(await press(sending, sending.window.copyIconForValue('a;rm -rf "$HOME"<b>', 'name')), "'a;rm -rf \"$HOME\"<b>'");
  const shown = sending.document.querySelector('.terminal-send-note');
  await press(sending, sending.window.copyIconForValue('two\nlines', 'name'));
  assert.match(shown.textContent, /control characters/);
  assert.equal(sending.pasted.at(-1).includes('lines'), false, 'a line break reached the terminal');
  // The Recent jobs page draws its buttons on the server, and gets the same ones.
  const jobsFile = process.argv[4];
  const jobs = load(jobsFile, { path: '/jobs/', stored: '1', shells: [plain('1', 1)] });
  await wait(50);
  const jobsSend = jobs.document.querySelector('.jobs-copy.terminal-send');
  assert.notEqual(jobsSend, null, 'the Recent jobs page offers no send button');
  assert.equal(jobsSend.dataset.action, 'sendToTerminal');
  assert.equal(jobsSend.dataset.copyValue, undefined);
  assert.equal(jobs.document.querySelectorAll('.jobs-copy:not(.terminal-send)').length, jobs.document.querySelectorAll('.jobs-copy.terminal-send').length);
  jobs.sockets[0].onopen?.();
  jobs.document.querySelector('.jobs-copy.terminal-send[aria-label="Send attempt ID to the terminal"]').click();
  await wait(20);
  assert.equal(jobs.pasted.at(-1), 'attempt-1');
  // A command is already a command, so it goes in as it is.
  jobs.document.querySelector('.jobs-copy.terminal-send[aria-label="Send command to the terminal"]').click();
  await wait(20);
  assert.equal(jobs.pasted.at(-1), "sh -c 'echo hi'");

  // A dialog's path goes in as a word, and its log as pasted text, only where the program
  // takes it as pasted text, with nothing left that a terminal would act on.
  const dialogSend = sending.document.getElementById('send-modal');
  const tailSend = sending.document.getElementById('send-tail');
  sending.window.showPath('/job path/j1');
  assert.equal(dialogSend.hidden, false, 'the path has no send button');
  assert.equal(tailSend.hidden, true);
  dialogSend.click();
  await wait(20);
  assert.equal(sending.pasted.at(-1), "'/job path/j1'");
  sending.window.fetchSelectedLog = async (tail) => (tail ? 'tail\r\nline\n' : '\u001b[31mred\u001b[0m\r\nend\u001b[201~\u001b]0;t\u0007\u0003\n');
  sending.document.getElementById('output-modal').dataset.view = 'log';
  sending.window.updateModalActions();
  assert.equal(tailSend.hidden, false, 'a log has no send button for its tail');
  const typedByNow = sending.pasted.length;
  dialogSend.click();
  await wait(20);
  assert.equal(sending.pasted.length, typedByNow, 'several lines reached a program that would run them');
  assert.match(sending.document.querySelector('.terminal-send-note').textContent, /would run each line/);
  sending.terminals[0].modes.bracketedPasteMode = true;
  dialogSend.click();
  await wait(20);
  assert.equal(sending.pasted.at(-1), 'red\nend', 'an escape or colour survived');
  tailSend.click();
  await wait(20);
  assert.equal(sending.pasted.at(-1), 'tail\nline');
  assert.match(await sending.window.sendToTerminal('x\n'.repeat(200000), { pasted: true }), /too long/);
  sending.terminals[0].modes.bracketedPasteMode = false;
  // One line after cleaning needs nothing from the program.
  assert.equal(await sending.window.sendToTerminal('\u001b[1mone\u001b[0m\n', { pasted: true }), null);
  assert.equal(sending.pasted.at(-1), 'one');
  sending.window.closeOutputModal?.();
  // A dialog on a page with no terminal has only its copy buttons.
  copyOnly.window.showPath('/p');
  assert.equal(copyOnly.document.getElementById('send-modal').hidden, true);
  assert.equal(copyOnly.document.getElementById('send-tail').hidden, true);

  // Control characters, a line break among them, are refused.
  const typedBefore = sending.pasted.length;
  for (const bad of ['a\nb', 'a\u001b[2Jb', 'a\u0003', 'a\u009bb']) {
    assert.match(await sending.window.sendToTerminal(bad), /control characters/);
  }
  assert.match(await sending.window.sendToTerminal(''), /nothing to send/);
  assert.equal(sending.pasted.length, typedBefore);

  // No shell for the project: a message, and neither a shell nor another project's.
  const noShell = load(process.argv[1], { path: '/project/p3', shells: mixed() });
  await wait(30);
  assert.match(await noShell.window.sendToTerminal('echo'), /No shell for p3/);
  assert.equal(noShell.requests.some((request) => request.startsWith('POST')), false, 'sending started a shell');
  assert.equal(noShell.sockets.length, 0);
  assert.equal(noShell.document.getElementById('terminal-panel'), null, 'sending opened a panel with nothing to show');
  // The same with the panel open and waiting.
  const noShellOpen = load(process.argv[1], { path: '/project/p3', stored: '1', shells: mixed() });
  await wait(50);
  assert.match(await noShellOpen.window.sendToTerminal('echo'), /No shell for p3/);
  assert.equal(noShellOpen.requests.some((request) => request.startsWith('POST')), false, 'sending started a shell');

  // A closed panel opens and connects without taking the shell, and the command waits for it.
  const later = load(process.argv[1], { path: '/project/p1', shells: mixed() });
  await wait(30);
  const sendingLater = later.window.sendToTerminal('echo later');
  await wait(40);
  assert.equal(later.document.documentElement.classList.contains('terminal-open'), true);
  assert.equal(later.sockets.length, 1);
  assert.match(later.sockets[0].url, /takeover=0/, 'sending took the shell from another browser tab');
  assert.deepEqual(later.pasted, [], 'typed before the shell was connected');
  output(later, 0);
  assert.equal(await sendingLater, null);
  assert.deepEqual(later.pasted, ['echo later']);

  // The shell being used elsewhere stops it, and says so.
  const elsewhere = load(process.argv[1], { path: '/project/p1', shells: mixed() });
  await wait(30);
  const refused = elsewhere.window.sendToTerminal('echo');
  await wait(40);
  elsewhere.sockets[0].onclose({ code: 4001, reason: '' });
  assert.match(await refused, /Another browser tab/);
  assert.deepEqual(elsewhere.pasted, []);
  // So does a hidden panel's lost connection, while one still connected just shows.
  const hiddenPanel = load(process.argv[1], { path: '/project/p1', stored: '1', shells: mixed() });
  await wait(50);
  hiddenPanel.document.getElementById('terminal-button').click();
  assert.equal(hiddenPanel.document.getElementById('terminal-panel').hidden, true);
  assert.equal(await hiddenPanel.window.sendToTerminal('echo shown'), null);
  assert.equal(hiddenPanel.document.getElementById('terminal-panel').hidden, false);
  assert.deepEqual(hiddenPanel.pasted, ['echo shown']);
  assert.equal(hiddenPanel.requests.some((request) => request.startsWith('POST')), false);
  hiddenPanel.sockets[0].onclose({ code: 1006, reason: '' });
  assert.match(await hiddenPanel.window.sendToTerminal('echo gone'), /not connected/);

  // A failure while starting is shown, not left as an empty panel.
  const broken = load(process.argv[1]);
  await wait(30);
  broken.window.Terminal = class { constructor() { throw new Error('xterm broke'); } };
  broken.document.getElementById('terminal-button').click();
  await wait(20);
  assert.match(broken.document.querySelector('.terminal-status').textContent, /could not start: xterm broke/);
  assert.equal(broken.document.querySelector('.terminal-status').hidden, false);

  // xterm.js loads only on opening, under the page's nonce.
  const real = load(process.argv[1], { stubs: false });
  await wait(50);
  const scripts = () => [...real.document.querySelectorAll('script[src]')];
  assert.equal(scripts().length, 0);
  real.document.getElementById('terminal-button').click();
  await wait(20);
  assert.equal(scripts().length, 1);
  assert.equal(scripts()[0].getAttribute('src'), '/_terminal/xterm.js');
  process.exit(0);
})().catch((error) => { console.error(error.stack || String(error)); process.exit(1); });
`
	if output, err := exec.Command("node", "-e", script, withTerminal, without, filepath.Join("assets", "web_shell.css"), jobsWithTerminal).CombinedOutput(); err != nil {
		t.Fatalf("terminal panel check failed: %v\n%s", err, output)
	}
}

// Each theme gives every colour the panel asks for, and the two differ.
func TestTerminalThemeTokens(t *testing.T) {
	names := regexp.MustCompile(`:\s*"([a-z-]+)",?\n`).FindAllStringSubmatch(webShellJS[strings.Index(webShellJS, "const themeTokens"):strings.Index(webShellJS, "const scriptNonce")], -1)
	if len(names) != 20 {
		t.Fatalf("found %d colours in themeTokens, want 20", len(names))
	}
	blocks := map[string]string{}
	darkMedia := strings.Index(webTokensCSS, "@media (prefers-color-scheme: dark)")
	darkChoice := strings.Index(webTokensCSS, `:root[data-theme="dark"] {`)
	blocks["light"] = webTokensCSS[:darkMedia]
	blocks["dark by system"] = webTokensCSS[darkMedia:darkChoice]
	blocks["dark by choice"] = webTokensCSS[darkChoice:]
	value := func(block, name string) string {
		match := regexp.MustCompile(`--terminal-` + name + `:\s*(#[0-9a-f]+);`).FindStringSubmatch(block)
		if match == nil {
			return ""
		}
		return match[1]
	}
	for _, match := range names {
		name := match[1]
		for theme, block := range blocks {
			if value(block, name) == "" {
				t.Errorf("%s has no --terminal-%s", theme, name)
			}
		}
		if got, want := value(blocks["dark by choice"], name), value(blocks["dark by system"], name); got != want {
			t.Errorf("--terminal-%s is %s by choice and %s by system", name, got, want)
		}
		if value(blocks["light"], name) == value(blocks["dark by choice"], name) {
			t.Errorf("--terminal-%s is the same in light and dark", name)
		}
	}
}

// The panel's scripts are served only with the terminal, and pages stay free of them.
func TestTerminalAssetsAreSeparateFromPages(t *testing.T) {
	for _, page := range []string{composeWebPage(livePage{terminal: true}, nil, false, "", ""), jobsPage(livePage{terminal: true}, "/", nil, nil, jobsSelection{}, true, false, "")} {
		if !strings.Contains(page, `id="terminal-button"`) {
			t.Error("a page with the terminal has no Terminal button")
		}
		if strings.Contains(page, "xterm.js authors") {
			t.Error("a page carries xterm.js itself")
		}
	}
	if strings.Contains(composeWebHTML(nil, false, ""), `id="terminal-button"`) {
		t.Error("a page without the terminal has a Terminal button")
	}
}
