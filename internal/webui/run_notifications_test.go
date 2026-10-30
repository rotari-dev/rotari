package webui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// inboxServerJS fakes the notification endpoints for a page under test:
// entries, newest first, each with the basedir its run belongs to; the shared
// cursors; and, while control is off, a state endpoint that refuses writes.
const inboxServerJS = `
const zero = {created_at: '', id: ''};
const after = (a, b) => a.created_at !== b.created_at ? a.created_at > b.created_at : a.id > b.id;
const server = {entries: [], state: {read: zero, cleared: zero}, control: true, fail: '', requested: [], posts: []};
let clock = 0;
function addEntry(basedir, title, url) {
  clock++;
  const entry = {created_at: '2026-10-31T00:00:' + String(clock).padStart(2, '0') + '.000000000Z', id: 'r-' + clock, basedir, title, body: 'project: <project>', url};
  server.entries.unshift(entry);
  return entry;
}
function fakeFetch(url, init) {
  url = String(url);
  if (url === '/api/notifications/state') {
    const next = JSON.parse(init.body);
    server.posts.push(next);
    if (!server.control) return Promise.resolve({ok: false, status: 403, text: async () => 'read-only'});
    for (const name of ['read', 'cleared']) if (after(next[name], server.state[name])) server.state[name] = next[name];
    if (after(server.state.cleared, server.state.read)) server.state.read = server.state.cleared;
    return Promise.resolve({ok: true, json: async () => server.state});
  }
  if (url.startsWith('/api/notifications?')) {
    if (server.fail) return Promise.resolve({ok: false, status: 500, text: async () => server.fail});
    const ids = new URLSearchParams(url.split('?')[1]).get('basedirs').split(',');
    server.requested.push(ids.join(','));
    const entries = server.entries.filter(entry => ids.includes(entry.basedir) && after(entry, server.state.cleared))
      .map(({basedir, ...entry}) => entry);
    return Promise.resolve({ok: true, json: async () => ({entries, ...server.state})});
  }
  return Promise.reject(new Error('offline'));
}
`

