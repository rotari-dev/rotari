package web

import (
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// hostileValues each attack one context. One value cannot attack them all: a
// double quote meant for HTML is a syntax error inside a JavaScript argument, so
// the handler would never run. None has a slash, so a project and a file can have
// it as their name, or a comma, so a matrix value can.
var hostileValues = map[string]string{
	"html":          "hostile-mark'\"><i data-injected=1><!--&amp;",
	"single quoted": "hostile-mark');window.__injected=1;('",
	"double quoted": "hostile-mark\");window.__injected=1;(\"",
	"template":      "hostile-mark`+(window.__injected=1)+`${window.__injected=1}",
}

// hostilePageCheck renders one page, clicks each control it finds, and lists
// what ran or became markup.
const hostilePageCheck = `
const { JSDOM, VirtualConsole } = require('jsdom');
const fs = require('fs');
const { pages, token, mark } = JSON.parse(fs.readFileSync(process.argv[1], 'utf8'));
// A click can start work that jsdom cannot finish; only the findings matter.
process.on('unhandledRejection', () => {});
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const signature = (node) => [node.tagName, node.className, node.getAttribute('onclick') || '', (node.textContent || '').slice(0, 60)].join('|');
const describe = (node) => node.outerHTML.slice(0, 300);

async function open(page) {
  const virtualConsole = new VirtualConsole();
  const beforeParse = (window) => {
    Object.assign(window, { Response, Request, Headers });
    window.Element.prototype.scrollIntoView = () => {};
    window.setInterval = () => 1;
    window.confirm = () => false;
    window.prompt = () => null;
    window.alert = () => {};
    window.open = () => null;
    window.Notification = undefined;
    window.fetch = (input, init = {}) => {
      if ((init.method || 'GET').toUpperCase() !== 'GET')
        return Promise.resolve(new Response('{"error":"blocked by the test"}', { status: 403 }));
      const headers = { ...init.headers };
      if (token) headers['X-Rotari-Token'] = token;
      return fetch(new URL(String(input), page.url), { ...init, headers });
    };
  };
  const options = { runScripts: 'dangerously', url: page.url, virtualConsole, beforeParse };
  if (page.scripts) options.resources = 'usable';
  if (page.file) return new JSDOM(fs.readFileSync(page.file, 'utf8'), options);
  const headers = token ? { 'X-Rotari-Token': token } : {};
  const response = await fetch(page.url, { headers });
  if (!response.ok) throw new Error('GET ' + page.url + ': ' + response.status);
  return new JSDOM(await response.text(), options);
}

async function search(document, window) {
  const basedir = document.querySelector('.history-search-scope-basedir');
  if (!basedir) return;
  basedir.selectedIndex = 1;
  basedir.dispatchEvent(new window.Event('change', { bubbles: true }));
  await wait(200);
  const word = document.querySelector('.history-search-word');
  if (word) word.value = 'hostile-mark';
  document.querySelector('form').dispatchEvent(new window.Event('submit', { bubbles: true, cancelable: true }));
  await wait(400);
}

async function check(page) {
  const dom = await open(page);
  const window = dom.window, document = window.document;
  const found = [];
  const inspect = (when) => {
    if (window.__injected !== undefined && !found.some((item) => item.startsWith('ran')))
      found.push('ran as code ' + when);
    document.querySelectorAll('[data-injected]').forEach((node) => {
      if (!found.some((item) => item.startsWith('markup')))
        found.push('markup ' + when + ': ' + describe(node.parentElement || node));
    });
    document.querySelectorAll('a[href]').forEach((node) => {
      if (/^\s*(javascript|data|vbscript):/i.test(node.getAttribute('href')))
        found.push('link ' + when + ': ' + describe(node));
    });
  };
  await wait(400);
  inspect('on load');
  if (page.search) { await search(document, window); inspect('after a search'); }
  const shown = (document.body.textContent || '').includes(mark);
  const seen = new Set();
  let clicks = 0;
  for (let round = 0; round < 4 && clicks < 500; round++) {
    const controls = [...document.querySelectorAll('button, summary, [onclick], [role="button"]')];
    let fresh = 0;
    for (const control of controls) {
      if (control.disabled || !control.isConnected) continue;
      const key = signature(control);
      if (seen.has(key) || clicks >= 500) continue;
      seen.add(key);
      fresh++;
      clicks++;
      try { control.click(); } catch {}
      await wait(10);
      inspect('after clicking ' + key.slice(0, 120));
    }
    if (!fresh) break;
    await wait(100);
  }
  await wait(200);
  inspect('at the end');
  const result = { name: page.name, shown, clicks, found };
  window.close();
  return result;
}

(async () => {
  const results = [];
  for (const page of pages) {
    try { results.push(await check(page)); }
    catch (error) { results.push({ name: page.name, shown: false, clicks: 0, found: ['failed: ' + (error.stack || error)] }); }
  }
  console.log(JSON.stringify(results));
  process.exit(0);
})();
`

type hostilePage struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	File   string `json:"file,omitempty"`
	Search bool   `json:"search,omitempty"`
	// Scripts loads the page's script files, which only the Table viewer has.
	Scripts bool `json:"scripts,omitempty"`
}

// TestStoredValuesStayText records hostile values through the CLI wherever a
// page can show them, then renders every page and control of the live server,
// a static export, and the Table viewer. None may become markup or run.
func TestStoredValuesStayText(t *testing.T) {
	covers(t, "WEB-28")
	node, err := exec.LookPath("node")
	if err != nil || exec.Command(node, "-e", `new Function("return null?.value")`).Run() != nil {
		t.Skip("node with current syntax is not installed")
	}
	for name, value := range hostileValues {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkStoredValueStaysText(t, node, value)
		})
	}
}

