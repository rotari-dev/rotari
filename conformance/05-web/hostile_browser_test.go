package web

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// hostileBrowserCheck is hostilePageCheck in a real browser, which runs what
// jsdom does not, such as an image's onerror, and enforces a
// Content-Security-Policy. It also reports each policy violation, since one on a
// page's own script is a control the policy broke.
const hostileBrowserCheck = `
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, pages, token, mark } = JSON.parse(readFileSync(input, 'utf8'));

// Runs first in every document. A click must not change anything on the server.
const prepare = ` + "`" + `
  window.__violations = [];
  document.addEventListener('securitypolicyviolation', (event) => {
    window.__violations.push(event.violatedDirective + ' ' + event.blockedURI + ' ' + event.sourceFile + ':' + event.lineNumber);
  });
  const pageFetch = window.fetch.bind(window);
  window.__pending = 0;
  window.fetch = (input, init = {}) => {
    if ((init.method || 'GET').toUpperCase() !== 'GET')
      return Promise.resolve(new Response('{"error":"blocked by the test"}', { status: 403 }));
    window.__pending++;
    return pageFetch(input, init).finally(() => { window.__pending--; });
  };
  window.confirm = () => false;
  window.prompt = () => null;
  window.alert = () => {};
  window.open = () => null;
  document.addEventListener('submit', (event) => {
    if ((event.target.method || '').toLowerCase() === 'post') event.preventDefault();
  }, true);
` + "`" + `;

// Adds what a value that became markup could: a handler and a script. Neither may
// run, and the browser must report them.
const plantExpression = ` + "`" + `(async () => {
  const holder = document.createElement('div');
  holder.innerHTML = '<img src=x onerror="window.__planted = 1">';
  document.body.append(holder);
  const script = document.createElement('script');
  script.textContent = 'window.__planted = 2';
  document.body.append(script);
  await new Promise((resolve) => setTimeout(resolve, 800));
  return { planted: window.__planted, violations: window.__violations };
})()` + "`" + `;

const inspectExpression = ` + "`" + `(() => ({
  injected: window.__injected !== undefined,
  markup: [...document.querySelectorAll('[data-injected]')].slice(0, 1).map((node) => (node.parentElement || node).outerHTML.slice(0, 300)),
  links: [...document.querySelectorAll('a[href]')].filter((node) => /^\\s*(javascript|data|vbscript):/i.test(node.getAttribute('href'))).slice(0, 1).map((node) => node.outerHTML.slice(0, 300)),
  violations: window.__violations || [],
  text: (document.body ? document.body.textContent : '') + [...document.querySelectorAll('input, textarea')].map((node) => node.value).join(' '),
  selectors: EXPECTED.filter((item) => item.startsWith('selector:') && document.querySelector(item.slice(9))),
  href: location.href,
}))()` + "`" + `;

const clickExpression = (seen) => ` + "`" + `(() => {
  const seen = new Set(${JSON.stringify(seen)});
  for (const control of document.querySelectorAll('button, summary, [data-action], [role="button"]')) {
    if (control.disabled || !control.isConnected) continue;
    const key = [control.tagName, control.className, control.getAttribute('data-action') || '', control.getAttribute('data-args') || '', (control.textContent || '').slice(0, 60)].join('|');
    if (seen.has(key)) continue;
    control.click();
    return key;
  }
  return null;
})()` + "`" + `;

const searchExpression = ` + "`" + `(async () => {
  const basedir = document.querySelector('.history-search-scope-basedir');
  if (!basedir) return;
  basedir.selectedIndex = 1;
  basedir.dispatchEvent(new Event('change', { bubbles: true }));
  await new Promise((resolve) => setTimeout(resolve, 300));
  const word = document.querySelector('.history-search-word');
  if (word) word.value = 'hostile-mark';
  document.querySelector('form').dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  await new Promise((resolve) => setTimeout(resolve, 500));
})()` + "`" + `;

async function check(cdp, page) {
  const address = page.file ? pathToFileURL(page.file).href : page.url;
  const found = new Set();
  const missing = new Set(page.expect || []);
  let shown = false;
  let clicks = 0;
  const beforeNavigate = async (session) => {
    await session.send('Page.addScriptToEvaluateOnNewDocument', { source: prepare });
    if (token) {
      await session.send('Network.enable');
      await session.send('Network.setExtraHTTPHeaders', { headers: { 'X-Rotari-Token': token } });
    }
  };
  await withPage(cdp, address, { width: 1280, height: 900, settleMs: 800, beforeNavigate }, async (session) => {
    if (page.plant) {
      const { planted, violations } = await session.evaluate(plantExpression);
      if (planted !== undefined) found.add('a planted script ran: ' + planted);
      if (violations.length < 2) found.add('the browser reported ' + violations.length + ' violations, want one each for the handler and the script');
      shown = true;
      return;
    }
    const inspect = async (when) => {
      const state = await session.evaluate(inspectExpression.replace('EXPECTED', JSON.stringify(page.expect || [])));
      if (state.injected) found.add('ran as code ' + when);
      state.markup.forEach((html) => found.add('markup ' + when + ': ' + html));
      state.links.forEach((html) => found.add('link ' + when + ': ' + html));
      state.violations.forEach((violation) => found.add('policy violation ' + violation));
      shown ||= state.text.includes(mark);
      for (const expected of [...missing]) {
        if (expected.startsWith('selector:') ? state.selectors.includes(expected) : state.text.includes(expected)) missing.delete(expected);
      }
      return state;
    };
    // A loaded host renders late, and scripts in files load after the page.
    for (const deadline = Date.now() + 30000; Date.now() < deadline; await sleep(100)) {
      if ((await inspect('on load')).text.includes(mark)) break;
    }
    if (page.search) {
      await session.evaluate(searchExpression);
      await inspect('after a search');
    }
    const seen = [];
    while (clicks < 300) {
      let key;
      try {
        key = await session.evaluate(clickExpression(seen));
      } catch {
        // The click navigated away while it was being evaluated.
        await sleep(300);
        continue;
      }
      if (key === null) break;
      seen.push(key);
      clicks++;
      let state;
      try {
        await sleep(60);
        for (const deadline = Date.now() + 10000; Date.now() < deadline && (await session.evaluate('window.__pending || 0')); ) await sleep(30);
        await sleep(40);
        state = await inspect('after clicking ' + key.slice(0, 120));
      } catch {
        await sleep(300);
        continue;
      }
      if (state.href !== address && !state.href.startsWith(address + '#')) {
        await session.send('Page.navigate', { url: address });
        await sleep(800);
      }
    }
    await sleep(200);
    await inspect('at the end');
  });
  return { name: page.name, shown, clicks, found: [...found], missing: [...missing] };
}

const { cdp, close } = await launchChrome(chrome);
const results = [];
for (const page of pages) {
  try {
    results.push(await check(cdp, page));
  } catch (error) {
    results.push({ name: page.name, shown: false, clicks: 0, found: ['failed: ' + (error.stack || error)], missing: [] });
  }
}
await close();
console.log(JSON.stringify(results));
process.exit(0);
`

