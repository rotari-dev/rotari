package webui

import (
	"os"
	"os/exec"
	"path/filepath"
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
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const load = (file, { stubs = true, stored = '' } = {}) => {
  const sockets = [], written = [], terminals = [];
  const dom = new JSDOM(fs.readFileSync(file, 'utf8'), {
    runScripts: 'dangerously',
    url: 'http://127.0.0.1:9/',
    virtualConsole: new VirtualConsole(),
    beforeParse(window) {
      if (stored) window.localStorage.setItem('rotari-terminal-open', stored);
      window.fetch = () => new Promise(() => {});
      window.setInterval = () => 1;
      window.confirm = () => true;
      window.ResizeObserver = class { observe() {} };
      window.WebSocket = class {
        static OPEN = 1;
        constructor(url) { this.url = url; this.sent = []; this.readyState = 1; sockets.push(this); }
        send(data) { this.sent.push(data); }
      };
      if (!stubs) return;
      window.Terminal = class {
        rows = 24; cols = 80;
        constructor() { terminals.push(this); }
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
  return { dom, window: dom.window, document: dom.window.document, sockets, written, terminals };
};
(async () => {
  // Without the option there is nothing to open.
  const off = load(process.argv[2], { stored: '1' });
  await wait(50);
  assert.equal(off.document.getElementById('terminal-button'), null);
  assert.equal(off.sockets.length, 0, 'connected with no terminal on offer');

  const { window, document, sockets, written, terminals } = load(process.argv[1]);
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
  assert.equal(sockets[0].url, 'ws://127.0.0.1:9/_terminal/ws?rows=24&cols=80');

  // Output, input, and resize.
  sockets[0].onmessage({ data: new TextEncoder().encode('hello').buffer });
  assert.deepEqual(written, ['hello']);
  terminals[0].data('typed');
  assert.equal(Buffer.from(sockets[0].sent[0]).toString(), 'typed');
  terminals[0].resize({ rows: 30, cols: 100 });
  assert.deepEqual(JSON.parse(sockets[0].sent[1]), { type: 'resize', rows: 30, cols: 100 });

  const status = () => document.querySelector('.terminal-status');
  const action = () => document.querySelector('.terminal-bar button');
  // Another tab took over: it says so, stays quiet, and can take it back.
  sockets[0].onclose({ code: 4001, reason: '' });
  await wait(1200);
  assert.equal(sockets.length, 1, 'fought the other tab for the shell');
  assert.match(status().textContent, /Another tab/);
  action().click();
  assert.equal(sockets.length, 2);
  assert.equal(status().hidden, true);
  assert.deepEqual(written, [], 'the old screen stayed under the replay');

  // The shell ended: it says how, and a new one is a choice.
  sockets[1].onclose({ code: 4002, reason: 'exit status 3' });
  await wait(1200);
  assert.equal(sockets.length, 2, 'started a new shell unasked');
  assert.match(status().textContent, /exit status 3/);
  action().click();
  assert.equal(sockets.length, 3);

  // A failed start says why.
  sockets[2].onclose({ code: 4000, reason: 'no such shell' });
  assert.match(status().textContent, /no such shell/);
  action().click();

  // A lost connection comes back by itself.
  sockets[3].onclose({ code: 1006, reason: '' });
  assert.match(status().textContent, /Reconnecting/);
  await wait(1200);
  assert.equal(sockets.length, 5, 'did not reconnect');

  // End asks the server; Hide leaves the shell and the connection alone.
  [...document.querySelectorAll('.terminal-bar button')].find((item) => item.textContent === 'End shell').click();
  assert.deepEqual(JSON.parse(sockets[4].sent.at(-1)), { type: 'end' });
  [...document.querySelectorAll('.terminal-bar button')].find((item) => item.textContent === 'Hide').click();
  assert.equal(document.getElementById('terminal-panel').hidden, true);
  assert.equal(window.localStorage.getItem('rotari-terminal-open'), '0');
  assert.equal(button.getAttribute('aria-pressed'), 'false');
  assert.equal(sockets.length, 5);
  button.click();
  assert.equal(document.getElementById('terminal-panel').hidden, false);
  assert.equal(sockets.length, 5, 'showing the panel again reconnected');

  // A panel left open comes back with the next page.
  const next = load(process.argv[1], { stored: '1' });
  await wait(50);
  assert.equal(next.sockets.length, 1, 'the next page did not reattach');

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
	if output, err := exec.Command("node", "-e", script, withTerminal, without).CombinedOutput(); err != nil {
		t.Fatalf("terminal panel check failed: %v\n%s", err, output)
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
