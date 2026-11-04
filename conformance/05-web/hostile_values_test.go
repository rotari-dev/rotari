package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
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
  const missing = new Set(page.expect || []);
  const inspect = (when) => {
    const text = (document.body.textContent || '') + [...document.querySelectorAll('input, textarea')].map((node) => node.value).join(' ');
    for (const expected of [...missing]) {
      if (expected.startsWith('selector:') ? document.querySelector(expected.slice(9)) : text.includes(expected)) missing.delete(expected);
    }
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
      await wait(40);
      inspect('after clicking ' + key.slice(0, 120));
    }
    if (!fresh) break;
    await wait(100);
  }
  await wait(200);
  inspect('at the end');
  const result = { name: page.name, shown, clicks, found, missing: [...missing] };
  window.close();
  return result;
}

(async () => {
  const results = [];
  for (const page of pages) {
    try { results.push(await check(page)); }
    catch (error) { results.push({ name: page.name, shown: false, clicks: 0, found: ['failed: ' + (error.stack || error)], missing: [] }); }
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
	// Expect lists text, or a selector after "selector:", that must appear at
	// some point, so that a view that stopped showing a value fails the test.
	Expect []string `json:"expect,omitempty"`
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
	e := support.NewEnv(t)
	// The basedir's own path is a value pages show.
	e.Base = filepath.Join(e.Root, "base-"+hostileValue)
	e.Setenv("ROTARI_BASEDIR", e.Base)

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
	projectDir := filepath.Join(e.Base, "projects", project)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "config.toml"), []byte("# config-"+hostileValue+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The Python exception is a diagnosis, whose evidence is this line.
	script := `printf '%s\n' "$HOSTILE"; printf 'Traceback (most recent call last):\nRuntimeError: diagnosis-%s\n' "$HOSTILE" >&2; exit 1`
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
	add("--job-name", "pending-job", "-e", "slurm", "--executor-option", "option-"+hostileValue)

	// A run its supervisor left behind offers the recovery dialog.
	interrupted := "interrupted-" + hostileValue
	interruptedRun := e.StartRun(interrupted, 1, false)
	support.KillStrays(t, e.Root)
	support.WaitForInterrupted(t, e, interrupted)

	// A launch that has started is listed with its target and output.
	writeLaunchers(t, e, launcherEntry("launcher-"+hostileValue, `printf 'launch-output-%s\n' "$ROTARI_LAUNCHER_ARTIFACT"`, `"file"`, "command"))

	escaped := url.PathEscape(project)
	projectPath := "/project/" + escaped
	live := e.WithVar("ROTARI_WEB_AUTH_TOKEN", launchToken).StartWeb("--allow-exec")
	startHostileLaunch(t, e, live, project, run, hostileValue)
	pages := []hostilePage{}
	for _, page := range []struct {
		name, path string
		search     bool
		expect     []string
	}{
		{"projects", "/", false, []string{"base-hostile-mark", "launcher-hostile-mark"}},
		{"project", projectPath, false, []string{"option-hostile-mark"}},
		{"run", projectPath + "/run/" + url.PathEscape(run), false, []string{"diagnosis-hostile-mark", "config-hostile-mark", "launch-output-"}},
		{"interrupted run", "/project/" + url.PathEscape(interrupted) + "/run/" + url.PathEscape(interruptedRun.RunID), false, []string{"selector:.recover-executions"}},
		{"recent jobs", "/jobs/", false, nil},
		{"activity", "/activity/", false, nil},
		{"history search", "/search/", true, nil},
	} {
		pages = append(pages, hostilePage{Name: "live " + page.name, URL: live + page.path, Search: page.search, Expect: page.expect})
	}
	if results := renderHostilePages(t, node, pages, launchToken); len(results) != len(pages) {
		t.Fatalf("results = %d, want %d", len(results), len(pages))
	}

	export := filepath.Join(e.Root, "export")
	e.MustRotari("web", "--static-dir", export)
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
		viewers = append(viewers, hostilePage{Name: "Table viewer " + name, URL: e.StartTableView(path), Scripts: true})
	}
	renderHostilePages(t, node, viewers, "")
}

// startHostileLaunch runs the registered command on the job's text artifact.
func startHostileLaunch(t *testing.T, e *support.Env, live, project, run, value string) {
	t.Helper()
	jobs := api(e, http.MethodGet, live+"/api/run?"+url.Values{"project_name": {project}, "run_id": {run}}.Encode(), "")
	var detail struct {
		Jobs []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(jobs.Body), &detail); err != nil {
		t.Fatalf("/api/run: %v\n%s", err, jobs.Body)
	}
	for _, job := range detail.Jobs {
		if job.Name != value {
			continue
		}
		query := url.Values{"project_name": {project}, "run_id": {run}, "job_id": {job.ID}}
		listed := api(e, http.MethodGet, live+"/api/artifacts?"+query.Encode(), "")
		var listing struct {
			Entries []struct {
				DisplayPath string `json:"display_path"`
			} `json:"entries"`
		}
		if err := json.Unmarshal([]byte(listed.Body), &listing); err != nil {
			t.Fatalf("/api/artifacts: %v\n%s", err, listed.Body)
		}
		for index, entry := range listing.Entries {
			if !strings.HasSuffix(entry.DisplayPath, ".txt") {
				continue
			}
			body, _ := json.Marshal(map[string]any{"project_name": project, "run_id": run, "job_id": job.ID, "entry": index, "name": "launcher-" + value})
			started := decodeLaunch(t, api(e, http.MethodPost, live+"/api/launch", string(body)))
			waitLaunch(t, e, live, started.ID, func(record launchRecord) bool { return record.State == "exited" || record.ExitCode != nil })
			return
		}
	}
	t.Fatalf("no job %q with a text artifact in %s", value, jobs.Body)
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
		for _, missing := range result.Missing {
			t.Errorf("%s never showed %q, so it checks nothing there", result.Name, missing)
		}
		for _, found := range result.Found {
			t.Errorf("%s: %s", result.Name, found)
		}
	}
	return results
}

type hostileResult struct {
	Name    string   `json:"name"`
	Shown   bool     `json:"shown"`
	Clicks  int      `json:"clicks"`
	Found   []string `json:"found"`
	Missing []string `json:"missing"`
}
