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
const load = (file, { stubs = true, stored = '', height = '', hidden = false, status = () => 200, shells = [], remembered = '', split = '', limit = 8, path = '/', maximized = false, fontSize = '' } = {}) => {
  const sockets = [], written = [], terminals = [], requests = [], polls = [], alive = [], aliveTimers = [], pasted = [], fits = [];
  // xterm.js parses what it is given later; a test can hold that back.
  const parser = { held: false, waiting: [], release() { this.held = false; this.waiting.splice(0).forEach((parse) => parse()); } };
  const server = { shells: [...shells], lastID: 100, limit, startError: '' };
  const fakeShells = (url, method, body) => {
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
      server.shells = [...server.shells, shell];
      return Promise.resolve({ ok: true, status: 201, json: async () => shell });
    }
    if (method === 'PATCH') {
      const shell = server.shells.find((item) => url.endsWith('/' + item.id));
      shell.name = JSON.parse(body).name;
      return Promise.resolve({ ok: true, status: 200, json: async () => shell });
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
      if (split) window.sessionStorage.setItem(split[0], split[1]);
      window.fetch = (url, init = {}) => {
        const address = String(url);
        if (address.includes('xterm-license')) {
          const answer = { ok: status() === 200, status: status() };
          return server.licenseHeld ? new Promise((resolve) => server.licenseHeld.push(() => resolve(answer))) : Promise.resolve(answer);
        }
        if (address === '/_terminal/alive') {
          alive.push(init.method);
          return Promise.resolve({ ok: true, status: 204 });
        }
        if (address === '/_terminal/order') {
          requests.push('PUT /_terminal/order');
          if (server.orderStatus) return Promise.resolve({ ok: false, status: server.orderStatus, text: async () => 'refused' });
          const ids = JSON.parse(init.body).ids;
          const named = ids.map((id) => server.shells.find((shell) => shell.id === id)).filter(Boolean);
          server.shells = [...named, ...server.shells.filter((shell) => !ids.includes(shell.id))];
          return Promise.resolve({ ok: true, status: 204 });
        }
        if (address.startsWith('/_terminal/shells')) return fakeShells(address, init.method || 'GET', init.body);
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
      window.ResizeObserver = class { observe() {} disconnect() {} };
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
        loadAddon(addon) { (this.addons ||= []).push(addon); } open() {} focus() { this.focuses = (this.focuses || 0) + 1; }
        reset() { written.length = 0; this.writes = []; }
        dispose() { this.disposed = true; }
        write(data, done) {
          const parse = () => { written.push(Buffer.from(data).toString()); (this.writes ||= []).push(Buffer.from(data).toString()); done?.(); };
          if (parser.held) parser.waiting.push(parse);
          else parse();
        }
        paste(text) { pasted.push(text); (this.pastes ||= []).push(text); }
        clear() { this.cleared = (this.cleared || 0) + 1; }
        onData(callback) { this.data = callback; }
        onBinary() {}
        onResize(callback) { this.resize = callback; }
      };
      window.FitAddon = { FitAddon: class { fit() { fits.push(1); } } };
      window.WebLinksAddon = { WebLinksAddon: class { constructor(handler) { this.handler = handler; } } };
    },
  });
  return { dom, window: dom.window, document: dom.window.document, sockets, written, terminals, requests, polls, server, alive, aliveTimers, pasted, fits, parser };
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

  // A link opens in a new tab that gets no hold on this page.
  const opened = [];
  window.open = (...args) => opened.push(args);
  terminals[0].addons.find((addon) => addon.handler).handler({}, 'https://example.test/a');
  assert.deepEqual(opened, [['https://example.test/a', '_blank', 'noopener']]);

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
  // The panel is a <section>, and the page's rule for those would make it a card that stops short of the window's edges.
  assert.match(webShellCSS, /\n#terminal-panel \{[^}]*margin: 0;\s*padding: 0;\s*border: 0;\s*border-top: 1px solid var\(--line\);\s*border-radius: 0;\s*box-shadow: none;/);

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

  // A tab's menu renames it, closes those to its right, and closes them all, in this
  // project only, asking once and ending the shown one last.
  const menuPage = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [
    inProject('1', 1, 'p1'), inProject('2', 2, 'p1'), inProject('3', 3, 'p1'), inProject('4', 4, 'p1'), inProject('9', 1, 'p2'),
  ], remembered: ['rotari-terminal-shell:/b:p1', '4'] });
  await wait(50);
  const mw = menuPage.window, md = menuPage.document;
  const tabEls = () => [...md.querySelectorAll('.terminal-tab')];
  const menuItem = (label) => [...md.querySelectorAll('.terminal-tab-menu button')].find((item) => item.textContent === label);
  const openMenu = (index) => {
    const event = new mw.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 });
    tabEls()[index].dispatchEvent(event);
    assert.equal(event.defaultPrevented, true, 'the browser menu opened too');
  };
  openMenu(0);
  assert.equal(md.querySelector('.terminal-tab-menu').getAttribute('role'), 'menu');
  md.body.dispatchEvent(new mw.Event('pointerdown', { bubbles: true }));
  assert.equal(md.querySelector('.terminal-tab-menu'), null, 'clicking away left the menu');
  openMenu(0);
  md.dispatchEvent(new mw.KeyboardEvent('keydown', { key: 'Escape' }));
  assert.equal(md.querySelector('.terminal-tab-menu'), null, 'Escape left the menu');
  // Renaming; markup stays text, and cancelling changes nothing.
  mw.prompt = () => null;
  openMenu(0);
  menuItem('Rename...').click();
  await wait(20);
  assert.equal(menuPage.requests.some((request) => request.startsWith('PATCH')), false, 'cancelling renamed the tab');
  mw.prompt = () => '<b>build</b>';
  openMenu(0);
  menuItem('Rename...').click();
  await wait(20);
  assert.equal(md.querySelector('.terminal-tab [role=tab]').textContent, '<b>build</b>');
  assert.equal(md.querySelector('.terminal-tab b'), null, 'a tab name ran as markup');
  // The last tab has nothing to its right.
  openMenu(3);
  assert.equal(menuItem('Close tabs to the right').disabled, true);
  md.dispatchEvent(new mw.KeyboardEvent('keydown', { key: 'Escape' }));
  // Declining the question ends nothing.
  let asked = [];
  mw.confirm = (text) => { asked.push(text); return false; };
  openMenu(1);
  menuItem('Close tabs to the right').click();
  await wait(20);
  assert.deepEqual(asked, ['End 2 shells, and anything running in them?']);
  assert.equal(menuPage.server.shells.length, 5, 'declining ended a shell');
  // Closing to the right of tab 2 ends 3 and the shown 4, 4 last, and moves to a neighbour.
  mw.confirm = () => true;
  openMenu(1);
  menuItem('Close tabs to the right').click();
  await wait(30);
  const deleted = () => menuPage.requests.filter((request) => request.startsWith('DELETE'));
  assert.deepEqual(deleted(), ['DELETE /_terminal/shells/3', 'DELETE /_terminal/shells/4']);
  assert.equal(menuPage.server.shells.some((shell) => shell.id === '9'), true);
  menuPage.sockets[0].onclose({ code: 4002, reason: 'hangup' });
  await wait(30);
  assert.equal(tabEls().length, 2);
  assert.match(menuPage.sockets[1].url, /shell=2&/);
  // Closing all ends this project's, and not another's.
  openMenu(0);
  menuItem('Close all tabs').click();
  await wait(30);
  assert.deepEqual(deleted().slice(2), ['DELETE /_terminal/shells/1', 'DELETE /_terminal/shells/2']);
  assert.deepEqual(menuPage.server.shells.map((shell) => shell.id), ['9'], 'closing all reached another project');

  // Dragging a tab, or the menu, arranges this project's tabs, on the server too, and the
  // menu's and neighbours' idea of right and left follows.
  const arranging = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [
    inProject('1', 1, 'p1'), inProject('2', 2, 'p1'), inProject('3', 3, 'p1'), inProject('9', 1, 'p2'),
  ], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  await wait(50);
  const ad = arranging.document, aw = arranging.window;
  const arrangedTabs = () => [...ad.querySelectorAll('.terminal-tab')];
  const arrangedNames = () => arrangedTabs().map((tab) => tab.querySelector('[role=tab]').textContent).join(',');
  const dragEvent = (type, clientX) => {
    const event = new aw.Event(type, { bubbles: true, cancelable: true });
    event.clientX = clientX;
    return event;
  };
  assert.equal(arrangedTabs()[0].draggable, true);
  const box = (tab) => { tab.getBoundingClientRect = () => ({ left: 100, width: 40, top: 0, bottom: 10 }); return tab; };
  const drag = (from, onto, clientX) => {
    const target = box(arrangedTabs()[onto]);
    arrangedTabs()[from].dispatchEvent(dragEvent('dragstart', 0));
    target.dispatchEvent(dragEvent('dragover', clientX));
    const drop = dragEvent('drop', clientX);
    target.dispatchEvent(drop);
    return target;
  };
  // Over a tab's right half lands after it, and over its left half before it.
  arrangedTabs()[1].dispatchEvent(dragEvent('dragstart', 0));
  const over = box(arrangedTabs()[2]);
  const overEvent = dragEvent('dragover', 130);
  over.dispatchEvent(overEvent);
  assert.equal(overEvent.defaultPrevented, true, 'the tab refused the drop');
  assert.equal(over.classList.contains('terminal-drop-after'), true);
  over.dispatchEvent(dragEvent('dragover', 110));
  assert.equal(over.classList.contains('terminal-drop-before'), true);
  assert.equal(over.classList.contains('terminal-drop-after'), false);
  // A list arriving during a drag does not end it.
  arranging.polls[0]();
  await wait(20);
  assert.equal(arrangedTabs()[2], over, 'the tabs were drawn again during a drag');
  arrangedTabs()[1].dispatchEvent(dragEvent('dragend', 0));
  assert.equal(over.classList.contains('terminal-drop-before'), false);
  // Dropping moves it, at once and on the server.
  drag(0, 2, 130);
  assert.equal(arrangedNames(), '2,3,1');
  await wait(20);
  assert.deepEqual(arranging.server.shells.map((shell) => shell.id), ['2', '3', '1', '9'], 'the server did not hear, or heard another project');
  drag(2, 0, 110);
  assert.equal(arrangedNames(), '1,2,3');
  await wait(20);
  // Dropping a tab on itself changes nothing.
  const putsBefore = arranging.requests.filter((request) => request.startsWith('PUT')).length;
  drag(1, 1, 130);
  await wait(20);
  assert.equal(arranging.requests.filter((request) => request.startsWith('PUT')).length, putsBefore);
  // The shown shell stays connected through all of it.
  assert.equal(arranging.sockets.length, 1, 'arranging tabs reconnected');
  assert.equal(arranging.sockets[0].closed, undefined);
  // The menu moves one place, and knows the ends.
  const arrangeMenu = (index) => arrangedTabs()[index].dispatchEvent(new aw.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
  const arrangeItem = (label) => [...ad.querySelectorAll('.terminal-tab-menu button')].find((item) => item.textContent === label);
  arrangeMenu(0);
  assert.equal(arrangeItem('Move left').disabled, true);
  arrangeItem('Move right').click();
  assert.equal(arrangedNames(), '2,1,3');
  await wait(20);
  arrangeMenu(2);
  assert.equal(arrangeItem('Move right').disabled, true);
  arrangeItem('Move left').click();
  assert.equal(arrangedNames(), '2,3,1');
  await wait(20);
  // Closing to the right follows the arranged order, not the numbers.
  aw.confirm = () => true;
  arrangeMenu(0);
  arrangeItem('Close tabs to the right').click();
  await wait(30);
  assert.equal(arranging.server.shells.some((shell) => shell.id === '3'), false);
  assert.equal(arranging.server.shells.some((shell) => shell.id === '2'), true, 'closed the tab it was asked to keep');
  // A refusal puts back what the server has, and says so.
  const refusedOrder = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1')] });
  await wait(50);
  refusedOrder.server.orderStatus = 500;
  refusedOrder.document.querySelectorAll('.terminal-tab')[0].dispatchEvent(new refusedOrder.window.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
  [...refusedOrder.document.querySelectorAll('.terminal-tab-menu button')].find((item) => item.textContent === 'Move right').click();
  await wait(30);
  assert.equal(tabsOf(refusedOrder), '1,2', 'a refused order stayed on screen');
  assert.match(refusedOrder.document.querySelector('.terminal-status').textContent, /not rearranged/);

  // A fast program's output does not reach the next shell's screen, whether it is still
  // being parsed or still arriving from the connection left behind.
  const fast = load(process.argv[1], { stored: '1', shells: [plain('1', 1), plain('2', 2)], remembered: ['rotari-terminal-shell::', '1'] });
  await wait(50);
  const fastTabs = () => [...fast.document.querySelectorAll('.terminal-tab [role=tab]')];
  const bytes = (text) => ({ data: new TextEncoder().encode(text).buffer });
  fast.parser.held = true;
  fast.sockets[0].onmessage(bytes('y\r\n'));
  fastTabs()[1].click();
  assert.equal(fast.sockets.length, 1, 'connected with the old output still being parsed');
  assert.match(fast.document.querySelector('.terminal-status').textContent, /Connecting/);
  assert.equal(fast.sockets[0].closed, true);
  fast.sockets[0].onmessage(bytes('late\r\n'));
  fast.parser.release();
  assert.equal(fast.sockets.length, 2);
  assert.match(fast.sockets[1].url, /shell=2&/);
  assert.deepEqual(fast.written, [], 'the old shell\'s output stayed on screen');
  fast.sockets[0].onmessage(bytes('later\r\n'));
  fast.sockets[1].onmessage(bytes('second\r\n'));
  assert.deepEqual(fast.written, ['second\r\n']);
  assert.equal(fast.document.querySelector('.terminal-status').hidden, true);
  // Moving on twice while waiting connects once, to the last.
  fast.parser.held = true;
  fastTabs()[0].click();
  fastTabs()[1].click();
  fast.parser.release();
  assert.equal(fast.sockets.length, 3);
  assert.match(fast.sockets[2].url, /shell=2&/);
  // A command sent while waiting goes in once the shell shows something.
  fast.parser.held = true;
  fastTabs()[0].click();
  const sentWhileWaiting = fast.window.sendToTerminal('echo waited');
  await wait(20);
  fast.parser.release();
  await wait(20);
  fast.sockets[3].onmessage(bytes('prompt'));
  assert.equal(await sentWhileWaiting, null);
  assert.deepEqual(fast.pasted, ['echo waited']);

  // A tab marks output nobody has seen, and the list marks a project with any. A shell
  // ending out of sight is told of, and one ended from here is not.
  const noticed = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [
    inProject('1', 1, 'p1'), inProject('2', 2, 'p1'), inProject('3', 3, 'p1'), inProject('7', 1, 'p2'),
  ], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  await wait(50);
  const nd = noticed.document;
  const unseenTabs = () => [...nd.querySelectorAll('.terminal-tab-unseen [role=tab]')].map((tab) => tab.textContent);
  const shellOf = (id) => noticed.server.shells.find((shell) => shell.id === id);
  assert.deepEqual(unseenTabs(), []);
  shellOf('2').unseen_output = true;
  shellOf('1').unseen_output = true; // The shown tab, whose flag is a moment behind.
  noticed.polls[0]();
  await wait(20);
  assert.deepEqual(unseenTabs(), ['2']);
  assert.match(nd.querySelectorAll('.terminal-tab [role=tab]')[1].getAttribute('aria-label'), /new output/);
  shellOf('2').unseen_output = false;
  noticed.polls[0]();
  await wait(20);
  assert.deepEqual(unseenTabs(), []);
  assert.deepEqual([...nd.querySelectorAll('.terminal-groups option')].map((option) => option.textContent), ['p1 (3)', 'p2 (1)']);
  shellOf('7').unseen_output = true;
  noticed.polls[0]();
  await wait(20);
  assert.equal(nd.querySelectorAll('.terminal-groups option')[1].textContent, '\u2022 p2 (1)');
  noticed.server.shells = noticed.server.shells.filter((shell) => shell.id !== '3');
  noticed.polls[0]();
  await wait(20);
  assert.match(nd.querySelector('.terminal-send-note').textContent, /Shell 3 of p1 ended/);
  nd.querySelector('.terminal-send-note').textContent = '';
  nd.querySelectorAll('.terminal-tab-close')[1].click();
  await wait(20);
  noticed.polls[0]();
  await wait(20);
  assert.equal(nd.querySelector('.terminal-send-note').textContent, '', 'ending a shell from here told of it');
  assert.equal(noticed.sockets.length, 1, 'noticing output moved to another tab');

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

  // Two panes. Each row of what happens is checked with two or three shells of a project.
  const splitShells = (...ids) => ids.map((id, index) => inProject(String(id), index + 1, 'p1'));
  const shownKey = 'rotari-terminal-shell:/b:p1', splitKey = 'rotari-terminal-split:/b:p1';
  const openSplit = async (ids, options = {}) => {
    const page = load(process.argv[1], { stored: '1', path: '/project/p1', shells: splitShells(...ids), remembered: [shownKey, String(ids[0])], ...options });
    await wait(50);
    return page;
  };
  const sp = (page) => {
    const bar = (text) => [...page.document.querySelectorAll('.terminal-bar button')].find((item) => item.textContent === text);
    const panes = () => [...page.document.querySelectorAll('.terminal-pane')];
    const tabEl = (name) => [...page.document.querySelectorAll('.terminal-tab')].find((tab) => tab.querySelector('[role=tab]').textContent === name);
    return {
      bar, panes,
      split: () => bar('Split').click(),
      tab: (name) => tabEl(name).querySelector('[role=tab]').click(),
      selected: () => page.document.querySelector('.terminal-tab [aria-selected=true]')?.textContent,
      otherPane: () => [...page.document.querySelectorAll('.terminal-tab-other-pane [role=tab]')].map((tab) => tab.textContent),
      live: () => page.sockets.filter((socket) => !socket.closed).map((socket) => /shell=(\d+)/.exec(socket.url)[1]),
      focus: (index) => panes()[index].dispatchEvent(new page.window.Event('focusin', { bubbles: true })),
      statusOf: (index) => panes()[index].querySelector('.terminal-status'),
      saved: () => JSON.parse(page.window.sessionStorage.getItem(splitKey)),
      inOrder: () => panes().sort((a, b) => Number(a.style.order) - Number(b.style.order)),
    };
  };
  // The server closing a connection, which the stand-in does not record by itself.
  const hangUp = (socket, code, reason) => { socket.closed = true; socket.onclose({ code, reason }); };
  const out = (text) => ({ data: new TextEncoder().encode(text).buffer });

  // Split shows the next shell in a new pane, which takes focus, and leaves the first alone.
  const splitting = await openSplit([1, 2, 3]);
  let ui = sp(splitting);
  assert.equal(ui.panes().length, 1);
  assert.equal(ui.bar('Split').getAttribute('aria-pressed'), 'false');
  assert.equal(splitting.document.querySelector('.terminal-bar .terminal-status'), null, 'a status stayed in the bar');
  ui.split();
  await wait(20);
  assert.equal(ui.panes().length, 2);
  assert.equal(splitting.terminals.length, 2);
  assert.deepEqual(ui.live(), ['1', '2']);
  assert.equal(ui.selected(), '2');
  assert.deepEqual(ui.otherPane(), ['1']);
  assert.equal(ui.bar('Unsplit').getAttribute('aria-pressed'), 'true');
  assert.deepEqual(ui.saved(), { shell: '2', focused: true, share: 0.5 });
  assert.equal(ui.panes()[1].classList.contains('terminal-pane-focused'), true);
  assert.equal(ui.panes()[0].classList.contains('terminal-pane-focused'), false);
  assert.equal(splitting.requests.some((request) => request.startsWith('POST')), false, 'split started a shell with one to spare');

  // Each pane's output, input, and size go to its own shell.
  splitting.sockets[0].onmessage(out('first'));
  splitting.sockets[1].onmessage(out('second'));
  assert.deepEqual(splitting.terminals[0].writes, ['first']);
  assert.deepEqual(splitting.terminals[1].writes, ['second']);
  assert.equal(ui.statusOf(0).hidden, true);
  splitting.terminals[0].data('a');
  splitting.terminals[1].data('b');
  assert.equal(Buffer.from(splitting.sockets[0].sent.at(-1)).toString(), 'a');
  assert.equal(Buffer.from(splitting.sockets[1].sent.at(-1)).toString(), 'b');
  splitting.terminals[1].resize({ rows: 9, cols: 9 });
  assert.deepEqual(JSON.parse(splitting.sockets[1].sent.at(-1)), { type: 'resize', rows: 9, cols: 9 });
  assert.equal(splitting.sockets[0].sent.length, 1, 'a resize reached the other shell');

  // Choosing a tab that is not shown changes the focused pane; choosing the other pane's swaps, connecting nothing.
  ui.tab('3');
  assert.deepEqual(ui.live(), ['1', '3']);
  assert.equal(ui.selected(), '3');
  assert.deepEqual(ui.otherPane(), ['1']);
  const socketsBeforeSwap = splitting.sockets.length;
  const [leftBefore, rightBefore] = ui.inOrder();
  ui.tab('1');
  assert.equal(splitting.sockets.length, socketsBeforeSwap, 'swapping reconnected');
  assert.equal(ui.selected(), '1');
  assert.deepEqual(ui.otherPane(), ['3']);
  assert.deepEqual(ui.inOrder(), [rightBefore, leftBefore], 'the panes did not change places');
  assert.equal(ui.inOrder()[1].classList.contains('terminal-pane-focused'), true, 'focus left the place it was in');
  assert.equal(splitting.window.sessionStorage.getItem(shownKey), '3');
  assert.deepEqual(ui.saved(), { shell: '1', focused: true, share: 0.5 });
  // Choosing the focused pane's own tab changes nothing.
  ui.tab('1');
  assert.equal(splitting.sockets.length, socketsBeforeSwap);
  // A shell in a pane is never marked unseen, and one in neither can be.
  const unseenShell = (id) => splitting.server.shells.find((shell) => shell.id === id);
  unseenShell('1').unseen_output = unseenShell('3').unseen_output = unseenShell('2').unseen_output = true;
  splitting.polls[0]();
  await wait(20);
  assert.deepEqual([...splitting.document.querySelectorAll('.terminal-tab-unseen [role=tab]')].map((tab) => tab.textContent), ['2']);
  // + starts a shell in the focused pane.
  splitting.document.querySelector('.terminal-add').click();
  await wait(20);
  assert.equal(ui.selected(), '4');
  assert.deepEqual(ui.live(), ['3', '101']);

  // Following focus: clicking into a pane moves it, without connecting, and the bar follows.
  ui.focus(0);
  assert.equal(ui.panes()[0].classList.contains('terminal-pane-focused'), true);
  assert.equal(ui.panes()[1].classList.contains('terminal-pane-focused'), false);
  assert.equal(ui.saved().focused, ui.inOrder()[1] === ui.panes()[0]);

  // Sending and Clear act on the focused pane, and Clear skips a full-screen program there alone.
  const focusing = await openSplit([1, 2]);
  ui = sp(focusing);
  ui.split();
  await wait(20);
  assert.equal(await focusing.window.sendToTerminal('to second'), null);
  assert.deepEqual(focusing.terminals[1].pastes, ['to second']);
  assert.equal(focusing.terminals[0].pastes, undefined, 'sent to the pane without focus');
  ui.focus(0);
  assert.equal(await focusing.window.sendToTerminal('to first'), null);
  assert.deepEqual(focusing.terminals[0].pastes, ['to first']);
  assert.deepEqual(focusing.terminals[1].pastes, ['to second']);
  ui.bar('Clear').click();
  assert.equal(focusing.terminals[0].cleared, 1);
  assert.equal(focusing.terminals[1].cleared, undefined);
  ui.focus(1);
  focusing.terminals[1].buffer.active.type = 'alternate';
  const clearsBefore = focusing.sockets[1].sent.length;
  ui.bar('Clear').click();
  assert.equal(focusing.terminals[1].cleared, undefined, 'cleared a full-screen program');
  assert.equal(focusing.sockets[1].sent.length, clearsBefore);
  assert.match(ui.statusOf(1).textContent, /full-screen/);
  assert.doesNotMatch(ui.statusOf(0).textContent, /full-screen/, 'the message reached the other pane');
  // Text size and theme reach both terminals, and both fit.
  const fitsAtSplit = focusing.fits.length;
  ui.bar('A+').click();
  assert.deepEqual(focusing.terminals.map((term) => term.options.fontSize), [14, 14]);
  assert.equal(focusing.fits.length, fitsAtSplit + 2, 'a pane did not fit its new text size');
  focusing.document.documentElement.style.setProperty('--terminal-background', '#444444');
  focusing.document.documentElement.dataset.theme = 'dark';
  await wait(20);
  assert.deepEqual(focusing.terminals.map((term) => term.options.theme.background), ['#444444', '#444444']);
  // Maximize changes no connection.
  const socketsAtMaximize = focusing.sockets.length;
  ui.bar('Maximize').click();
  assert.equal(focusing.sockets.length, socketsAtMaximize);
  ui.bar('Restore').click();

  // A pane's connection problem is its own: the other keeps its state, its retry, and a waiting command.
  const trouble = await openSplit([1, 2]);
  ui = sp(trouble);
  ui.split();
  await wait(20);
  trouble.sockets[0].onmessage(out('x'));
  hangUp(trouble.sockets[0], 4001, '');
  assert.match(ui.statusOf(0).textContent, /Another tab/);
  assert.equal(ui.statusOf(1).textContent.includes('Another tab'), false, 'the other pane said it was taken');
  assert.equal(trouble.sockets[1].closed, undefined);
  const waitingCommand = trouble.window.sendToTerminal('waits');
  await wait(20);
  trouble.sockets[1].readyState = 0;
  const secondWaits = trouble.window.sendToTerminal('second');
  await wait(20);
  assert.equal(await waitingCommand, null, 'an open connection did not take the command');
  ui.panes()[0].querySelector('.terminal-action').click();
  await wait(20);
  assert.equal(trouble.sockets.length, 3, 'taking it back did not connect');
  assert.match(trouble.sockets[2].url, /shell=1&/);
  // That connection opening leaves the second pane's command waiting, and its own output delivers it.
  trouble.sockets[2].onmessage(out('prompt'));
  assert.deepEqual(trouble.terminals[1].pastes, ['waits']);
  trouble.sockets[1].readyState = 1;
  trouble.sockets[1].onmessage(out('prompt'));
  assert.equal(await secondWaits, null);
  assert.deepEqual(trouble.terminals[1].pastes, ['waits', 'second']);
  // A lost connection reconnects by itself, to its own shell, and leaves the other.
  hangUp(trouble.sockets[1], 1006, '');
  assert.match(ui.statusOf(1).textContent, /Reconnecting/);
  assert.equal(ui.statusOf(0).hidden, true);
  await wait(1200);
  assert.equal(trouble.sockets.length, 4);
  assert.match(trouble.sockets[3].url, /shell=2&.*takeover=0/);
  assert.notEqual(trouble.sockets[2].closed, true);

  // A pane's shell ends: it moves to one not shown, without taking it, and focus stays.
  const ending = await openSplit([1, 2, 3]);
  ui = sp(ending);
  ui.split();
  await wait(20);
  ui.focus(1);
  const focusesBefore = ending.terminals[0].focuses;
  ending.server.shells = ending.server.shells.filter((shell) => shell.id !== '1');
  hangUp(ending.sockets[0], 4002, 'exit status 3');
  await wait(30);
  assert.deepEqual(ui.live(), ['2', '3']);
  assert.match(ending.sockets[2].url, /shell=3&.*takeover=0/);
  assert.equal(ui.panes().length, 2);
  assert.equal(ui.selected(), '2', 'the pane that moved on took focus');
  assert.equal(ending.terminals[0].focuses, focusesBefore, 'the pane that moved on took the keys');
  assert.deepEqual(ui.otherPane(), ['3']);
  // With every other shell in the other pane, the split closes and the other pane stays connected.
  ending.server.shells = ending.server.shells.filter((shell) => shell.id !== '3');
  hangUp(ending.sockets[2], 4002, 'exit status 4');
  await wait(30);
  assert.equal(ui.panes().length, 1);
  assert.equal(ending.document.querySelector('.terminal-split-handle'), null);
  assert.equal(ending.terminals[0].disposed, true);
  assert.equal(ending.terminals[1].disposed, undefined);
  assert.deepEqual(ui.live(), ['2']);
  assert.equal(ending.sockets[1].closed, undefined, 'the split closing disturbed the other pane');
  assert.equal(ui.selected(), '2');
  assert.equal(ui.saved(), null);
  assert.equal(ui.bar('Split').disabled, false);
  // The focused pane's shell ending with the other holding the rest gives focus to the other.
  const focusedEnds = await openSplit([1, 2]);
  ui = sp(focusedEnds);
  ui.split();
  await wait(20);
  focusedEnds.server.shells = focusedEnds.server.shells.filter((shell) => shell.id !== '2');
  hangUp(focusedEnds.sockets[1], 4002, 'done');
  await wait(30);
  assert.equal(ui.panes().length, 1);
  assert.equal(ui.selected(), '1');
  assert.equal(await focusedEnds.window.sendToTerminal('still here'), null);
  assert.deepEqual(focusedEnds.terminals[0].pastes, ['still here']);
  // The last shell ending is as it always was.
  focusedEnds.server.shells = [];
  hangUp(focusedEnds.sockets[0], 4002, 'exit status 5');
  await wait(30);
  assert.match(ui.statusOf(0).textContent, /exit status 5/);
  assert.equal(ui.panes()[0].querySelector('.terminal-action').textContent, 'New shell');
  assert.equal(ui.bar('Split').disabled, true, 'offered to split nothing');

  // Both ending at once leaves one pane that says how, and no error.
  const bothEnd = await openSplit([1, 2]);
  ui = sp(bothEnd);
  ui.split();
  await wait(20);
  bothEnd.server.shells = [];
  hangUp(bothEnd.sockets[0], 4002, 'one');
  hangUp(bothEnd.sockets[1], 4002, 'two');
  await wait(30);
  assert.equal(ui.panes().length, 1);
  assert.match(ui.statusOf(0).textContent, /one|two/);
  assert.equal(ui.panes()[0].querySelector('.terminal-action').textContent, 'New shell');

  // Close all ends those not shown, then the other pane's, then the focused pane's.
  const closingAll = await openSplit([1, 2, 3, 4]);
  ui = sp(closingAll);
  ui.split();
  await wait(20);
  ui.focus(0);
  closingAll.document.querySelectorAll('.terminal-tab')[0].dispatchEvent(new closingAll.window.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
  [...closingAll.document.querySelectorAll('.terminal-tab-menu button')].find((item) => item.textContent === 'Close all tabs').click();
  await wait(40);
  assert.deepEqual(closingAll.requests.filter((request) => request.startsWith('DELETE')), ['DELETE /_terminal/shells/3', 'DELETE /_terminal/shells/4', 'DELETE /_terminal/shells/2', 'DELETE /_terminal/shells/1']);
  // Closing a tab shown in a pane waits for its connection to say so.
  const closingOne = await openSplit([1, 2, 3]);
  ui = sp(closingOne);
  ui.split();
  await wait(20);
  closingOne.document.querySelectorAll('.terminal-tab-close')[0].click();
  await wait(20);
  assert.equal(closingOne.sockets[0].closed, undefined);
  hangUp(closingOne.sockets[0], 4002, 'hangup');
  await wait(30);
  assert.deepEqual(ui.live(), ['2', '3']);

  // Unsplit closes the other pane and leaves its shell running, with a waiting command told.
  const unsplitting = await openSplit([1, 2]);
  ui = sp(unsplitting);
  ui.split();
  await wait(20);
  unsplitting.sockets[1].readyState = 0;
  const orphan = unsplitting.window.sendToTerminal('orphan');
  await wait(20);
  ui.focus(0);
  ui.bar('Unsplit').click();
  assert.match(await orphan, /closed/);
  assert.equal(ui.panes().length, 1);
  assert.equal(unsplitting.sockets[1].closed, true);
  assert.equal(unsplitting.terminals[1].disposed, true);
  assert.equal(unsplitting.requests.some((request) => request.startsWith('DELETE')), false, 'closing a pane ended its shell');
  assert.deepEqual(ui.live(), ['1']);
  assert.equal(ui.saved(), null);
  assert.deepEqual([...unsplitting.document.querySelectorAll('.terminal-tab [role=tab]')].map((tab) => tab.textContent), ['1', '2']);
  // A retry under way does not come back to life in a closed pane.
  const closedRetry = await openSplit([1, 2]);
  ui = sp(closedRetry);
  ui.split();
  await wait(20);
  // Asking whether the server is back takes a while, and the pane closes meanwhile.
  closedRetry.server.licenseHeld = [];
  hangUp(closedRetry.sockets[1], 1006, '');
  await wait(1100);
  assert.equal(closedRetry.server.licenseHeld.length, 1, 'the retry did not ask the server');
  ui.focus(0);
  ui.bar('Unsplit').click();
  closedRetry.server.licenseHeld[0]();
  await wait(30);
  assert.equal(closedRetry.sockets.length, 2, 'a closed pane reconnected');
  // Output and parsing left over from a closed pane go nowhere.
  const closedParse = await openSplit([1, 2, 3]);
  ui = sp(closedParse);
  ui.split();
  await wait(20);
  closedParse.parser.held = true;
  closedParse.sockets[1].onmessage(out('late'));
  ui.tab('3');
  ui.focus(0);
  ui.bar('Unsplit').click();
  closedParse.parser.release();
  assert.equal(closedParse.sockets.length, 2, 'a closed pane connected once its output was parsed');
  // Fast output in one pane stays there when the other changes tab.
  const fastSplit = await openSplit([1, 2, 3]);
  ui = sp(fastSplit);
  ui.split();
  await wait(20);
  ui.focus(0);
  fastSplit.sockets[0].onmessage(out('y'));
  fastSplit.sockets[1].onmessage(out('z'));
  ui.tab('3');
  fastSplit.sockets[0].onmessage(out('late'));
  assert.deepEqual(fastSplit.terminals[0].writes, []);
  assert.deepEqual(fastSplit.terminals[1].writes, ['z']);

  // Split with no other shell starts one, in the project.
  const alone = await openSplit([1]);
  ui = sp(alone);
  ui.split();
  await wait(20);
  assert.match(alone.requests.at(-1), /^POST .*project=p1$/);
  assert.equal(ui.panes().length, 2);
  assert.deepEqual(ui.live(), ['1', '101']);
  assert.equal(ui.statusOf(0).hidden, true, 'the first pane kept saying it was starting a shell');
  // At the limit it starts nothing and says so, and a refusal leaves one pane and can be retried.
  const full = await openSplit([1], { limit: 1 });
  ui = sp(full);
  ui.split();
  await wait(20);
  assert.equal(ui.panes().length, 1);
  assert.equal(full.requests.some((request) => request.startsWith('POST')), false, 'split started a shell past the limit');
  assert.match(ui.statusOf(0).textContent, /No more shells/);
  const failing = await openSplit([1]);
  ui = sp(failing);
  failing.server.startError = 'no such shell';
  ui.split();
  await wait(20);
  assert.equal(ui.panes().length, 1);
  assert.match(ui.statusOf(0).textContent, /did not start: no such shell/);
  failing.server.startError = '';
  ui.panes()[0].querySelector('.terminal-action').click();
  await wait(20);
  assert.equal(ui.panes().length, 2);
  // Pressing twice while it starts makes one pane and one shell.
  const twice = await openSplit([1]);
  ui = sp(twice);
  ui.split();
  ui.split();
  await wait(20);
  assert.equal(ui.panes().length, 2);
  assert.equal(twice.requests.filter((request) => request.startsWith('POST')).length, 1);

  // The handle moves the share by pointer and keyboard, within limits, and it is kept.
  const handled = await openSplit([1, 2]);
  ui = sp(handled);
  ui.split();
  await wait(20);
  const handle = handled.document.querySelector('.terminal-split-handle');
  assert.equal(handle.getAttribute('role'), 'separator');
  handled.document.querySelector('.terminal-panes').getBoundingClientRect = () => ({ left: 100, width: 1000, top: 0, height: 300 });
  handle.onpointerdown({ preventDefault() {}, pointerId: 1 });
  handle.onpointermove({ clientX: 400, clientY: 0 });
  assert.equal(ui.panes()[0].style.flexGrow, '0.3');
  assert.equal(ui.panes()[1].style.flexGrow, '0.7');
  handle.onpointerup();
  assert.equal(handle.onpointermove, null, 'the drag went on after the button came up');
  assert.equal(ui.saved().share, 0.3);
  handle.onpointerdown({ preventDefault() {}, pointerId: 1 });
  handle.onpointermove({ clientX: 100, clientY: 0 });
  assert.equal(ui.panes()[0].style.flexGrow, '0.2', 'a pane went under its least');
  handle.onpointerup();
  handle.onkeydown({ key: 'ArrowRight', preventDefault() {} });
  assert.equal(Number(ui.panes()[0].style.flexGrow).toFixed(2), '0.25');
  assert.equal(ui.saved().share.toFixed(2), '0.25');
  assert.match(webShellCSS, /@media \(max-width: 720px\) \{\s*\.terminal-panes-split \{\s*flex-direction: column;/);

  // The next page restores both panes, with focus and share, and takes neither by sending.
  const layout = JSON.stringify({ shell: '3', focused: true, share: 0.3 });
  const restored = await openSplit([1, 2, 3], { split: [splitKey, layout] });
  ui = sp(restored);
  assert.equal(ui.panes().length, 2);
  assert.deepEqual(ui.live(), ['1', '3']);
  assert.equal(ui.selected(), '3');
  assert.equal(ui.panes()[0].style.flexGrow, '0.3');
  assert.deepEqual(ui.saved(), { shell: '3', focused: true, share: 0.3 });
  // Sizes were settled first: each connected after both had fitted, so neither connects at the whole width.
  assert.ok(restored.fits.length >= 3, 'the panes did not fit before connecting');
  // A shell that has gone leaves one pane, and what is saved says so.
  const lost = await openSplit([1, 2], { split: [splitKey, JSON.stringify({ shell: '9', focused: true, share: 0.3 })] });
  ui = sp(lost);
  assert.equal(ui.panes().length, 1);
  assert.deepEqual(ui.live(), ['1']);
  assert.equal(ui.saved(), null);
  // So does the same shell in both.
  const same = await openSplit([1, 2], { split: [splitKey, JSON.stringify({ shell: '1', focused: false, share: 0.5 })] });
  ui = sp(same);
  assert.equal(ui.panes().length, 1);
  // What someone edited does not break the page, or a pane's size.
  for (const bad of ['{', 'null', '[]', '"3"', JSON.stringify({ shell: '2', share: 'wide' }), JSON.stringify({ shell: '2', share: 40 })]) {
    const edited = await openSplit([1, 2], { split: [splitKey, bad] });
    ui = sp(edited);
    assert.equal(edited.sockets.length >= 1, true, 'saved layout ' + bad);
    const grown = Number(ui.panes()[0].style.flexGrow || 0.5);
    assert.ok(grown >= 0.2 && grown <= 0.8, 'saved layout ' + bad + ' made a share of ' + grown);
  }
  // Another project has its own layout: here, one pane offering to start a shell.
  const otherProject = load(process.argv[1], { stored: '1', path: '/project/p3', shells: splitShells(1, 2), split: [splitKey, layout] });
  await wait(50);
  ui = sp(otherProject);
  assert.equal(ui.panes().length, 1);
  assert.equal(otherProject.sockets.length, 0);
  assert.match(ui.statusOf(0).textContent, /No shell for p3 yet/);
  assert.equal(ui.bar('Split').disabled, true);
  // A browser tab out of sight connects neither, until it is looked at.
  const hiddenSplit = await openSplit([1, 2, 3], { hidden: true, split: [splitKey, layout] });
  assert.equal(hiddenSplit.sockets.length, 0, 'a background tab connected a pane');
  hiddenSplit.window.__hidden = false;
  hiddenSplit.document.dispatchEvent(new hiddenSplit.window.Event('visibilitychange'));
  await wait(30);
  assert.equal(hiddenSplit.sockets.length, 2);
  // A pane's lost connection waits for a hidden tab too, and each comes back when it is shown.
  hiddenSplit.window.__hidden = true;
  hangUp(hiddenSplit.sockets[0], 1006, '');
  hangUp(hiddenSplit.sockets[1], 1006, '');
  await wait(1200);
  assert.equal(hiddenSplit.sockets.length, 2, 'a hidden pane kept retrying');
  hiddenSplit.window.__hidden = false;
  hiddenSplit.document.dispatchEvent(new hiddenSplit.window.Event('visibilitychange'));
  await wait(1200);
  assert.equal(hiddenSplit.sockets.length, 4, 'a pane did not reconnect when shown');
  // Sending with a closed panel waits for the pane that had focus, and takes nothing.
  const sentLater = load(process.argv[1], { path: '/project/p1', shells: splitShells(1, 2, 3), remembered: [shownKey, '1'], split: [splitKey, layout] });
  await wait(30);
  const sendingToSplit = sentLater.window.sendToTerminal('for the focused');
  await wait(40);
  assert.equal(sentLater.sockets.length, 2);
  for (const socket of sentLater.sockets) assert.match(socket.url, /takeover=0/, 'sending took a shell');
  sentLater.sockets[0].onmessage(out('prompt'));
  assert.equal(sentLater.terminals[0].pastes, undefined, 'went to the pane without focus');
  sentLater.sockets[1].onmessage(out('prompt'));
  assert.equal(await sendingToSplit, null);
  assert.deepEqual(sentLater.terminals[1].pastes, ['for the focused']);

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