// The inbox shows the notifications the server recorded, so history from
// before the page opened is there and unread. Opening it marks entries read
// through the shared cursors, Clear hides them, and a read-only server keeps
// both in page memory. It needs neither the desktop API nor its permission.
func TestNotificationInboxWithoutDesktopAPI(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	script := `
const fs = require('fs');
const { JSDOM, VirtualConsole } = require('jsdom');
` + inboxServerJS + `
const basedir = process.argv[2];
const errors = [];
const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', error => errors.push(String(error)));
addEntry(basedir, '<b>rotari: run failed</b>', '/_basedir/' + basedir + '/project/p/run/r');
const dom = new JSDOM(fs.readFileSync(process.argv[1], 'utf8'), {
  runScripts: 'dangerously', url: 'http://example.test/', virtualConsole,
  beforeParse(window) {
    window.Notification = undefined;
    window.isSecureContext = false;
    window.localStorage.setItem('rotari-notifications-enabled', 'false');
    window.setInterval = () => 1;
    window.fetch = fakeFetch;
  },
});
const settle = () => new Promise(resolve => setTimeout(resolve, 20));
(async () => {
  try {
    await settle();
    const w = dom.window, d = w.document;
    const poll = async () => { await w.refreshNotificationInbox(); await settle(); };
    const button = d.querySelector('.notification-inbox > button');
    const panel = d.getElementById('notification-inbox-panel');
    const announcement = d.querySelector('.notification-announcement');
    if (!button || button.textContent !== 'Inbox (1)' || !panel.hidden) throw Error('history is not unread: ' + button?.textContent);
    if (announcement.textContent) throw Error('history was announced');
    if (!panel.textContent.includes('<b>rotari') || !panel.textContent.includes('<project>') || panel.querySelector('b'))
      throw Error('unsafe content');
    const link = panel.querySelector('a');
    if (!link || !link.href.endsWith('/_basedir/' + basedir + '/project/p/run/r')) throw Error('missing run link');
    button.click();
    await settle();
    if (panel.hidden || button.textContent !== 'Inbox (0)') throw Error('not marked read');
    if (server.state.read.id !== 'r-1') throw Error('read cursor not shared: ' + JSON.stringify(server.posts));
    addEntry(basedir, 'second', '');
    await poll();
    if (button.textContent !== 'Inbox (0)' || announcement.textContent !== 'second') throw Error('open inbox became unread');
    d.dispatchEvent(new w.KeyboardEvent('keydown', {key: 'Escape'}));
    if (!panel.hidden || button.getAttribute('aria-expanded') !== 'false') throw Error('Escape failed');
    addEntry(basedir, 'third', '');
    addEntry('elsewhere', 'other basedir', '');
    await poll();
    if (button.textContent !== 'Inbox (1)') throw Error('new entry not unread: ' + button.textContent);
    if (server.requested.some(ids => ids !== basedir)) throw Error('wrong basedirs: ' + server.requested);
    button.click();
    panel.querySelector('button').click();
    await poll();
    if (panel.querySelector('article') || button.textContent !== 'Inbox (0)') throw Error('clear failed');
    if (server.state.cleared.id !== 'r-3') throw Error('cleared cursor not shared');
    server.control = false;
    button.click();
    addEntry(basedir, 'read-only', '');
    await poll();
    if (button.textContent !== 'Inbox (1)') throw Error('read-only entry not unread');
    button.click();
    await poll();
    if (button.textContent !== 'Inbox (0)') throw Error('read-only server did not keep the page read state');
    button.click();
    panel.querySelector('button').click();
    await poll();
    if (panel.querySelector('article')) throw Error('read-only server did not keep the page clear state');
    server.fail = 'disk full';
    await poll();
    if (!panel.textContent.includes('Cannot load notifications: disk full')) throw Error('load error hidden');
    if (errors.length) throw Error(errors.join('\n'));
    dom.window.close();
  } catch (error) { console.error(error); process.exit(1); }
})();
`
	basedirs := []webBaseDir{newWebBaseDir("current", "/state", true)}
	for name, page := range map[string]string{
		"application": siteFor("/state").webHTML(),
		"recent jobs": jobsHTML("/", nil, nil, jobsSelection{}, true, true, basedirs),
	} {
		t.Run(name, func(t *testing.T) {
			id := "current"
			if name == "application" {
				id = siteFor("/state").currentBaseID()
			}
			path := filepath.Join(t.TempDir(), "index.html")
			if err := os.WriteFile(path, []byte(page), 0o600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("node", "-e", script, path, id).CombinedOutput(); err != nil {
				t.Fatalf("notification inbox: %v\n%s", err, output)
			}
		})
	}
}

// A desktop notification shows only an entry that arrives while the page is
// open. Entries already recorded when the page opens, or when a basedir's
// bell is turned on, are history: they enter the inbox without a popup.
func TestNotificationInboxAnnouncesOnlyNewEntries(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	script := `
const fs = require('fs');
const { JSDOM, VirtualConsole } = require('jsdom');
` + inboxServerJS + `
const errors = [];
const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', error => errors.push(error.stack || String(error)));
const shown = [];
addEntry('one', 'old in one', '');
addEntry('two', 'old in two', '');
const dom = new JSDOM(fs.readFileSync(process.argv[1], 'utf8'), {
  runScripts: 'dangerously', url: 'https://example.test/', virtualConsole,
  beforeParse(window) {
    window.Notification = class {
      constructor(title, options) { shown.push(title + ' | ' + options.body); }
    };
    window.Notification.permission = 'granted';
    window.Notification.requestPermission = async () => 'granted';
    window.localStorage.setItem('rotari-notifications-enabled', 'true');
    window.setInterval = () => 1;
    window.fetch = fakeFetch;
  },
});
const settle = () => new Promise(resolve => setTimeout(resolve, 20));
(async () => {
  try {
    await settle();
    const w = dom.window, d = w.document;
    const poll = async () => { await w.refreshNotificationInbox(); await settle(); };
    if (shown.length) throw Error('history popped up: ' + shown);
    addEntry('one', 'new in one', '');
    await poll();
    if (shown.join('\n') !== 'new in one | project: <project>') throw Error('new entry: ' + shown);
    const other = d.querySelector('.basedir-notification-toggle[data-basedir-id="two"]');
    other.checked = true;
    w.toggleJobsNotificationBasedir(other);
    await settle();
    if (shown.length !== 1) throw Error('a newly selected basedir popped up its history: ' + shown);
    if (!d.getElementById('notification-inbox-panel').textContent.includes('old in two')) throw Error('selected basedir history missing');
    addEntry('two', 'new in two', '');
    await poll();
    if (shown.length !== 2 || !shown[1].startsWith('new in two')) throw Error('new entry in two: ' + shown);
    if (errors.length) throw Error(errors.join('\n'));
    dom.window.close();
  } catch (error) { console.error(error); process.exit(1); }
})();
`
	page := jobsHTML("/", nil, nil, jobsSelection{}, true, true, []webBaseDir{newWebBaseDir("one", "/one", true), newWebBaseDir("two", "/two", false)})
	path := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(path, []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("node", "-e", script, path).CombinedOutput(); err != nil {
		t.Fatalf("notification inbox: %v\n%s", err, output)
	}
}

// Browsers allow notifications only in a secure context, and an insecure page
// may not expose Notification at all. The toggle on the application and Recent
// jobs pages then stays visible but disabled, and says why, instead of
// disappearing or being disabled silently.
func TestNotificationToggleExplainsWhyItIsUnavailable(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	script := `
const fs = require('fs');
const { JSDOM, VirtualConsole } = require('jsdom');
const html = fs.readFileSync(process.argv[1], 'utf8');
const cases = [
	{ name: 'secure, permission not asked', secure: true, permission: 'default', hidden: false, disabled: false, title: '' },
	{ name: 'secure, permission denied', secure: true, permission: 'denied', hidden: false, disabled: true, title: 'blocked' },
	{ name: 'secure, no Notification API', secure: true, permission: null, hidden: true },
	{ name: 'insecure, permission denied', secure: false, permission: 'denied', hidden: false, disabled: true, title: 'localhost' },
	{ name: 'insecure, no Notification API', secure: false, permission: null, hidden: false, disabled: true, title: 'localhost' },
];
const failures = [];
let pending = cases.length;
for (const test of cases) {
	const errors = [];
	const virtualConsole = new VirtualConsole();
	virtualConsole.on('jsdomError', error => errors.push(error.stack || String(error)));
	const dom = new JSDOM(html, {
		runScripts: 'dangerously',
		url: 'http://example.test/',
		virtualConsole,
		beforeParse(window) {
			window.isSecureContext = test.secure;
			if (test.permission !== null)
				window.Notification = { permission: test.permission, requestPermission: async () => test.permission };
			window.setInterval = () => 1;
			window.fetch = async () => { throw new Error('offline'); };
		},
	});
	setTimeout(() => {
		const button = dom.window.document.getElementById('notify-toggle');
		const got = !button ? 'no button'
			: button.hidden ? 'hidden'
			: 'disabled=' + button.disabled + ' title=' + JSON.stringify(button.title);
		const ok = button && button.hidden === test.hidden &&
			(test.hidden || (button.disabled === test.disabled && (test.title === '' ? button.title === '' : button.title.includes(test.title))));
		if (!ok) failures.push(test.name + ': got ' + got);
		failures.push(...errors.map(error => test.name + ': ' + error));
		if (--pending === 0 && failures.length) {
			console.error(failures.join('\n'));
			process.exit(1);
		}
	}, 50);
}
`
	pages := map[string]string{
		"application": testSite().webHTML(),
		"recent jobs": jobsHTML("/", nil, nil, jobsSelection{}, true, true),
	}
	for name, page := range pages {
		t.Run(name, func(t *testing.T) {
			htmlPath := filepath.Join(t.TempDir(), "index.html")
			if err := os.WriteFile(htmlPath, []byte(page), 0o600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("node", "-e", script, htmlPath).CombinedOutput(); err != nil {
				t.Fatalf("notification toggle: %v\n%s", err, output)
			}
		})
	}
}