func checkStoredValueStaysText(t *testing.T, node, hostileValue string) {
	t.Helper()
	base := support.NewEnv(t)
	e := base.WithVar("ROTARI_WEB_AUTH_TOKEN", launchToken)

	work := filepath.Join(e.Root, hostileValue)
	if err := os.MkdirAll(filepath.Join(work, hostileValue), 0o755); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(work, hostileValue+".csv")
	document := filepath.Join(work, hostileValue+".md")
	for path, contents := range map[string]string{
		table:                                    "name,\"" + strings.ReplaceAll(hostileValue, `"`, `""`) + "\"\n\"" + strings.ReplaceAll(hostileValue, `"`, `""`) + "\",1\n",
		document:                                 "# " + hostileValue + "\n\n" + hostileValue + "\n\n[link](javascript:window.__injected=1)\n\n`" + hostileValue + "`\n",
		filepath.Join(work, hostileValue+".txt"): hostileValue + "\n",
		filepath.Join(work, hostileValue+".py"):  "# " + hostileValue + "\nprint('" + "x" + "')\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	project := hostileValue
	script := `printf '%s\n' "$HOSTILE"; printf '%s\n' "$HOSTILE" >&2; exit 1`
	common := []string{"-p", project, "--working-directory", work, "--env", "HOSTILE=" + hostileValue}
	add := func(extra ...string) {
		t.Helper()
		args := append([]string{"add"}, common...)
		e.MustRotari(append(append(args, extra...), "--", "sh", "-c", script, hostileValue)...)
	}
	add("--job-name", hostileValue, "--stage", hostileValue+"-stage", "--retry", "1",
		"--artifact", table, "--artifact", document, "--artifact", filepath.Join(work, hostileValue),
		"--artifact", filepath.Join(work, hostileValue+".txt"))
	add("--job-name", "matrix-job", "--matrix", "param="+hostileValue+",plain")
	add("--job-name", "array-job", "--array", "1-2")
	e.Rotari("run", "-p", project, "--quiet")
	note := e.Rotari("runs", "-p", project, "--json")
	run := hostileRunID(t, note.Stdout)
	e.MustRotari("note", "-p", project, run, hostileValue+"\n\n[link](javascript:window.__injected=1)")
	e.Rotari("retry", "-p", project, "--quiet")
	add("--job-name", "pending-job")

	escaped := url.PathEscape(project)
	projectPath := "/project/" + escaped
	live := e.StartWeb("--allow-exec")
	pages := []hostilePage{}
	for _, page := range []struct {
		name, path string
		search     bool
	}{
		{"projects", "/", false},
		{"project", projectPath, false},
		{"run", projectPath + "/run/" + url.PathEscape(run), false},
		{"recent jobs", "/jobs/", false},
		{"activity", "/activity/", false},
		{"history search", "/search/", true},
	} {
		pages = append(pages, hostilePage{Name: "live " + page.name, URL: live + page.path, Search: page.search})
	}
	if results := renderHostilePages(t, node, pages, launchToken); len(results) != len(pages) {
		t.Fatalf("results = %d, want %d", len(results), len(pages))
	}

	export := filepath.Join(e.Root, "export")
	base.MustRotari("web", "--static-dir", export)
	static := []hostilePage{}
	if err := filepath.WalkDir(export, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr == nil && entry.Name() == "index.html" {
			rel, _ := filepath.Rel(export, path)
			route := strings.TrimSuffix(filepath.ToSlash(rel), "index.html")
			static = append(static, hostilePage{Name: "static " + rel, URL: "http://localhost/" + route, File: path})
		}
		return walkErr
	}); err != nil {
		t.Fatal(err)
	}
	renderHostilePages(t, node, static, "")

	viewers := []hostilePage{}
	for name, path := range map[string]string{"table": table, "Markdown": document} {
		viewers = append(viewers, hostilePage{Name: "Table viewer " + name, URL: base.StartTableView(path), Scripts: true})
	}
	renderHostilePages(t, node, viewers, "")
}

func hostileRunID(t *testing.T, listing string) string {
	t.Helper()
	var runs []struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(listing), &runs); err != nil || len(runs) == 0 {
		t.Fatalf("rotari runs --json: %v\n%s", err, listing)
	}
	return runs[0].RunID
}

func renderHostilePages(t *testing.T, node string, pages []hostilePage, token string) []hostileResult {
	t.Helper()
	input, err := json.Marshal(map[string]any{"pages": pages, "token": token, "mark": "hostile-mark"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pages.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, "-e", hostilePageCheck, path).CombinedOutput()
	if err != nil {
		t.Fatalf("page check: %v\n%s", err, output)
	}
	var results []hostileResult
	if err := json.Unmarshal(output, &results); err != nil {
		t.Fatalf("page check output: %v\n%s", err, output)
	}
	for _, result := range results {
		t.Logf("%s: shown=%t clicks=%d", result.Name, result.Shown, result.Clicks)
		if !result.Shown {
			t.Errorf("%s does not show the value, so it checks nothing", result.Name)
		}
		for _, found := range result.Found {
			t.Errorf("%s: %s", result.Name, found)
		}
	}
	return results
}

type hostileResult struct {
	Name   string   `json:"name"`
	Shown  bool     `json:"shown"`
	Clicks int      `json:"clicks"`
	Found  []string `json:"found"`
}
