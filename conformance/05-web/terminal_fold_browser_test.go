package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

const terminalFoldBrowserCheck = `
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, base, token } = JSON.parse(readFileSync(input, 'utf8'));
const { cdp, close } = await launchChrome(chrome);
let failure = '';
await withPage(cdp, base + '/_auth/login?next=/activity', { width: 1280, height: 900, settleMs: 200 }, async (page) => {
  const rows = '[...document.querySelectorAll(".xterm-rows > div")].map((row) => row.textContent.trimEnd())';
  const until = async (what, expression) => {
    for (const deadline = Date.now() + 30000; Date.now() < deadline; await sleep(100)) {
      try { if (await page.evaluate(expression)) return; } catch {}
    }
    throw new Error('timed out waiting for ' + what + '; the terminal shows: ' + await page.evaluate('JSON.stringify(' + rows + ')').catch(() => '(not available)'));
  };
  const screenHas = (text) => rows + '.some((row) => row === ' + JSON.stringify(text) + ')';
  const typeLine = async (line) => {
    await page.evaluate('document.querySelector(".xterm-helper-textarea").focus()');
    await page.send('Input.insertText', { text: line });
    const enter = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13, text: '\r', unmodifiedText: '\r' };
    await page.send('Input.dispatchKeyEvent', { type: 'keyDown', ...enter });
    await page.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
  };
  await until('sign-in', 'document.querySelector("#password") !== null');
  await page.evaluate('document.querySelector("#password").value = ' + JSON.stringify(token) + '; document.querySelector("form").submit()');
  await until('the page', 'location.pathname === "/activity" && document.getElementById("terminal-button") !== null');
  await page.evaluate('document.getElementById("terminal-button").click()');
  await until('the shell', 'document.querySelector(".terminal-status")?.hidden === true && document.querySelector(".xterm-helper-textarea") !== null');
  // Forty lines, folded from Commands to their first and last 3 and the line between.
  await typeLine('seq 101 140');
  await until('the output', screenHas('140'));
  const commands = '[...document.querySelectorAll(".terminal-bar button")].find((button) => button.getAttribute("aria-label") === "Commands")';
  const foldButton = '[...document.querySelectorAll(".terminal-commands .terminal-dirs-row button")].find((button) => button.textContent === "Fold")';
  // Commands lists what it has when opened, so it is opened again until the block has ended.
  await until('Commands to offer the fold', '(() => { if (document.querySelector(".terminal-commands") && ' + foldButton + ') return true; ' + commands + '.click(); return false; })()');
  await page.evaluate(foldButton + '.click()');
  await until('the folded line', screenHas('⋯ 34 lines folded — click to unfold'));
  await until('the middle to go', '!' + screenHas('104') + ' && ' + screenHas('103') + ' && ' + screenHas('138'));
  // Clicking the folded line brings it back.
  const line = await page.evaluate('(() => { const box = document.querySelector(".terminal-fold").getBoundingClientRect(); return { x: box.left + 40, y: box.top + box.height / 2 }; })()');
  await page.send('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', clickCount: 1, ...line });
  await page.send('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...line });
  // The view stays on its prompt, where the fourth line is back.
  await until('the output to come back', screenHas('104') + ' && !' + screenHas('⋯ 34 lines folded — click to unfold'));
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure }));
process.exit(0);
`

// TestTerminalFoldInBrowser checks in a real browser that a long output folds from Commands
// to its first and last lines, and that clicking the folded line unfolds it.
func TestTerminalFoldInBrowser(t *testing.T) {
	covers(t, "WEB-58")
	chrome, node, helper := browserCheckTools(t)
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	base := support.NewEnv(t)
	e := base.WithVar("ROTARI_WEB_AUTH_TOKEN", launchToken).WithVar("SHELL", bash).WithVar("HOME", base.Root)
	live := e.StartWeb("--allow-terminal")
	input, err := json.Marshal(map[string]string{"chrome": chrome, "base": live, "token": launchToken})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, "--input-type=module", "-e", terminalFoldBrowserCheck, helper, path).CombinedOutput()
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
