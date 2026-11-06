package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

const terminalPanelBrowserCheck = `
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, base, token } = JSON.parse(readFileSync(input, 'utf8'));

const prepare = ` + "`" + `
  window.__violations = [];
  document.addEventListener('securitypolicyviolation', (event) => {
    window.__violations.push(event.violatedDirective + ' ' + event.blockedURI + ' ' + event.sourceFile + ':' + event.lineNumber);
  });
` + "`" + `;

const { cdp, close } = await launchChrome(chrome);
const violations = new Set();
let failure = '';
await withPage(cdp, base + '/_auth/login?next=/', { width: 1280, height: 900, settleMs: 300, beforeNavigate: (page) => page.send('Page.addScriptToEvaluateOnNewDocument', { source: prepare }) }, async (page) => {
  // An expression can fail while a page is loading, so it is retried.
  const until = async (what, expression) => {
    for (const deadline = Date.now() + 30000; Date.now() < deadline; await sleep(100)) {
      try {
        if (await page.evaluate(expression)) return;
      } catch {}
    }
    const screen = await page.evaluate('document.querySelector(".xterm-rows")?.textContent || ""').catch(() => '');
    throw new Error('timed out waiting for ' + what + '; the terminal shows: ' + screen);
  };
  const collect = async () => {
    (await page.evaluate('window.__violations')).forEach((item) => violations.add(item));
  };
  const typeLine = async (line, pane = 0) => {
    await page.evaluate('document.querySelectorAll(".xterm-helper-textarea")[' + pane + '].focus()');
    await page.send('Input.insertText', { text: line });
    const enter = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13, text: '\r', unmodifiedText: '\r' };
    await page.send('Input.dispatchKeyEvent', { type: 'keyDown', ...enter });
    await page.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
  };
  const screenHas = (text) => 'document.querySelector(".xterm-rows")?.textContent.includes(' + JSON.stringify(text) + ')';
  // The output differs from the typed line, so the echo cannot satisfy this.
  await until('the sign-in form', 'document.querySelector("#password") !== null');
  await page.evaluate('document.querySelector("#password").value = ' + JSON.stringify(token) + '; document.querySelector("form").submit()');
  await until('the application page', 'document.getElementById("terminal-button") !== null');
  if (await page.evaluate('typeof window.Terminal !== "undefined"')) throw new Error('xterm.js loaded before the panel opened');
  await page.evaluate('document.getElementById("terminal-button").click()');
  await until('the terminal', 'document.querySelector(".xterm-helper-textarea") !== null && document.querySelector(".xterm-rows") !== null');
  if (await page.evaluate('typeof window.WebLinksAddon?.WebLinksAddon !== "function"')) throw new Error('the links addon did not load');
  await sleep(300);
  await typeLine('VALUE=kept; echo "one-$((1+1))"');
  await until('the first output', screenHas('one-2'));
  await collect();

  // Following a link keeps the shell and shows its output again.
  await page.evaluate('location.href = "/jobs/"');
  await until('the Recent jobs page', 'location.pathname === "/jobs/" && document.getElementById("terminal-panel") !== null');
  await until('the replayed output', screenHas('one-2'));
  await until('the terminal on the next page', 'document.querySelector(".xterm-helper-textarea") !== null');
  await sleep(300);
  await typeLine('echo "value-$VALUE"');
  await until('the kept shell', screenHas('value-kept'));
  await collect();

  // A second tab is another shell, and the first comes back as it was. The status
  // clears once the shell has shown something, so typing cannot come too soon.
  const settled = 'document.querySelector(".terminal-status")?.hidden === true';
  await page.evaluate('document.querySelector(".terminal-add").click()');
  await until('a second tab', 'document.querySelectorAll(".terminal-tab [role=tab]")[1]?.getAttribute("aria-selected") === "true"');
  await until('the second shell', settled);
  await typeLine('echo "second-[$VALUE]"');
  await until('the second shell to run a command', screenHas('second-[]'));
  await page.evaluate('document.querySelectorAll(".terminal-tab [role=tab]")[0].click()');
  await until('the first tab', 'document.querySelectorAll(".terminal-tab [role=tab]")[0]?.getAttribute("aria-selected") === "true"');
  await until('the first shell replayed', screenHas('value-kept'));
  await until('the first shell', settled);
  await typeLine('echo "again-$VALUE"');
  await until('the first shell to keep its state', screenHas('again-kept'));
  await collect();

  // A project's page has tabs of its own, and starts none by itself.
  await page.evaluate('location.href = "/project/p1"');
  await until('the project page', 'location.pathname === "/project/p1" && document.querySelector(".terminal-action")?.textContent === "Start a shell"');
  if (await page.evaluate('document.querySelectorAll(".terminal-tab").length !== 0')) throw new Error('another page\'s tabs showed on the project page');
  await page.evaluate('document.querySelector(".terminal-action").click()');
  await until('the project\'s shell', 'document.querySelectorAll(".terminal-tab").length === 1 && document.querySelectorAll(".terminal-groups option").length === 2');
  await until('the project\'s shell to settle', settled);
  await collect();

  // A command from the page is typed and waits for Enter.
  const problem = await page.evaluate('window.sendToTerminal(\'echo "sent-$((1+1))"\').then((value) => value || "")', { awaitPromise: true });
  if (problem) throw new Error('sending a command: ' + problem);
  await until('the sent command on screen', screenHas('sent-$((1+1))'));
  await sleep(300);
  if (await page.evaluate(screenHas('sent-2'))) throw new Error('a sent command ran without Enter');
  await typeLine('');
  await until('the sent command to run', screenHas('sent-2'));
  await collect();

  // A dialog fits above the panel, whatever its usual height.
  const fits = await page.evaluate('(() => {' +
    'window.showPath("/p");' +
    'const dialog = document.querySelector(".output-panel");' +
    'dialog.classList.remove("compact");' +
    'const box = dialog.getBoundingClientRect();' +
    'const terminal = document.getElementById("terminal-panel").getBoundingClientRect();' +
    'window.closeOutputModal();' +
    'return box.top >= 0 && box.bottom <= terminal.top;' +
    '})()');
  if (!fits) throw new Error('a dialog does not fit above the panel');
  await collect();

  // The panel lies flat along the bottom of the window, not as a card like the page's sections.
  const flat = await page.evaluate('(() => {' +
    'const panel = document.getElementById("terminal-panel");' +
    'const style = getComputedStyle(panel);' +
    'return { bottom: panel.getBoundingClientRect().bottom, innerHeight, padding: style.padding, radius: style.borderRadius };' +
    '})()');
  if (flat.bottom !== flat.innerHeight || flat.padding !== '0px' || flat.radius !== '0px') throw new Error('the panel is not flat along the bottom: ' + JSON.stringify(flat));

  // Maximized, the panel takes the whole window, over the top bar, and gives it back.
  const covers = '(() => {' +
    'const box = document.getElementById("terminal-panel").getBoundingClientRect();' +
    'const top = document.elementFromPoint(innerWidth / 2, 2);' +
    'return box.top === 0 && box.bottom === innerHeight && top.closest("#terminal-panel") !== null;' +
    '})()';
  if (await page.evaluate(covers)) throw new Error('the panel covered the page before it was maximized');
  await page.evaluate('[...document.querySelectorAll(".terminal-bar button")].find((item) => item.textContent === "Maximize").click()');
  // Says what it found, since nothing but CI runs this.
  const measured = 'JSON.stringify((() => {' +
    'const box = document.getElementById("terminal-panel").getBoundingClientRect();' +
    'const top = document.elementFromPoint(innerWidth / 2, 2);' +
    'return { classes: document.documentElement.className, panelTop: box.top, panelBottom: box.bottom, innerHeight, atTop: top && (top.id || top.className || top.tagName) };' +
    '})())';
  await until('the panel to cover the window', covers).catch(async (error) => {
    throw new Error(error.message + '; measured ' + (await page.evaluate(measured)));
  });
  await page.evaluate('[...document.querySelectorAll(".terminal-bar button")].find((item) => item.textContent === "Restore").click()');
  await until('the panel to give the window back', '!' + covers);
  await collect();

  // Split shows two shells side by side, each typed into and sized on its own.
  const barButton = (text) => '[...document.querySelectorAll(".terminal-bar button")].find((item) => item.textContent === ' + JSON.stringify(text) + ')';
  const paneHas = (index, text) => 'document.querySelectorAll(".terminal-pane")[' + index + ']?.querySelector(".xterm-rows")?.textContent.includes(' + JSON.stringify(text) + ')';
  const bothSettled = '[...document.querySelectorAll(".terminal-status")].every((item) => item.hidden)';
  await page.evaluate(barButton('Split') + '.click()');
  await until('a second pane', 'document.querySelectorAll(".xterm-helper-textarea").length === 2 && document.querySelectorAll(".terminal-tab").length === 2');
  await until('both panes to settle', bothSettled);
  await typeLine('echo "left-$((1+1))"', 0);
  await until('the first pane to run a command', paneHas(0, 'left-2'));
  await typeLine('echo "right-$((2+2))"', 1);
  await until('the second pane to run a command', paneHas(1, 'right-4'));
  if (await page.evaluate(paneHas(0, 'right-4'))) throw new Error('the second pane\'s output reached the first');
  // Measured, since the layout is what this is for. Says what it found, as nothing but CI runs this.
  const layout = 'JSON.stringify((() => {' +
    'const box = document.querySelector(".terminal-panes").getBoundingClientRect();' +
    'const [first, second] = [...document.querySelectorAll(".terminal-pane")].map((pane) => pane.getBoundingClientRect());' +
    'const [left, right] = [...document.querySelectorAll(".terminal-pane .xterm-screen")].map((item) => item.getBoundingClientRect().width);' +
    'return { box: [box.left, box.right, box.width], first: [first.left, first.right, first.width], second: [second.left, second.right, second.width], left, right };' +
    '})())';
  const sides = 'document.querySelectorAll(".terminal-pane .xterm-screen").length === 2 && (() => {' +
    'const box = document.querySelector(".terminal-panes").getBoundingClientRect();' +
    'const [first, second] = [...document.querySelectorAll(".terminal-pane")].map((pane) => pane.getBoundingClientRect());' +
    'const [left, right] = [...document.querySelectorAll(".terminal-pane .xterm-screen")].map((item) => item.getBoundingClientRect().width);' +
    'return first.left === box.left && second.right === box.right && first.right <= second.left && second.left - first.right <= 8 &&' +
    ' Math.abs(first.width - second.width) <= 2 &&' +
    ' left <= first.width && right <= second.width && left > first.width - 60 && right > second.width - 60;' +
    '})()';
  await until('the panes to share the panel', sides).catch(async (error) => {
    throw new Error(error.message + '; measured ' + (await page.evaluate(layout)));
  });
  // The handle moves the share, and each terminal fits it.
  const narrower = await page.evaluate('document.querySelectorAll(".terminal-pane .xterm-screen")[0].getBoundingClientRect().width');
  await page.evaluate('for (let press = 0; press < 3; press++) document.querySelector(".terminal-split-handle").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }))');
  await until('the first terminal to take its larger share', 'document.querySelectorAll(".terminal-pane .xterm-screen")[0].getBoundingClientRect().width > ' + (narrower + 60));
  await collect();

  // Following a link finds both, each with its own shell.
  await page.evaluate('location.href = "/project/p1"');
  await until('both panes on the next page', 'document.querySelectorAll(".xterm-helper-textarea").length === 2');
  await until('the first pane replayed', paneHas(0, 'left-2'));
  await until('the second pane replayed', paneHas(1, 'right-4'));
  await until('both panes to settle again', bothSettled);
  await collect();

  // Closing a pane leaves its shell, with its tab.
  await page.evaluate(barButton('Unsplit') + '.click()');
  await until('one pane again', 'document.querySelectorAll(".terminal-pane").length === 1 && document.querySelectorAll(".xterm-helper-textarea").length === 1');
  if (await page.evaluate('document.querySelectorAll(".terminal-tab").length !== 2')) throw new Error('closing a pane ended its shell');
  await collect();
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure, violations: [...violations] }));
process.exit(0);
`

