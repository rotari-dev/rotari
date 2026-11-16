package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

const terminalTranscriptBrowserCheck = `
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, base, token } = JSON.parse(readFileSync(input, 'utf8'));
const { cdp, close } = await launchChrome(chrome);
let failure = '';
let saved = '';
await withPage(cdp, base + '/_auth/login?next=/activity', { width: 1280, height: 900, settleMs: 200 }, async (page) => {
  const rows = '[...document.querySelectorAll(".xterm-rows > div")].map((row) => row.textContent.trimEnd())';
  const until = async (target, what, expression) => {
    for (const deadline = Date.now() + 30000; Date.now() < deadline; await sleep(100)) {
      try { if (await target.evaluate(expression)) return; } catch {}
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
  await until(page, 'sign-in', 'document.querySelector("#password") !== null');
  await page.evaluate('document.querySelector("#password").value = ' + JSON.stringify(token) + '; document.querySelector("form").submit()');
  await until(page, 'the page', 'location.pathname === "/activity" && document.getElementById("terminal-button") !== null');
  await page.evaluate('document.getElementById("terminal-button").click()');
  await until(page, 'the shell', 'document.querySelector(".terminal-status")?.hidden === true && document.querySelector(".xterm-helper-textarea") !== null');
  // A green line, then a failure.
  await typeLine('printf "\\033[32mgreen-%s\\033[0m\\n" "$((40+2))"');
  await until(page, 'the green line', screenHas('green-42'));
  await typeLine('false');
  await until(page, 'false to end', 'document.querySelector(".terminal-pane").terminalPane.blocks.some((block) => block.ended && block.command() === "false")');
  // Open transcript, from the pane's menu, as a click of the viewer's.
  await page.evaluate('document.querySelector(".terminal-viewport").dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 300, clientY: 300 }))');
  await page.send('Runtime.evaluate', { expression: '[...document.querySelectorAll(".terminal-tab-menu button")].find((item) => item.textContent === "Open transcript").click()', userGesture: true });
  let target;
  for (const deadline = Date.now() + 30000; Date.now() < deadline && !target; await sleep(100)) {
    const { targetInfos } = await cdp.send('Target.getTargets');
    target = targetInfos.find((item) => item.url === base + '/_terminal/transcript');
  }
  if (!target) throw new Error('the transcript did not open');
  const { sessionId } = await cdp.send('Target.attachToTarget', { targetId: target.targetId, flatten: true });
  const transcript = {
    async evaluate(expression) {
      const result = await cdp.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }, sessionId);
      if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text);
      return result.result.value;
    },
  };
  const cards = '[...document.querySelectorAll(".transcript-card")]';
  await until(transcript, 'the cards', cards + '.some((card) => card.querySelector(".transcript-command").textContent === "false")');
  const facts = JSON.parse(await transcript.evaluate('JSON.stringify(' + cards + '.map((card) => ({ command: card.querySelector(".transcript-command").textContent, state: card.dataset.state, green: [...card.querySelectorAll(".transcript-output span")].some((span) => span.textContent === "green-42" && span.style.getPropertyValue("color") === "var(--terminal-green)") })))'));
  const green = facts.find((card) => card.command.startsWith('printf'));
  const failed = facts.find((card) => card.command === 'false');
  if (!green || !green.green || green.state !== 'ok' || !failed || failed.state !== 'failed') throw new Error('the cards: ' + JSON.stringify(facts));
  // The shell script, made in the page.
  await transcript.evaluate('URL.createObjectURL = (blob) => { window.__saved = blob; return "blob:saved"; }; HTMLAnchorElement.prototype.click = function () { window.__savedName = this.download; }');
  await transcript.evaluate('(() => { const radio = document.querySelector(".transcript-format input[value=\\"sh\\"]"); radio.checked = true; radio.dispatchEvent(new Event("change")); })()');
  await transcript.evaluate('[...document.querySelectorAll(".transcript-tools button")].find((button) => button.textContent === "Download").click()');
  const script = await transcript.evaluate('window.__saved.text()');
  const name = await transcript.evaluate('window.__savedName');
  if (!script.startsWith('#!/usr/bin/env bash\n') || !script.includes('\n# Failed with exit 1:\nfalse\n') || script.includes('green-42\n') || !name.endsWith('.sh')) throw new Error('the shell script ' + name + ':\n' + script);
  // Saved on the server, where the shell is: its home here.
  await transcript.evaluate('[...document.querySelectorAll(".transcript-tools button")].find((button) => button.textContent === "Save on the server\u2026").click()');
  await until(transcript, 'the path to save to', 'document.querySelector(".transcript-save input").value.endsWith(".sh")');
  const savedPath = await transcript.evaluate('document.querySelector(".transcript-save input").value');
  await transcript.evaluate('document.querySelector(".transcript-save button[type=submit]").click()');
  await until(transcript, 'the file saved', 'document.querySelector(".transcript-status").textContent.startsWith("Saved ")');
  saved = savedPath;
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure, saved }));
process.exit(0);
`

// TestTerminalTranscriptInBrowser checks in a real browser that Open transcript opens a page
// that takes the pane's commands from it, with the output's colours and the failure, and
// saves them as a shell script.
func TestTerminalTranscriptInBrowser(t *testing.T) {
	covers(t, "WEB-60")
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
	output, err := exec.Command(node, "--input-type=module", "-e", terminalTranscriptBrowserCheck, helper, path).CombinedOutput()
	if err != nil {
		t.Fatalf("browser check: %v\n%s", err, output)
	}
	var result struct {
		Failure string `json:"failure"`
		Saved   string `json:"saved"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("browser check output: %v\n%s", err, output)
	}
	if result.Failure != "" {
		t.Fatal(result.Failure)
	}
	// The file the page saved on the server is the shell script, in the shell's home.
	if filepath.Dir(result.Saved) != base.Root && filepath.Dir(result.Saved) != realPath(t, base.Root) {
		t.Errorf("saved to %s, not in the shell's home %s", result.Saved, base.Root)
	}
	if content, err := os.ReadFile(result.Saved); err != nil || !strings.Contains(string(content), "\n# Failed with exit 1:\nfalse\n") {
		t.Errorf("the saved script %s: %q %v", result.Saved, content, err)
	}
}

// realPath is a directory with its symbolic links followed, as a shell reports it.
func realPath(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
