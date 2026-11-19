package web

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

const terminalSharedShellsBrowserCheck = `
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, base, token, p1, p2, shared } = JSON.parse(readFileSync(input, 'utf8'));
const { cdp, close } = await launchChrome(chrome);
let failure = '';
await withPage(cdp, base + '/_auth/login?next=/project/p1', { width: 1280, height: 900, settleMs: 200 }, async (page) => {
  const state = 'JSON.stringify({ path: location.pathname, panes: [...document.querySelectorAll(".terminal-pane")].map((pane) => pane.terminalPane?.shown), tabs: [...document.querySelectorAll(".terminal-tabs > *, .terminal-shared > *")].map((item) => item.className + ":" + item.textContent), status: document.querySelector(".terminal-status")?.textContent })';
  const until = async (what, expression) => {
    for (const deadline = Date.now() + 30000; Date.now() < deadline; await sleep(100)) {
      try { if (await page.evaluate(expression)) return; } catch {}
    }
    throw new Error('timed out waiting for ' + what + ': ' + await page.evaluate(state).catch(() => '(not available)'));
  };
  const shownAre = (...ids) => 'JSON.stringify([...document.querySelectorAll(".terminal-pane")].map((pane) => pane.terminalPane?.shown)) === ' + JSON.stringify(JSON.stringify(ids));
  // The tab with the keys is one of the state directory's own, after the divider.
  const sharedFocused = 'document.querySelector(".terminal-shared-tabs [aria-selected=true]") !== null';
  const go = (path) => page.evaluate('location.href = ' + JSON.stringify(path));
  const typeLine = async (line) => {
    await page.send('Input.insertText', { text: line });
    const enter = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13, text: '\r', unmodifiedText: '\r' };
    await page.send('Input.dispatchKeyEvent', { type: 'keyDown', ...enter });
    await page.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
  };
  await until('sign-in', 'document.querySelector("#password") !== null');
  await page.evaluate('document.querySelector("#password").value = ' + JSON.stringify(token) + '; document.querySelector("form").submit()');
  await until('the project page', 'location.pathname === "/project/p1" && document.getElementById("terminal-button") !== null');
  await page.evaluate('document.getElementById("terminal-button").click()');
  await until('the project\'s shell', shownAre(p1) + ' && document.querySelector(".terminal-status")?.hidden');
  // The project's tab and +, then the divider, the state directory's own tab, and its +.
  const barOrder = 'JSON.stringify([...document.querySelectorAll(".terminal-tabs > .terminal-tab, .terminal-add, .terminal-shared > .terminal-tab-divider, .terminal-shared-tabs > .terminal-tab")].map((item) => item.classList.contains("terminal-tab-divider") ? "|" + item.textContent : item.classList.contains("terminal-shared-add") ? "shared+" : item.classList.contains("terminal-add") ? "+" : item.closest(".terminal-shared") ? "shared" : "tab"))';
  await until('the shared tab after the divider', barOrder + ' === ' + JSON.stringify(JSON.stringify(['tab', '+', '|Shared across projects', 'shared', 'shared+'])));
  // Split shows the shared shell, the only one in no pane, in the new pane with the keys.
  await page.evaluate('[...document.querySelectorAll(".terminal-bar button")].find((item) => item.getAttribute("aria-label") === "Split right" || item.textContent === "Split right").click()');
  // Both panes connected before typing: what is typed before a pane connects reaches no shell.
  const connected = '[...document.querySelectorAll(".terminal-status")].every((item) => item.hidden)';
  await until('the shared shell in a pane', shownAre(p1, shared) + ' && ' + sharedFocused + ' && ' + connected);
  await page.evaluate('document.querySelectorAll(".terminal-pane")[1].querySelector(".xterm-helper-textarea").focus()');
  await typeLine('echo "shared-$((2+3))"');
  await until('the shared shell\'s output', '[...document.querySelectorAll(".xterm-rows")].some((rows) => rows.textContent.includes("shared-5"))');

  // Following a link to another project keeps the shared shell in its pane, with the keys.
  await go('/project/p2');
  await until('the other project with the shared shell kept', 'location.pathname === "/project/p2" && ' + shownAre(p2, shared) + ' && ' + sharedFocused);
  await until('the shared shell\'s output again', '[...document.querySelectorAll(".terminal-pane")][1].querySelector(".xterm-rows")?.textContent.includes("shared-5")');
  // And coming back finds the first project's shell beside it.
  await go('/project/p1');
  await until('the first project again', 'location.pathname === "/project/p1" && ' + shownAre(p1, shared) + ' && ' + sharedFocused);
  // The shared + starts another of the state directory's own shells, shown with the keys.
  await page.evaluate('document.querySelector(".terminal-shared-add").click()');
  await until('a new shared shell', 'document.querySelectorAll(".terminal-shared-tabs .terminal-tab").length === 2 && ' + sharedFocused);
  const sharedCount = await page.evaluate('fetch("/_terminal/shells").then((response) => response.json()).then((answer) => answer.shells.filter((shell) => shell.basedir && !shell.project).length)');
  if (sharedCount !== 2) throw new Error('the shared + did not start a shell of the state directory: ' + sharedCount + ' of its own');
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure }));
process.exit(0);
`

// TestTerminalSharedShellsInBrowser checks in a real browser that a project's page shows its
// state directory's own shells after its tabs and that following a link to another project
// keeps a pane showing one.
func TestTerminalSharedShellsInBrowser(t *testing.T) {
	covers(t, "WEB-56")
	chrome, node, helper := browserCheckTools(t)
	base := support.NewEnv(t)
	base.MustRotari("add", "-p", "p1", "--", "true")
	base.MustRotari("add", "-p", "p2", "--", "true")
	e := base.WithVar("ROTARI_WEB_AUTH_TOKEN", launchToken).WithVar("SHELL", "/bin/sh").WithVar("HOME", base.Root)
	live := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	start := func(query string) terminalShell {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodPost, live+"/_terminal/shells?rows=24&cols=80"+query, headers)
		if status != http.StatusCreated {
			t.Fatalf("starting %s: status %d: %s", query, status, body)
		}
		var shell terminalShell
		if err := json.Unmarshal(body, &shell); err != nil {
			t.Fatal(err)
		}
		return shell
	}
	p1, p2, shared := start("&project=p1"), start("&project=p2"), start("&basedir=")
	input, err := json.Marshal(map[string]string{
		"chrome": chrome, "base": live, "token": launchToken,
		"p1": p1.ID, "p2": p2.ID, "shared": shared.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, "--input-type=module", "-e", terminalSharedShellsBrowserCheck, helper, path).CombinedOutput()
	if err != nil {
		t.Fatalf("browser check: %v\n%s", err, output)
	}
	var result struct {
		Failure string `json:"failure"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("browser check output: %v\n%s", err, output)
	}
	if result.Failure != "" {
		t.Error(result.Failure)
	}
}
