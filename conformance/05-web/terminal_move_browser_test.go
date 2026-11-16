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

const terminalMoveBrowserCheck = `
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, base, token, moving } = JSON.parse(readFileSync(input, 'utf8'));
const { cdp, close } = await launchChrome(chrome);
let failure = '';
await withPage(cdp, base + '/_auth/login?next=/project/p1', { width: 1280, height: 900, settleMs: 200 }, async (page) => {
  const until = async (what, expression) => {
    for (const deadline = Date.now() + 30000; Date.now() < deadline; await sleep(100)) {
      try { if (await page.evaluate(expression)) return; } catch {}
    }
    throw new Error('timed out waiting for ' + what + '; the overview says: ' + await page.evaluate('JSON.stringify(document.querySelector(".terminal-overview")?.textContent)').catch(() => '?') + '; the status: ' + await page.evaluate('JSON.stringify(document.querySelector(".terminal-status")?.textContent)').catch(() => '?'));
  };
  await until('sign-in', 'document.querySelector("#password") !== null');
  await page.evaluate('document.querySelector("#password").value = ' + JSON.stringify(token) + '; document.querySelector("form").submit()');
  await until('the page', 'location.pathname === "/project/p1" && document.getElementById("terminal-button") !== null');
  await page.evaluate('document.getElementById("terminal-button").click()');
  await until('p1\'s shell', 'document.querySelector(".terminal-status")?.hidden === true && document.querySelector(".xterm-helper-textarea") !== null');
  await page.evaluate('[...document.querySelectorAll(".terminal-bar button")].find((button) => button.getAttribute("aria-label") === "All shells").click()');
  const item = '.terminal-overview-card[data-shell="' + moving + '"]';
  await until('the cards', 'document.querySelector(' + JSON.stringify(item) + ') !== null && [...document.querySelectorAll(".terminal-overview-group")].some((group) => group.getAttribute("aria-label") === "p2")');
  // The card dragged onto p2's group, with the browser's own drag events.
  await page.evaluate('(() => { const card = document.querySelector(' + JSON.stringify(item) + ').parentElement; const target = [...document.querySelectorAll(".terminal-overview-group")].find((group) => group.getAttribute("aria-label") === "p2"); const data = new DataTransfer(); card.dispatchEvent(new DragEvent("dragstart", { bubbles: true, dataTransfer: data })); target.dispatchEvent(new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: data })); target.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: data })); card.dispatchEvent(new DragEvent("dragend", { bubbles: true, dataTransfer: data })); })()');
  await until('the note of the move', 'document.querySelector(".terminal-send-note")?.textContent.startsWith("Moved shell 1 of p1 to p2.")');
  await until('the card under p2', '[...document.querySelectorAll(".terminal-overview-group")].find((group) => group.getAttribute("aria-label") === "p2")?.querySelector(' + JSON.stringify(item) + ') !== null');
  // The pane that showed it, on p1's page, has no p1 shell left to show.
  await until('the pane to let it go', 'document.querySelector(".terminal-status")?.textContent.startsWith("Shell 1 of p1 moved to p2.")');
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure }));
process.exit(0);
`

// TestTerminalMoveInBrowser checks in a real browser that dragging a shell's card in All
// shells onto another project's group moves it there, and that the pane showing it lets
// it go.
func TestTerminalMoveInBrowser(t *testing.T) {
	covers(t, "WEB-57")
	chrome, node, helper := browserCheckTools(t)
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	base := support.NewEnv(t)
	e := base.WithVar("ROTARI_WEB_AUTH_TOKEN", launchToken).WithVar("SHELL", bash).WithVar("HOME", base.Root)
	e.MustRotari("add", "-p", "p1", "--", "true")
	e.MustRotari("add", "-p", "p2", "--", "true")
	live := e.StartWeb("--allow-terminal")
	token := http.Header{"X-Rotari-Token": {launchToken}}
	start := func(project string) terminalShell {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodPost, live+"/_terminal/shells?rows=24&cols=80&project="+project, token)
		var shell terminalShell
		if err := json.Unmarshal(body, &shell); status != http.StatusCreated || err != nil {
			t.Fatalf("starting %s's shell: status %d: %s", project, status, body)
		}
		return shell
	}
	moving := start("p1")
	start("p2")
	input, err := json.Marshal(map[string]string{"chrome": chrome, "base": live, "token": launchToken, "moving": moving.ID})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, "--input-type=module", "-e", terminalMoveBrowserCheck, helper, path).CombinedOutput()
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
	for _, shell := range listTerminalShells(t, live, token) {
		if shell.ID == moving.ID && shell.Project != "p2" {
			t.Errorf("after the drag, the shell is of %q", shell.Project)
		}
	}
}
