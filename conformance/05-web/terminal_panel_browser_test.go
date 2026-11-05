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
  const typeLine = async (line) => {
    await page.evaluate('document.querySelector(".xterm-helper-textarea").focus()');
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
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure, violations: [...violations] }));
process.exit(0);
`

// TestTerminalPanelInBrowser uses the real xterm.js in Chrome, signed in as a browser
// is: opening the panel, typing, and following a link with the shell still there.
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