// browserForTests returns the Chrome to test with: ROTARI_TEST_CHROME, or one on
// PATH. Without one the test is skipped, unless ROTARI_REQUIRE_BROWSER_TESTS is
// set, as in CI, where a missing browser must not pass unnoticed.
func browserForTests(t *testing.T) string {
	t.Helper()
	if chrome := os.Getenv("ROTARI_TEST_CHROME"); chrome != "" {
		return chrome
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if os.Getenv("ROTARI_REQUIRE_BROWSER_TESTS") != "" {
		t.Fatal("no Chrome or Chromium found; set ROTARI_TEST_CHROME")
	}
	t.Skip("no Chrome or Chromium found; set ROTARI_TEST_CHROME")
	return ""
}

// browserCheckTools finds what a browser check runs with: Chrome, a Node with
// WebSocket, and the protocol helper. A check skips without them unless
// ROTARI_REQUIRE_BROWSER_TESTS is set.
func browserCheckTools(t *testing.T) (chrome, node, helper string) {
	t.Helper()
	chrome = browserForTests(t)
	node, err := exec.LookPath("node")
	if err != nil || exec.Command(node, "-e", `if (typeof WebSocket !== "function") process.exit(1)`).Run() != nil {
		if os.Getenv("ROTARI_REQUIRE_BROWSER_TESTS") != "" {
			t.Fatal("the browser check needs Node 22 or later")
		}
		t.Skip("the browser check needs Node 22 or later")
	}
	helper, err = filepath.Abs(filepath.Join("..", "..", "scripts", "web-browser.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	return chrome, node, helper
}

// TestStoredValuesStayTextInBrowser repeats TestStoredValuesStayText in a real
// browser, with the values that attack markup and a quoted argument. It also
// fails on any Content-Security-Policy violation.
func TestStoredValuesStayTextInBrowser(t *testing.T) {
	covers(t, "WEB-28")
	chrome, node, helper := browserCheckTools(t)
	for _, name := range []string{"html", "single quoted"} {
		// One at a time: starting two browsers on a loaded host is slow enough
		// to time out.
		t.Run(name, func(t *testing.T) {
			site := buildHostileSite(t, hostileValues[name])
			renderHostilePagesInBrowser(t, node, chrome, helper, site.live, launchToken)
			renderHostilePagesInBrowser(t, node, chrome, helper, site.static, "")
			renderHostilePagesInBrowser(t, node, chrome, helper, site.viewers, "")
		})
	}
}

// TestPagePolicyBlocksPlantedScript runs what a value that became markup could,
// on each kind of live page. It also shows that the browser check sees violations,
// which is what makes its finding none on the other pages mean something.
func TestPagePolicyBlocksPlantedScript(t *testing.T) {
	covers(t, "WEB-29")
	chrome, node, helper := browserCheckTools(t)
	e := support.NewEnv(t)
	run := e.CreateFinishedRun()
	table := filepath.Join(e.Root, "data.csv")
	if err := os.WriteFile(table, []byte("a,b\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := e.StartWeb()
	pages := []hostilePage{
		{Name: "projects", URL: live + "/", Plant: true},
		{Name: "run", URL: live + "/project/" + url.PathEscape(run.Project) + "/run/" + url.PathEscape(run.RunID), Plant: true},
		{Name: "recent jobs", URL: live + "/jobs/", Plant: true},
		{Name: "Table viewer", URL: e.StartTableView(table), Plant: true},
	}
	renderHostilePagesInBrowser(t, node, chrome, helper, pages, "")
}

func renderHostilePagesInBrowser(t *testing.T, node, chrome, helper string, pages []hostilePage, token string) {
	t.Helper()
	input, err := json.Marshal(map[string]any{"chrome": chrome, "pages": pages, "token": token, "mark": "hostile-mark"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pages.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, "--input-type=module", "-e", hostileBrowserCheck, helper, path).CombinedOutput()
	if err != nil {
		t.Fatalf("browser check: %v\n%s", err, output)
	}
	reportHostileResults(t, output)
}
