package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

const terminalStickyBrowserCheck = `
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
    throw new Error('timed out waiting for ' + what + '; the terminal shows: ' + await page.evaluate('JSON.stringify(' + rows + ')').catch(() => '(not available)') + '; the sticky line: ' + await page.evaluate('JSON.stringify(document.querySelector(".terminal-sticky")?.textContent)').catch(() => '?'));
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
  await typeLine('seq 1001 1200');
  await until('the output', screenHas('1200'));
  const term = 'document.querySelector(".terminal-pane").terminalPane.term';
  const sticky = 'document.querySelector(".terminal-sticky")';
  // Scrolled into the middle of the output: the line says the command.
  await until('the block to end', 'document.querySelector(".terminal-pane").terminalPane.blocks.some((block) => block.ended && block.command() === "seq 1001 1200")');
  await page.evaluate(term + '.scrollToLine(' + term + '.buffer.active.baseY - 100)');
  await until('the sticky line', sticky + ' && !' + sticky + '.hidden && ' + sticky + '.textContent.includes("$ seq 1001 1200")');
  // A click goes to the prompt, where the line goes.
  const point = await page.evaluate('(() => { const box = ' + sticky + '.getBoundingClientRect(); return { x: box.left + 80, y: box.top + box.height / 2 }; })()');
  await page.send('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', clickCount: 1, ...point });
  await page.send('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...point });
  await until('the prompt at the top', 'document.querySelector(".xterm-rows > div")?.textContent.trim().endsWith("seq 1001 1200") && ' + sticky + '.hidden');
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure }));
process.exit(0);
`

// TestTerminalStickyCommandInBrowser checks in a real browser that a command's line stays at
// the top while its output is scrolled through, and that clicking it goes to its prompt.
func TestTerminalStickyCommandInBrowser(t *testing.T) {
	covers(t, "WEB-59")
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
	output, err := exec.Command(node, "--input-type=module", "-e", terminalStickyBrowserCheck, helper, path).CombinedOutput()
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