// TestTerminalPanelInBrowser uses the real xterm.js in Chrome, signed in as a browser
// is: opening the panel, typing, following a link with the shell still there, a second
// tab, and two panes.
func TestTerminalPanelInBrowser(t *testing.T) {
	covers(t, "WEB-33")
	chrome := browserForTests(t)
	node, err := exec.LookPath("node")
	if err != nil || exec.Command(node, "-e", `if (typeof WebSocket !== "function") process.exit(1)`).Run() != nil {
		if os.Getenv("ROTARI_REQUIRE_BROWSER_TESTS") != "" {
			t.Fatal("the browser check needs Node 22 or later")
		}
		t.Skip("the browser check needs Node 22 or later")
	}
	helper, err := filepath.Abs(filepath.Join("..", "..", "scripts", "web-browser.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	base := support.NewEnv(t)
	base.CreateFinishedRun()
	e := base.WithVar("ROTARI_WEB_AUTH_TOKEN", launchToken).WithVar("SHELL", "/bin/sh").WithVar("HOME", base.Root)
	live := e.StartWeb("--allow-terminal")
	input, err := json.Marshal(map[string]string{"chrome": chrome, "base": live, "token": launchToken})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, "--input-type=module", "-e", terminalPanelBrowserCheck, helper, path).CombinedOutput()
	if err != nil {
		t.Fatalf("browser check: %v\n%s", err, output)
	}
	var result struct {
		Failure    string   `json:"failure"`
		Violations []string `json:"violations"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("browser check output: %v\n%s", err, output)
	}
	if result.Failure != "" {
		t.Error(result.Failure)
	}
	for _, violation := range result.Violations {
		t.Errorf("policy violation: %s", violation)
	}
}
