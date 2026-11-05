package webui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
	script := `
const fs = require('fs');
const assert = require('node:assert/strict');
const { JSDOM, VirtualConsole } = require('jsdom');
const webShellCSS = fs.readFileSync(process.argv[3], 'utf8');
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const home = '/home/u';
const plain = (id, number) => ({ id, number, basedir: '', project: '', dir: home, link: '/' });
const inProject = (id, number, project, basedir = '/b') => ({ id, number, basedir, project, dir: '/w/' + project, link: '/project/' + encodeURIComponent(project) });
const load = (file, { stubs = true, stored = '', height = '', hidden = false, status = () => 200, shells = [], remembered = '', limit = 8, path = '/' } = {}) => {
  const sockets = [], written = [], terminals = [], requests = [], polls = [], alive = [], aliveTimers = [];
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
        options = {};
        constructor(options) { Object.assign(this.options, options); terminals.push(this); }
        loadAddon() {} open() {} focus() {}
        reset() { written.length = 0; }
        write(data) { written.push(Buffer.from(data).toString()); }
        onData(callback) { this.data = callback; }
        onBinary() {}
        onResize(callback) { this.resize = callback; }
      };
      window.FitAddon = { FitAddon: class { fit() {} } };
    },
  });
  return { dom, window: dom.window, document: dom.window.document, sockets, written, terminals, requests, polls, server, alive, aliveTimers };
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

  const { window, document, sockets, written, terminals, requests, polls, server } = load(process.argv[1], { limit: 2 });
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
	if output, err := exec.Command("node", "-e", script, withTerminal, without, filepath.Join("assets", "web_shell.css")).CombinedOutput(); err != nil {
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
