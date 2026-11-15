package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

const terminalCommandDirsBrowserCheck = `
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
    throw new Error('timed out waiting for ' + what + '; the terminal shows: ' + await page.evaluate('JSON.stringify(' + rows + ')').catch(() => '(not available)') + '; the title is ' + await page.evaluate('JSON.stringify(document.querySelector(".terminal-viewport")?.title)').catch(() => '?'));
  };
  const screenHas = (text) => rows + '.some((row) => row.includes(' + JSON.stringify(text) + '))';
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
  // A relative path printed in one directory, then a cd elsewhere: the path still links
  // to the file where it was printed, from the directory bash said at that prompt (OSC 7).
  // The echo is a command line of its own: a block takes the directory its prompt said, so
  // a cd earlier in the same line would not count.
  await typeLine('mkdir -p "$HOME/ran here/sub" && echo y > "$HOME/ran here/sub/y.txt" && cd "$HOME/ran here" && echo "in-$((1+2))"');
  await until('the first cd', screenHas('in-3'));
  await typeLine('echo "sub/""y.txt"');
  const printed = '[...document.querySelectorAll(".xterm-rows > div")].find((row) => row.textContent.trim() === "sub/y.txt")';
  await until('the printed path', printed + ' !== undefined');
  await typeLine('cd / && echo "moved-$((1+1))"');
  await until('the cd', screenHas('moved-2'));
  const point = await page.evaluate('(() => { const box = ' + printed + '.getBoundingClientRect(); return { x: box.left + 20, y: box.top + box.height / 2 }; })()');
  await page.send('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point });
  await until('the path to link where it was printed', 'document.querySelector(".terminal-viewport")?.title.startsWith("Open ") && document.querySelector(".terminal-viewport").title.endsWith("/ran here/sub/y.txt in the Table viewer; right-click for more")');
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure }));
process.exit(0);
`

// TestTerminalCommandDirsInBrowser checks in a real browser that a relative path a command
// printed links where that command ran after the shell has moved, from the directory
// rotari's bash says at each prompt.
func TestTerminalCommandDirsInBrowser(t *testing.T) {
	covers(t, "WEB-46")
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
	output, err := exec.Command(node, "--input-type=module", "-e", terminalCommandDirsBrowserCheck, helper, path).CombinedOutput()
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
