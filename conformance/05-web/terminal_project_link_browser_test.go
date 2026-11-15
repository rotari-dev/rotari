package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

const terminalProjectLinkBrowserCheck = `
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, base, token, runID, p1, p2, p3, mounted, mountedLink, mountedBase, special, specialLink } = JSON.parse(readFileSync(input, 'utf8'));
const { cdp, close } = await launchChrome(chrome);
let failure = '';
const prepare = [
  '(() => {',
  '  if (sessionStorage.getItem("rotari-test-hold-messages") !== "1") return;',
  '  sessionStorage.removeItem("rotari-test-hold-messages");',
  '  const add = window.addEventListener, later = window.setTimeout;',
  '  const held = window.__heldTerminalMessages = [];',
  '  window.addEventListener = function(type, listener, options) {',
  '    if (type === "message") held.push([listener, options]);',
  '    else return add.call(this, type, listener, options);',
  '  };',
  '  window.setTimeout = function(callback, delay, ...args) {',
  '    if (delay === 3000 && !window.__terminalWindowWait) { window.__terminalWindowWait = () => callback(...args); return 0; }',
  '    return later.call(this, callback, delay, ...args);',
  '  };',
  '  window.__releaseTerminalMessages = () => {',
  '    window.addEventListener = add; window.setTimeout = later;',
  '    for (const [listener, options] of held) add.call(window, "message", listener, options);',
  '    window.__terminalWindowWait();',
  '  };',
  '})();',
].join('\n');
await withPage(cdp, base + '/_auth/login?next=/project/p1', { width: 1280, height: 900, settleMs: 200, beforeNavigate: (page) => page.send('Page.addScriptToEvaluateOnNewDocument', { source: prepare }) }, async (page) => {
  const until = async (target, what, expression) => {
    for (const deadline = Date.now() + 30000; Date.now() < deadline; await sleep(100)) {
      try { if (await target.evaluate(expression)) return; } catch {}
    }
    const state = await target.evaluate('JSON.stringify({ path: location.pathname, linked: document.querySelector(".terminal-project-link")?.getAttribute("aria-pressed"), shown: document.querySelector(".terminal-pane")?.terminalPane?.shown, groups: [...document.querySelectorAll(".terminal-groups option")].map((item) => [item.value, item.textContent]), status: document.querySelector(".terminal-status")?.textContent, storedLink: sessionStorage.getItem("rotari-terminal-project-link"), storedPath: sessionStorage.getItem("rotari-terminal-project-path"), windowed: sessionStorage.getItem("rotari-terminal-windowed") })').catch(() => '(not available)');
    throw new Error('timed out waiting for ' + what + ': ' + state);
  };
  const check = (condition, message) => { if (!condition) throw new Error(message); };
  const shown = (id) => 'document.querySelector(".terminal-pane")?.terminalPane?.shown === ' + JSON.stringify(id);
  const pressed = (value) => 'document.querySelector(".terminal-project-link")?.getAttribute("aria-pressed") === ' + JSON.stringify(value);
  const barButton = (label) => '[...document.querySelectorAll(".terminal-bar button")].find((item) => item.getAttribute("aria-label") === ' + JSON.stringify(label) + ')';
  const chooseGroup = async (target, path) => {
    await until(target, 'terminal group ' + path, '[...document.querySelectorAll(".terminal-groups option")].some((option) => option.value === ' + JSON.stringify(path) + ')');
    await target.evaluate('(() => { const select = document.querySelector(".terminal-groups"); select.value = ' + JSON.stringify(path) + '; select.dispatchEvent(new Event("change", { bubbles: true })); })()');
  };
  const go = (path) => page.evaluate('location.href = ' + JSON.stringify(path));
  const typeLine = async (line) => {
    await page.evaluate('document.querySelector(".xterm-helper-textarea").focus()');
    await page.send('Input.insertText', { text: line });
    const enter = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13, text: '\r', unmodifiedText: '\r' };
    await page.send('Input.dispatchKeyEvent', { type: 'keyDown', ...enter });
    await page.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
  };
  await until(page, 'sign-in', 'document.querySelector("#password") !== null');
  await page.evaluate('document.querySelector("#password").value = ' + JSON.stringify(token) + '; document.querySelector("form").submit()');
  await until(page, 'the project page', 'location.pathname === "/project/p1" && document.getElementById("terminal-button") !== null');
  await page.evaluate('document.getElementById("terminal-button").click()');
  await until(page, 'the first project shell', shown(p1) + ' && document.querySelector(".terminal-status")?.hidden');
  await until(page, 'the default link', pressed('true'));
  await typeLine('MARK=kept; echo "link-$((1+1))"');
  await until(page, 'the shell output', 'document.querySelector(".xterm-rows")?.textContent.includes("link-2")');

  // The default still takes the page to the chosen terminal's project.
  await chooseGroup(page, '/project/p2');
  await until(page, 'the linked project and shell', 'location.pathname === "/project/p2" && ' + shown(p2));
  await go('/project/p1');
  await until(page, 'the linked shell again', 'location.pathname === "/project/p1" && ' + shown(p1));
  await page.evaluate('document.querySelector(".terminal-project-link").click()');
  await until(page, 'unlinking', pressed('false'));
  await go('/project/p3');
  await until(page, 'the same shell on another project', 'location.pathname === "/project/p3" && ' + shown(p1) + ' && ' + pressed('false'));
  await until(page, 'the same shell output', 'document.querySelector(".xterm-rows")?.textContent.includes("link-2")');

  // A group choice, +, and an All shells choice affect the terminal, not the page.
  await chooseGroup(page, '/project/p2');
  await until(page, 'the independently chosen shell', shown(p2) + ' && document.querySelector(".terminal-status")?.hidden');
  check(await page.evaluate('location.pathname === "/project/p3"'), 'choosing a terminal moved the page');
  await page.evaluate('document.querySelector(".terminal-add").click()');
  await until(page, 'a new shell in the selected terminal group', 'document.querySelectorAll(".terminal-tab [role=tab]").length === 2 && !(' + shown(p2) + ') && document.querySelector(".terminal-status")?.hidden');
  const added = await page.evaluate('(() => { const id = document.querySelector(".terminal-pane").terminalPane.shown; return fetch("/_terminal/shells").then((response) => response.json()).then((answer) => answer.shells.find((shell) => shell.id === id)); })()');
  check(added?.project === 'p2', '+ started a shell in the page project');
  await page.evaluate('window.sendToTerminalAndSay("echo linked-check")');
  check(await page.evaluate('document.querySelector(".terminal-send-note")?.textContent.includes("terminal, in p2.")'), 'sending to another project did not name it');
  await page.evaluate(barButton('All shells') + '.click()');
  const firstCard = 'document.querySelector(' + JSON.stringify('.terminal-overview-card[data-shell="' + p1 + '"]') + ')';
  await until(page, 'the overview', firstCard + ' !== null');
  await page.evaluate(firstCard + '.click()');
  await until(page, 'the overview choice', shown(p1));
  check(await page.evaluate('location.pathname === "/project/p3"'), 'an overview choice moved the page');

  // Projects with the same name in another basedir remain distinct, including +.
  await chooseGroup(page, mountedLink);
  await until(page, 'the mounted project shell', shown(mounted) + ' && document.querySelector(".terminal-status")?.hidden');
  await go('/jobs/');
  await until(page, 'the mounted shell on Recent jobs', 'location.pathname === "/jobs/" && ' + shown(mounted));
  await page.evaluate('document.querySelector(".terminal-add").click()');
  await until(page, 'a new mounted shell', 'document.querySelectorAll(".terminal-tab [role=tab]").length === 2 && !(' + shown(mounted) + ')');
  const mountedAdded = await page.evaluate('(() => { const id = document.querySelector(".terminal-pane").terminalPane.shown; return fetch("/_terminal/shells").then((response) => response.json()).then((answer) => answer.shells.find((shell) => shell.id === id)); })()');
  check(mountedAdded?.project === 'p1' && mountedAdded?.basedir === mountedBase, '+ lost the selected basedir');
  await go('/project/p3');
  await until(page, 'the mounted shell after navigation', 'location.pathname === "/project/p3" && ' + pressed('false'));
  await chooseGroup(page, '/project/p2');
  await until(page, 'another independent group', 'document.querySelector(".terminal-groups option")?.textContent.startsWith("p2")');
  await page.evaluate('document.querySelector(".terminal-project-link").click()');
  await until(page, 'relinking to the current page', pressed('true') + ' && ' + shown(p3));
  check(await page.evaluate('location.pathname === "/project/p3"'), 'relinking moved the page');

  // The window inherits the unlinked group, neither direction follows until relinked,
  // and Back to page returns the window's selected group.
  await page.evaluate('document.querySelector(".terminal-project-link").click()');
  await chooseGroup(page, '/project/p1');
  await until(page, 'the first shell before popping out', shown(p1));
  await page.evaluate(barButton('Panel position') + '.click()');
  // Earlier keyboard input must not accidentally authorize this synthetic click.
  await until(page, 'the earlier user activation to expire', '!navigator.userActivation.isActive');
  await page.send('Runtime.evaluate', { expression: '[...document.querySelectorAll(".terminal-tab-menu button")].find((item) => item.textContent === "Window").click()', userGesture: true });
  let target;
  for (const deadline = Date.now() + 30000; Date.now() < deadline && !target; await sleep(100)) {
    const { targetInfos } = await cdp.send('Target.getTargets');
    target = targetInfos.find((item) => item.url.startsWith(base + '/_terminal/window/'));
  }
  check(target, 'the terminal window did not open');
  const { sessionId } = await cdp.send('Target.attachToTarget', { targetId: target.targetId, flatten: true });
  const win = {
    async evaluate(expression) {
      const result = await cdp.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }, sessionId);
      if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text);
      return result.result.value;
    },
  };
  await until(win, 'the unlinked window', shown(p1) + ' && ' + pressed('false'));
  await go('/project/p2');
  await until(page, 'another page while the window is open', 'location.pathname === "/project/p2"');
  await sleep(1300);
  check(await win.evaluate(shown(p1)), 'the page moved the unlinked window');
  await go('/project/p3');
  await until(page, 'the third page while the window is open', 'location.pathname === "/project/p3"');
  await sleep(1300);
  await chooseGroup(win, '/_terminal/window/project/p2');
  await until(win, 'the independently chosen window group', shown(added.id));
  await sleep(300);
  check(await page.evaluate('location.pathname === "/project/p3"'), 'the unlinked window moved the page');

  // Another terminal window can have the same opener and a cloned sessionStorage,
  // but must not overwrite the state of the window this tab actually opened.
  const savedContext = await page.evaluate('JSON.stringify([sessionStorage.getItem("rotari-terminal-project-link"), sessionStorage.getItem("rotari-terminal-project-path")])');
  const otherHandover = encodeURIComponent(JSON.stringify({ 'rotari-terminal-project-link': '1', 'rotari-terminal-project-path': '/project/p3' }));
  const openedOther = await page.send('Runtime.evaluate', { expression: 'window.__otherTerminal = window.open(' + JSON.stringify('/_terminal/window/project/p3#' + otherHandover) + ', "_blank", "popup,width=900,height=600"); Boolean(window.__otherTerminal)', returnByValue: true, userGesture: true });
  check(openedOther.result?.value === true, 'the independent terminal window did not open');
  let otherTarget;
  for (const deadline = Date.now() + 30000; Date.now() < deadline && !otherTarget; await sleep(100)) {
    const { targetInfos } = await cdp.send('Target.getTargets');
    otherTarget = targetInfos.find((item) => item.targetId !== target.targetId && item.url.startsWith(base + '/_terminal/window/project/p3'));
  }
  check(otherTarget, 'the independent terminal target was not found');
  const { sessionId: otherSession } = await cdp.send('Target.attachToTarget', { targetId: otherTarget.targetId, flatten: true });
  const other = {
    async evaluate(expression) {
      const result = await cdp.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }, otherSession);
      if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text);
      return result.result.value;
    },
  };
  await until(other, 'the independent terminal', shown(p3) + ' && ' + pressed('true'));
  await other.evaluate('document.querySelector(".terminal-project-link").click()');
  await chooseGroup(other, '/_terminal/window/project/p1');
  await until(other, 'the independent window group', shown(p1));
  check(await page.evaluate('JSON.stringify([sessionStorage.getItem("rotari-terminal-project-link"), sessionStorage.getItem("rotari-terminal-project-path")])') === savedContext, 'an independent window changed this tab\'s terminal state');
  await page.evaluate('window.__otherTerminal.close(); true');

  // An explicit run link moves the page without moving an unlinked window. The
  // run's terminal button then explicitly starts a shell in that run's project.
  await win.evaluate(barButton('All shells') + '.click()');
  const runButton = firstCard + '?.closest(".terminal-overview-group")?.querySelector(".terminal-overview-project-state-run")';
  await until(win, 'the run link in the window', runButton + ' !== null && ' + runButton + ' !== undefined');
  await win.evaluate(runButton + '.click()');
  await until(page, 'the explicitly opened run', 'location.pathname === "/project/p1/run/' + runID + '"');
  await sleep(1300);
  check(await win.evaluate(shown(added.id)), 'opening a run moved the unlinked window');
  const hereButton = 'document.querySelector(' + JSON.stringify('[data-action="openTerminalHere"]') + ')';
  await until(page, 'the run terminal button', hereButton + ' !== null');
  await page.evaluate(hereButton + '.click()');
  await until(win, 'a new shell at the run in the window', 'document.querySelector(".terminal-pane")?.terminalPane?.shown && !(' + shown(p1) + ') && !(' + shown(added.id) + ')');
  const windowRunShell = await win.evaluate('(() => { const id = document.querySelector(".terminal-pane").terminalPane.shown; return fetch("/_terminal/shells").then((response) => response.json()).then((answer) => answer.shells.find((shell) => shell.id === id)); })()');
  check(windowRunShell?.project === 'p1', 'Open a terminal here started the window shell in the wrong project');
  await go('/project/p3');
  await until(page, 'the page after opening the run shell', 'location.pathname === "/project/p3"');
  await sleep(1300);
  check(await win.evaluate(shown(windowRunShell.id)), 'the page moved the new unlinked window shell');

  await win.evaluate('document.querySelector(".terminal-project-link").click()');
  await until(win, 'the relinked window following the page', pressed('true') + ' && ' + shown(p3));
  // Hold the new page's message listeners while the window changes and closes:
  // state must survive even if every postMessage during that reload is lost.
  await page.evaluate('sessionStorage.setItem("rotari-test-hold-messages", "1")');
  await chooseGroup(win, '/_terminal/window/project/p1');
  await until(page, 'the page following the relinked window', 'location.pathname === "/project/p1" && window.__heldTerminalMessages?.length >= 2 && typeof window.__terminalWindowWait === "function"');
  await until(win, 'the relinked window group', shown(windowRunShell.id));
  await win.evaluate('document.querySelector(".terminal-project-link").click()');
  await chooseGroup(win, '/_terminal/window/project/p2');
  await until(win, 'the window unlinked again', pressed('false') + ' && ' + shown(added.id));
  await win.evaluate(barButton('Split right') + '.click()');
  await until(win, 'the split in the unlinked window', 'document.querySelectorAll(".terminal-pane").length === 2 && [...document.querySelectorAll(".terminal-pane")].some((pane) => pane.terminalPane.shown === ' + JSON.stringify(p2) + ')');
  await until(page, 'the window state handed to the reloading page', 'sessionStorage.getItem("rotari-terminal-project-link") === "0" && sessionStorage.getItem("rotari-terminal-project-path") === "/project/p2"');
  await win.evaluate(barButton('Panel position') + '.click()');
  await win.evaluate('[...document.querySelectorAll(".terminal-tab-menu button")].find((item) => item.textContent === "Back to page").click()');
  await page.evaluate('window.__releaseTerminalMessages()');
  await until(page, 'the returned unlinked panel', 'location.pathname === "/project/p1" && ' + pressed('false') + ' && ' + shown(added.id) + ' && document.querySelectorAll(".terminal-pane").length === 2');

  // A run's explicit button still starts in that run's project.
  await go('/project/p1/run/' + runID);
  await until(page, 'the run and retained shell', 'location.pathname === "/project/p1/run/' + runID + '" && ' + shown(added.id));
  await until(page, 'the run terminal button', hereButton + ' !== null');
  await page.evaluate(hereButton + '.click()');
  await until(page, 'a new shell at the run', 'document.querySelector(".terminal-pane")?.terminalPane?.shown && !(' + shown(windowRunShell.id) + ') && !(' + shown(added.id) + ')');
  const runShell = await page.evaluate('(() => { const id = document.querySelector(".terminal-pane").terminalPane.shown; return fetch("/_terminal/shells").then((response) => response.json()).then((answer) => answer.shells.find((shell) => shell.id === id)); })()');
  check(runShell?.project === 'p1', 'Open a terminal here started in the wrong project');
  check(await page.evaluate('location.pathname === "/project/p1/run/' + runID + '"'), 'Open a terminal here moved the page');

  // The page and the server may encode the same project name differently.
  await go('/project/x%2By');
  await until(page, 'the differently encoded page', 'location.pathname === "/project/x%2By" && ' + pressed('false'));
  await chooseGroup(page, specialLink);
  await until(page, 'the differently encoded shell', shown(special) + ' && document.querySelector(".terminal-status")?.hidden');
  await page.evaluate('window.sendToTerminalAndSay("echo same")');
  check(await page.evaluate('document.querySelector(".terminal-send-note")?.textContent.startsWith("Sent. Nothing runs")'), 'equivalent paths were said to be different projects');
  await page.evaluate('window.__linkTestSocket = document.querySelector(".terminal-pane").terminalPane.socket; document.querySelector(".terminal-project-link").click()');
  await until(page, 'the same project relinked', pressed('true'));
  check(await page.evaluate('window.__linkTestSocket === document.querySelector(".terminal-pane").terminalPane.socket'), 'relinking equivalent paths reconnected the shell');
  await go(specialLink);
  await until(page, 'the raw project link', 'location.pathname === ' + JSON.stringify(specialLink) + ' && ' + shown(special));
  await page.evaluate('document.querySelector(".terminal-add").click()');
  await until(page, 'a new shell on the raw project link', 'document.querySelectorAll(".terminal-tab [role=tab]").length === 2 && !(' + shown(special) + ')');
  const specialAdded = await page.evaluate('(() => { const id = document.querySelector(".terminal-pane").terminalPane.shown; return fetch("/_terminal/shells").then((response) => response.json()).then((answer) => answer.shells.find((shell) => shell.id === id)); })()');
  check(specialAdded?.project === 'x+y', '+ on the server link changed the project name');
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure }));
process.exit(0);
`

