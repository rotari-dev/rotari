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
	if err := os.WriteFile(withTerminal, []byte(composeWebPage(livePage{nonce: "page-nonce", terminal: true, about: About{Version: "rotari v9", Executable: "/opt/rotari bin/rotari"}, stoppable: true}, nil, false, "", "")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(without, []byte(composeWebPage(livePage{nonce: "page-nonce", stoppable: true}, nil, false, "", "")), 0o600); err != nil {
		t.Fatal(err)
	}
	jobsWithTerminal := filepath.Join(dir, "jobs.html")
	jobRows := []joblist.Row{{State: "success", Project: "p", RunID: "r", JobName: "decode", Command: "sh -c echo…", FullCommand: "sh -c 'echo hi'", AttemptID: "attempt-1"}}
	if err := os.WriteFile(jobsWithTerminal, []byte(jobsPage(livePage{nonce: "page-nonce", terminal: true, about: About{Version: "rotari v9", Executable: "/opt/rotari bin/rotari"}}, "/", nil, jobRows, jobsSelection{}, true, false, "")), 0o600); err != nil {
		t.Fatal(err)
	}
	terminalWindow := filepath.Join(dir, "window.html")
	if err := os.WriteFile(terminalWindow, []byte(composeTerminalWindow("page-nonce")), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
const fs = require('fs');
const assert = require('node:assert/strict');
const { JSDOM, VirtualConsole } = require('jsdom');
const webShellCSS = fs.readFileSync(process.argv[3], 'utf8');
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const home = '/home/u';
// A shell of no project shows on Activity, and a basedir's own on its All projects.
const plain = (id, number) => ({ id, number, basedir: '', project: '', dir: home, link: '/activity' });
const ofBasedir = (id, number, basedir = '/b') => ({ id, number, basedir, project: '', dir: '/w', link: basedir === '/b' ? '/' : '/_basedir/' + basedir.slice(1) + '/' });
const inProject = (id, number, project, basedir = '/b') => ({ id, number, basedir, project, dir: '/w/' + project, link: '/project/' + encodeURIComponent(project) });
const load = (file, { opener = null, stubs = true, stored = '', height = '', hidden = false, status = () => 200, shells = [], remembered = '', split = '', place = '', width = '', tight = false, limit = 8, path = '/activity', maximized = false, fontSize = '', explorer = '' } = {}) => {
  const sockets = [], written = [], terminals = [], requests = [], polls = [], alive = [], aliveTimers = [], pasted = [], fits = [];
  // xterm.js parses what it is given later; a test can hold that back.
  const parser = { held: false, waiting: [], release() { this.held = false; this.waiting.splice(0).forEach((parse) => parse()); } };
  const server = { shells: [...shells], lastID: 100, limit, startError: '', tight, observers: [] };
  const fakeShells = (url, method, body) => {
    requests.push(method + ' ' + url);
    const refused = (code, text) => Promise.resolve({ ok: false, status: code, text: async () => text });
    if (status() !== 200) return refused(status(), 'refused');
    const linksAsked = /\/_terminal\/shells\/([^/]+)\/links$/.exec(url);
    if (linksAsked) {
      const candidates = JSON.parse(body).candidates;
      (server.linkAsks ||= []).push(candidates);
      if (server.linksStatus) return refused(server.linksStatus, 'links refused');
      const links = candidates.filter((text) => server.links?.[text]).map((text) => ({ text, ...server.links[text] }));
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ links }) });
    }
    const dirsAsked = /\/_terminal\/shells\/([^/]+)\/dirs$/.exec(url);
    if (dirsAsked) {
      const id = decodeURIComponent(dirsAsked[1]);
      if (!server.shells.some((shell) => shell.id === id)) return refused(404, 'no such shell');
      const answer = server.dirs?.[id];
      if (typeof answer === 'number') return refused(answer, 'dirs refused');
      return Promise.resolve({ ok: true, status: 200, json: async () => answer || { history: [], runs: [] } });
    }
    const previewed = /\/_terminal\/shells\/([^/]+)\/preview$/.exec(url);
    if (previewed) {
      const id = decodeURIComponent(previewed[1]);
      if (!server.shells.some((shell) => shell.id === id) || server.endedSinceListed?.includes(id)) return refused(404, 'no such shell');
      const cwd = server.cwds?.[id];
      const command = server.commands?.[id];
      const below = server.processes?.[id];
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ ...(cwd ? { cwd } : { cwd_error: 'no such process' }), ...(command === '?' ? { command_error: 'no such group' } : command ? { command } : {}), ...(below === '?' ? { processes: null, processes_error: 'no /proc' } : { processes: below || [] }) }) });
    }
    if (method === 'GET') return Promise.resolve({ ok: true, status: 200, json: async () => ({ shells: [...server.shells], limit: 32, group_limit: server.limit, ...(server.now ? { now: server.now } : {}) }) });
    if (method === 'POST') {
      if (server.startError) return refused(500, server.startError);
      const project = decodeURIComponent(/project=([^&]*)/.exec(url)?.[1] || '');
      const basedir = project || /[?&]basedir=/.test(url) ? '/b' : '';
      const inGroup = server.shells.filter((shell) => shell.basedir === basedir && shell.project === project);
      if (inGroup.length >= server.limit) return refused(409, 'at most ' + server.limit);
      let number = 1;
      while (inGroup.some((shell) => shell.number === number)) number++;
      const id = String(++server.lastID);
      const shell = project ? inProject(id, number, project) : basedir ? ofBasedir(id, number) : plain(id, number);
      server.shells = [...server.shells, shell];
      return Promise.resolve({ ok: true, status: 201, json: async () => shell });
    }
    if (server.refuse && (method === 'PATCH' || method === 'DELETE')) return refused(500, server.refuse);
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
      if (explorer) window.localStorage.setItem('rotari-terminal-explorer', explorer);
      if (remembered) window.sessionStorage.setItem(remembered[0], remembered[1]);
      if (opener) Object.defineProperty(window, 'opener', { configurable: true, value: opener });
      if (split) window.sessionStorage.setItem(split[0], split[1]);
      window.fetch = (url, init = {}) => {
        const address = String(url);
        if (address.includes('xterm-license')) {
          const answer = { ok: status() === 200, status: status() };
          return server.licenseHeld ? new Promise((resolve) => server.licenseHeld.push(() => resolve(answer))) : Promise.resolve(answer);
        }
        if (address === '/_server/shutdown') {
          requests.push(init.method + ' ' + address + ' ' + (init.headers || {})['Content-Type'] + ' ' + init.body);
          const code = server.shutdownStatus || 202;
          return Promise.resolve({ ok: code < 300, status: code, text: async () => (code === 202 ? '{}' : 'refused') });
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
        if (address === '/_terminal/projects') {
          requests.push('GET ' + address);
          if (server.projectStates === undefined) return new Promise(() => {});
          if (typeof server.projectStates === 'number') return Promise.resolve({ ok: false, status: server.projectStates, text: async () => 'refused' });
          return Promise.resolve({ ok: true, status: 200, json: async () => server.projectStates });
        }
        if (address.startsWith('/api/launchers?')) {
          requests.push('GET ' + address);
          return Promise.resolve({ ok: true, status: 200, json: async () => server.launchers || { available: false, reason: 'Launching is disabled on this server.', hint: 'Restart with rotari web --allow-exec.' } });
        }
        if (address.startsWith('/_terminal/files/')) {
          const op = address.slice('/_terminal/files/'.length);
          const body = JSON.parse(init.body);
          (server.fileChanges ||= []).push([op, body, (init.headers || {})['Content-Type']]);
          if (server.refuseChange) return Promise.resolve({ ok: false, status: 409, text: async () => server.refuseChange });
          const path = op === 'copy' && body.unique ? body.to.replace(/(\.[^./]*)?$/, ' copy$1') : body.to || body.path;
          return Promise.resolve({ ok: true, status: 200, json: async () => ({ path }) });
        }
        if (address.startsWith('/_terminal/files?')) {
          // Apart from requests: the Explorer is open by default, on every page that opens the panel.
          (server.fileRequests ||= []).push('GET ' + address);
          const query = new URLSearchParams(address.slice(address.indexOf('?') + 1));
          const dir = query.has('shell') ? server.cwds?.[query.get('shell')] : query.get('path');
          const refuse = (text) => Promise.resolve({ ok: false, status: 422, text: async () => text });
          if (!dir) return refuse('where the shell is now is not known: no such process');
          const answer = server.files?.[dir];
          if (!answer) return refuse('stat ' + dir + ': no such file or directory');
          if (answer.error) return refuse(answer.error);
          const entries = Array.isArray(answer) ? answer : answer.entries;
          return Promise.resolve({ ok: true, status: 200, json: async () => ({ dir, entries, total: answer.total || entries.length }) });
        }
        if (address === '/api/tableview-sources') {
          requests.push('POST /api/tableview-sources ' + init.body);
          const answer = server.tableviewSource || { source: 'S 1', dir: '/w/p1/logs' };
          return Promise.resolve({ ok: !answer.error, status: answer.error ? 400 : 200, json: async () => answer });
        }
        if (address.startsWith('/api/launch-preview?')) {
          requests.push('GET ' + address);
          return Promise.resolve({ ok: true, status: 200, json: async () => server.launchPreview });
        }
        if (address === '/api/launch') {
          requests.push('POST /api/launch ' + init.body);
          const answer = server.launchAnswer;
          return Promise.resolve({ ok: !answer.error, status: answer.error ? 400 : 200, json: async () => answer });
        }
        if (address === '/api/launches') {
          return Promise.resolve({ ok: true, status: 200, json: async () => ({ launches: [server.launchStates.length > 1 ? server.launchStates.shift() : server.launchStates[0]] }) });
        }
        if (address === '/api/basedirs') {
          requests.push('GET ' + address);
          if (server.basedirsStatus) return Promise.resolve({ ok: false, status: server.basedirsStatus, text: async () => 'refused' });
          return Promise.resolve({ ok: true, status: 200, json: async () => ({ basedirs: server.basedirs || [{ id: 'b', path: '/b', current: true }] }) });
        }
        if (address.startsWith('/api/projects?basedir_id=')) {
          const id = decodeURIComponent(address.slice('/api/projects?basedir_id='.length));
          const projects = (server.projects || {})[id];
          if (!projects) return Promise.resolve({ ok: false, status: 404, text: async () => 'gone' });
          return Promise.resolve({ ok: true, status: 200, json: async () => ({ projects }) });
        }
        if (address.startsWith('/_terminal/here')) {
          requests.push('GET ' + address);
          if (status() !== 200) return Promise.resolve({ ok: false, status: status(), text: async () => 'refused' });
          const project = decodeURIComponent(/project=([^&]*)/.exec(address)?.[1] || '');
          const basedirID = /[?&]basedir=([^&]*)/.exec(address)?.[1];
          // A project's shells here are all of /b, as the fixtures make them.
          const basedir = project ? '/b' : basedirID ? '/' + basedirID : basedirID !== undefined ? '/b' : '';
          return Promise.resolve({ ok: true, status: 200, json: async () => ({ basedir, project, dir: project ? '/w/' + project : basedir ? '/w' : home }) });
        }
        return new Promise(() => {});
      };
      // Uploads go through XMLHttpRequest, for their progress. One held (server.holdUploads)
      // waits for the test to report progress and finish it.
      window.XMLHttpRequest = class {
        constructor() { this.upload = {}; this.headers = {}; (server.xhrs ||= []).push(this); }
        open(method, url) { this.method = method; this.url = url; }
        setRequestHeader(name, value) { this.headers[name] = value; }
        send(body) { this.body = body; if (!server.holdUploads) this.finish(); }
        finish() {
          const query = new URLSearchParams(this.url.slice(this.url.indexOf('?') + 1));
          (server.uploads ||= []).push([query.get('dir'), query.get('name'), this.headers['Content-Type'], this.body?.constructor?.name]);
          const refused = server.refuseUpload?.[query.get('name')];
          this.status = refused ? 409 : 200;
          this.responseText = refused || JSON.stringify({ path: query.get('dir') + '/' + query.get('name') });
          Promise.resolve().then(() => this.onload());
        }
      };
      window.setInterval = (callback, delay) => { if (delay === 5000) polls.push(callback); if (delay === 60000) aliveTimers.push(callback); return 1; };
      window.confirm = () => true;
      window.ResizeObserver = class { constructor(callback) { server.observers.push(callback); } observe() {} disconnect() {} };
      window.matchMedia = (query) => ({ get matches() { return query.includes('min-width: 1000px') ? !server.tight : false; }, addEventListener(type, listener) { if (query.includes('min-width')) (server.mediaListeners ||= []).push(listener); } });
      if (hidden) Object.defineProperty(window.Document.prototype, 'hidden', { configurable: true, get: () => window.__hidden !== false });
      if (height) window.localStorage.setItem('rotari-terminal-height', height);
      if (place) window.localStorage.setItem('rotari-terminal-place', place);
      if (width) window.localStorage.setItem('rotari-terminal-width', width);
      window.WebSocket = class {
        static OPEN = 1;
        constructor(url) { this.url = url; this.sent = []; this.readyState = 1; sockets.push(this); }
        send(data) { this.sent.push(data); }
        close() { this.closed = true; }
      };
      if (!stubs) return;
      window.Terminal = class {
        rows = 24; cols = 80;
        // A test sets lines, each a string or { text, wrapped }, for the link provider to read.
        buffer = { active: { type: 'normal', cursorX: 0, cursorY: 0, baseY: 0, viewportY: 0, getLine: (y) => {
          const line = this.lines?.[y];
          if (line === undefined) return undefined;
          const text = typeof line === 'string' ? line : line.text;
          return {
            isWrapped: Boolean(line.wrapped), length: this.cols,
            getCell: (x) => ({ getChars: () => text[x] || '', getWidth: () => 1 }),
            translateToString: (trim, start = 0, end = this.cols) => {
              const part = text.padEnd(this.cols).slice(start, end);
              return trim ? part.trimEnd() : part;
            },
          };
        } } };
        // A test calls osc to parse a sequence where the cursor is, as xterm.js does.
        oscHandlers = {};
        markers = [];
        parser = { registerOscHandler: (ident, handler) => { (this.oscHandlers[ident] ||= []).push(handler); return { dispose() {} }; } };
        osc(ident, data, x = this.buffer.active.cursorX, y = this.buffer.active.cursorY) {
          Object.assign(this.buffer.active, { cursorX: x, cursorY: y });
          return (this.oscHandlers[ident] || []).some((handler) => handler(data));
        }
        registerMarker(offset = 0) {
          const listeners = [];
          const marker = {
            line: this.buffer.active.cursorY + offset, isDisposed: false,
            onDispose: (listener) => { listeners.push(listener); return { dispose() {} }; },
            dispose: () => { if (marker.isDisposed) return; marker.isDisposed = true; marker.line = -1; listeners.forEach((listener) => listener()); },
          };
          this.markers.push(marker);
          return marker;
        }
        // A decoration is drawn when a test calls render, as xterm.js draws one in view.
        decorations = [];
        registerDecoration(options) {
          if (!this.options.allowProposedApi) throw new Error('You must set the allowProposedApi option to true to use proposed API');
          if (options.marker.isDisposed) return undefined;
          const renders = [];
          const decoration = {
            options, element: undefined, isDisposed: false,
            onRender: (listener) => { renders.push(listener); return { dispose() {} }; },
            dispose: () => { if (decoration.isDisposed) return; decoration.isDisposed = true; decoration.element?.remove(); },
            render: () => { decoration.element ||= window.document.createElement('div'); renders.forEach((listener) => listener(decoration.element)); return decoration.element; },
          };
          options.marker.onDispose(() => decoration.dispose());
          this.decorations.push(decoration);
          return decoration;
        }
        scrollToLine(line) { this.buffer.active.viewportY = line; }
        scrollToBottom() { this.buffer.active.viewportY = this.buffer.active.baseY; this.bottomed = (this.bottomed || 0) + 1; }
        select(column, row, length) { this.selection = { column, row, length }; }
        attachCustomKeyEventHandler(handler) { this.keyHandler = handler; }
        modes = { bracketedPasteMode: false };
        options = {};
        constructor(options) { Object.assign(this.options, options); terminals.push(this); }
        loadAddon(addon) { (this.addons ||= []).push(addon); } open() {}
        registerLinkProvider(provider) { (this.linkProviders ||= []).push(provider); } focus() { this.focuses = (this.focuses || 0) + 1; }
        reset() { written.length = 0; this.writes = []; }
        dispose() { this.disposed = true; }
        write(data, done) {
          const parse = () => { written.push(Buffer.from(data).toString()); (this.writes ||= []).push(Buffer.from(data).toString()); done?.(); };
          if (parser.held) parser.waiting.push(parse);
          else parse();
        }
        paste(text) { pasted.push(text); (this.pastes ||= []).push(text); }
        clear() { this.cleared = (this.cleared || 0) + 1; this.markers.forEach((marker) => marker.dispose()); }
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
  assert.deepEqual(requests, ['GET /_terminal/shells', 'GET /_terminal/here?', 'GET /_terminal/shells', 'POST /_terminal/shells?rows=24&cols=80'], 'the first shell was not started on opening');

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
  const clearButton = [...document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'Clear');
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
  const maximizeButton = [...document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'Maximize');
  assert.equal(maximizeButton.getAttribute('aria-pressed'), 'false');
  maximizeButton.click();
  assert.equal(document.documentElement.classList.contains('terminal-maximized'), true);
  assert.equal(maximizeButton.getAttribute('aria-label'), 'Restore');
  assert.equal(maximizeButton.getAttribute('aria-pressed'), 'true');
  assert.equal(window.localStorage.getItem('rotari-terminal-maximized'), '1');
  assert.equal(sockets.length, 12, 'maximizing reconnected');
  maximizeButton.click();
  assert.equal(document.documentElement.classList.contains('terminal-maximized'), false);
  assert.equal(maximizeButton.getAttribute('aria-label'), 'Maximize');
  assert.equal(window.localStorage.getItem('rotari-terminal-maximized'), '0');
  const big = load(process.argv[1], { stored: '1', maximized: true });
  await wait(50);
  assert.equal(big.document.documentElement.classList.contains('terminal-maximized'), true, 'the next page lost it');
  assert.equal([...big.document.querySelectorAll('.terminal-bar button')].some((item) => item.getAttribute('aria-label') === 'Restore'), true);
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
  [...document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'Hide').click();
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
  assert.equal(runPage.requests.find((request) => request.includes('/here')), 'GET /_terminal/here?basedir=abc&project=p1&run=r%201');
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
  // Declining the question ends nothing. The question counts what runs in them, naming a few.
  menuPage.server.processes = { 3: [{ pid: 31, ppid: 30, depth: 1, command: 'make' }, { pid: 32, ppid: 31, depth: 2, command: 'make' }, { pid: 33, ppid: 31, depth: 2, command: 'cc a.c' }], 4: [{ pid: 41, ppid: 40, depth: 1, command: 'tail -f log' }, { pid: 42, ppid: 40, depth: 1, command: 'sleep 9' }] };
  let asked = [];
  mw.confirm = (text) => { asked.push(text); return false; };
  openMenu(1);
  menuItem('Close tabs to the right').click();
  await wait(20);
  assert.deepEqual(asked, ['End 2 shells? 5 processes run in them: make, cc a.c, tail -f log, and others.']);
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
  // A command that finished out of sight rings a bell on its tab, in place of the dot, and
  // on its All shells card; the shown shell's does not.
  const belling = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [{ ...inProject('1', 1, 'p1'), command_finished: true }, { ...inProject('2', 2, 'p1'), command_finished: true, unseen_output: true }, { ...inProject('3', 3, 'p1'), unseen_output: true }], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  await wait(50);
  const bellTabs = [...belling.document.querySelectorAll('.terminal-tab')];
  assert.deepEqual(bellTabs.map((tab) => [tab.classList.contains('terminal-tab-finished'), tab.classList.contains('terminal-tab-unseen')]), [[false, false], [true, false], [false, true]]);
  assert.equal(bellTabs[1].querySelector('[role=tab]').getAttribute('aria-label'), 'Shell 2, command finished');
  assert.match(bellTabs[1].querySelector('[role=tab]').title, /^A command finished\. /);
  [...belling.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'All shells').click();
  await wait(30);
  const bellCard = (id) => belling.document.querySelector('.terminal-overview-card[data-shell="' + id + '"]');
  assert.deepEqual(['1', '2', '3'].map((id) => [bellCard(id).classList.contains('terminal-overview-finished'), bellCard(id).querySelector('.terminal-overview-state').textContent]), [[false, 'shown'], [true, 'command finished'], [false, 'new output']]);
  assert.match(webShellCSS, /\.terminal-tab-finished \[role="tab"\]::before,\s*\.terminal-overview-finished \.terminal-overview-name::before \{/);

  // Signing out asks first when anything runs in a shell, a command in front or processes
  // left behind, naming each and what runs there; a shell it cannot look into counts too.
  // With nothing running it signs out at once, and when it cannot list the shells it asks.
  const signingPage = load(process.argv[1], { shells: [
    { ...inProject('1', 1, 'p1'), running: true, name: 'train' }, { ...inProject('2', 2, 'p1'), running: true },
    inProject('3', 3, 'p1'), inProject('4', 4, 'p1'), inProject('5', 5, 'p1'),
  ] });
  signingPage.server.commands = { 1: 'python train.py --epochs 90' };
  const bg = (pid, command) => ({ pid, ppid: 1, depth: 1, command });
  signingPage.server.processes = { 1: [bg(11, 'python train.py --epochs 90'), bg(12, 'nvidia-smi')], 3: [bg(31, 'sleep 600'), bg(32, 'tail -f log'), bg(33, 'make'), bg(34, 'cc')], 5: '?' };
  await wait(50);
  const signOutForm = signingPage.document.createElement('form');
  signOutForm.setAttribute('action', '/_auth/logout');
  signOutForm.setAttribute('method', 'post');
  let signOutCount = 0;
  signOutForm.submit = () => signOutCount++;
  signingPage.document.body.append(signOutForm);
  const signAsked = [];
  const submitSignOut = async (answer) => {
    signingPage.window.confirm = (text) => { signAsked.push(text); return answer; };
    const event = new signingPage.window.Event('submit', { bubbles: true, cancelable: true });
    signOutForm.dispatchEvent(event);
    assert.equal(event.defaultPrevented, true, 'the form went before asking');
    await wait(30);
  };
  await submitSignOut(false);
  assert.deepEqual(signAsked, ['Something is running in 4 shells:\n  train of p1: python train.py --epochs 90 (2 processes)\n  Shell 2 of p1: a command\n  Shell 3 of p1: 4 background processes (sleep 600, tail -f log, make, ...)\n  Shell 5 of p1: its processes are not known (no /proc)\nSigning out ends every shell, and what runs in it. Sign out anyway?']);
  assert.equal(signOutCount, 0, 'declining signed out');
  await submitSignOut(true);
  assert.equal(signOutCount, 1);
  signingPage.server.shells = signingPage.server.shells.map((shell) => ({ ...shell, running: false }));
  const quietSignOut = load(process.argv[1], { shells: [inProject('3', 3, 'p1')] });
  await wait(50);
  const quietForm = quietSignOut.document.createElement('form');
  quietForm.setAttribute('action', '/_auth/logout');
  let quietOut = 0;
  quietForm.submit = () => quietOut++;
  quietSignOut.document.body.append(quietForm);
  quietSignOut.window.confirm = () => assert.fail('asked with nothing running');
  quietForm.dispatchEvent(new quietSignOut.window.Event('submit', { bubbles: true, cancelable: true }));
  await wait(30);
  assert.equal(quietOut, 1);
  let signStatus = 200;
  const unsureSignOut = load(process.argv[1], { status: () => signStatus });
  await wait(50);
  signStatus = 503;
  const unsureForm = unsureSignOut.document.createElement('form');
  unsureForm.setAttribute('action', '/_auth/logout');
  unsureForm.submit = () => {};
  unsureSignOut.document.body.append(unsureForm);
  const unsureAsked = [];
  unsureSignOut.window.confirm = (text) => { unsureAsked.push(text); return false; };
  unsureForm.dispatchEvent(new unsureSignOut.window.Event('submit', { bubbles: true, cancelable: true }));
  await wait(30);
  assert.deepEqual(unsureAsked, ['Whether anything is running in a shell could not be checked: refused. Signing out ends every shell. Sign out anyway?']);

  // Stop server in the Settings menu asks as signing out does, naming what runs, and asks
  // even with nothing running, since the page loses its server; it sends a JSON request
  // and says beside the button what happened. A refusal says why and leaves it usable.
  const stopping = load(process.argv[1], { shells: [{ ...inProject('1', 1, 'p1'), running: true }] });
  await wait(50);
  const stopButton = stopping.document.getElementById('stop-server-button');
  assert.ok(stopButton, 'no Stop server button');
  const stopAsked = [];
  const stopRequests = () => stopping.requests.filter((request) => request.includes('/_server/'));
  stopping.window.confirm = (text) => { stopAsked.push(text); return false; };
  stopButton.click();
  await wait(30);
  assert.deepEqual(stopAsked, ['Something is running in a shell:\n  Shell 1 of p1: a command\nStopping the server ends every shell, and what runs in it. Stop the rotari web server anyway?']);
  assert.deepEqual(stopRequests(), [], 'declining stopped the server');
  stopping.server.shutdownStatus = 403;
  stopping.window.confirm = () => true;
  stopButton.click();
  await wait(30);
  const stopStatus = () => stopping.document.getElementById('stop-server-status').textContent;
  assert.equal(stopStatus(), 'Not stopped: refused');
  assert.equal(stopButton.disabled, false);
  stopping.server.shutdownStatus = 202;
  stopButton.click();
  await wait(30);
  assert.deepEqual(stopRequests(), ['POST /_server/shutdown application/json {}', 'POST /_server/shutdown application/json {}']);
  assert.match(stopStatus(), /^The server stopped\./);
  assert.equal(stopButton.disabled, true);
  assert.match(stopping.document.getElementById('disconnect-banner').textContent, /stopped from this page/);
  const idleStop = load(process.argv[1], { shells: [inProject('3', 3, 'p1')] });
  await wait(50);
  const idleAsked = [];
  idleStop.window.confirm = (text) => { idleAsked.push(text); return false; };
  idleStop.document.getElementById('stop-server-button').click();
  await wait(30);
  assert.deepEqual(idleAsked, ['Stop the rotari web server? This page will lose its connection.']);
  const plainStop = load(process.argv[2]);
  await wait(50);
  const plainAsked = [];
  plainStop.window.confirm = (text) => { plainAsked.push(text); return true; };
  plainStop.document.getElementById('stop-server-button').click();
  await wait(30);
  assert.deepEqual(plainAsked, ['Stop the rotari web server? This page will lose its connection.']);
  assert.equal(plainStop.requests.filter((request) => request.startsWith('GET /_terminal')).length, 0, 'a page without the terminal listed shells');
  assert.deepEqual(plainStop.requests.filter((request) => request.includes('/_server/')), ['POST /_server/shutdown application/json {}']);

  // A card says when its shell started and how long it has been idle, by the server's clock,
  // and a tab's tooltip says so as the pointer reaches it.
  const timed = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [
    { ...inProject('1', 1, 'p1'), started_at: '2026-11-10T09:55:00Z', active_at: '2026-11-10T11:45:00Z' },
    { ...inProject('2', 2, 'p1'), started_at: '2026-11-08T11:59:30Z', active_at: '2026-11-10T11:59:40Z' },
    inProject('3', 3, 'p1'),
  ], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  timed.server.now = '2026-11-10T12:00:00Z';
  await wait(50);
  await timed.polls[0]();
  const timedTabs = [...timed.document.querySelectorAll('.terminal-tab [role=tab]')];
  timedTabs[1].dispatchEvent(new timed.window.Event('pointerenter'));
  assert.match(timedTabs[1].title, /^Started in \/w\/p1 at \d\d:\d\d, 2d 0h ago\. Active now\.$/);
  timedTabs[0].dispatchEvent(new timed.window.Event('pointerenter'));
  assert.match(timedTabs[0].title, /, 2h 5m ago\. Idle 15m\.$/);
  assert.equal(timedTabs[2].title, 'Started in /w/p1', 'a shell with no times was given some');
  [...timed.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'All shells').click();
  await wait(30);
  const timesOf = (id) => { const line = timed.document.querySelector('.terminal-overview-card[data-shell="' + id + '"] .terminal-overview-times'); return line.hidden ? null : line.textContent; };
  assert.deepEqual(['1', '2', '3'].map(timesOf), ['started 2h 5m ago \u00b7 idle 15m', 'started 2d 0h ago \u00b7 active now', null]);

  // Closing every project's shells ends them all, the shown one last, after asking in so many
  // words; with only this project's, it is off, since Close all tabs does that.
  const everyMenu = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1'), inProject('9', 1, 'p2')], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  await wait(50);
  const everyItem = (label) => [...everyMenu.document.querySelectorAll('.terminal-tab-menu button')].find((item) => item.textContent === label);
  const everyAsked = [];
  everyMenu.window.confirm = (text) => { everyAsked.push(text); return true; };
  everyMenu.document.querySelector('.terminal-tab').dispatchEvent(new everyMenu.window.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
  everyItem('Close all shells of every project').click();
  await wait(50);
  assert.deepEqual(everyAsked, ['End all 3 shells, of every project? Nothing runs in them.']);
  assert.deepEqual(everyMenu.requests.filter((request) => request.startsWith('DELETE')), ['DELETE /_terminal/shells/2', 'DELETE /_terminal/shells/9', 'DELETE /_terminal/shells/1']);
  assert.deepEqual(everyMenu.server.shells, []);
  const onlyHere = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1')] });
  await wait(50);
  onlyHere.document.querySelector('.terminal-tab').dispatchEvent(new onlyHere.window.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
  assert.equal([...onlyHere.document.querySelectorAll('.terminal-tab-menu button')].find((item) => item.textContent === 'Close all shells of every project').disabled, true);

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
  // A backslash goes outside the quotes too, which fish reads as POSIX shells do.
  assert.equal(await press(sending, sending.window.copyIconForValue('a\\b', 'name')), "'a'\\\\'b'");
  assert.equal(sending.window.shellQuote("it's\\a"), "'it'\\''s'\\\\'a'");
  assert.equal(sending.window.shellWord('/w/plain-1.txt'), '/w/plain-1.txt');
  assert.equal(await press(sending, sending.window.copyIconForValue('a;rm -rf "$HOME"<b>', 'name')), "'a;rm -rf \"$HOME\"<b>'");
  const shown = sending.document.querySelector('.terminal-send-note');
  await press(sending, sending.window.copyIconForValue('two\nlines', 'name'));
  assert.match(shown.textContent, /control characters/);
  assert.equal(sending.pasted.at(-1).includes('lines'), false, 'a line break reached the terminal');
  // The Recent jobs page draws its buttons on the server, and gets the same ones. It is a
  // page of its basedir, so it shows the basedir's own shells.
  const jobsFile = process.argv[4];
  const jobs = load(jobsFile, { path: '/jobs/', stored: '1', shells: [plain('2', 1), ofBasedir('1', 1)] });
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

  // The Terminal button counts the shells of every project, with the panel closed or open,
  // and asks nothing of the server from a page out of sight.
  const counted = load(process.argv[1], { path: '/project/p1', shells: mixed() });
  const countLabel = (page) => page.document.getElementById('terminal-button').textContent;
  await wait(30);
  assert.equal(countLabel(counted), 'Terminal (5)');
  assert.equal(counted.sockets.length, 0, 'counting connected to a shell');
  assert.equal(counted.document.getElementById('terminal-panel'), null, 'counting opened the panel');
  assert.deepEqual(counted.requests, ['GET /_terminal/shells'], 'counting asked for more than the list');
  // Others starting and ending shells show on the next look, and no note for a panel never opened.
  counted.server.shells = [...counted.server.shells, plain('20', 2)];
  counted.polls[0]();
  await wait(10);
  assert.equal(countLabel(counted), 'Terminal (6)');
  counted.server.shells = counted.server.shells.filter((shell) => shell.id !== '20');
  counted.polls[0]();
  await wait(10);
  assert.equal(countLabel(counted), 'Terminal (5)');
  assert.equal(counted.document.querySelector('.terminal-send-note'), null, 'a closed panel told of a shell ending');
  // Starting one here counts at once, and so does one ending in view, over every project.
  counted.document.getElementById('terminal-button').click();
  await wait(30);
  assert.equal(countLabel(counted), 'Terminal (5)');
  counted.document.querySelector('.terminal-add').click();
  await wait(20);
  assert.equal(countLabel(counted), 'Terminal (6)');
  // The panel hidden still counts, and an answer that does not come shows no number.
  counted.document.getElementById('terminal-button').click();
  counted.server.shells = counted.server.shells.slice(1);
  counted.polls[0]();
  await wait(10);
  assert.equal(countLabel(counted), 'Terminal (5)', 'a hidden panel stopped counting');
  let countAnswer = 200;
  const unanswered = load(process.argv[1], { status: () => countAnswer, shells: [plain('1', 1)] });
  await wait(30);
  assert.equal(countLabel(unanswered), 'Terminal (1)');
  countAnswer = 500;
  unanswered.polls[0]();
  await wait(10);
  assert.equal(countLabel(unanswered), 'Terminal', 'kept a number it could not check');
  countAnswer = 200;
  unanswered.polls[0]();
  await wait(10);
  assert.equal(countLabel(unanswered), 'Terminal (1)');
  // No shells is a count too, and a page out of sight waits to be looked at.
  const noneYet = load(process.argv[1]);
  await wait(30);
  assert.equal(countLabel(noneYet), 'Terminal (0)');
  const unseenPage = load(process.argv[1], { hidden: true, shells: [plain('1', 1)] });
  await wait(30);
  assert.deepEqual(unseenPage.requests, [], 'a page out of sight asked for the list');
  unseenPage.polls[0]();
  await wait(10);
  assert.deepEqual(unseenPage.requests, [], 'a page out of sight went on asking');
  unseenPage.window.__hidden = false;
  unseenPage.document.dispatchEvent(new unseenPage.window.Event('visibilitychange'));
  await wait(10);
  assert.equal(countLabel(unseenPage), 'Terminal (1)');

  // Two panes. Each row of what happens is checked with two or three shells of a project.
  const splitShells = (...ids) => ids.map((id, index) => inProject(String(id), index + 1, 'p1'));
  const shownKey = 'rotari-terminal-shell:/b:p1', splitKey = 'rotari-terminal-layout:/b:p1';
  const twoColumns = (first, second, share, focused) => ({ columns: [{ share, rows: [{ share: 1, shell: first }] }, { share: 1 - share, rows: [{ share: 1, shell: second }] }], focused });
  const openSplit = async (ids, options = {}) => {
    const page = load(process.argv[1], { stored: '1', path: '/project/p1', shells: splitShells(...ids), remembered: [shownKey, String(ids[0])], ...options });
    await wait(50);
    return page;
  };
  const partOf = (length) => Number(/([\d.]+)%/.exec(length)[1]) / 100;
  const sp = (page) => {
    const bar = (text) => [...page.document.querySelectorAll('.terminal-bar button')].find((item) => item.textContent === text || item.getAttribute('aria-label') === text);
    const panes = () => [...page.document.querySelectorAll('.terminal-pane')];
    const tabEl = (name) => [...page.document.querySelectorAll('.terminal-tab')].find((tab) => tab.querySelector('[role=tab]').textContent === name);
    return {
      bar, panes,
      split: () => bar('Split right').click(),
      splitDown: () => bar('Split down').click(),
      closePane: () => bar('Close pane').click(),
      tab: (name) => tabEl(name).querySelector('[role=tab]').click(),
      selected: () => page.document.querySelector('.terminal-tab [aria-selected=true]')?.textContent,
      otherPane: () => [...page.document.querySelectorAll('.terminal-tab-other-pane [role=tab]')].map((tab) => tab.textContent),
      live: () => page.sockets.filter((socket) => !socket.closed).map((socket) => /shell=(\d+)/.exec(socket.url)[1]),
      focus: (index) => panes()[index].dispatchEvent(new page.window.Event('focusin', { bubbles: true })),
      statusOf: (index) => panes()[index].querySelector('.terminal-status'),
      saved: () => JSON.parse(page.window.sessionStorage.getItem(splitKey)),
      inOrder: () => panes().sort((a, b) => partOf(a.style.left) - partOf(b.style.left)),
    };
  };
  // The server closing a connection, which the stand-in does not record by itself.
  const hangUp = (socket, code, reason) => { socket.closed = true; socket.onclose({ code, reason }); };
  const out = (text) => ({ data: new TextEncoder().encode(text).buffer });

  // Split shows the next shell in a new pane, which takes focus, and leaves the first alone.
  const splitting = await openSplit([1, 2, 3]);
  let ui = sp(splitting);
  assert.equal(ui.panes().length, 1);
  assert.equal(ui.bar('Close pane').disabled, true, 'offered to close the only pane');
  // The splits are pictures with names, a column's and a row's as in the Table viewer.
  assert.match(webShellCSS, /button\.terminal-icon \{[^}]*align-items: center;[^}]*min-height: 0;[^}]*border: 0;[^}]*background: none;/);
  for (const [label, line] of [['Split right', 'M8 2.5v11'], ['Split down', 'M1.5 8h13']]) {
    const icon = ui.bar(label);
    assert.equal(icon.textContent, '', label + ' has words as well as a picture');
    assert.equal(icon.title, label);
    assert.equal(icon.querySelector('svg path').getAttribute('d'), line);
    assert.equal(icon.querySelector('svg').getAttribute('aria-hidden'), 'true');
    assert.ok(fs.readFileSync(process.argv[5], 'utf8').includes('<path d="' + line + '"/>'), label + ' differs from the Table viewer\'s');
  }
  assert.equal(splitting.document.querySelector('.terminal-bar .terminal-status'), null, 'a status stayed in the bar');
  ui.split();
  await wait(20);
  assert.equal(ui.panes().length, 2);
  assert.equal(splitting.terminals.length, 2);
  assert.deepEqual(ui.live(), ['1', '2']);
  assert.equal(ui.selected(), '2');
  assert.deepEqual(ui.otherPane(), ['1']);
  assert.equal(ui.bar('Close pane').disabled, false);
  assert.deepEqual(ui.saved(), twoColumns('1', '2', 0.5, '2'));
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
  assert.deepEqual(ui.saved(), twoColumns('3', '1', 0.5, '1'));
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
  ui.focus(1);
  assert.equal(ui.panes()[1].classList.contains('terminal-pane-focused'), true);
  assert.equal(ui.panes()[0].classList.contains('terminal-pane-focused'), false);
  assert.equal(ui.saved().focused, '3');
  assert.equal(ui.selected(), '3');

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
  assert.equal(ui.bar('Split right').disabled, false);
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
  assert.equal(ui.bar('Split right').disabled, true, 'offered to split nothing');
  assert.equal(ui.bar('Split down').disabled, true, 'offered to split nothing');

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

  // Closing a pane closes the focused one and leaves its shell running, with a waiting command told.
  const unsplitting = await openSplit([1, 2]);
  ui = sp(unsplitting);
  ui.split();
  await wait(20);
  unsplitting.sockets[1].readyState = 0;
  const orphan = unsplitting.window.sendToTerminal('orphan');
  await wait(20);
  ui.closePane();
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
  ui.closePane();
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
  ui.closePane();
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
  assert.equal(partOf(ui.panes()[0].style.width), 0.3);
  assert.equal(partOf(ui.panes()[1].style.width), 0.7);
  assert.equal(partOf(ui.panes()[1].style.left), 0.3);
  assert.equal(partOf(handle.style.left), 0.3);
  handle.onpointerup();
  assert.equal(handle.onpointermove, null, 'the drag went on after the button came up');
  assert.equal(ui.saved().columns[0].share, 0.3);
  handle.onpointerdown({ preventDefault() {}, pointerId: 1 });
  handle.onpointermove({ clientX: 100, clientY: 0 });
  assert.equal(partOf(ui.panes()[0].style.width), 0.2, 'a pane went under its least');
  handle.onpointerup();
  handle.onkeydown({ key: 'ArrowRight', preventDefault() {} });
  assert.equal(partOf(ui.panes()[0].style.width).toFixed(2), '0.25');
  assert.equal(ui.saved().columns[0].share.toFixed(2), '0.25');
  // A narrow panel stacks them, moves the handle along the other axis, and keeps the share.
  handled.document.querySelector('.terminal-panes').getBoundingClientRect = () => ({ left: 100, width: 600, top: 0, height: 300 });
  handled.server.observers.forEach((observer) => observer());
  assert.equal(ui.panes()[1].style.width, '100%');
  assert.equal(partOf(ui.panes()[1].style.top), 0.25);
  assert.equal(handle.getAttribute('aria-orientation'), 'horizontal');
  handle.onpointerdown({ preventDefault() {}, pointerId: 1 });
  handle.onpointermove({ clientX: 0, clientY: 150 });
  handle.onpointerup();
  assert.equal(ui.saved().columns[0].share, 0.5);
  handled.document.querySelector('.terminal-panes').getBoundingClientRect = () => ({ left: 100, width: 1000, top: 0, height: 300 });
  handled.server.observers.forEach((observer) => observer());
  assert.equal(partOf(ui.panes()[1].style.left), 0.5);
  assert.equal(handle.getAttribute('aria-orientation'), 'vertical');
  // One pane fills the panel, and leaves no handle.
  ui.closePane();
  assert.equal(ui.inOrder().at(-1).style.width, '100%');
  assert.equal(handled.document.querySelector('.terminal-split-handle'), null);

  // The next page restores both panes, with focus and share, and takes neither by sending.
  const layout = JSON.stringify(twoColumns('1', '3', 0.3, '3'));
  const restored = await openSplit([1, 2, 3], { split: [splitKey, layout] });
  ui = sp(restored);
  assert.equal(ui.panes().length, 2);
  assert.deepEqual(ui.live(), ['1', '3']);
  assert.equal(ui.selected(), '3');
  assert.equal(partOf(ui.panes()[0].style.width), 0.3);
  assert.deepEqual(ui.saved(), JSON.parse(layout));
  // Sizes were settled first: each connected after both had fitted, so neither connects at the whole width.
  assert.ok(restored.fits.length >= 3, 'the panes did not fit before connecting');
  // A shell that has gone leaves one pane, and what is saved says so.
  const lost = await openSplit([1, 2], { split: [splitKey, JSON.stringify(twoColumns('1', '9', 0.3, '9'))] });
  ui = sp(lost);
  assert.equal(ui.panes().length, 1);
  assert.deepEqual(ui.live(), ['1']);
  assert.equal(ui.saved(), null);
  // With the first gone too, the one left shows, not the project's first.
  const firstLost = await openSplit([1, 2], { split: [splitKey, JSON.stringify(twoColumns('9', '2', 0.3, '2'))] });
  assert.deepEqual(sp(firstLost).live(), ['2']);
  // So does the same shell in both.
  const same = await openSplit([1, 2], { split: [splitKey, JSON.stringify(twoColumns('1', '1', 0.5, '1'))] });
  ui = sp(same);
  assert.equal(ui.panes().length, 1);
  // What someone edited does not break the page, or a pane's size.
  for (const bad of ['{', 'null', '[]', '"3"', '{"shell":"2","share":0.5}', '{"columns":"x"}', '{"columns":[null,{"rows":"x"},{"rows":[null]}]}', JSON.stringify(twoColumns('1', '2', 'wide', '1')), JSON.stringify(twoColumns('1', '2', 40, '1')), JSON.stringify(twoColumns('1', '2', -1, 'nobody'))]) {
    const edited = await openSplit([1, 2], { split: [splitKey, bad] });
    ui = sp(edited);
    assert.equal(edited.sockets.length >= 1, true, 'saved layout ' + bad);
    const grown = ui.panes().length > 1 ? partOf(ui.panes()[0].style.width) : 0.5;
    assert.ok(grown >= 0.05 && grown <= 0.95, 'saved layout ' + bad + ' made a share of ' + grown);
    assert.equal(new Set(ui.live()).size, ui.live().length);
    assert.equal(edited.document.querySelectorAll('.terminal-pane-focused').length, 1, 'saved layout ' + bad + ' left no focus');
  }
  // Another project has its own layout: here, one pane offering to start a shell.
  const otherProject = load(process.argv[1], { stored: '1', path: '/project/p3', shells: splitShells(1, 2), split: [splitKey, layout] });
  await wait(50);
  ui = sp(otherProject);
  assert.equal(ui.panes().length, 1);
  assert.equal(otherProject.sockets.length, 0);
  assert.match(ui.statusOf(0).textContent, /No shell for p3 yet/);
  assert.equal(ui.bar('Split right').disabled, true);
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

  // Up to four panes, in columns of rows. Every shape fills the panel without overlapping,
  // and closing any pane of any shape leaves a shape that does.
  const boxesOf = (page) => [...page.document.querySelectorAll('.terminal-pane')].map((pane) => {
    const [x, y, w, h] = [pane.style.left, pane.style.top, pane.style.width, pane.style.height].map(partOf);
    return { x, y, w, h };
  });
  const near = (a, b) => Math.abs(a - b) < 1e-3;
  const assertFills = (page, what) => {
    const boxes = boxesOf(page);
    assert.ok(near(boxes.reduce((sum, box) => sum + box.w * box.h, 0), 1), what + ': the panes do not fill the panel: ' + JSON.stringify(boxes));
    boxes.forEach((one, i) => boxes.slice(i + 1).forEach((other) => {
      const apart = one.x + one.w <= other.x + 1e-3 || other.x + other.w <= one.x + 1e-3 || one.y + one.h <= other.y + 1e-3 || other.y + other.h <= one.y + 1e-3;
      assert.ok(apart, what + ': panes overlap: ' + JSON.stringify(boxes));
    }));
    assert.equal(page.document.querySelectorAll('.terminal-pane-focused').length, 1, what + ': focus is not on one pane');
  };
  const shapeOf = (page) => {
    const saved = JSON.parse(page.window.sessionStorage.getItem(splitKey));
    return saved ? saved.columns.map((column) => column.rows.length) : [1];
  };
  const buildShape = async (shape) => {
    const page = await openSplit([1, 2, 3, 4, 5, 6]);
    const shapeUI = sp(page);
    for (const [c, size] of shape.entries()) {
      if (c) { shapeUI.split(); await wait(2); }
      for (let row = 1; row < size; row++) { shapeUI.splitDown(); await wait(2); }
    }
    return page;
  };
  const shapes = [[2], [1, 1], [3], [2, 1], [1, 2], [1, 1, 1], [4], [3, 1], [1, 3], [2, 2], [2, 1, 1], [1, 2, 1], [1, 1, 2], [1, 1, 1, 1]];
  for (const shape of shapes) {
    const count = shape.reduce((sum, size) => sum + size, 0);
    for (let closing = 0; closing < count; closing++) {
      const page = await buildShape(shape);
      const shapeUI = sp(page);
      const what = 'shape ' + JSON.stringify(shape) + ' closing ' + closing;
      assert.equal(shapeUI.panes().length, count, what);
      assert.deepEqual(shapeOf(page), shape, what);
      assertFills(page, what);
      assert.equal(new Set(shapeUI.live()).size, count, what + ': a shell is in two panes');
      assert.equal(page.document.querySelectorAll('.terminal-split-handle').length, count - 1, what + ': one handle per boundary');
      shapeUI.focus(closing);
      shapeUI.closePane();
      await wait(2);
      assert.equal(shapeUI.panes().length, count - 1, what);
      assertFills(page, what + ', after');
      assert.equal(shapeUI.live().length, count - 1, what);
      assert.equal(page.document.querySelectorAll('.terminal-split-handle').length, count - 2, what);
      assert.equal(page.requests.some((request) => request.startsWith('DELETE')), false, what + ': closing a pane ended a shell');
    }
  }
  // Four is the limit, and the buttons say so. A split takes half of what it splits.
  const fullLayout = await buildShape([2, 2]);
  ui = sp(fullLayout);
  assert.equal(ui.bar('Split right').disabled, true);
  assert.equal(ui.bar('Split down').disabled, true);
  ui.closePane();
  assert.equal(ui.bar('Split right').disabled, false);
  const halves = await buildShape([1]);
  ui = sp(halves);
  ui.split();
  await wait(5);
  ui.splitDown();
  await wait(5);
  assert.deepEqual(boxesOf(halves)[0], { x: 0, y: 0, w: 0.5, h: 1 });
  assert.deepEqual(boxesOf(halves)[1], { x: 0.5, y: 0, w: 0.5, h: 0.5 });
  assert.deepEqual(boxesOf(halves)[2], { x: 0.5, y: 0.5, w: 0.5, h: 0.5 });
  // Split right from a row makes a column of the whole height, so that this goes no deeper.
  ui.split();
  await wait(5);
  assert.deepEqual(shapeOf(halves), [1, 2, 1]);
  assert.deepEqual(boxesOf(halves)[3], { x: 0.75, y: 0, w: 0.25, h: 1 });
  // A closed pane's room goes to its neighbor in the column, then to the next column to it, and so does focus.
  const inColumn = await buildShape([3]);
  ui = sp(inColumn);
  ui.focus(1);
  const shellAbove = ui.saved().columns[0].rows[0].shell;
  ui.closePane();
  assert.equal(ui.saved().focused, shellAbove);
  assert.deepEqual(ui.saved().columns[0].rows.map((row) => row.share.toFixed(2)), ['0.75', '0.25']);
  const lastInColumn = await buildShape([3]);
  ui = sp(lastInColumn);
  ui.focus(2);
  const shellJustAbove = ui.saved().columns[0].rows[1].shell;
  ui.closePane();
  assert.equal(ui.saved().focused, shellJustAbove, 'focus did not go to the neighbor');
  const inColumns = await buildShape([1, 1, 1]);
  ui = sp(inColumns);
  ui.focus(1);
  const widths = ui.saved().columns.map((column) => column.share);
  ui.closePane();
  assert.deepEqual(ui.saved().columns.map((column) => column.share), [widths[0] + widths[1], widths[2]]);
  const firstGoes = await buildShape([1, 1]);
  ui = sp(firstGoes);
  ui.focus(0);
  const secondShell = ui.saved().columns[1].rows[0].shell;
  ui.closePane();
  assert.deepEqual(ui.live().includes(secondShell), true);
  assert.equal(firstGoes.document.querySelector('.terminal-tab [aria-selected=true]').textContent, String(Number(secondShell)));
  // A split shows a shell not in any pane, not just the next tab along.
  const nextShown = await buildShape([1, 1]);
  ui = sp(nextShown);
  ui.focus(0);
  ui.splitDown();
  await wait(5);
  assert.equal(new Set(ui.live()).size, 3, 'a shell is in two panes');
  assert.deepEqual(ui.live().sort(), ['1', '2', '3']);
  // Too little room says so, and starts and opens nothing.
  const cramped = await openSplit([1, 2, 3]);
  ui = sp(cramped);
  const crampedBox = cramped.document.querySelector('.terminal-panes');
  crampedBox.getBoundingClientRect = () => ({ left: 0, top: 0, width: 300, height: 150 });
  ui.split();
  await wait(5);
  assert.equal(ui.panes().length, 1, 'split under the smallest width');
  assert.match(ui.statusOf(0).textContent, /too little room/);
  ui.splitDown();
  await wait(5);
  assert.equal(ui.panes().length, 1, 'split under the smallest height');
  crampedBox.getBoundingClientRect = () => ({ left: 0, top: 0, width: 300, height: 400 });
  ui.splitDown();
  await wait(5);
  assert.equal(ui.panes().length, 2);
  // A row's handle moves along the column, leaves the other column alone, and stops at the smallest.
  const rowHandles = await buildShape([1, 2]);
  ui = sp(rowHandles);
  const rowBox = rowHandles.document.querySelector('.terminal-panes');
  rowBox.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 400 });
  const rowHandle = [...rowHandles.document.querySelectorAll('.terminal-split-handle')].find((item) => item.getAttribute('aria-orientation') === 'horizontal');
  assert.equal(partOf(rowHandle.style.left), 0.5);
  assert.equal(partOf(rowHandle.style.width), 0.5);
  const leftColumnBefore = boxesOf(rowHandles)[0];
  rowHandle.onpointerdown({ preventDefault() {}, pointerId: 1 });
  rowHandle.onpointermove({ clientX: 0, clientY: 300 });
  assert.equal(boxesOf(rowHandles)[1].h, 0.75);
  assert.equal(boxesOf(rowHandles)[2].y, 0.75);
  assert.deepEqual(boxesOf(rowHandles)[0], leftColumnBefore);
  rowHandle.onpointermove({ clientX: 0, clientY: 390 });
  assert.equal(boxesOf(rowHandles)[2].h, 0.25, 'a row went under its least');
  rowHandle.onpointerup();
  assertFills(rowHandles, 'after dragging a row');
  assert.equal(ui.saved().columns[1].rows[0].share, 0.75);
  // A narrow panel stacks every pane in reading order, and keeps the columns for a wider one.
  const stackedThree = await buildShape([1, 2]);
  stackedThree.document.querySelector('.terminal-panes').getBoundingClientRect = () => ({ left: 0, top: 0, width: 400, height: 800 });
  stackedThree.server.observers.forEach((observer) => observer());
  assert.deepEqual(boxesOf(stackedThree).map((box) => [box.x, box.w, box.y, box.h]), [[0, 1, 0, 0.5], [0, 1, 0.5, 0.25], [0, 1, 0.75, 0.25]]);
  assertFills(stackedThree, 'stacked');
  assert.deepEqual(shapeOf(stackedThree), [1, 2]);
  // A boundary inside a stacked column counts within that column's room.
  const stackedBox = stackedThree.document.querySelector('.terminal-panes');
  const stackedHandle = [...stackedThree.document.querySelectorAll('.terminal-split-handle')].find((item) => partOf(item.style.top) === 0.75);
  stackedHandle.onpointerdown({ preventDefault() {}, pointerId: 1 });
  stackedHandle.onpointermove({ clientX: 0, clientY: 700 });
  stackedHandle.onpointerup();
  assert.equal(sp(stackedThree).saved().columns[1].rows[0].share, 0.75);
  assert.ok(near(boxesOf(stackedThree)[1].h, 0.375));
  assertFills(stackedThree, 'stacked, after dragging');
  stackedBox.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 800 });
  stackedThree.server.observers.forEach((observer) => observer());
  assert.equal(boxesOf(stackedThree)[0].w, 0.5);
  // Exchanging places works across columns and rows, connects nothing, and leaves the other two.
  const exchanging = await buildShape([1, 2, 1]);
  ui = sp(exchanging);
  ui.focus(0);
  const inFirst = ui.saved().columns[0].rows[0].shell;
  const inRow = ui.saved().columns[1].rows[1].shell;
  const untouched = [ui.saved().columns[1].rows[0].shell, ui.saved().columns[2].rows[0].shell];
  const boxesBefore = boxesOf(exchanging);
  const socketsBefore = exchanging.sockets.length;
  ui.tab(inRow);
  assert.equal(exchanging.sockets.length, socketsBefore, 'exchanging places reconnected');
  assert.equal(ui.saved().columns[0].rows[0].shell, inRow);
  assert.equal(ui.saved().columns[1].rows[1].shell, inFirst);
  assert.deepEqual([ui.saved().columns[1].rows[0].shell, ui.saved().columns[2].rows[0].shell], untouched);
  assert.equal(ui.saved().focused, inRow);
  assert.deepEqual(boxesOf(exchanging)[0], boxesBefore[2], 'the first pane did not go to the row');
  assert.deepEqual(boxesOf(exchanging)[2], boxesBefore[0], 'the row\'s pane did not come to the first place');
  assert.equal(exchanging.terminals.length, 4);
  // One of four being taken over, reconnecting, or moving on leaves the other three.
  const fourTrouble = await buildShape([2, 2]);
  ui = sp(fourTrouble);
  const quiet4 = () => fourTrouble.sockets.slice(1, 4).map((socket) => socket.closed);
  hangUp(fourTrouble.sockets[0], 4001, '');
  assert.match(ui.statusOf(0).textContent, /Another tab/);
  for (const index of [1, 2, 3]) assert.doesNotMatch(ui.statusOf(index).textContent, /Another tab/);
  assert.deepEqual(quiet4(), [undefined, undefined, undefined]);
  // With all of them shown, one ending closes its pane, and with one to spare it moves on.
  const fourEnd = await buildShape([2, 2]);
  ui = sp(fourEnd);
  fourEnd.server.shells = fourEnd.server.shells.filter((shell) => ['1', '2', '3', '4', '5'].includes(shell.id));
  const shellOfSocket = (socket) => /shell=(\d+)/.exec(socket.url)[1];
  const endedHere = fourEnd.sockets[1];
  fourEnd.server.shells = fourEnd.server.shells.filter((shell) => shell.id !== shellOfSocket(endedHere));
  hangUp(endedHere, 4002, 'done');
  await wait(20);
  assert.equal(ui.panes().length, 4, 'closed a pane with a shell to spare');
  assert.match(fourEnd.sockets[4].url, /shell=5&.*takeover=0/);
  const endsLast = fourEnd.sockets[4];
  fourEnd.server.shells = fourEnd.server.shells.filter((shell) => shell.id !== '5');
  hangUp(endsLast, 4002, 'done');
  await wait(20);
  assert.equal(ui.panes().length, 3, 'kept a pane with nothing to show');
  assertFills(fourEnd, 'after a shell ended');
  // Close all ends those not shown, then the panes', the focused one's last.
  const fourClosing = await buildShape([2, 2]);
  ui = sp(fourClosing);
  ui.focus(1);
  const focusedShell = ui.saved().focused;
  fourClosing.document.querySelectorAll('.terminal-tab')[0].dispatchEvent(new fourClosing.window.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
  [...fourClosing.document.querySelectorAll('.terminal-tab-menu button')].find((item) => item.textContent === 'Close all tabs').click();
  await wait(40);
  const closedOrder = fourClosing.requests.filter((request) => request.startsWith('DELETE')).map((request) => request.split('/').at(-1));
  const shownFour = ui.saved().columns.flatMap((column) => column.rows.map((row) => row.shell));
  assert.equal(closedOrder.length, 6);
  assert.deepEqual(closedOrder.slice(0, 2).sort(), ['1', '2', '3', '4', '5', '6'].filter((id) => !shownFour.includes(id)));
  assert.equal(closedOrder.at(-1), focusedShell);
  // Text size and theme reach all four, each fits, and a hidden browser tab connects none.
  const fourSized = await buildShape([2, 2]);
  ui = sp(fourSized);
  const fitsAtFour = fourSized.fits.length;
  ui.bar('A+').click();
  assert.deepEqual(fourSized.terminals.map((term) => term.options.fontSize), [14, 14, 14, 14]);
  assert.equal(fourSized.fits.length, fitsAtFour + 4);
  // The next page restores every shape, with focus and shares, each pane fitted before connecting.
  const saved22 = { columns: [
    { share: 0.6, rows: [{ share: 0.3, shell: '4' }, { share: 0.7, shell: '2' }] },
    { share: 0.4, rows: [{ share: 0.5, shell: '1' }, { share: 0.5, shell: '3' }] },
  ], focused: '1' };
  const restored22 = load(process.argv[1], { stored: '1', path: '/project/p1', shells: splitShells(1, 2, 3, 4, 5), split: [splitKey, JSON.stringify(saved22)] });
  await wait(50);
  ui = sp(restored22);
  assert.equal(ui.panes().length, 4);
  assert.deepEqual(ui.live().sort(), ['1', '2', '3', '4']);
  assert.deepEqual(ui.saved(), saved22);
  assert.equal(ui.selected(), '1');
  assertFills(restored22, 'restored');
  assert.deepEqual(boxesOf(restored22)[1], { x: 0, y: 0.3, w: 0.6, h: 0.7 });
  assert.equal(restored22.window.sessionStorage.getItem(shownKey), '4');
  // A shell gone gives its room to its neighbor, as closing its pane would.
  const oneGone = load(process.argv[1], { stored: '1', path: '/project/p1', shells: splitShells(1, 2, 3), split: [splitKey, JSON.stringify(saved22)] });
  await wait(50);
  ui = sp(oneGone);
  assert.deepEqual(ui.saved().columns.map((column) => column.rows.map((row) => row.shell)), [['2'], ['1', '3']]);
  assert.ok(near(ui.saved().columns[0].rows[0].share, 1));
  assertFills(oneGone, 'restored with one gone');
  // More than four, and a shell named twice, come back within the rules.
  const tooMany = { columns: [{ share: 1, rows: ['1', '2', '1', '3', '4', '5'].map((shell) => ({ share: 1, shell })) }], focused: '5' };
  const overfull = load(process.argv[1], { stored: '1', path: '/project/p1', shells: splitShells(1, 2, 3, 4, 5), split: [splitKey, JSON.stringify(tooMany)] });
  await wait(50);
  ui = sp(overfull);
  assert.equal(ui.panes().length, 4);
  assert.deepEqual(ui.live().sort(), ['1', '2', '3', '4']);
  assertFills(overfull, 'restored past the limit');
  // A hidden browser tab connects none of them, until it is looked at.
  const hiddenFour = load(process.argv[1], { stored: '1', hidden: true, path: '/project/p1', shells: splitShells(1, 2, 3, 4), split: [splitKey, JSON.stringify(saved22)] });
  await wait(50);
  assert.equal(hiddenFour.sockets.length, 0, 'a background tab connected a pane');
  hiddenFour.window.__hidden = false;
  hiddenFour.document.dispatchEvent(new hiddenFour.window.Event('visibilitychange'));
  await wait(30);
  assert.equal(hiddenFour.sockets.length, 4);
  // A command sent with a closed panel waits for the pane that had focus, and takes nothing.
  const sentToFour = load(process.argv[1], { path: '/project/p1', shells: splitShells(1, 2, 3, 4), split: [splitKey, JSON.stringify(saved22)] });
  await wait(30);
  const toFour = sentToFour.window.sendToTerminal('for the focused');
  await wait(40);
  assert.equal(sentToFour.sockets.length, 4);
  for (const socket of sentToFour.sockets) {
    assert.match(socket.url, /takeover=0/, 'sending took a shell');
    socket.onmessage(out('prompt'));
  }
  assert.equal(await toFour, null);
  const paneOf1 = sentToFour.sockets.findIndex((socket) => /shell=1&/.test(socket.url));
  assert.deepEqual(sentToFour.terminals[paneOf1].pastes, ['for the focused']);
  assert.equal(sentToFour.terminals.filter((term) => term.pastes).length, 1, 'went to more than one pane');

  // The panel at a side has its own size, and comes back there with the next page. The
  // page's room is a rule of web_shell.css, which only an open panel follows.
  const placeOf = (page) => page.document.documentElement.dataset.terminalPlace;
  const sideRoom = (page) => page.document.documentElement.style.getPropertyValue('--terminal-saved-width');
  const heightSaved = (page) => page.window.localStorage.getItem('rotari-terminal-height');
  for (const [where, growing, shrinking, drag] of [['right', 'ArrowLeft', 'ArrowRight', { clientX: 980 }], ['left', 'ArrowRight', 'ArrowLeft', { clientX: 1020 }]]) {
    const side = load(process.argv[1], { stored: '1', place: where, width: '500', height: '300', shells: [plain('1', 1)] });
    await wait(50);
    assert.equal(placeOf(side), where);
    assert.equal(sideRoom(side), '500px');
    const edge = side.document.querySelector('.terminal-resizer');
    assert.equal(edge.getAttribute('aria-orientation'), 'vertical');
    side.document.getElementById('terminal-panel').getBoundingClientRect = () => ({ width: 500, height: 900 });
    // Dragging the edge away from its side makes it wider, and the height is left alone.
    edge.onpointerdown({ preventDefault() {}, clientX: 1000, clientY: 100, pointerId: 1 });
    edge.onpointermove({ ...drag, clientY: 400 });
    assert.equal(sideRoom(side), '520px');
    side.document.getElementById('terminal-panel').getBoundingClientRect = () => ({ width: 520, height: 900 });
    edge.onpointerup();
    assert.equal(side.window.localStorage.getItem('rotari-terminal-width'), '520');
    assert.equal(heightSaved(side), '300', 'moving a side changed the height');
    assert.equal(edge.onpointermove, null);
    // Keys go the way the edge moves, and the page keeps its room.
    side.document.getElementById('terminal-panel').getBoundingClientRect = () => ({ width: 400, height: 900 });
    edge.onkeydown({ key: growing, preventDefault() {} });
    assert.equal(sideRoom(side), '424px');
    edge.onkeydown({ key: shrinking, preventDefault() {} });
    assert.equal(sideRoom(side), '376px');
    edge.onkeydown({ key: 'ArrowUp', preventDefault() {} });
    assert.equal(sideRoom(side), '376px', 'a key across the edge moved it');
    side.document.getElementById('terminal-panel').getBoundingClientRect = () => ({ width: 330, height: 900 });
    edge.onkeydown({ key: shrinking, preventDefault() {} });
    assert.equal(sideRoom(side), '320px', 'the panel went under its least');
    assert.equal(side.sockets.length, 1);
    assert.match(webShellCSS, new RegExp('html\\.terminal-open\\[data-terminal-place="' + where + '"\\] \\{\\s*--terminal-' + where + ': var\\(--terminal-saved-width, 480px\\);'));
  }
  // The page clears the panel on every side, and a maximized one still covers it all.
  assert.match(webShellCSS, /html\.terminal-open body \{[^}]*padding-left: var\(--terminal-left, 0px\);\s*padding-right: var\(--terminal-right, 0px\);/);
  assert.match(webShellCSS, /html\.terminal-open \.output-modal \{\s*right: var\(--terminal-right, 0px\);\s*bottom: var\(--terminal-height\);\s*left: var\(--terminal-left, 0px\);/);
  assert.match(webShellCSS, /\.terminal-send-note \{[^}]*right: calc\(var\(--terminal-right, 0px\) \+ 16px\);/);
  assert.match(webShellCSS, /terminal-maximized #terminal-panel \{[^}]*right: 0;\s*left: 0;\s*width: auto;/);
  assert.ok(webShellCSS.indexOf('[data-terminal-place="right"] #terminal-panel') < webShellCSS.indexOf('.terminal-maximized #terminal-panel'), 'a side would win over maximizing');
  // Anything else saved is the bottom, and a width stays within what leaves the page room.
  const badPlace = load(process.argv[1], { stored: '1', place: 'top', shells: [plain('1', 1)] });
  await wait(30);
  assert.equal(placeOf(badPlace), undefined);
  assert.equal(badPlace.document.querySelector('.terminal-resizer').getAttribute('aria-orientation'), 'horizontal');
  const wideSaved = load(process.argv[1], { place: 'right', width: '99999' });
  assert.equal(sideRoom(wideSaved), (wideSaved.window.innerWidth - 400) + 'px');
  // That room counts the sidebar, as wide as someone has made it, and only the panel's room gives way.
  const besideSidebar = load(process.argv[1], { place: 'right', width: '99999' });
  await wait(20);
  const sidebarOf = besideSidebar.document.querySelector('.sidebar');
  Object.defineProperty(sidebarOf, 'offsetWidth', { configurable: true, get: () => 200 });
  besideSidebar.server.observers.forEach((observer) => observer());
  assert.equal(sideRoom(besideSidebar), (besideSidebar.window.innerWidth - 200 - 400) + 'px');
  Object.defineProperty(sidebarOf, 'offsetWidth', { configurable: true, get: () => 300 });
  besideSidebar.server.observers.forEach((observer) => observer());
  assert.equal(sideRoom(besideSidebar), '324px');
  assert.equal(besideSidebar.window.localStorage.getItem('rotari-terminal-width'), '99999', 'the choice was lost to the room');
  // The sidebar's height follows a panel at the bottom, which must not put the panel's size back.
  const bottomDrag = load(process.argv[1], { stored: '1', height: '300', shells: [plain('1', 1)] });
  await wait(30);
  const bottomEdge = bottomDrag.document.querySelector('.terminal-resizer');
  const bottomHeight = () => bottomDrag.document.documentElement.style.getPropertyValue('--terminal-saved-height');
  bottomDrag.document.getElementById('terminal-panel').getBoundingClientRect = () => ({ height: 300 });
  bottomEdge.onpointerdown({ preventDefault() {}, clientY: 500, pointerId: 1 });
  bottomEdge.onpointermove({ clientY: 400 });
  bottomDrag.server.observers.forEach((observer) => observer());
  assert.equal(bottomHeight(), '400px', 'the panel went back to its saved height during a drag');
  bottomDrag.document.getElementById('terminal-panel').getBoundingClientRect = () => ({ height: parseInt(bottomHeight(), 10) });
  bottomEdge.onpointerup();
  assert.equal(bottomDrag.window.localStorage.getItem('rotari-terminal-height'), '400', 'the drag was not kept');
  bottomEdge.onkeydown({ key: 'ArrowUp', preventDefault() {} });
  bottomDrag.server.observers.forEach((observer) => observer());
  assert.equal(bottomHeight(), '424px', 'the panel went back to its saved height after a key');
  // The top bar is over the panel, so it wraps rather than running under it.
  assert.match(webShellCSS, /\[data-terminal-place\] \.content > \.topbar,\s*html\.terminal-open\[data-terminal-place\] \.content > \.topbar \.toolbar \{\s*flex-wrap: wrap;/);
  assert.equal(sideRoom(load(process.argv[1], { place: 'left', width: '5' })), '320px');
  // A closed panel still knows its place, before the page is drawn, and takes no room.
  const closedSide = load(process.argv[1], { place: 'left' });
  assert.equal(placeOf(closedSide), 'left');
  assert.equal(closedSide.document.documentElement.classList.contains('terminal-open'), false);
  assert.equal(closedSide.document.documentElement.style.getPropertyValue('--terminal-left'), '', 'a closed panel took room');
  // Under 1000 pixels it is at the bottom, and the choice comes back with the room, keeping every connection.
  const tightSide = load(process.argv[1], { stored: '1', place: 'right', tight: true, path: '/project/p1', shells: splitShells(1, 2), split: [splitKey, layout] });
  await wait(50);
  assert.equal(placeOf(tightSide), undefined, 'at a side in a narrow window');
  assert.equal(tightSide.document.querySelector('.terminal-resizer').getAttribute('aria-orientation'), 'horizontal');
  assert.equal(tightSide.window.localStorage.getItem('rotari-terminal-place'), 'right', 'a narrow window lost the choice');
  const tightSockets = tightSide.sockets.length;
  tightSide.server.tight = false;
  tightSide.server.mediaListeners.forEach((listener) => listener());
  assert.equal(placeOf(tightSide), 'right');
  assert.equal(tightSide.document.querySelector('.terminal-resizer').getAttribute('aria-orientation'), 'vertical');
  tightSide.server.tight = true;
  tightSide.server.mediaListeners.forEach((listener) => listener());
  assert.equal(placeOf(tightSide), undefined);
  assert.equal(tightSide.sockets.length, tightSockets, 'moving the panel reconnected');
  assert.equal(tightSide.sockets.some((socket) => socket.closed), false, 'moving the panel closed a connection');

  // The menu moves the panel at once, keeps it there with the next page, and changes no connection.
  const moving = await openSplit([1, 2]);
  ui = sp(moving);
  ui.split();
  await wait(10);
  const placeButton = ui.bar('Panel position');
  const placeMenu = () => [...moving.document.querySelectorAll('.terminal-tab-menu button')];
  const choosePlace = (label) => {
    placeButton.click();
    placeMenu().find((item) => item.textContent === label).click();
  };
  assert.equal(placeButton.textContent, '');
  assert.equal(placeButton.disabled, false);
  assert.equal(placeButton.querySelector('path').getAttribute('d'), 'M1.5 9.5h13');
  placeButton.click();
  assert.deepEqual(placeMenu().map((item) => [item.textContent, item.getAttribute('aria-checked'), item.getAttribute('role')]), [['Bottom', 'true', 'menuitemradio'], ['Left', 'false', 'menuitemradio'], ['Right', 'false', 'menuitemradio'], ['Window', 'false', 'menuitemradio']]);
  moving.document.dispatchEvent(new moving.window.KeyboardEvent('keydown', { key: 'Escape' }));
  assert.equal(moving.document.querySelector('.terminal-tab-menu'), null, 'Escape left the menu');
  const movingSockets = moving.sockets.length;
  const focusesBeforeMoving = moving.terminals[1].focuses;
  for (const [label, attribute, line] of [['Right', 'right', 'M10.5 2.5v11'], ['Left', 'left', 'M5.5 2.5v11'], ['Bottom', undefined, 'M1.5 9.5h13']]) {
    choosePlace(label);
    assert.equal(placeOf(moving), attribute);
    assert.equal(placeButton.querySelector('path').getAttribute('d'), line);
    assert.equal(moving.window.localStorage.getItem('rotari-terminal-place'), label.toLowerCase());
    assert.equal(moving.document.querySelector('.terminal-tab-menu'), null, 'choosing left the menu open');
  }
  assert.equal(moving.sockets.length, movingSockets, 'moving the panel reconnected');
  assert.equal(moving.sockets.some((socket) => socket.closed), false);
  assert.ok(moving.terminals[1].focuses > focusesBeforeMoving, 'the keys did not come back to the focused pane');
  assert.equal(ui.panes().length, 2);
  // A panel loaded at a side shows that in its picture and its menu.
  const loadedRight = load(process.argv[1], { stored: '1', place: 'right', shells: [plain('1', 1)] });
  await wait(30);
  const loadedButton = sp(loadedRight).bar('Panel position');
  assert.equal(loadedButton.querySelector('path').getAttribute('d'), 'M10.5 2.5v11');
  loadedButton.click();
  assert.equal(loadedRight.document.querySelector('.terminal-tab-menu [aria-checked=true]').textContent, 'Right');
  // A narrow window offers only the bottom, says why, and keeps the choice marked.
  const tightMenu = load(process.argv[1], { stored: '1', place: 'left', tight: true, shells: [plain('1', 1)] });
  await wait(30);
  const tightButton = sp(tightMenu).bar('Panel position');
  assert.equal(tightButton.querySelector('path').getAttribute('d'), 'M1.5 9.5h13', 'the picture showed a place it is not in');
  tightButton.click();
  const tightItems = [...tightMenu.document.querySelectorAll('.terminal-tab-menu button')];
  assert.deepEqual(tightItems.map((item) => [item.textContent, item.disabled]), [['Bottom', false], ['Left (needs a wider window)', true], ['Right (needs a wider window)', true], ['Window', false]]);
  assert.equal(tightItems[1].getAttribute('aria-checked'), 'true');
  // At a side the tabs have a row of their own, and maximized there is room for one row.
  assert.match(webShellCSS, /\[data-terminal-place\]:not\(\.terminal-maximized\) \.terminal-bar \{\s*flex-wrap: wrap;/);
  assert.match(webShellCSS, /\[data-terminal-place\]:not\(\.terminal-maximized\)\s+\.terminal-tabs \{\s*order: 1;\s*flex: 1 0 100%;/);

  // A window of its own opens at once on the project its path names, fills itself, has
  // nothing to resize, maximize, hide, or move, and leaves the pages' panel as it was.
  const win = load(process.argv[6], { path: '/_terminal/window/project/p1', shells: [inProject('1', 1, 'p1'), inProject('2', 1, 'p2')] });
  await wait(50);
  assert.equal(win.document.getElementById('terminal-panel').hidden, false);
  assert.ok(win.requests.includes('GET /_terminal/here?project=p1'), win.requests.join(', '));
  assert.equal(win.sockets.length, 1);
  assert.match(win.sockets[0].url, /[?]shell=1&/);
  assert.equal(win.window.localStorage.getItem('rotari-terminal-open'), null, 'the window changed whether the pages open the panel');
  assert.equal(win.document.title, 'p1 - Terminal - rotari');
  assert.equal(win.document.querySelector('.terminal-resizer'), null);
  const winLabels = [...win.document.querySelectorAll('.terminal-bar button')].map((item) => item.getAttribute('aria-label'));
  for (const gone of ['Maximize', 'Hide']) assert.ok(!winLabels.includes(gone), gone + ' is in the window');
  assert.ok(winLabels.includes('Split right') && winLabels.includes('Clear'), winLabels.join(', '));
  // Its project list goes to the other project's window, not its page.
  assert.deepEqual([...win.document.querySelectorAll('.terminal-groups option')].map((option) => option.value), ['', '/_terminal/window/project/p2']);
  assert.match(webShellCSS, /html\[data-terminal-window\] #terminal-panel \{\s*top: 0;\s*height: auto;/);
  // Opening one on a project with no shell starts one there.
  const emptyWin = load(process.argv[6], { path: '/_terminal/window/project/p3' });
  await wait(50);
  assert.ok(emptyWin.requests.includes('POST /_terminal/shells?rows=24&cols=80&project=p3'), emptyWin.requests.join(', '));
  // Opened for a shell in a page's place, it starts a new one there, once.
  const startWin = load(process.argv[6], { path: '/_terminal/window/project/p1?start=' + encodeURIComponent('project=p1&run=r1'), shells: [inProject('1', 1, 'p1')] });
  await wait(50);
  assert.ok(startWin.requests.includes('POST /_terminal/shells?rows=24&cols=80&project=p1&run=r1'), startWin.requests.join(', '));
  assert.equal(startWin.window.location.search, '');

  // Window moves the panel to a window of its own, opened straight from the click, with the
  // panes this browser tab has. The page lets go of its shells saying nothing, and the
  // Terminal button brings the window to the front.
  const origin = 'http://127.0.0.1:9';
  const post = (page, data, source) => {
    const event = new page.window.Event('message');
    Object.defineProperties(event, { data: { value: data }, origin: { value: origin }, source: { value: source } });
    page.window.dispatchEvent(event);
  };
  const fakeWindow = () => ({ closed: false, focuses: 0, messages: [], focus() { this.focuses++; }, postMessage(message, target) { this.messages.push(JSON.parse(JSON.stringify([message, target]))); } });
  const popping = load(process.argv[1], { stored: '1', path: '/project/p1/run/r1', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1')] });
  await wait(50);
  sp(popping).split();
  await wait(30);
  assert.deepEqual(sp(popping).live(), ['1', '2']);
  const popped = [];
  let poppedWindow = fakeWindow();
  popping.window.open = (...args) => { popped.push(args); return poppedWindow; };
  const chooseWindow = (page) => {
    sp(page).bar('Panel position').click();
    [...page.document.querySelectorAll('.terminal-tab-menu button')].find((item) => item.textContent === 'Window').click();
  };
  chooseWindow(popping);
  assert.equal(popped.length, 1);
  const [address, hash] = popped[0][0].split('#');
  assert.equal(address, '/_terminal/window/project/p1', 'the window is not on the page\'s project');
  assert.equal(popped[0][2], 'popup,width=900,height=600');
  const handover = JSON.parse(decodeURIComponent(hash));
  assert.equal(handover['rotari-terminal-shell:/b:p1'], '1');
  assert.deepEqual(JSON.parse(handover['rotari-terminal-layout:/b:p1']).columns.map((column) => column.rows.map((row) => row.shell)), [['1'], ['2']]);
  assert.deepEqual(sp(popping).live(), [], 'the page kept a shell');
  assert.equal(popping.document.getElementById('terminal-panel').hidden, true);
  assert.equal(popping.document.documentElement.classList.contains('terminal-open'), false, 'the page kept room for the panel');
  assert.equal(popping.window.localStorage.getItem('rotari-terminal-open'), '1', 'moving the panel closed it on other pages');
  assert.equal(popping.window.sessionStorage.getItem('rotari-terminal-windowed'), '1');
  for (const socket of popping.sockets) socket.onclose?.({ code: 4001, reason: '' });
  for (const pane of sp(popping).panes()) assert.doesNotMatch(pane.querySelector('.terminal-pane-status').textContent, /Another tab/, 'a pane said another tab took its shell');
  popping.document.getElementById('terminal-button').click();
  assert.equal(poppedWindow.focuses, 1);
  assert.equal(popping.sockets.length, 2, 'the Terminal button connected');
  // A send button types into the window's focused pane, and says where when that is in
  // another project; one the window does not answer says so.
  const sendNote = () => popping.document.querySelector('.terminal-send-note').textContent;
  const toWindow = popping.window.sendToTerminalAndSay('ls -l');
  await wait(10);
  assert.deepEqual(poppedWindow.messages.at(-1), [{ type: 'rotari-terminal-send', id: 1, text: 'ls -l', path: '/project/p1' }, origin]);
  assert.equal(poppedWindow.focuses, 2);
  post(popping, { type: 'rotari-terminal-sent', id: 1, problem: null, project: '' }, poppedWindow);
  await toWindow;
  assert.match(sendNote(), /^Sent\. Nothing runs/);
  const toOther = popping.window.sendToTerminalAndSay('ls');
  await wait(10);
  post(popping, { type: 'rotari-terminal-sent', id: 2, problem: null, project: 'p2' }, poppedWindow);
  await toOther;
  assert.match(sendNote(), /window, in p2\./);
  const refusedSend = popping.window.sendToTerminalAndSay('ls');
  await wait(10);
  post(popping, { type: 'rotari-terminal-sent', id: 3, problem: 'No shell here.', project: '' }, poppedWindow);
  await refusedSend;
  assert.equal(sendNote(), 'No shell here.');
  assert.equal(popping.sockets.length, 2, 'sending connected the page');
  // Terminal here asks the window for a shell in the page's place.
  popping.window.openTerminalHere();
  assert.deepEqual(poppedWindow.messages.at(-1), [{ type: 'rotari-terminal-here', path: '/project/p1', place: 'project=p1&run=r1' }, origin]);
  assert.equal(popping.sockets.length, 2, 'Terminal here connected the page');
  // What the window's panes become reaches the page, from that window only.
  post(popping, { type: 'rotari-terminal-saved', entries: { 'rotari-terminal-shell:/b:p1': '2', 'rotari-terminal-layout:/b:p1': null, 'rotari-theme': 'dark' } }, fakeWindow());
  assert.equal(popping.window.sessionStorage.getItem('rotari-terminal-shell:/b:p1'), '1', 'another window changed the panes');
  post(popping, { type: 'rotari-terminal-saved', entries: { 'rotari-terminal-shell:/b:p1': '2', 'rotari-terminal-layout:/b:p1': null, 'rotari-theme': 'dark' } }, poppedWindow);
  assert.equal(popping.window.sessionStorage.getItem('rotari-terminal-shell:/b:p1'), '2');
  assert.equal(popping.window.sessionStorage.getItem('rotari-terminal-layout:/b:p1'), null);
  assert.equal(popping.window.localStorage.getItem('rotari-theme'), null, 'the window set something not its panes');
  // The window's project list takes the page along, only from that window and only on this server.
  const assigned = [];
  post(popping, { type: 'rotari-terminal-go', path: '//attacker.example/x' }, poppedWindow);
  post(popping, { type: 'rotari-terminal-go', path: '/project/p2' }, fakeWindow());
  assert.equal(popping.window.sessionStorage.getItem('rotari-terminal-followed'), '/project/p1', 'a path not on this server, or another window, moved the page');
  post(popping, { type: 'rotari-terminal-go', path: '/project/p2' }, poppedWindow);
  assert.equal(popping.window.sessionStorage.getItem('rotari-terminal-followed'), '/project/p2', 'the page did not go with its window');
  popping.window.sessionStorage.setItem('rotari-terminal-followed', '/project/p1');
  // Closing the window puts the panel back, with the panes it last had.
  poppedWindow.closed = true;
  await wait(600);
  assert.equal(popping.document.getElementById('terminal-panel').hidden, false);
  assert.equal(popping.document.documentElement.classList.contains('terminal-open'), true);
  assert.deepEqual(sp(popping).live(), ['2']);
  assert.equal(sp(popping).panes().length, 1);
  assert.equal(popping.window.sessionStorage.getItem('rotari-terminal-windowed'), null);
  // Back to page says so first.
  poppedWindow = fakeWindow();
  chooseWindow(popping);
  assert.deepEqual(sp(popping).live(), []);
  post(popping, { type: 'rotari-terminal-back' }, poppedWindow);
  await wait(30);
  assert.deepEqual(sp(popping).live(), ['2']);
  // A blocked window leaves the panel as it was, and says why.
  poppedWindow = null;
  chooseWindow(popping);
  assert.deepEqual(sp(popping).live(), ['2']);
  assert.match(popping.document.querySelector('.terminal-status').textContent, /blocked the window/);
  // Following a link while the panel is in a window takes no shell: the page waits for the
  // window to say it is there, and puts the panel back once it closes.
  const following = load(process.argv[1], { stored: '1', path: '/project/p1/run/r1', remembered: ['rotari-terminal-windowed', '1'], split: ['rotari-terminal-followed', '/project/p0'], shells: [inProject('1', 1, 'p1')] });
  await wait(50);
  assert.equal(following.sockets.length, 0, 'a page took the window\'s shell');
  assert.equal(following.document.documentElement.classList.contains('terminal-open'), false);
  const found = fakeWindow();
  post(following, { type: 'rotari-terminal-window' }, found);
  // The page is of another project than the window last followed, so the window follows it.
  assert.deepEqual(found.messages, [[{ type: 'rotari-terminal-follow', path: '/project/p1' }, origin]]);
  assert.equal(following.window.sessionStorage.getItem('rotari-terminal-followed'), '/project/p1');
  following.document.getElementById('terminal-button').click();
  assert.equal(found.focuses, 1);
  found.closed = true;
  await wait(600);
  assert.deepEqual(sp(following).live(), ['1']);
  // A page of the project or basedir the window last followed, or of none, leaves the window
  // as it is, whatever was chosen in it since. Recent jobs is a page of the basedir that All
  // projects is.
  for (const [path, last] of [['/project/p1', '/project/p1'], ['/jobs/', '/'], ['/', '/'], ['/activity', '/project/p1'], ['/_basedir/c/search', '/project/p1']]) {
    const staying = load(process.argv[1], { stored: '1', path, remembered: ['rotari-terminal-windowed', '1'], split: ['rotari-terminal-followed', last] });
    await wait(30);
    const kept = fakeWindow();
    post(staying, { type: 'rotari-terminal-window' }, kept);
    assert.deepEqual(kept.messages, [], path + ' moved the window');
  }
  // A basedir's page the window did not follow takes it to the basedir's All projects.
  for (const [path, want] of [['/jobs/', '/'], ['/_basedir/c/', '/_basedir/c/'], ['/_basedir/c/jobs/', '/_basedir/c/']]) {
    const moving = load(process.argv[1], { stored: '1', path, remembered: ['rotari-terminal-windowed', '1'], split: ['rotari-terminal-followed', '/project/p1'] });
    await wait(30);
    const moved = fakeWindow();
    post(moving, { type: 'rotari-terminal-window' }, moved);
    assert.deepEqual(moved.messages, [[{ type: 'rotari-terminal-follow', path: want }, origin]], path + ' did not move the window');
  }
  // Popping out remembers the page's project as followed.
  assert.equal(popping.window.sessionStorage.getItem('rotari-terminal-followed'), '/project/p1');
  // A window that does not answer has gone, and the panel comes back.
  const orphaned = load(process.argv[1], { stored: '1', path: '/project/p1', remembered: ['rotari-terminal-windowed', '1'], shells: [inProject('1', 1, 'p1')] });
  await wait(2800);
  assert.equal(orphaned.sockets.length, 0);
  await wait(400);
  assert.deepEqual(sp(orphaned).live(), ['1']);

  // The window takes the panes from its address, once, and tells its page what they become.
  const told = [];
  const handing = encodeURIComponent(JSON.stringify({ 'rotari-terminal-shell:/b:p1': '2', 'rotari-terminal-other': 'x' }));
  const takes = load(process.argv[6], { path: '/_terminal/window/project/p1#' + handing, shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1'), inProject('3', 1, 'p2')], opener: { closed: false, postMessage: (message, target) => told.push(JSON.parse(JSON.stringify([message, target]))) } });
  await wait(50);
  assert.deepEqual(sp(takes).live(), ['2']);
  assert.equal(takes.window.location.hash, '');
  assert.equal(takes.window.sessionStorage.getItem('rotari-terminal-other'), null);
  assert.deepEqual(told[0], [{ type: 'rotari-terminal-window' }, origin]);
  sp(takes).tab('1');
  const saved = told.filter(([message]) => message.type === 'rotari-terminal-saved').at(-1);
  assert.deepEqual(saved, [{ type: 'rotari-terminal-saved', entries: { 'rotari-terminal-shell:/b:p1': '1', 'rotari-terminal-layout:/b:p1': null } }, origin]);
  // It types what its page sends, answering which project took it, and takes nothing from
  // anyone else.
  const takesOpener = takes.window.opener;
  for (const socket of takes.sockets) socket.readyState = 1;
  post(takes, { type: 'rotari-terminal-send', id: 7, text: 'make', path: '/project/p1' }, takesOpener);
  await wait(30);
  assert.deepEqual(takes.terminals.flatMap((term) => term.pastes || []), ['make']);
  assert.deepEqual(told.at(-1), [{ type: 'rotari-terminal-sent', id: 7, problem: null, project: '' }, origin]);
  post(takes, { type: 'rotari-terminal-send', id: 8, text: 'more', path: '/project/p9' }, takesOpener);
  await wait(30);
  assert.deepEqual(told.at(-1), [{ type: 'rotari-terminal-sent', id: 8, problem: null, project: 'p1' }, origin]);
  post(takes, { type: 'rotari-terminal-send', id: 9, text: 'rm', path: '/project/p1' }, fakeWindow());
  await wait(30);
  assert.deepEqual(takes.terminals.flatMap((term) => term.pastes || []), ['make', 'more'], 'typed what another window sent');
  // Terminal here on its project starts a shell in the page's place.
  post(takes, { type: 'rotari-terminal-here', path: '/project/p1', place: 'project=p1&run=r1' }, takesOpener);
  await wait(30);
  assert.ok(takes.requests.includes('POST /_terminal/shells?rows=24&cols=80&project=p1&run=r1'), takes.requests.join(', '));
  // Choosing a project in it takes its page along, as the panel's list does.
  takes.document.querySelector('.terminal-groups').value = '';
  const otherOption = [...takes.document.querySelectorAll('.terminal-groups option')].find((option) => option.value);
  assert.equal(otherOption.value, '/_terminal/window/project/p2');
  takes.document.querySelector('.terminal-groups').value = otherOption.value;
  takes.document.querySelector('.terminal-groups').onchange();
  assert.deepEqual(told.at(-1), [{ type: 'rotari-terminal-go', path: '/project/p2' }, origin]);
  // Its Panel position offers only Back to page, which tells the page and closes.
  sp(takes).bar('Panel position').click();
  const backItems = [...takes.document.querySelectorAll('.terminal-tab-menu button')];
  assert.deepEqual(backItems.map((item) => item.textContent), ['Back to page']);
  let closedWindow = false;
  takes.window.close = () => (closedWindow = true);
  backItems[0].click();
  assert.deepEqual(told.at(-1), [{ type: 'rotari-terminal-back' }, origin]);
  assert.equal(closedWindow, true);
  // With its page gone, it stays.
  sp(win).bar('Panel position').click();
  win.window.close = () => assert.fail('closed a window with no page to go back to');
  [...win.document.querySelectorAll('.terminal-tab-menu button')][0].click();
  assert.match(win.document.querySelector('.terminal-status').textContent, /page that opened this window is gone/);
  // Closing the browser tab that opened it closes it too, leaving the shells running; until
  // then it stays, and one opened by its address has no tab to go with.
  const tabOpener = { closed: false, postMessage() {} };
  const tabbed = load(process.argv[6], { path: '/_terminal/window/project/p1', shells: [inProject('1', 1, 'p1')], opener: tabOpener });
  let tabbedClosed = 0;
  tabbed.window.close = () => tabbedClosed++;
  await wait(1100);
  assert.equal(tabbedClosed, 0, 'closed with its tab still open');
  tabOpener.closed = true;
  await wait(1100);
  assert.equal(tabbedClosed, 1, 'stayed after its tab closed');
  assert.equal(tabbed.requests.filter((request) => request.startsWith('DELETE')).length, 0, 'ended a shell on closing');

  // A basedir's All projects shows the basedir's own shells, and Activity those of no
  // project; each names its group when asking where a shell would start.
  for (const [path, query, shown] of [['/', 'basedir=', '1'], ['/_basedir/c/', 'basedir=c', '3'], ['/activity', '', '2'], ['/search', '', '2']]) {
    const grouped = load(process.argv[1], { stored: '1', path, shells: [ofBasedir('1', 1), plain('2', 1), ofBasedir('3', 1, '/c'), inProject('4', 1, 'p1')] });
    await wait(50);
    assert.ok(grouped.requests.includes('GET /_terminal/here?' + query), path + ': ' + grouped.requests.join(', '));
    assert.equal(grouped.sockets.length, 1, path + ' did not show one shell');
    assert.match(grouped.sockets[0].url, new RegExp('shell=' + shown + '&'), path + ' showed another group\'s shell');
  }

  // The sidebar marks each project with shells, with the panel closed, and again after
  // the sidebar is drawn again, and a basedir's All projects for the basedir's own shells;
  // a basedir's row and a shell of no project mark nothing.
  const marking = load(process.argv[1], { shells: [inProject('1', 1, 'p 1'), inProject('2', 2, 'p 1'), inProject('3', 1, 'p3'), plain('4', 1), ofBasedir('5', 1, '/c')] });
  const sidebarRows = '<div class="sidebar-project-row basedir-row"><a class="sidebar-project-link" href="/">b</a></div>' +
    '<div class="sidebar-project all-projects"><div class="sidebar-project-row"><a class="sidebar-project-link" href="/">All projects</a></div></div>' +
    '<div class="sidebar-project-row basedir-row"><a class="sidebar-project-link" href="/_basedir/c/">c</a></div>' +
    '<div class="sidebar-project all-projects"><div class="sidebar-project-row"><a class="sidebar-project-link" href="/_basedir/c/">All projects</a></div></div>' +
    '<div class="sidebar-project"><div class="sidebar-project-row"><a class="sidebar-project-link" href="/project/p%201">p 1</a></div></div>' +
    '<div class="sidebar-project"><div class="sidebar-project-row"><a class="sidebar-project-link" href="/project/p2">p2</a></div></div>';
  marking.document.getElementById('sidebar-basedirs').innerHTML = sidebarRows;
  await wait(50);
  const marks = () => [...marking.document.querySelectorAll('#sidebar-basedirs .sidebar-project-row')].map((row) => [row.textContent, row.dataset.terminalShells || '', row.title]);
  assert.deepEqual(marks(), [['b', '', ''], ['All projects', '', ''], ['c', '', ''], ['All projects', '1', '1 shell in the terminal'], ['p 1', '2', '2 shells in the terminal'], ['p2', '', '']]);
  marking.document.getElementById('sidebar-basedirs').innerHTML = sidebarRows;
  await wait(10);
  assert.deepEqual(marks()[4], ['p 1', '2', '2 shells in the terminal'], 'a sidebar drawn again lost its mark');
  assert.deepEqual(marks()[3], ['All projects', '1', '1 shell in the terminal'], 'a sidebar drawn again lost its basedir\'s mark');
  marking.server.shells = marking.server.shells.filter((shell) => shell.id !== '2');
  marking.polls[0]();
  await wait(20);
  assert.deepEqual(marks()[4], ['p 1', '1', '1 shell in the terminal']);
  marking.server.shells = marking.server.shells.filter((shell) => shell.id !== '1');
  marking.polls[0]();
  await wait(20);
  assert.deepEqual(marks()[4], ['p 1', '', ''], 'a project with no shell kept its mark');
  assert.match(webShellCSS, /\.sidebar-project-row\[data-terminal-shells\]::after \{/);

  // All shells shows every project's shells in the page's dialog, as a log is, this page's
  // first, each with where it is now, or where it started, said as such, when the server
  // cannot tell.
  const elsewhereShell = { ...inProject('4', 1, 'p1', '/c'), name: 'build', link: '/_basedir/c/project/p1', unseen_output: true };
  const touring = load(process.argv[1], { stored: '1', path: '/project/p2', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1'), inProject('3', 1, 'p2'), elsewhereShell] });
  touring.server.cwds = { 1: '/w/p1/src', 3: '/w/p2/deep', 4: '/c/p1' };
  touring.server.commands = { 1: 'vim notes.md', 2: '?' };
  touring.server.processes = { 1: [{ pid: 41, ppid: 40, depth: 1, command: 'vim notes.md' }, { pid: 42, ppid: 41, depth: 2, command: 'sh -c make' }, { pid: 43, ppid: 42, depth: 3, command: 'make' }], 2: '?', 4: [{ pid: 51, ppid: 50, depth: 1, command: 'sleep 600' }] };
  await wait(50);
  const allShellsButton = (page) => [...page.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'All shells');
  const overviewButton = allShellsButton(touring);
  assert.equal(overviewButton.getAttribute('aria-pressed'), 'false');
  const tour = () => touring.document.querySelector('.terminal-overview');
  const cards = () => [...touring.document.querySelectorAll('.terminal-overview-card')];
  const card = (id) => cards().find((item) => item.dataset.shell === id);
  // Under each project's name, what its last run is doing, read once for the overview.
  const stateNow = Date.parse('2026-11-10T12:00:00Z');
  touring.server.projectStates = { now: new Date(stateNow).toISOString(), projects: [
    { basedir: '/b', project: 'p2', runs: 3, last_run: { run_id: 'r-3', run_name: 'nightly', lifecycle: 'running', tone: 'run', started_at: new Date(stateNow - 12.5 * 60000).toISOString(), jobs: { run: 3, pending: 5, ok: 2 } } },
    { basedir: '/b', project: 'p1', runs: 0 },
    { basedir: '/c', project: 'p1', status_error: 'permission denied' },
  ] };
  overviewButton.click();
  await wait(30);
  const projectLines = () => [...tour().querySelectorAll('.terminal-overview-project-state')];
  assert.deepEqual(projectLines().map((line) => line.textContent), ['Last run running, started 12m ago \u00b7 3 in progress \u00b7 5 pending \u00b7 0 failed', 'No runs yet', 'Its state could not be read: permission denied']);
  assert.deepEqual(projectLines().map((line) => line.previousElementSibling.tagName), ['H3', 'H3', 'H3'], 'a project\'s state is not under its name');
  const runLine = projectLines()[0].querySelector('button.terminal-overview-project-state-run');
  assert.ok(runLine.classList.contains('tone-run'));
  assert.equal(runLine.title, 'Open run nightly (r-3)');
  assert.deepEqual([...runLine.querySelectorAll('.terminal-overview-project-state-count')].map((part) => part.className), ['terminal-overview-project-state-count tone-run', 'terminal-overview-project-state-count tone-pending', 'terminal-overview-project-state-count terminal-overview-project-state-none']);
  assert.ok(projectLines()[2].classList.contains('terminal-overview-project-state-error'));
  assert.equal(touring.requests.filter((request) => request === 'GET /_terminal/projects').length, 1, 'the projects were not read once for the overview');
  touring.polls.forEach((poll) => poll());
  await wait(20);
  assert.equal(touring.requests.filter((request) => request === 'GET /_terminal/projects').length, 1, 'the five-second list read the projects');
  assert.match(webShellCSS, /\.terminal-overview-project-state-count\.tone-bad \{\s*color: var\(--s-bad\);/);
  assert.equal(overviewButton.getAttribute('aria-pressed'), 'true');
  const tourDialog = tour().parentElement;
  assert.equal(tourDialog.parentElement, touring.document.body, 'the overview is not the page\'s dialog');
  assert.deepEqual([...tourDialog.classList], ['output-modal', 'terminal-overview-modal']);
  assert.ok(tour().classList.contains('output-panel'));
  assert.equal(tourDialog.getAttribute('aria-modal'), 'true');
  assert.equal(touring.document.getElementById(tourDialog.getAttribute('aria-labelledby')).textContent, 'All shells');
  assert.deepEqual([...tour().querySelectorAll('h3 span')].map((item) => item.textContent), ['p2', 'p1 in /b', 'p1 in /c']);
  assert.deepEqual(cards().map((item) => [item.dataset.shell, item.querySelector('.terminal-overview-name').textContent, item.querySelector('.terminal-overview-state').textContent, item.querySelector('.terminal-overview-dir').textContent]), [
    ['3', 'Shell 1', 'shown', '/w/p2/deep'],
    ['1', 'Shell 1', '', '/w/p1/src'],
    ['2', 'Shell 2', '', 'started in /w/p1'],
    ['4', 'build', 'new output', '/c/p1'],
  ]);
  assert.ok(card('2').querySelector('.terminal-overview-dir-unknown').title.includes('no such process'));
  // A card names what its shell runs, and nothing while the shell waits at its prompt.
  const runningOf = (id) => { const line = card(id).parentElement.querySelector('.terminal-overview-command'); return line.hidden ? null : [line.textContent, line.title, line.classList.contains('terminal-overview-command-unknown')]; };
  assert.deepEqual(runningOf('1'), ['vim notes.md', 'Running vim notes.md', false]);
  assert.deepEqual(runningOf('2'), ['running program not known', 'no such group', true]);
  assert.equal(runningOf('3'), null);
  // Beside it, how many processes the shell has started, which open into their tree; with
  // nothing in the foreground, they are background ones.
  assert.equal(card('3').parentElement.querySelector('.terminal-overview-run').hidden, true, 'a shell at rest has a line of what runs');
  const countOf = (id) => card(id).parentElement.querySelector('.terminal-overview-processes');
  const treeOf = (id) => card(id).parentElement.querySelector('.terminal-overview-process-list');
  assert.deepEqual([countOf('1').textContent, countOf('1').hidden, countOf('1').getAttribute('aria-expanded'), treeOf('1').hidden], ['3 processes', false, 'false', true]);
  assert.equal(countOf('1').parentElement.parentElement, card('1').parentElement, 'the count is not in the card\'s box');
  assert.equal(countOf('1').closest('button.terminal-overview-card'), null);
  countOf('1').click();
  assert.deepEqual([countOf('1').getAttribute('aria-expanded'), treeOf('1').hidden], ['true', false]);
  assert.deepEqual([...treeOf('1').querySelectorAll('li')].map((line) => [line.style.paddingLeft, line.querySelector('.terminal-overview-pid').textContent, line.textContent.slice(2)]), [['0px', '41', 'vim notes.md'], ['14px', '42', 'sh -c make'], ['28px', '43', 'make']]);
  assert.notEqual(touring.document.querySelector('.terminal-overview'), null, 'opening the tree chose the shell');
  countOf('1').click();
  assert.equal(treeOf('1').hidden, true);
  assert.deepEqual([countOf('4').textContent, countOf('4').title], ['1 background process', 'Show the processes this shell has started']);
  assert.deepEqual([countOf('2').textContent, countOf('2').disabled, countOf('2').title], ['processes not known', true, 'no /proc']);
  assert.match(webShellCSS, /\.terminal-overview-run\s+\.terminal-overview-command:not\(\[hidden\]\)\s+\+ \.terminal-overview-processes::before \{\s*content: "\\00b7\\00a0";/);
  assert.ok(card('3').classList.contains('terminal-overview-current'));
  assert.ok(card('4').classList.contains('terminal-overview-unseen'));
  assert.equal(touring.sockets.length, 1, 'the overview connected to a shell');
  // Keys move between cards, and Escape goes back to the shell.
  assert.equal(touring.document.activeElement, card('3'));
  const key = (name) => touring.document.activeElement.dispatchEvent(new touring.window.KeyboardEvent('keydown', { key: name, bubbles: true }));
  key('ArrowRight');
  assert.equal(touring.document.activeElement, card('1'));
  key('End');
  assert.equal(touring.document.activeElement, card('4'));
  key('ArrowLeft');
  assert.equal(touring.document.activeElement, card('2'));
  const termFocuses = touring.terminals[0].focuses || 0;
  key('Escape');
  assert.equal(tour(), null);
  assert.equal(overviewButton.getAttribute('aria-pressed'), 'false');
  assert.equal(touring.terminals[0].focuses, termFocuses + 1);
  // A click beside it closes it, and one inside does not.
  overviewButton.click();
  await wait(30);
  tour().click();
  assert.notEqual(tour(), null, 'a click inside closed it');
  tour().parentElement.click();
  assert.equal(tour(), null, 'a click beside it left it open');
  // A shell of this page's project is shown in the focused pane, closing it; Refresh takes a new view.
  overviewButton.click();
  await wait(30);
  touring.server.shells = [...touring.server.shells, inProject('5', 2, 'p2')];
  [...tour().querySelectorAll('button')].find((item) => item.textContent === 'Refresh').click();
  await wait(30);
  assert.deepEqual(cards().map((item) => item.dataset.shell), ['3', '5', '1', '2', '4']);
  // When the projects cannot be read, each line says why.
  touring.server.projectStates = 500;
  [...tour().querySelectorAll('button')].find((item) => item.textContent === 'Refresh').click();
  await wait(30);
  assert.deepEqual(projectLines().map((line) => line.textContent), ['Its state could not be read: refused', 'Its state could not be read: refused', 'Its state could not be read: refused']);
  card('5').click();
  await wait(20);
  assert.equal(tour(), null);
  assert.match(touring.sockets.at(-1).url, /[?]shell=5&/);
  // Another project's shell is the one its page shows next, in the layout it had.
  touring.window.sessionStorage.setItem('rotari-terminal-layout:/b:p1', JSON.stringify({ columns: [{ share: 0.5, rows: [{ share: 1, shell: '1' }] }, { share: 0.5, rows: [{ share: 1, shell: '9' }] }], focused: '9' }));
  overviewButton.click();
  await wait(30);
  card('2').click();
  assert.equal(touring.window.sessionStorage.getItem('rotari-terminal-shell:/b:p1'), '2');
  assert.deepEqual(JSON.parse(touring.window.sessionStorage.getItem('rotari-terminal-layout:/b:p1')), { columns: [{ share: 0.5, rows: [{ share: 1, shell: '1' }] }, { share: 0.5, rows: [{ share: 1, shell: '2' }] }], focused: '2' });
  overviewButton.click();
  await wait(30);
  card('4').click();
  assert.equal(touring.window.sessionStorage.getItem('rotari-terminal-shell:/c:p1'), '4');
  assert.equal(touring.window.sessionStorage.getItem('rotari-terminal-layout:/c:p1'), null);
  // Hiding the panel closes it.
  overviewButton.click();
  await wait(30);
  touring.window.toggleTerminalPanel();
  assert.equal(tour(), null);
  // A shell that ends before its card asks where it is says why that is not known.
  touring.window.toggleTerminalPanel();
  await wait(30);
  touring.server.endedSinceListed = ['1'];
  overviewButton.click();
  await wait(30);
  assert.equal(card('1').querySelector('.terminal-overview-dir').textContent, 'started in /w/p1');
  assert.match(card('1').querySelector('.terminal-overview-dir').title, /not known: no such shell/);
  touring.server.endedSinceListed = [];
  // Each card renames its tab and ends its shell, the overview staying open with the keys
  // on that card, or the one after it.
  assert.deepEqual(cards().map((item) => item.dataset.shell), ['3', '5', '1', '2', '4']);
  const cardAction = (id, label) => card(id).parentElement.querySelector('[aria-label="' + label + '"]');
  const prompts = [];
  touring.window.prompt = (text, value) => { prompts.push([text, value]); return 'logs'; };
  cardAction('2', 'Rename shell 2').click();
  await wait(30);
  assert.deepEqual(prompts, [['Name for shell 2 (empty for its number):', '']]);
  assert.ok(touring.requests.includes('PATCH /_terminal/shells/2'), touring.requests.join(', '));
  assert.notEqual(tour(), null, 'renaming closed the overview');
  assert.equal(card('2').querySelector('.terminal-overview-name').textContent, 'logs');
  assert.equal(touring.document.activeElement, card('2'));
  // F2 renames the card with the keys; cancelling sends nothing.
  touring.window.prompt = (text, value) => { prompts.push([text, value]); return null; };
  key('F2');
  await wait(30);
  assert.deepEqual(prompts.at(-1), ['Name for shell 2 (empty for its number):', 'logs']);
  assert.equal(touring.requests.filter((request) => request === 'PATCH /_terminal/shells/2').length, 1);
  // Ending asks first, naming another project's shell by its project.
  const endAsked = [];
  touring.window.confirm = (text) => { endAsked.push(text); return true; };
  cardAction('2', 'End shell 2, logs').click();
  await wait(30);
  assert.deepEqual(endAsked, ['End shell 2 of p1? What runs in it is not known: no /proc.']);
  assert.ok(touring.requests.includes('DELETE /_terminal/shells/2'));
  assert.equal(card('2'), undefined);
  assert.notEqual(tour(), null, 'ending closed the overview');
  assert.equal(touring.document.activeElement, card('4'));
  // Delete ends the card with the keys, after asking; declining ends nothing.
  touring.window.confirm = (text) => { endAsked.push(text); return false; };
  key('Delete');
  await wait(30);
  assert.equal(endAsked.at(-1), 'End shell 1 of p1? 1 process runs in it: sleep 600.');
  assert.ok(!touring.requests.includes('DELETE /_terminal/shells/4'), 'a declined question ended the shell');
  assert.notEqual(card('4'), undefined);
  // A refusal is told in the overview, not in a pane under it.
  touring.server.refuse = 'not today';
  touring.window.confirm = () => true;
  cardAction('4', 'End shell 1, build').click();
  await wait(30);
  const tourProblem = tour().querySelector('.terminal-overview-problem');
  assert.equal(tourProblem.hidden, false);
  assert.equal(tourProblem.textContent, 'The shell did not end: not today');
  touring.window.prompt = () => 'x';
  cardAction('4', 'Rename shell 1, build').click();
  await wait(30);
  assert.equal(tourProblem.textContent, 'The tab was not renamed: not today');
  touring.server.refuse = '';
  assert.doesNotMatch(touring.document.querySelector('.terminal-pane-status').textContent, /not today/);
  // A project's + starts another shell where its page would, shown in no pane, with the
  // keys on its new card; at the limit it is off and says why, and a refusal is told.
  const growing = load(process.argv[1], { stored: '1', path: '/project/p2', limit: 2, shells: [inProject('1', 1, 'p1'), inProject('3', 1, 'p2'), { ...inProject('4', 1, 'q', '/c'), link: '/_basedir/c/project/q' }] });
  await wait(50);
  allShellsButton(growing).click();
  await wait(30);
  const growTour = () => growing.document.querySelector('.terminal-overview');
  const addFor = (name) => growTour().querySelector('[aria-label="New shell in ' + name + '"]');
  assert.deepEqual([...growTour().querySelectorAll('h3 span')].map((item) => item.textContent), ['p2', 'p1', 'q']);
  const growSockets = growing.sockets.length;
  addFor('p1').click();
  await wait(30);
  assert.ok(growing.requests.includes('POST /_terminal/shells?rows=24&cols=80&project=p1'), growing.requests.join(', '));
  assert.equal(growing.sockets.length, growSockets, 'the new shell was shown in a pane');
  assert.notEqual(growTour(), null, 'starting closed the overview');
  assert.equal(growing.document.activeElement.dataset.shell, '101');
  assert.equal(addFor('p1').disabled, true);
  assert.match(addFor('p1').title, /at most 2 shells can run for one project/);
  addFor('q').click();
  await wait(30);
  assert.ok(growing.requests.includes('POST /_terminal/shells?rows=24&cols=80&basedir=c&project=q'), growing.requests.join(', '));
  // A project or basedir with no shell yet starts one from the list under the cards, where
  // its page would; no project starts in the home directory, and a basedir whose projects
  // cannot be listed says so, still offering its own shells.
  growing.server.basedirs = [{ id: 'b', path: '/b', current: true }, { id: 'c', path: '/c', current: false }, { id: 'd', path: '/d', current: false }];
  growing.server.projects = { b: ['p1', 'p2', 'fresh'], c: ['q', 'other one'] };
  [...growTour().querySelectorAll('button')].find((item) => item.textContent === 'Refresh').click();
  await wait(50);
  const newRow = () => growTour().querySelector('.terminal-overview-new');
  const newChoice = () => newRow().querySelector('select');
  assert.deepEqual([...newChoice().children].map((item) => item.tagName === 'OPTGROUP' ? [item.label, [...item.children].map((option) => [option.textContent, option.value])] : [item.textContent, item.value]), [
    ['No project (home directory)', '/activity'],
    ['/b', [['All projects (the basedir)', '/'], ['fresh', '/project/fresh']]],
    ['/c', [['All projects (the basedir)', '/_basedir/c/'], ['other one', '/_basedir/c/project/other%20one']]],
    ['/d', [['All projects (the basedir)', '/_basedir/d/']]],
  ]);
  assert.equal(newRow().querySelector('.terminal-overview-new-note').textContent, 'Not listed: /d: gone');
  newChoice().value = '/_basedir/c/project/other%20one';
  [...newRow().querySelectorAll('button')].find((item) => item.textContent === 'Start').click();
  await wait(50);
  assert.ok(growing.requests.includes('POST /_terminal/shells?rows=24&cols=80&basedir=c&project=other%20one'), growing.requests.join(', '));
  assert.equal(growing.document.activeElement.dataset.shell, String(growing.server.lastID));
  newChoice().value = '/';
  [...newRow().querySelectorAll('button')].find((item) => item.textContent === 'Start').click();
  await wait(50);
  assert.ok(growing.requests.includes('POST /_terminal/shells?rows=24&cols=80&basedir='), growing.requests.join(', '));
  assert.equal([...growTour().querySelectorAll('h3 span')].at(-1).textContent, '/b', 'the basedir\'s shells are not named by it');
  assert.deepEqual([...newChoice().children].map((item) => item.label || item.textContent)[1], '/b');
  assert.deepEqual([...newChoice().children][1] && [...[...newChoice().children][1].children].map((option) => option.textContent), ['fresh'], 'the basedir with a shell was still offered');
  assert.equal(growing.sockets.length, growSockets, 'the new shell was shown in a pane');
  growing.server.basedirsStatus = 500;
  [...growTour().querySelectorAll('button')].find((item) => item.textContent === 'Refresh').click();
  await wait(50);
  assert.equal(newRow().querySelector('.terminal-overview-new-note').textContent, 'The projects could not be listed: refused');
  assert.equal(newRow().querySelector('button').disabled, true);
  growing.server.startError = 'disk full';
  addFor('p2').click();
  await wait(30);
  assert.equal(growTour().querySelector('.terminal-overview-problem').textContent, 'The shell did not start: disk full');

  // The overview ends a project's shells, or every shell, after asking, and stays open.
  const endingAll = load(process.argv[1], { stored: '1', path: '/project/p2', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1'), inProject('3', 1, 'p2')] });
  await wait(50);
  allShellsButton(endingAll).click();
  await wait(30);
  const endTour = () => endingAll.document.querySelector('.terminal-overview');
  const endAllAsked = [];
  endingAll.window.confirm = (text) => { endAllAsked.push(text); return true; };
  endTour().querySelector('[aria-label="End all shells of p1"]').click();
  await wait(50);
  assert.deepEqual(endAllAsked, ['End the 2 shells of p1? Nothing runs in them.']);
  assert.deepEqual(endingAll.requests.filter((request) => request.startsWith('DELETE')), ['DELETE /_terminal/shells/1', 'DELETE /_terminal/shells/2']);
  assert.notEqual(endTour(), null, 'ending closed the overview');
  assert.deepEqual([...endTour().querySelectorAll('h3 span')].map((item) => item.textContent), ['p2']);
  // A refusal is told in the overview.
  endingAll.server.refuse = 'busy';
  const endEveryButton = () => [...endTour().querySelectorAll('header button')].find((item) => item.textContent === 'End all shells');
  endEveryButton().click();
  await wait(50);
  assert.equal(endTour().querySelector('.terminal-overview-problem').textContent, 'The shell did not end: busy');
  endingAll.server.refuse = '';
  endEveryButton().click();
  await wait(50);
  assert.equal(endAllAsked.at(-1), 'End the one shell, of every project? Nothing runs in it.');
  assert.deepEqual(endingAll.server.shells, []);
  endingAll.sockets[0].onclose({ code: 4002, reason: 'hangup' });
  await wait(30);
  allShellsButton(endingAll).click();
  await wait(30);
  allShellsButton(endingAll).click();
  await wait(30);
  assert.equal(endTour().querySelector('.terminal-overview-body').textContent, 'No shells are running.');
  assert.equal(endEveryButton().disabled, true);

  // Launch here starts a registered command on the directory the pane's shell is in now:
  // the commands that accept it, what one would run, the launch, and a service's Open link.
  const launching = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1')], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  launching.server.cwds = { 1: '/w/p1/logs', 2: '/w/p1/other' };
  await wait(50);
  const launchBar = [...launching.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'Launch here');
  assert.equal(launchBar.disabled, false);
  // Without launching, it says why and how to have it.
  launchBar.click();
  await wait(30);
  const launchBox = () => launching.document.querySelector('.terminal-launch');
  assert.equal(launchBox().parentElement.parentElement, launching.document.body);
  assert.equal(launchBox().querySelector('#terminal-launch-title').textContent, 'Launch in /w/p1/logs');
  assert.deepEqual([...launchBox().querySelectorAll('.terminal-launch-problem, .terminal-launch-hint')].map((item) => item.textContent), ['Launching is disabled on this server.', 'Restart with rotari web --allow-exec.']);
  launching.document.activeElement.dispatchEvent(new launching.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.equal(launchBox(), null, 'Escape left the launch dialog');
  // The Web UI's own viewers need no launching: the shell's directory is pinned under a
  // source, and the viewer opens it in a new tab, with why commands are not offered.
  const tableViewer = { name: 'Table viewer', path: '/tableview/', targets: ['directory'] };
  launching.server.launchers = { available: false, reason: 'Launching is disabled on this server.', hint: 'Restart with rotari web --allow-exec.', viewers: [tableViewer] };
  launchBar.click();
  await wait(30);
  assert.ok(launching.requests.includes('POST /api/tableview-sources {"shell":"1"}'), launching.requests.join(', '));
  const viewerLink = () => launchBox().querySelector('.terminal-launch-viewer');
  assert.deepEqual([viewerLink().textContent, viewerLink().getAttribute('href'), viewerLink().target, viewerLink().rel, viewerLink().title], ['Table viewer', '/tableview/?source=S%201', '_blank', 'noopener', 'Open /w/p1/logs in a new tab']);
  assert.equal(launching.document.activeElement, viewerLink());
  assert.deepEqual([...launchBox().querySelectorAll('.terminal-launch-intro, .terminal-launch-problem, .terminal-launch-hint')].map((item) => [item.className, item.textContent]), [['terminal-launch-intro', 'View /w/p1/logs in this server:'], ['terminal-launch-hint', 'Launching is disabled on this server.'], ['terminal-launch-hint', 'Restart with rotari web --allow-exec.']]);
  viewerLink().click();
  assert.equal(launchBox(), null, 'opening the viewer left the launch dialog');
  // A refused source says why, and leaves the commands.
  launching.server.tableviewSource = { error: 'shell 1 is not running' };
  launching.server.launchers = { available: true, user: 'u', host: 'h', commands: [{ name: 'TensorBoard' }], viewers: [tableViewer] };
  launchBar.click();
  await wait(30);
  assert.equal(viewerLink(), null);
  assert.equal(launchBox().querySelector('.terminal-launch-problem').textContent, 'Cannot open a viewer here: shell 1 is not running');
  assert.deepEqual([...launchBox().querySelectorAll('.terminal-launch-commands button')].map((item) => item.textContent), ['TensorBoard']);
  launchBox().querySelector('header button').click();
  delete launching.server.tableviewSource;
  launching.server.launchers = { available: true, user: 'u', host: 'h', commands: [{ name: 'TensorBoard' }, { name: 'Jupyter Lab', unavailable: 'jupyter is not on PATH' }] };
  launching.server.launchPreview = { name: 'TensorBoard', command: 'tensorboard --logdir "$ROTARI_LAUNCHER_ARTIFACT"', shell: '/bin/sh', mode: 'service', target: '/w/p1/logs', kind: 'directory', cwd: '/w/p1/logs', host: 'h', user: 'u', labels: { project: 'p1', shell: '1' } };
  launching.server.launchAnswer = { id: 'L1', mode: 'service', state: 'starting' };
  launching.server.launchStates = [{ id: 'L1', mode: 'service', state: 'starting' }, { id: 'L1', mode: 'service', state: 'running', base_path: '/_launch/L1' }];
  launchBar.click();
  await wait(30);
  assert.ok(launching.requests.includes('GET /api/launchers?kind=directory&name=logs'), launching.requests.join(', '));
  const commandButtons = [...launchBox().querySelectorAll('.terminal-launch-commands button')];
  assert.deepEqual(commandButtons.map((item) => [item.textContent, item.disabled, item.title]), [['TensorBoard', false, ''], ['Jupyter Lab', true, 'Not installed: jupyter is not on PATH']]);
  assert.equal(launchBox().querySelector('.terminal-launch-intro').textContent, 'Run a registered command on this directory, as you, on u@h:');
  commandButtons[0].click();
  await wait(30);
  assert.ok(launching.requests.includes('GET /api/launch-preview?shell=1&name=TensorBoard'));
  assert.deepEqual([...launchBox().querySelectorAll('.terminal-launch-details dd')].map((item) => item.textContent), ['TensorBoard', '/w/p1/logs (directory)', '/w/p1/logs', 'service', 'u@h', 'project p1, shell 1', '/bin/sh -c tensorboard --logdir "$ROTARI_LAUNCHER_ARTIFACT"']);
  launchBox().querySelector('.terminal-launch-confirm').click();
  await wait(700);
  assert.ok(launching.requests.includes('POST /api/launch {"shell":"1","name":"TensorBoard","new_instance":false}'), launching.requests.join(', '));
  assert.match(launchBox().textContent, /TensorBoard is ready\./);
  assert.equal(launchBox().querySelector('.terminal-launch-open').getAttribute('href'), '/_launch/L1/');
  assert.equal(launchBox().querySelector('.terminal-launch-open').target, '_blank');
  // A refused launch says why.
  launching.server.launchAnswer = { error: 'launcher "TensorBoard" does not accept a file', hint: '' };
  launchBox().querySelector('header button').click();
  launchBar.click();
  await wait(30);
  launchBox().querySelector('.terminal-launch-commands button').click();
  await wait(30);
  launchBox().querySelector('.terminal-launch-confirm').click();
  await wait(30);
  assert.equal(launchBox().querySelector('.terminal-launch-problem').textContent, 'launcher "TensorBoard" does not accept a file');
  launchBox().querySelector('header button').click();
  // An All shells card launches in its own shell's directory, closing the overview.
  allShellsButton(launching).click();
  await wait(30);
  launching.document.querySelector('.terminal-overview-card[data-shell="2"]').parentElement.querySelector('[aria-label="Launch in shell 2"]').click();
  await wait(30);
  assert.equal(launching.document.querySelector('.terminal-overview'), null);
  assert.equal(launchBox().querySelector('#terminal-launch-title').textContent, 'Launch in /w/p1/other');
  // With no shell shown, it is off.
  const launchless = load(process.argv[1], { stored: '1', path: '/project/p9' });
  await wait(50);
  assert.equal([...launchless.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'Launch here').disabled, true);

  // In a window, another project's shell takes the window and the page that opened it along.
  const toldTour = [];
  const tourWin = load(process.argv[6], { path: '/_terminal/window/project/p2', shells: [inProject('1', 1, 'p1'), inProject('3', 1, 'p2')], opener: { closed: false, postMessage: (message, target) => toldTour.push(JSON.parse(JSON.stringify([message, target]))) } });
  await wait(50);
  [...tourWin.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'All shells').click();
  await wait(30);
  [...tourWin.document.querySelectorAll('.terminal-overview-card')].find((item) => item.dataset.shell === '1').click();
  assert.deepEqual(toldTour.filter(([message]) => message.type === 'rotari-terminal-go'), [[{ type: 'rotari-terminal-go', path: '/project/p1' }, origin]]);
  assert.equal(tourWin.window.sessionStorage.getItem('rotari-terminal-shell:/b:p1'), '1');
  // A project's line opens its last run's page, taking the page that opened the window
  // along; shells without a project have no line.
  const toldState = [];
  const stateWin = load(process.argv[6], { path: '/_terminal/window/project/p2', shells: [inProject('1', 1, 'p1'), plain('9', 1)], opener: { closed: false, postMessage: (message, target) => toldState.push(JSON.parse(JSON.stringify([message, target]))) } });
  stateWin.server.projectStates = { now: '2026-11-12T10:05:00Z', projects: [{ basedir: '/b', project: 'p2', runs: 0 }, { basedir: '/b', project: 'p1', runs: 1, last_run: { run_id: 'r 1', lifecycle: 'failed', tone: 'bad', started_at: '2026-11-10T09:00:00Z', finished_at: '2026-11-10T09:05:00Z', jobs: { bad: 2, ok: 4 } } }] };
  await wait(50);
  [...stateWin.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'All shells').click();
  await wait(30);
  const stateLines = [...stateWin.document.querySelectorAll('.terminal-overview-project-state')];
  assert.deepEqual(stateLines.map((line) => [line.parentElement.getAttribute('aria-label'), line.hidden]), [['p2', false], ['p1', false], ['No project', true]]);
  assert.match(stateLines[1].textContent, /^Last run failed 2d 1h ago \u00b7 0 in progress \u00b7 0 pending \u00b7 2 failed$/);
  stateLines[1].querySelector('button').click();
  assert.equal(stateWin.document.querySelector('.terminal-overview'), null, 'opening the run left the overview open');
  assert.deepEqual(toldState.filter(([message]) => message.type === 'rotari-terminal-go'), [[{ type: 'rotari-terminal-go', path: '/project/p1/run/r%201' }, origin]]);
  // With no list, it says why.
  let tourStatus = 200;
  const tourless = load(process.argv[1], { stored: '1', path: '/project/p2', shells: [inProject('3', 1, 'p2')], status: () => tourStatus });
  await wait(50);
  tourStatus = 503;
  [...tourless.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'All shells').click();
  await wait(30);
  assert.equal(tourless.document.querySelector('.terminal-overview-body').textContent, 'The shells could not be listed: refused');
  assert.match(webShellCSS, /html\[data-terminal-window\] \.terminal-overview-modal,\s*html\[data-terminal-window\] \.terminal-launch-modal \{\s*inset: 0;/);
  // The dialogs bring their own frame, since the Recent jobs page and the panel's window
  // have no web_styles.css, where .output-modal's is.
  assert.match(webShellCSS, /\.terminal-overview-modal,\s*\.terminal-launch-modal \{\s*position: fixed;\s*inset: 0;/);
  // Every picture button says in words what it does, and the pointer reaches the button, not its picture.
  const pictured = [...touring.document.querySelectorAll('button.terminal-icon')];
  assert.ok(pictured.length >= 8, String(pictured.length));
  for (const button of pictured) assert.ok(button.title && button.getAttribute('aria-label'), button.outerHTML.slice(0, 120));
  assert.match(webShellCSS, /\.terminal-icon svg \{\s*pointer-events: none;/);

  // The Settings menu's path of this rotari copies, and sends as one word, on both pages.
  for (const file of [process.argv[1], process.argv[4]]) {
    const aboutPage = load(file);
    await wait(50);
    const copiedPaths = [];
    Object.defineProperty(aboutPage.window.navigator, 'clipboard', { configurable: true, value: { writeText: async (value) => copiedPaths.push(value) } });
    const copyPath = aboutPage.document.querySelector('.settings-about .jobs-copy');
    const sendPath = copyPath.nextElementSibling;
    assert.ok(sendPath.classList.contains('terminal-send'), file + ': no send button beside the path');
    assert.equal(sendPath.dataset.sendValue, '/opt/rotari bin/rotari');
    assert.equal(sendPath.title, 'Send path of this rotari to the terminal');
    copyPath.click();
    await wait(20);
    assert.deepEqual(copiedPaths, ['/opt/rotari bin/rotari']);
    assert.ok(copyPath.classList.contains('copied'));
  }

  // xterm.js loads only on opening, under the page's nonce.
  const real = load(process.argv[1], { stubs: false });
  await wait(50);
  const scripts = () => [...real.document.querySelectorAll('script[src]')];
  assert.equal(scripts().length, 0);
  real.document.getElementById('terminal-button').click();
  await wait(20);
  assert.equal(scripts().length, 1);
  assert.equal(scripts()[0].getAttribute('src'), '/_terminal/xterm.js');

  // The Explorer: the files where the focused pane's shell is, following it as it moves.
  {
  const exploring = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1')], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  const ex = exploring;
  ex.server.cwds = { 1: '/w/p1', 2: '/w/p1/out' };
  ex.server.files = {
    '/w': [{ name: 'other', kind: 'file' }, { name: 'p1', kind: 'dir' }],
    '/w/p1': [
      { name: 'conf', kind: 'dir' },
      { name: 'shared', kind: 'symlink', target: '/data/shared', target_kind: 'dir' },
      { name: 'a b.csv', kind: 'file' },
      { name: "it's\\x.txt", kind: 'file' },
      { name: 'bad\nname', kind: 'file' },
      { name: 'odd�', kind: 'file', not_utf8: true },
    ],
    '/w/p1/conf': [{ name: 'train.yaml', kind: 'file' }],
    '/w/p1/out': { error: 'open /w/p1/out: permission denied' },
    '/w/p1/shared': { entries: [{ name: 'one', kind: 'file' }], total: 3 },
  };
  const exOpened = [];
  ex.window.open = (...args) => {
    const tab = { location: { href: args[0] }, opener: 'page', close() { this.closed = true; } };
    exOpened.push([args, tab]);
    return tab;
  };
  const exCopied = [];
  Object.defineProperty(ex.window.navigator, 'clipboard', { configurable: true, value: { writeText: async (text) => exCopied.push(text) } });
  await wait(50);
  const exToggle = ex.document.querySelector('.terminal-explorer-toggle');
  const exPanel = () => ex.document.getElementById('terminal-panel');
  const exAside = () => ex.document.querySelector('.terminal-explorer');
  const exRows = () => [...ex.document.querySelectorAll('.terminal-explorer-row')];
  const exRow = (text) => exRows().find((row) => row.querySelector('.terminal-explorer-name').textContent === text);
  const exNames = () => exRows().map((row) => row.querySelector('.terminal-explorer-name').textContent);
  const exFooter = () => ex.document.querySelector('.terminal-explorer-footer').textContent;
  const exNote = () => ex.document.querySelector('.terminal-send-note')?.textContent;
  assert.equal(ex.document.querySelector('.terminal-bar').firstElementChild, exToggle, 'the Explorer button is not first in the bar');
  // It opens with the panel unless it was closed, which the browser remembers.
  assert.equal(exToggle.getAttribute('aria-pressed'), 'true');
  assert.equal(exAside().hidden, false, 'the Explorer did not open with the panel');
  assert.equal(exAside().contains(ex.document.activeElement), false, 'opening with the panel took the keys from the shell');
  exToggle.click();
  assert.equal(ex.window.localStorage.getItem('rotari-terminal-explorer'), '0');
  assert.deepEqual([exToggle.getAttribute('aria-pressed'), exAside().hidden], ['false', true]);
  const closedBefore = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1')], explorer: '0' });
  closedBefore.server.cwds = { 1: '/w/p1' };
  await wait(50);
  assert.equal(closedBefore.document.querySelector('.terminal-explorer').hidden, true, 'a closed Explorer opened again');
  closedBefore.server.fileRequests = [];
  closedBefore.polls.forEach((poll) => poll());
  await wait(20);
  assert.deepEqual(closedBefore.server.fileRequests, [], 'a closed Explorer listed files');
  exToggle.click();
  await wait(30);
  assert.equal(ex.window.localStorage.getItem('rotari-terminal-explorer'), '1');
  assert.equal(exToggle.getAttribute('aria-pressed'), 'true');
  assert.equal(exAside().hidden, false);
  assert.ok(ex.server.fileRequests.includes('GET /_terminal/files?shell=1'), ex.server.fileRequests.join(', '));
  assert.equal(ex.document.querySelector('.terminal-explorer-title').textContent, 'p1');
  assert.equal(ex.document.querySelector('.terminal-explorer-where').textContent, '/w/p1');
  assert.equal(exFooter(), 'Following Shell 1 of p1');
  assert.deepEqual(exNames(), ['conf', 'shared', 'a b.csv', "it's\\x.txt", 'bad\\nname', 'odd�']);
  // A link says where it goes, a control character shows as its escape, and a name that is
  // not UTF-8 offers nothing.
  assert.equal(exRow('shared').querySelector('.terminal-explorer-target').textContent, '→ /data/shared');
  assert.equal(exRow('shared').getAttribute('aria-expanded'), 'false');
  assert.equal(exRow('bad\\nname').querySelector('.terminal-explorer-escape').textContent, '\\n');
  assert.equal(exRow('odd�').querySelector('.terminal-explorer-actions'), null);
  assert.equal(exRow('odd�').draggable, false);
  assert.match(exRow('odd�').title, /not valid UTF-8/);
  // A folder opens in place, a level down.
  exRow('conf').click();
  await wait(20);
  assert.ok(ex.server.fileRequests.includes('GET /_terminal/files?path=%2Fw%2Fp1%2Fconf'), ex.server.fileRequests.join(', '));
  assert.deepEqual([exRow('train.yaml').getAttribute('aria-level'), exRow('conf').getAttribute('aria-expanded'), exRow('conf').getAttribute('aria-selected')], ['2', 'true', 'true']);
  // A folder with more than the server lists says so, and offers the Table viewer.
  exRow('shared').click();
  await wait(20);
  assert.equal(ex.document.querySelector('.terminal-explorer-truncated').textContent, '1 of 3 shown. Open in the Table viewer');
  exRow('shared').click();
  // A path goes in relative to the shell, as one word, quoted as every shell reads it.
  const exSocket = ex.sockets.at(-1);
  exSocket.onopen?.();
  const sendOf = (row) => row.querySelector('[aria-label="Send the path to the terminal"]');
  sendOf(exRow('a b.csv')).click();
  await wait(20);
  assert.equal(ex.pasted.at(-1), "'a b.csv'");
  sendOf(exRow('train.yaml')).click();
  await wait(20);
  assert.equal(ex.pasted.at(-1), 'conf/train.yaml');
  sendOf(exRow("it's\\x.txt")).click();
  await wait(20);
  assert.equal(ex.pasted.at(-1), "'it'\\''s'\\\\'x.txt'");
  const pastedBefore = ex.pasted.length;
  sendOf(exRow('bad\\nname')).click();
  await wait(20);
  assert.equal(ex.pasted.length, pastedBefore, 'a name with a line break was sent');
  assert.match(exNote(), /control characters/);
  exRow('odd�').dispatchEvent(new ex.window.MouseEvent('contextmenu', { bubbles: true, cancelable: true }));
  assert.match(exNote(), /not valid UTF-8/);
  assert.equal(ex.document.querySelector('.terminal-tab-menu'), null, 'a name that cannot be named got a menu');
  // The menu copies the path whole or from the shell's directory.
  const menuOf = (row) => {
    row.dispatchEvent(new ex.window.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
    return [...ex.document.querySelectorAll('.terminal-tab-menu button')];
  };
  assert.deepEqual(menuOf(exRow('train.yaml')).map((item) => [item.textContent, item.disabled]), [['Open in the Table viewer', false], ['Open in a new terminal tab', false], ['Send the path to the terminal', false], ['Download', false], ['Upload...', false], ['New File...', false], ['New Folder...', false], ['Cut', false], ['Copy', false], ['Paste', true], ['Rename...', false], ['Delete...', false], ['Copy the path', false], ['Copy the path from here', false]]);
  menuOf(exRow('train.yaml'))[13].click();
  await wait(10);
  menuOf(exRow('train.yaml'))[12].click();
  await wait(10);
  assert.deepEqual(exCopied, ['conf/train.yaml', '/w/p1/conf/train.yaml']);
  // A new tab opens in a folder, or in a file's folder, as a shell of the page's project.
  const startsBefore = ex.server.shells.length;
  menuOf(exRow('train.yaml'))[1].click();
  await wait(30);
  assert.ok(ex.requests.includes('POST /_terminal/shells?rows=24&cols=80&project=p1&dir=%2Fw%2Fp1%2Fconf'), ex.requests.join(', '));
  assert.equal(ex.server.shells.length, startsBefore + 1);
  const exStarted = ex.server.shells.at(-1);
  assert.equal(ex.document.querySelector('.terminal-tab [aria-selected="true"]').textContent, String(exStarted.number));
  [...ex.document.querySelectorAll('.terminal-tab [role="tab"]')][0].click();
  await wait(30);
  // Changes to files: each a JSON request, with the names asked and the deletions and
  // moves by dragging confirmed first.
  const lastChange = () => ex.server.fileChanges?.at(-1);
  const changesSoFar = () => ex.server.fileChanges?.length || 0;
  const prompts = [];
  let promptAnswer = '';
  ex.window.prompt = (question, value) => { prompts.push([question, value]); return promptAnswer; };
  const asked = [];
  let confirmAnswer = true;
  ex.window.confirm = (question) => { asked.push(question); return confirmAnswer; };
  assert.equal(menuOf(exRow('conf')).find((item) => item.textContent === 'Copy').disabled, true, 'a folder offered Copy');
  ex.document.body.click();
  ex.document.querySelector('.terminal-tab-menu')?.remove();
  exRow('conf').focus();
  if (exRow('conf').getAttribute('aria-selected') !== 'true') exRow('conf').click();
  if (exRow('conf').getAttribute('aria-expanded') !== 'true') exRow('conf').click();
  await wait(20);
  promptAnswer = 'out';
  ex.document.querySelector('[aria-label="New Folder"]').click();
  await wait(20);
  assert.deepEqual(prompts.at(-1), ['Name of the new folder in /w/p1/conf:', '']);
  assert.deepEqual(lastChange(), ['mkdir', { path: '/w/p1/conf/out' }, 'application/json']);
  const keyOn = (name, options = {}) => ex.document.querySelector('.terminal-explorer-tree').dispatchEvent(new ex.window.KeyboardEvent('keydown', { key: name, bubbles: true, ...options }));
  exRow('train.yaml').click();
  promptAnswer = 'train2.yaml';
  keyOn('F2');
  await wait(20);
  assert.deepEqual(prompts.at(-1), ['New name for /w/p1/conf/train.yaml:', 'train.yaml']);
  assert.deepEqual(lastChange().slice(0, 2), ['move', { from: '/w/p1/conf/train.yaml', to: '/w/p1/conf/train2.yaml' }]);
  let before = changesSoFar();
  promptAnswer = 'train.yaml';
  exRow('train.yaml').click();
  keyOn('F2');
  await wait(20);
  assert.equal(changesSoFar(), before, 'renaming to the same name asked the server');
  // A folder's deletion says how much it holds directly, and nothing happens unless confirmed.
  exRow('conf').click();
  await wait(20);
  if (exRow('conf').getAttribute('aria-expanded') !== 'true') { exRow('conf').click(); await wait(20); }
  confirmAnswer = false;
  keyOn('Delete');
  await wait(20);
  assert.equal(asked.at(-1), 'Delete the folder /w/p1/conf, the 1 entry in it, and everything below? This cannot be undone.');
  assert.equal(changesSoFar(), before, 'a deletion not confirmed went ahead');
  confirmAnswer = true;
  keyOn('Delete');
  await wait(20);
  assert.deepEqual(lastChange().slice(0, 2), ['delete', { path: '/w/p1/conf' }]);
  // Copy is for files: pasted beside its file it gets a name of its own.
  exRow('a b.csv').click();
  keyOn('c', { ctrlKey: true });
  keyOn('v', { ctrlKey: true });
  await wait(20);
  assert.deepEqual(lastChange().slice(0, 2), ['copy', { from: '/w/p1/a b.csv', to: '/w/p1/a b.csv', unique: true }]);
  exRow('conf').click();
  await wait(20);
  before = changesSoFar();
  keyOn('c', { ctrlKey: true });
  assert.match(exNote(), /Folders are not copied here; use cp -r/);
  // Cut and Paste move it, and the clipboard empties.
  menuOf(exRow('a b.csv')).find((item) => item.textContent === 'Cut').click();
  assert.equal(exRow('a b.csv').classList.contains('terminal-explorer-cut'), true);
  menuOf(exRow('conf')).find((item) => item.textContent === 'Paste').click();
  await wait(20);
  assert.deepEqual(lastChange().slice(0, 2), ['move', { from: '/w/p1/a b.csv', to: '/w/p1/conf/a b.csv' }]);
  assert.equal(menuOf(exRow('conf')).find((item) => item.textContent === 'Paste').disabled, true, 'a cut was pasted twice');
  ex.document.querySelector('.terminal-tab-menu')?.remove();
  // A refusal says why.
  ex.server.refuseChange = 'mkdir /w/p1/x: file exists';
  promptAnswer = 'x';
  ex.document.querySelector('.terminal-explorer-tree').dispatchEvent(new ex.window.MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
  const rootMenu = [...ex.document.querySelectorAll('.terminal-tab-menu button')];
  assert.deepEqual(rootMenu.map((item) => item.textContent), ['New File...', 'New Folder...', 'Paste', 'Upload...']);
  rootMenu[1].click();
  await wait(20);
  assert.deepEqual(lastChange().slice(0, 2), ['mkdir', { path: '/w/p1/x' }]);
  assert.equal(exNote(), 'Making x failed: mkdir /w/p1/x: file exists');
  delete ex.server.refuseChange;
  // A row dragged onto a folder moves into it after asking; held over a closed folder,
  // the folder opens.
  const dragEvent = (type, target) => {
    const event = new ex.window.Event(type, { bubbles: true, cancelable: true });
    Object.defineProperty(event, 'dataTransfer', { value: { types: ['application/x-rotari-explorer-path'], setData() {}, effectAllowed: '', dropEffect: '' } });
    target.dispatchEvent(event);
    return event;
  };
  dragEvent('dragstart', exRow('a b.csv'));
  assert.equal(dragEvent('dragover', exRow('a b.csv')).defaultPrevented, false, 'a row took a drop into its own folder');
  assert.equal(dragEvent('dragover', exRow('conf')).defaultPrevented, true);
  assert.equal(exRow('conf').classList.contains('terminal-explorer-drop'), true);
  dragEvent('drop', exRow('conf'));
  await wait(20);
  assert.equal(asked.at(-1), 'Move /w/p1/a b.csv into /w/p1/conf?');
  assert.deepEqual(lastChange().slice(0, 2), ['move', { from: '/w/p1/a b.csv', to: '/w/p1/conf/a b.csv' }]);
  dragEvent('dragstart', exRow('a b.csv'));
  assert.equal(exRow('shared').getAttribute('aria-expanded'), 'false');
  dragEvent('dragover', exRow('shared'));
  await wait(750);
  assert.equal(exRow('shared').getAttribute('aria-expanded'), 'true', 'holding over a closed folder did not open it');
  dragEvent('dragend', exRow('a b.csv'));
  exRow('shared').click();
  await wait(20);

  // Download: a link to the file, or to the folder as a zip, and a row dragged out of the
  // browser carries the same address.
  const followed = [];
  ex.window.HTMLAnchorElement.prototype.click = function () { followed.push([this.getAttribute('href'), this.hasAttribute('download')]); };
  menuOf(exRow('conf')).find((item) => item.textContent === 'Download').click();
  assert.deepEqual(followed.at(-1), ['/_terminal/files/download?path=%2Fw%2Fp1%2Fconf', true]);
  const carried = {};
  const outward = new ex.window.Event('dragstart', { bubbles: true });
  Object.defineProperty(outward, 'dataTransfer', { value: { types: [], setData(type, value) { carried[type] = value; }, effectAllowed: '' } });
  exRow('conf').dispatchEvent(outward);
  assert.equal(carried.DownloadURL, 'application/zip:conf.zip:http://127.0.0.1:9/_terminal/files/download?path=%2Fw%2Fp1%2Fconf');
  exRow('conf').dispatchEvent(new ex.window.Event('dragend', { bubbles: true }));
  // Upload: chosen files go one at a time, each refused rather than replacing a file.
  ex.window.HTMLInputElement.prototype.click = function () {
    if (this.type !== 'file') return;
    Object.defineProperty(this, 'files', { value: [new ex.window.File(['a'], 'one.csv'), new ex.window.File(['b'], 'two.csv')] });
    this.onchange();
  };
  ex.server.refuseUpload = { 'two.csv': 'write /w/p1/conf/two.csv: file already exists' };
  menuOf(exRow('conf')).find((item) => item.textContent === 'Upload...').click();
  await wait(30);
  assert.deepEqual(ex.server.uploads, [['/w/p1/conf', 'one.csv', 'application/octet-stream', 'File'], ['/w/p1/conf', 'two.csv', 'application/octet-stream', 'File']]);
  assert.equal(exNote(), 'Uploaded 1 of 2. Not uploaded: two.csv: write /w/p1/conf/two.csv: file already exists');
  assert.equal(ex.document.querySelector('.terminal-input-file, input[type="file"]'), null, 'the chooser stayed in the page');
  delete ex.server.refuseUpload;
  // Files dropped from the desktop on a folder upload there; a folder dropped with them does not.
  const fromDesktop = (type, target, folders = [], names = []) => {
    const event = new ex.window.Event(type, { bubbles: true, cancelable: true });
    Object.defineProperty(event, 'dataTransfer', { value: {
      types: ['Files'], dropEffect: '',
      items: [...folders.map((name) => ({ webkitGetAsEntry: () => ({ isDirectory: true, name }) })), ...names.map((name) => ({ webkitGetAsEntry: () => ({ isDirectory: false, name }) }))],
      files: [...folders, ...names].map((name) => new ex.window.File([''], name)),
    } });
    target.dispatchEvent(event);
    return event;
  };
  assert.equal(fromDesktop('dragover', exRow('conf')).defaultPrevented, true, 'a folder refused files from the desktop');
  const notes = [];
  const noteBox = ex.document.querySelector('.terminal-send-note');
  new ex.window.MutationObserver(() => notes.push(noteBox.textContent)).observe(noteBox, { childList: true, characterData: true, subtree: true });
  fromDesktop('drop', exRow('conf'), ['dataset'], ['three.txt']);
  await wait(30);
  assert.deepEqual(ex.server.uploads.at(-1), ['/w/p1/conf', 'three.txt', 'application/octet-stream', 'File']);
  assert.equal(ex.server.uploads.some((upload) => upload[1] === 'dataset'), false, 'a folder was uploaded');
  assert.ok(notes.includes('Folders are not uploaded: dataset. The files are.'), notes.join(' | '));
  assert.equal(exNote(), 'Uploaded three.txt to /w/p1/conf.');
  // A large upload says how far it has come, in the footer and its bar.
  ex.server.holdUploads = true;
  fromDesktop('drop', exRow('conf'), [], ['big.bin']);
  await wait(10);
  const held = ex.server.xhrs.at(-1);
  assert.deepEqual([held.method, held.url, held.headers['Content-Type']], ['POST', '/_terminal/files/upload?dir=%2Fw%2Fp1%2Fconf&name=big.bin', 'application/octet-stream']);
  assert.equal(exFooter(), 'Uploading big.bin...');
  held.upload.onprogress({ lengthComputable: true, loaded: 3 * 1048576, total: 12 * 1048576 });
  assert.equal(exFooter(), 'Uploading big.bin: 25% (3.0 MiB of 12.0 MiB)');
  assert.equal(ex.document.querySelector('.terminal-explorer-progress').value, 0.25);
  ex.server.holdUploads = false;
  held.finish();
  await wait(20);
  assert.equal(ex.document.querySelector('.terminal-explorer-progress'), null, 'the bar stayed after the upload');
  assert.equal(exNote(), 'Uploaded big.bin to /w/p1/conf.');
  // Dropped on a pane instead, nothing opens in place of the page.
  const besideTree = fromDesktop('drop', ex.document.querySelector('.terminal-pane'), [], ['four.txt']);
  assert.equal(besideTree.defaultPrevented, true, 'a file dropped on a pane was left to the browser');
  assert.equal(exNote(), 'Drop files on a folder in the Explorer to upload them.');
  // A file opens in the Table viewer in a tab opened at once, so that no pop-up is blocked.
  ex.server.tableviewSource = { source: 'E 1', dir: '/w/p1', child: 'a b.csv' };
  exRow('a b.csv').dispatchEvent(new ex.window.MouseEvent('dblclick', { bubbles: true }));
  await wait(20);
  assert.ok(ex.requests.includes('POST /api/tableview-sources {"path":"/w/p1/a b.csv"}'), ex.requests.join(', '));
  assert.deepEqual([exOpened.at(-1)[0][0], exOpened.at(-1)[0][1], exOpened.at(-1)[1].opener, exOpened.at(-1)[1].location.href], ['about:blank', '_blank', null, 'http://127.0.0.1:9/tableview/?source=E%201&child=a%20b.csv']);
  ex.server.tableviewSource = { error: 'a path from the Explorer requires HTTPS' };
  exRow('a b.csv').dispatchEvent(new ex.window.MouseEvent('dblclick', { bubbles: true }));
  await wait(20);
  assert.equal(exOpened.at(-1)[1].closed, true, 'a refused viewer left its tab');
  assert.match(exNote(), /Cannot open \/w\/p1\/a b\.csv in the Table viewer: a path from the Explorer requires HTTPS/);
  // The keys move through the rows and open and close folders.
  exRow('conf').focus();
  exRow('conf').click();
  await wait(10);
  const key = (name) => ex.document.querySelector('.terminal-explorer-tree').dispatchEvent(new ex.window.KeyboardEvent('keydown', { key: name, bubbles: true }));
  key('ArrowRight');
  await wait(10);
  key('ArrowRight');
  assert.equal(ex.document.activeElement, exRow('train.yaml'));
  key('ArrowLeft');
  assert.equal(ex.document.activeElement, exRow('conf'));
  key('ArrowLeft');
  assert.equal(exRow('conf').getAttribute('aria-expanded'), 'false');
  key('ArrowDown');
  assert.equal(ex.document.activeElement, exRow('shared'));
  // Following: Enter in the shell lists again, and the tree moves with it.
  ex.server.cwds[1] = '/w/p1/conf';
  ex.terminals.at(-1).data('cd conf\r');
  await wait(450);
  assert.equal(ex.document.querySelector('.terminal-explorer-where').textContent, '/w/p1/conf');
  assert.deepEqual(exNames(), ['train.yaml']);
  // Up shows the parent and stops following; the shell's place stays in the footer.
  ex.document.querySelector('[aria-label="Up one directory"]').click();
  await wait(20);
  assert.equal(ex.document.querySelector('.terminal-explorer-where').textContent, '/w/p1');
  assert.equal(ex.document.querySelector('[aria-label="Follow the shell"]').getAttribute('aria-pressed'), 'false');
  assert.deepEqual([exRow('conf').getAttribute('aria-expanded'), exRow('conf').getAttribute('aria-selected')], ['true', 'true']);
  assert.equal(exFooter(), 'Not following · Shell 1 of p1 is in /w/p1/confFollow');
  ex.server.cwds[1] = '/w/p1/out';
  ex.terminals.at(-1).data('cd ../out\r');
  await wait(450);
  assert.equal(ex.document.querySelector('.terminal-explorer-where').textContent, '/w/p1', 'the tree moved while not following');
  // A path outside the shell's directory goes in whole.
  ex.server.cwds[1] = '/w/p1/conf';
  ex.document.querySelector('[aria-label="Up one directory"]').click();
  await wait(20);
  sendOf(exRow('other')).click();
  await wait(20);
  assert.equal(ex.pasted.at(-1), '/w/other');
  ex.document.querySelector('.terminal-explorer-follow-link').click();
  await wait(20);
  assert.equal(ex.document.querySelector('.terminal-explorer-where').textContent, '/w/p1/conf');
  assert.equal(ex.document.querySelector('[aria-label="Follow the shell"]').getAttribute('aria-pressed'), 'true');
  // Another shell in the pane is followed in its turn, and a directory that cannot be
  // listed says why rather than showing nothing.
  [...ex.document.querySelectorAll('.terminal-tab [role="tab"]')][1].click();
  await wait(30);
  assert.ok(ex.server.fileRequests.includes('GET /_terminal/files?shell=2'), ex.server.fileRequests.join(', '));
  assert.equal(ex.document.querySelector('.terminal-explorer-problem').textContent, 'open /w/p1/out: permission denied');
  assert.equal(exFooter(), 'Following Shell 2 of p1');
  // A path dropped on a pane goes into that pane.
  [...ex.document.querySelectorAll('.terminal-tab [role="tab"]')][0].click();
  await wait(30);
  const dropping = new ex.window.Event('drop', { bubbles: true, cancelable: true });
  Object.defineProperty(dropping, 'dataTransfer', { value: { types: ['application/x-rotari-explorer-path'], getData: () => '/w/p1/conf/train.yaml' } });
  ex.document.querySelector('.terminal-pane').dispatchEvent(dropping);
  await wait(20);
  assert.equal(ex.pasted.at(-1), 'train.yaml');
  // It stays a column however narrow the panel, as at a side, leaving the panes 200
  // pixels, and its edge drags to any width they leave room for.
  const narrow = ex.window.HTMLElement.prototype.getBoundingClientRect;
  ex.window.HTMLElement.prototype.getBoundingClientRect = function () {
    return this.classList.contains('terminal-body') ? { width: 420, height: 300, left: 10, top: 0, right: 430, bottom: 300 } : narrow.call(this);
  };
  ex.server.observers.forEach((callback) => callback([]));
  assert.equal(exAside().hidden, false);
  assert.equal(exPanel().style.getPropertyValue('--terminal-explorer-width'), '220px');
  const exHandle = ex.document.querySelector('.terminal-explorer-resizer');
  // While it is dragged, the shell hears only of the last size, when the drag ends.
  const resizesSent = () => ex.sockets.at(-1).sent.filter((data) => typeof data === 'string' && data.includes('"resize"'));
  const resizesBefore = resizesSent().length;
  exHandle.onpointerdown({ preventDefault() {}, pointerId: 1 });
  exHandle.onpointermove({ clientX: 190 });
  ex.terminals.at(-1).resize({ rows: 24, cols: 61 });
  ex.terminals.at(-1).resize({ rows: 24, cols: 58 });
  assert.equal(resizesSent().length, resizesBefore, 'a drag told the shell of every size');
  exHandle.onpointerup();
  assert.deepEqual(resizesSent().slice(resizesBefore).map((data) => JSON.parse(data)), [{ type: 'resize', rows: 24, cols: 58 }]);
  ex.terminals.at(-1).resize({ rows: 24, cols: 70 });
  assert.equal(resizesSent().length, resizesBefore + 2, 'a size after the drag waited');
  assert.equal(exPanel().style.getPropertyValue('--terminal-explorer-width'), '180px');
  assert.equal(ex.window.localStorage.getItem('rotari-terminal-explorer-width'), '180');
  exHandle.onpointerdown({ preventDefault() {}, pointerId: 1 });
  exHandle.onpointermove({ clientX: 400 });
  assert.equal(exPanel().style.getPropertyValue('--terminal-explorer-width'), '220px', 'the panes lost their room');
  exHandle.onpointerup();
  // Using a row leaves it open, and nothing covers the panes.
  sendOf(exRow('train.yaml')).click();
  await wait(20);
  assert.equal(exAside().hidden, false, 'sending closed the Explorer');
  assert.equal(ex.document.querySelector('.terminal-explorer-scrim'), null);
  ex.window.HTMLElement.prototype.getBoundingClientRect = narrow;
  }

  // Recent directories: the group's history, then where the project's runs started.
  // Choosing one by click or keys types cd and the quoted path without Enter, only while
  // the shell is at its prompt; a gone directory, a name that is not UTF-8, and a run with
  // no directory are shown but cannot be chosen.
  {
  const rd = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1')], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  const at = '2026-11-14T00:00:00Z';
  rd.server.cwds = { 1: '/w/p1' };
  rd.server.files = { '/w/p1': [], '/w/p1/a b': [{ name: 'x', kind: 'file' }] };
  rd.server.dirs = { 1: {
    history: [{ dir: '/w/p1/a b', entered_at: at }, { dir: '/w/p1', entered_at: at }, { dir: '/w/gone', missing: true, entered_at: at }, { dir: '/w/odd\uFFFD', not_utf8: true, entered_at: at }],
    runs: [{ dir: '/w/p1', run: '20261114-000700-aaaaaaaa', started_at: '2026-11-14T00:07:00Z' }, { dir: '', run: 'r-bad', error: 'run "r-bad" does not say where it started' }],
  } };
  const rdCopied = [];
  Object.defineProperty(rd.window.navigator, 'clipboard', { configurable: true, value: { writeText: async (text) => rdCopied.push(text) } });
  await wait(50);
  rd.sockets.at(-1).onopen?.();
  const rdButton = [...rd.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'Recent directories');
  const rdBox = () => rd.document.querySelector('.terminal-dirs');
  const rdRows = () => [...rd.document.querySelectorAll('.terminal-dirs-row')];
  const rdPaths = () => rdRows().map((row) => row.querySelector('.terminal-dirs-path').textContent);
  const rdNote = () => rd.document.querySelector('.terminal-send-note')?.textContent;
  const rdKey = (key) => rdBox().dispatchEvent(new rd.window.KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  const rdOpen = async (anchor = rdButton) => { anchor.click(); await wait(20); };
  assert.equal(rdButton.disabled, false);
  await rdOpen();
  assert.ok(rd.requests.includes('GET /_terminal/shells/1/dirs'), rd.requests.join(', '));
  assert.equal(rd.document.activeElement, rd.document.querySelector('.terminal-dirs-filter'), 'the filter did not take the keys');
  assert.deepEqual([...rd.document.querySelectorAll('.terminal-dirs-heading')].map((item) => item.textContent), ['Recent', 'Runs']);
  assert.deepEqual(rdPaths(), ['/w/p1/a b', '/w/p1', '/w/gone', '/w/odd\uFFFD', '/w/p1', '—']);
  assert.deepEqual(rdRows().map((row) => row.getAttribute('aria-disabled')), [null, null, 'true', 'true', null, 'true']);
  assert.match(rdRows()[2].textContent, /Not on this host/);
  assert.match(rdRows()[3].title, /not valid UTF-8/);
  assert.match(rdRows()[4].textContent, /20261114-000700-aaaaaaaa/);
  assert.match(rdRows()[5].textContent, /does not say where it started/);
  // Only a gone directory's path can still be copied; the others offer nothing.
  assert.deepEqual(rdRows().map((row) => [...row.querySelectorAll('.terminal-dirs-actions button')].map((item) => item.getAttribute('aria-label')).join(',')),
    ['Show in the Explorer,Open in a new terminal tab,Copy the path', 'Show in the Explorer,Open in a new terminal tab,Copy the path', 'Copy the path', '', 'Show in the Explorer,Open in a new terminal tab,Copy the path', '']);
  // The arrow keys pass over what cannot be chosen, and Enter types it.
  assert.equal(rdRows()[0].getAttribute('aria-selected'), 'true');
  rdKey('ArrowDown');
  assert.equal(rdRows()[1].getAttribute('aria-selected'), 'true');
  rdKey('ArrowDown');
  assert.equal(rdRows()[4].getAttribute('aria-selected'), 'true', 'the keys stopped on a row that cannot be chosen');
  rdKey('ArrowUp');
  rdKey('Enter');
  await wait(30);
  assert.equal(rdBox(), null, 'choosing left the dropdown open');
  assert.ok(rd.requests.includes('GET /_terminal/shells/1/preview'), 'it typed without asking what runs');
  assert.equal(rd.pasted.at(-1), 'cd /w/p1');
  // A click types it too, quoted as every shell reads it.
  await rdOpen();
  rdRows()[0].click();
  await wait(30);
  assert.equal(rd.pasted.at(-1), "cd '/w/p1/a b'");
  // Clicking what cannot be chosen types nothing.
  const typedBefore = rd.pasted.length;
  await rdOpen();
  rdRows()[2].click();
  rdRows()[5].click();
  await wait(30);
  assert.equal(rd.pasted.length, typedBefore, 'a row that cannot be chosen was typed');
  // The filter narrows the rows, and says when nothing matches.
  const rdFilter = rd.document.querySelector('.terminal-dirs-filter');
  rdFilter.value = 'GONE';
  rdFilter.dispatchEvent(new rd.window.Event('input'));
  assert.deepEqual(rdPaths(), ['/w/gone']);
  rdFilter.value = 'zzz';
  rdFilter.dispatchEvent(new rd.window.Event('input'));
  assert.deepEqual(rdPaths(), []);
  assert.match(rdBox().textContent, /Nothing matches/);
  // Escape closes it and gives the keys back to the shell, as the button does again.
  rdKey('Escape');
  assert.equal(rdBox(), null);
  await rdOpen();
  await rdOpen();
  assert.equal(rdBox(), null, 'the button did not close what it opened');
  // A program in front of the shell gets nothing typed into it, and the note says what runs.
  rd.server.commands = { 1: 'vim notes' };
  await rdOpen();
  rdRows()[1].click();
  await wait(30);
  assert.equal(rd.pasted.length, typedBefore, 'cd was typed into vim');
  assert.match(rdNote(), /vim notes is running in this shell/);
  rd.server.commands = { 1: '?' };
  await rdOpen();
  rdRows()[1].click();
  await wait(30);
  assert.equal(rd.pasted.length, typedBefore, 'cd was typed though what runs was not known');
  assert.match(rdNote(), /Cannot tell what runs in this shell \(no such group\)/);
  rd.server.commands = {};
  // A row's other actions: copy, a new tab there, and the Explorer there without following.
  await rdOpen();
  rdRows()[2].querySelector('[aria-label="Copy the path"]').click();
  await wait(20);
  assert.deepEqual(rdCopied, ['/w/gone']);
  await rdOpen();
  rdRows()[0].querySelector('[aria-label="Open in a new terminal tab"]').click();
  await wait(30);
  assert.ok(rd.requests.includes('POST /_terminal/shells?rows=24&cols=80&project=p1&dir=%2Fw%2Fp1%2Fa%20b'), rd.requests.join(', '));
  [...rd.document.querySelectorAll('.terminal-tab [role="tab"]')][0].click();
  await wait(30);
  await rdOpen();
  rdRows()[0].querySelector('[aria-label="Show in the Explorer"]').click();
  await wait(30);
  assert.equal(rd.document.querySelector('.terminal-explorer-where').textContent, '/w/p1/a b');
  assert.equal(rd.document.querySelector('.terminal-explorer .terminal-explorer-head [aria-label="Follow the shell"]').getAttribute('aria-pressed'), 'false');
  // The Explorer's path opens the same dropdown.
  await rdOpen(rd.document.querySelector('.terminal-explorer-where'));
  assert.notEqual(rdBox(), null, 'the Explorer\'s path did not open the dropdown');
  rdKey('Escape');
  // A refusal is shown, not an empty list.
  rd.server.dirs[1] = 500;
  await rdOpen();
  assert.equal(rd.document.querySelector('.terminal-dirs-list').textContent, 'Cannot list the directories: dirs refused');
  rdKey('Escape');
  // A project with no history and no runs says so.
  rd.server.dirs[1] = { history: [], runs: [] };
  await rdOpen();
  assert.deepEqual([...rd.document.querySelectorAll('.terminal-dirs-heading')].map((item) => item.textContent), ['Recent']);
  assert.match(rdBox().textContent, /No directories yet/);
  rdKey('Escape');
  }

  // Links in the output: the words of a line that the server says name a run, a job, a
  // directory, or a file, asked once and kept; a run or job takes the page there, or a new
  // tab with Ctrl, and a path opens in the Explorer or the Table viewer.
  {
  const run = '20261114-000700-aaaaaaaa';
  const lk = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1')], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  lk.server.cwds = { 1: '/w/p1' };
  lk.server.files = { '/w/p1': [], '/w/p1/out': [] };
  lk.server.links = {
    [run]: { kind: 'run', project: 'p1', link: '/project/p1/run/' + run },
    aaaaaaaa1: { kind: 'job', project: 'p1', link: '/project/p1/run/' + run + '?job_id=aaaaaaaa1' },
    'out/x.csv:3': { kind: 'file', path: '/w/p1/out/x.csv' },
    'out/': { kind: 'dir', path: '/w/p1/out' },
    ['/w/p1/runs/' + run + '/log']: { kind: 'file', path: '/w/p1/runs/' + run + '/log' },
    '/w/p1/out/': { kind: 'dir', path: '/w/p1/out' },
  };
  const lkOpened = [];
  lk.window.open = (...args) => { lkOpened.push(args); return { location: {}, close() {} }; };
  await wait(50);
  lk.sockets.at(-1).onopen?.();
  const lkTerm = lk.terminals.at(-1);
  assert.equal(lkTerm.linkProviders?.length, 1, 'the pane has no link provider');
  const linksOn = (y) => new Promise((resolve) => lkTerm.linkProviders[0].provideLinks(y, resolve));
  // The links come from the page's realm, so they are copied into this one to compare.
  const shape = (links) => Array.from(links || [], (link) => [link.text, link.range.start.x, link.range.start.y, link.range.end.x, link.range.end.y]);
  lkTerm.lines = [
    'run ' + run + ' done; job aaaaaaaa1 and deadbeef0',
    'see out/x.csv:3. or out/ or https://example.com/a/b',
    'log /w/p1/runs/' + run + '/log',
    'nothing here',
  ];
  assert.deepEqual(shape(await linksOn(1)), [[run, 5, 1, 28, 1], ['aaaaaaaa1', 40, 1, 48, 1]]);
  assert.deepEqual(Array.from(lk.server.linkAsks.at(-1)), [run, 'aaaaaaaa1', 'deadbeef0'], 'the candidates asked');
  // A sentence's full stop is left off, and a web address is the web links addon's.
  assert.deepEqual(shape(await linksOn(2)), [['out/x.csv:3', 5, 2, 15, 2], ['out/', 21, 2, 24, 2]]);
  assert.ok(!lk.server.linkAsks.at(-1).some((text) => text.includes('example.com')), 'a web address was asked about');
  // A path holding a run ID is one link.
  assert.deepEqual(shape(await linksOn(3)), [['/w/p1/runs/' + run + '/log', 5, 3, 43, 3]]);
  const asksBefore = lk.server.linkAsks.length;
  assert.equal(await linksOn(4), undefined);
  // A run or job asked once is not asked again.
  await linksOn(1);
  assert.equal(lk.server.linkAsks.length, asksBefore, 'a line without candidates, or a known run, was asked about');
  // A link may run across a wrapped line.
  lkTerm.lines = [{ text: 'x'.repeat(76) + ' /w/' }, { text: 'p1/out/ end', wrapped: true }];
  assert.deepEqual(shape(await linksOn(2)), [['/w/p1/out/', 78, 1, 7, 2]]);
  // Hovering says what a link opens.
  lkTerm.lines = ['run ' + run + ' done; job aaaaaaaa1', 'see out/x.csv:3 or out/'];
  const [runLink, jobLink] = await linksOn(1);
  const [fileLink, dirLink] = await linksOn(2);
  const lkViewport = lk.document.querySelector('.terminal-viewport');
  runLink.hover();
  assert.equal(lkViewport.title, 'Open this run of p1; Ctrl- or Cmd-click opens a new tab');
  fileLink.hover();
  assert.equal(lkViewport.title, 'Open /w/p1/out/x.csv in the Table viewer');
  fileLink.leave();
  assert.equal(lkViewport.hasAttribute('title'), false);
  // Ctrl- or Cmd-click opens a run or job in a new tab.
  runLink.activate({ ctrlKey: true });
  jobLink.activate({ metaKey: true });
  assert.deepEqual(lkOpened.map((args) => args[0]), ['/project/p1/run/' + run, '/project/p1/run/' + run + '?job_id=aaaaaaaa1']);
  // A directory opens in the Explorer without following the shell, and a file in the Table viewer.
  dirLink.activate({});
  await wait(30);
  assert.equal(lk.document.querySelector('.terminal-explorer-where').textContent, '/w/p1/out');
  fileLink.activate({});
  await wait(30);
  assert.ok(lk.requests.includes('POST /api/tableview-sources {"path":"/w/p1/out/x.csv"}'), lk.requests.join(', '));
  // A plain click in the panel's own window takes its page there, and the window along.
  const lkTold = [];
  const lkWin = load(process.argv[6], { path: '/_terminal/window/project/p1', shells: [inProject('1', 1, 'p1')], opener: { closed: false, postMessage: (message) => lkTold.push(JSON.parse(JSON.stringify(message))) } });
  lkWin.server.links = lk.server.links;
  await wait(50);
  const winTerm = lkWin.terminals.at(-1);
  winTerm.lines = ['run ' + run];
  const [winRun] = await new Promise((resolve) => winTerm.linkProviders[0].provideLinks(1, resolve));
  winRun.activate({});
  assert.ok(lkTold.some((message) => message.type === 'rotari-terminal-go' && message.path === '/project/p1/run/' + run), JSON.stringify(lkTold));
  // When the server cannot be asked, nothing is a link, and a note says so once.
  lk.server.linksStatus = 500;
  lkTerm.lines = ['other 20261114-000800-bbbbbbbb'];
  assert.equal(await linksOn(1), undefined);
  assert.match(lk.document.querySelector('.terminal-send-note').textContent, /did not say which words in the terminal are links/);
  }

  // Blocks: OSC 133 marks make one block per command, with its command line and exit
  // code; times only for what comes after the replay; nothing in the alternate screen;
  // gone with their lines, and with the screen on a replay.
  {
  const bk = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1')], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  await wait(50);
  const bkSocket = bk.sockets.at(-1);
  bkSocket.onopen?.();
  const bkTerm = bk.terminals.at(-1);
  assert.equal(bkTerm.oscHandlers[133]?.length, 1, 'the pane reads no OSC 133');
  const bkPane = () => bk.document.querySelector('.terminal-pane').terminalPane;
  const blocks = () => Array.from(bkPane().blocks, (block) => ({
    prompt: block.prompt.marker.line, command: block.command(),
    ended: block.ended, exit: block.exit, timed: block.startedAt !== null && block.endedAt !== null,
  }));
  bkTerm.lines = ['$ make', 'built', '$ false', '$ sleep 9', '^C', '$ vim', '$ ', ''];
  // The replay, its marks parsed before it is done: a block per prompt, without times.
  bk.parser.held = true;
  bkSocket.onmessage({ data: new TextEncoder().encode('replay').buffer });
  assert.equal(bkTerm.osc(133, 'A', 0, 0), true);
  bkTerm.osc(133, 'B', 2, 0);
  bkTerm.osc(133, 'C', 0, 1);
  bkTerm.osc(133, 'D;0', 0, 2);
  bk.parser.release();
  assert.deepEqual(blocks(), [{ prompt: 0, command: 'make', ended: true, exit: 0, timed: false }]);
  bkSocket.onmessage({ data: new TextEncoder().encode('live').buffer });
  // After the replay, a command seen running has its times; a bad or missing exit code is none.
  bkTerm.osc(133, 'A', 0, 2);
  bkTerm.osc(133, 'B', 2, 2);
  bkTerm.osc(133, 'C', 0, 3);
  bkTerm.osc(133, 'D;1', 0, 3);
  bkTerm.osc(133, 'A', 0, 3);
  bkTerm.osc(133, 'B', 2, 3);
  bkTerm.osc(133, 'C', 0, 4);
  // A prompt with no D before it ends the command without an exit code.
  bkTerm.osc(133, 'A', 0, 5);
  bkTerm.osc(133, 'B', 2, 5);
  bkTerm.osc(133, 'C', 0, 6);
  // In the alternate screen, marks are vim's or less's, not the shell's.
  bkTerm.buffer.active.type = 'alternate';
  bkTerm.osc(133, 'A', 0, 0);
  bkTerm.osc(133, 'D;0', 0, 0);
  bkTerm.buffer.active.type = 'normal';
  assert.deepEqual(blocks(), [
    { prompt: 0, command: 'make', ended: true, exit: 0, timed: false },
    { prompt: 2, command: 'false', ended: true, exit: 1, timed: true },
    { prompt: 3, command: 'sleep 9', ended: true, exit: null, timed: false },
    { prompt: 5, command: 'vim', ended: false, exit: null, timed: false },
  ]);
  bkTerm.osc(133, 'D;x', 0, 6);
  assert.equal(blocks().at(-1).exit, null, 'an exit code that is no number');
  // A mark with no prompt before it, or after its block's end, adds nothing.
  bkTerm.osc(133, 'C', 0, 6);
  assert.equal(bkPane().blocks.at(-1).output.marker.line, 6);
  // A block whose prompt scrolled away goes with it.
  bkTerm.markers.find((marker) => marker.line === 0).dispose();
  assert.deepEqual(blocks().map((block) => block.command), ['false', 'sleep 9', 'vim']);
  // A command line wrapped onto the next line is one line; one with its own lines keeps them.
  // A line wraps when it is full.
  const bkFull = '$ echo ' + 'a'.repeat(73);
  bkTerm.lines = [bkFull, { text: 'bbbb', wrapped: true }, '> cccc', 'out', ''];
  bkTerm.osc(133, 'A', 0, 0);
  bkTerm.osc(133, 'B', 2, 0);
  bkTerm.osc(133, 'C', 0, 3);
  assert.equal(blocks().at(-1).command, bkFull.slice(2) + 'bbbb\n> cccc');
  // Another shell's replay starts with no blocks, and none of the old markers are left.
  const bkMarkers = bkTerm.markers.length;
  bk.document.querySelectorAll('.terminal-tab [role=tab]')[1].click();
  await wait(50);
  assert.equal(bkPane().blocks.length, 0, 'blocks outlived the replay');
  assert.ok(bkTerm.markers.slice(0, bkMarkers).every((marker) => marker.isDisposed), 'markers outlived the replay');
  assert.equal(bkPane().blocksLive, false, 'a new replay counted as seen live');
  // Clear takes the blocks with the scrollback.
  bk.sockets.at(-1).onmessage({ data: new TextEncoder().encode('x').buffer });
  bkTerm.lines = ['$ ', ''];
  bkTerm.osc(133, 'A', 0, 0);
  bkTerm.clear();
  assert.equal(bkPane().blocks.length, 0, 'blocks outlived Clear');
  }

  // Blocks shown: a gutter only for a shell that marks, a mark per prompt in its state's
  // colour that says on hover how the command ended, separators after output, and failures
  // on the scroll bar in the theme's red.
  {
  const bd = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1'), inProject('2', 2, 'p1')], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  await wait(50);
  const bdSocket = bd.sockets.at(-1);
  bdSocket.onopen?.();
  const bdTerm = bd.terminals.at(-1);
  const bdPane = () => bd.document.querySelector('.terminal-pane').terminalPane;
  const gutter = () => bdPane().viewport.classList.contains('terminal-viewport-blocks');
  assert.equal(bdTerm.options.allowProposedApi, true, 'decorations need the proposed API');
  // A shell that sends no marks gets no gutter, decorations, or scroll bar marks.
  bdSocket.onmessage({ data: new TextEncoder().encode('plain').buffer });
  assert.equal(gutter(), false, 'a gutter without marks');
  assert.equal(bdTerm.decorations.length, 0);
  assert.equal(bdTerm.options.overviewRulerWidth, undefined, 'a scroll bar ruler without failures');
  // shown is what the decoration of the n-th block shows once xterm.js draws it.
  const shown = (n) => {
    const block = bdPane().blocks[n];
    const element = block.decoration.render();
    const mark = element.querySelector('.terminal-block-mark');
    return { state: mark.dataset.state, hidden: mark.hidden, title: mark.getAttribute('title'),
      separated: element.classList.contains('terminal-block-separated'), spans: element.classList.contains('terminal-block'),
      ruler: block.decoration.options.overviewRulerOptions?.color };
  };
  let now = 1000000;
  bd.window.Date.now = () => now;
  bd.document.documentElement.style.setProperty('--terminal-red', '#b3261e');
  bdTerm.lines = ['$ make', 'built', '$ false', '$ sleep 9', '^C', '$ ', '$ vi', 'edited', ''];
  bdTerm.osc(133, 'A', 0, 0);
  assert.equal(gutter(), true, 'no gutter for a shell that marks');
  assert.deepEqual(shown(0), { state: 'prompt', hidden: true, title: null, separated: false, spans: true, ruler: undefined });
  bdTerm.osc(133, 'B', 2, 0);
  bdTerm.osc(133, 'C', 0, 1);
  assert.deepEqual(shown(0), { state: 'running', hidden: false, title: 'make\nRunning', separated: false, spans: true, ruler: undefined });
  now += 1500;
  bdTerm.osc(133, 'D;0', 0, 2);
  assert.deepEqual(shown(0), { state: 'ok', hidden: false, title: 'make\nExit 0 after 1.5 s', separated: false, spans: true, ruler: undefined });
  bdTerm.osc(133, 'A', 0, 2);
  bdTerm.osc(133, 'B', 2, 2);
  bdTerm.osc(133, 'C', 0, 3);
  const running = bdPane().blocks[1].decoration;
  now += 75000;
  bdTerm.osc(133, 'D;1', 0, 3);
  assert.ok(running.isDisposed, 'the decoration of a failure was not made again for the scroll bar');
  assert.deepEqual(shown(1), { state: 'failed', hidden: false, title: 'false\nExit 1 after 1 min 15 s', separated: true, spans: true, ruler: '#b3261e' });
  assert.equal(bdTerm.options.overviewRulerWidth, 10, 'no scroll bar ruler for a failure');
  // Another prompt before D: the command ended without saying how.
  bdTerm.osc(133, 'A', 0, 3);
  bdTerm.osc(133, 'B', 2, 3);
  bdTerm.osc(133, 'C', 0, 4);
  bdTerm.osc(133, 'A', 0, 5);
  assert.deepEqual(shown(2), { state: 'unknown', hidden: false, title: 'sleep 9\nNo exit code: another prompt came first', separated: true, spans: true, ruler: undefined });
  // An empty prompt is no command: no mark, and no separator after it.
  bdTerm.osc(133, 'A', 0, 6);
  assert.equal(shown(3).hidden, true);
  assert.equal(shown(3).separated, true, 'a prompt after output has no separator');
  assert.equal(shown(4).separated, false, 'a prompt after an empty one has a separator');
  bdTerm.osc(133, 'B', 2, 6);
  bdTerm.osc(133, 'C', 0, 7);
  bdTerm.osc(133, 'D', 0, 7);
  assert.deepEqual(shown(4), { state: 'unknown', hidden: false, title: 'vi\nEnded without an exit code after 0.0 s', separated: false, spans: true, ruler: undefined });
  // A new theme makes the scroll bar's marks again in its red.
  const failed = bdPane().blocks[1].decoration;
  bd.document.documentElement.style.setProperty('--terminal-red', '#f07a72');
  bd.document.documentElement.dataset.theme = 'dark';
  await wait(10);
  assert.ok(failed.isDisposed, 'the old scroll bar mark outlived the theme');
  assert.equal(shown(1).ruler, '#f07a72');
  // A replay that brings no marks takes the gutter away, and the decorations with the blocks.
  bd.document.querySelectorAll('.terminal-tab [role=tab]')[1].click();
  await wait(50);
  assert.equal(gutter(), true, 'the gutter went before the replay was done');
  bd.sockets.at(-1).onmessage({ data: new TextEncoder().encode('plain').buffer });
  assert.equal(gutter(), false, 'a gutter after a replay without marks');
  assert.ok(bdTerm.decorations.every((decoration) => decoration.isDisposed), 'decorations outlived the replay');
  }

  // Blocks acted on: Commands lists the pane's commands, newest first, to go to one or copy
  // its output or line; no key is taken from the shell; a mark's menu copies a command's
  // output or line, selects the output, or types the line again at the shell's prompt.
  {
  const bp = load(process.argv[1], { stored: '1', path: '/project/p1', shells: [inProject('1', 1, 'p1')], remembered: ['rotari-terminal-shell:/b:p1', '1'] });
  await wait(50);
  const bpSocket = bp.sockets.at(-1);
  bpSocket.onopen?.();
  const bpTerm = bp.terminals.at(-1);
  const bpPane = () => bp.document.querySelector('.terminal-pane').terminalPane;
  const bpCopied = [];
  Object.defineProperty(bp.window.navigator, 'clipboard', { configurable: true, value: { writeText: async (text) => bpCopied.push(text) } });
  const cmdButton = [...bp.document.querySelectorAll('.terminal-bar button')].find((item) => item.getAttribute('aria-label') === 'Commands');
  const cmdBox = () => bp.document.querySelector('.terminal-commands');
  const cmdRows = () => [...bp.document.querySelectorAll('.terminal-commands .terminal-dirs-row')];
  const cmdOpen = async () => { cmdButton.click(); await wait(20); };
  const cmdKey = (key) => cmdBox().dispatchEvent(new bp.window.KeyboardEvent('keydown', { key, bubbles: true }));
  const cmdCopy = async (n, label) => {
    await cmdOpen();
    [...cmdRows()[n].querySelectorAll('button')].find((item) => item.textContent === label).click();
    await wait(20);
  };
  // A shell that sends no marks lists no commands, and says where they come from.
  bpSocket.onmessage({ data: new TextEncoder().encode('plain').buffer });
  assert.equal(cmdButton.disabled, false);
  await cmdOpen();
  assert.match(cmdBox().textContent, /No commands yet\. They are listed from shells that mark them/);
  await cmdOpen();
  assert.equal(cmdBox(), null, 'the button left Commands open');
  bpTerm.lines = ['$ make', 'built', 'ok', '$ false', 'bad', '$ echo a', '> b', 'a', 'b', '$ sleep 9', 'zz', ''];
  bpTerm.osc(133, 'A', 0, 0); bpTerm.osc(133, 'B', 2, 0); bpTerm.osc(133, 'C', 0, 1); bpTerm.osc(133, 'D;0', 0, 3);
  bpTerm.osc(133, 'A', 0, 3); bpTerm.osc(133, 'B', 2, 3); bpTerm.osc(133, 'C', 0, 4); bpTerm.osc(133, 'D;1', 0, 5);
  bpTerm.osc(133, 'A', 0, 5); bpTerm.osc(133, 'B', 2, 5); bpTerm.osc(133, 'C', 0, 7); bpTerm.osc(133, 'D;0', 0, 9);
  bpTerm.osc(133, 'A', 0, 9); bpTerm.osc(133, 'B', 2, 9); bpTerm.osc(133, 'C', 0, 10);
  Object.assign(bpTerm.buffer.active, { cursorX: 2, cursorY: 10, viewportY: 11 });
  // Newest first, each with its state, line, and end.
  await cmdOpen();
  assert.deepEqual(cmdRows().map((row) => [row.querySelector('.terminal-commands-state').dataset.state, row.querySelector('.terminal-dirs-path').textContent, row.querySelector('.terminal-dirs-detail').textContent.replace(/ after .*/, '')]), [
    ['running', 'sleep 9', 'Running'], ['ok', 'echo a\n> b', 'Exit 0'], ['failed', 'false', 'Exit 1'], ['ok', 'make', 'Exit 0']]);
  assert.equal(cmdRows()[0].getAttribute('aria-selected'), 'true');
  // The arrow keys and Enter go to one: its prompt at the top, lit up for a moment.
  cmdKey('ArrowDown');
  cmdKey('ArrowDown');
  cmdKey('Enter');
  assert.equal(bpTerm.buffer.active.viewportY, 3, 'Enter did not go to false');
  assert.equal(cmdBox(), null, 'choosing left Commands open');
  assert.ok(bpPane().blocks[1].decoration.render().classList.contains('terminal-block-flash'), 'the prompt gone to was not lit up');
  await wait(1600);
  assert.ok(!bpPane().blocks[1].decoration.render().classList.contains('terminal-block-flash'), 'the prompt stayed lit up');
  // The filter narrows them by command line, and a click goes to one.
  await cmdOpen();
  const cmdFilter = cmdBox().querySelector('input');
  cmdFilter.value = 'MAK';
  cmdFilter.dispatchEvent(new bp.window.Event('input'));
  assert.deepEqual(cmdRows().map((row) => row.querySelector('.terminal-dirs-path').textContent), ['make']);
  cmdRows()[0].click();
  assert.equal(bpTerm.buffer.active.viewportY, 0, 'a click did not go to make');
  // A row copies its output, so far for one running, or its line.
  await cmdCopy(3, 'Copy output');
  await cmdCopy(2, 'Copy command');
  await cmdCopy(0, 'Copy output');
  assert.deepEqual(bpCopied, ['built\nok', 'false', 'zz']);
  assert.equal(bp.document.querySelector('.terminal-send-note').textContent, 'Copied the output');
  bpCopied.length = 0;
  // No key is the panel's: Ctrl+Shift+ArrowUp reaches the shell.
  assert.equal(bpTerm.keyHandler, undefined, 'the panel takes keys from the shell');
  // menu opens the n-th block's menu from its mark, which keeps the pointer from the text.
  const menu = (n) => {
    const element = bpPane().blocks[n].decoration.render();
    const mark = element.querySelector('.terminal-block-mark');
    let reached = false;
    element.addEventListener('mousedown', () => (reached = true));
    mark.dispatchEvent(new bp.window.MouseEvent('mousedown', { bubbles: true }));
    assert.equal(reached, false, 'a press on a mark reached the terminal');
    mark.dispatchEvent(new bp.window.MouseEvent('click', { bubbles: true }));
    return [...bp.document.querySelectorAll('.terminal-tab-menu button')];
  };
  const choose = async (n, label) => {
    menu(n).find((item) => item.textContent === label).click();
    await wait(20);
  };
  assert.deepEqual(menu(0).map((item) => [item.textContent, item.disabled]), [
    ['Copy output', false], ['Copy command', false], ['Select output', false], ['Type command again', false]]);
  await choose(0, 'Copy output');
  await choose(0, 'Copy command');
  await choose(2, 'Copy output');
  // A running command's output so far, up to the cursor.
  await choose(3, 'Copy output');
  assert.deepEqual(bpCopied, ['built\nok', 'make', 'a\nb', 'zz']);
  assert.equal(bp.document.querySelector('.terminal-send-note').textContent, 'Copied the output');
  await choose(0, 'Select output');
  assert.deepEqual(bpTerm.selection, { column: 0, row: 1, length: 2 * 80 }, 'Select output took other lines');
  // A command on lines of its own would be typed with its continuation prompts.
  assert.deepEqual(menu(2).at(-1).textContent, 'Type command again (only a one-line command)');
  assert.equal(menu(2).at(-1).disabled, true);
  // Typing again asks what runs first, and never presses Enter.
  await choose(1, 'Type command again');
  assert.ok(bp.requests.includes('GET /_terminal/shells/1/preview'), 'it typed without asking what runs');
  assert.equal(bp.pasted.at(-1), 'false');
  const typed = bp.pasted.length;
  bp.server.commands = { 1: 'vim' };
  await choose(1, 'Type command again');
  assert.equal(bp.pasted.length, typed, 'typed while vim runs');
  assert.equal(bp.document.querySelector('.terminal-send-note').textContent, 'vim is running in this shell, so the command was not typed.');
  // An empty prompt is no command, and its mark is hidden.
  bpTerm.osc(133, 'A', 0, 11);
  assert.equal(bpPane().blocks[4].decoration.render().querySelector('.terminal-block-mark').hidden, true);
  }
  process.exit(0);
})().catch((error) => { console.error(error.stack || String(error)); process.exit(1); });
`
	// The script is longer than one argument may be, so -e reads it from a file. Run by -e,
	// it finds jsdom from this directory and its arguments start at argv[1].
	scriptFile := filepath.Join(dir, "panel_check.js")
	if err := os.WriteFile(scriptFile, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	check := exec.Command("node", "-e", "eval(require('fs').readFileSync(process.env.ROTARI_PANEL_CHECK, 'utf8'))", withTerminal, without, filepath.Join("assets", "web_shell.css"), jobsWithTerminal, filepath.Join("assets", "table_view_panes.js"), terminalWindow)
	check.Env = append(os.Environ(), "ROTARI_PANEL_CHECK="+scriptFile)
	if output, err := check.CombinedOutput(); err != nil {
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