func TestTerminalProjectLinkInBrowser(t *testing.T) {
	covers(t, "WEB-54")
	chrome, node, helper := browserCheckTools(t)
	base := support.NewEnv(t)
	run := base.CreateFinishedRun()
	base.MustRotari("add", "-p", "p2", "--", "true")
	base.MustRotari("add", "-p", "p3", "--", "true")
	base.MustRotari("add", "-p", "x+y", "--", "true")
	mountedBase := filepath.Join(base.Root, "another-base")
	base.MustRotari("add", "-b", mountedBase, "-p", "p1", "--", "true")
	mountedID := base64.RawURLEncoding.EncodeToString([]byte(mountedBase))
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
	p1, p2, p3 := start("&project=p1"), start("&project=p2"), start("&project=p3")
	mounted := start("&basedir=" + mountedID + "&project=p1")
	special := start("&project=x%2By")
	input, err := json.Marshal(map[string]string{
		"chrome": chrome, "base": live, "token": launchToken, "runID": run.RunID,
		"p1": p1.ID, "p2": p2.ID, "p3": p3.ID,
		"mounted": mounted.ID, "mountedLink": mounted.Link, "mountedBase": mountedBase,
		"special": special.ID, "specialLink": special.Link,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, "--input-type=module", "-e", terminalProjectLinkBrowserCheck, helper, path).CombinedOutput()
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
