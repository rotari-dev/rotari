package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

const terminalFastOutputBrowserCheck = `
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, base, token, scenario, root } = JSON.parse(readFileSync(input, 'utf8'));
const prepare = 'window.__errors = []; window.addEventListener("error", (event) => window.__errors.push(event.message));';
const { cdp, close } = await launchChrome(chrome);
let failure = '';
const figures = {};
const check = async (page) => {
  const pane = 'document.querySelector(".terminal-pane").terminalPane';
  // The last lines of the buffer, where the prompt is, whatever the view is scrolled to.
  const tail = '(() => { const buffer = ' + pane + '.term.buffer.active; const lines = []; for (let i = Math.max(0, buffer.length - 1000); i < buffer.length; i++) lines.push(buffer.getLine(i)?.translateToString(true) || ""); return lines.filter(Boolean); })()';
  const errors = () => page.evaluate('JSON.stringify(window.__errors)');
  const until = async (what, expression) => {
    const start = Date.now();
    for (; Date.now() - start < 30000; await sleep(50)) {
      try { if (typeof expression === 'function' ? expression() : await page.evaluate(expression)) return Date.now() - start; } catch {}
    }
    throw new Error('timed out waiting for ' + what + '; page errors: ' + await errors().catch(() => '(not available)') + '; the terminal ends: ' + JSON.stringify(await page.evaluate(tail + '.slice(-8)').catch(() => '(not available)')));
  };
  const typeLine = async (line) => {
    await page.evaluate('document.querySelector(".xterm-helper-textarea").focus()');
    await page.send('Input.insertText', { text: line });
    const enter = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13, text: '\r', unmodifiedText: '\r' };
    await page.send('Input.dispatchKeyEvent', { type: 'keyDown', ...enter });
    await page.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
  };
  const interrupt = async () => {
    await page.evaluate('document.querySelector(".xterm-helper-textarea").focus()');
    await page.send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'c', code: 'KeyC', modifiers: 2, windowsVirtualKeyCode: 67, nativeVirtualKeyCode: 67 });
    await page.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'c', code: 'KeyC', modifiers: 2, windowsVirtualKeyCode: 67 });
  };
  const shell = 'location.pathname === "/activity" && document.querySelector(".terminal-status")?.hidden === true && document.querySelector(".xterm-helper-textarea") !== null';
  await until('sign-in', 'document.querySelector("#password") !== null');
  await page.evaluate('document.querySelector("#password").value = ' + JSON.stringify(token) + '; document.querySelector("form").submit()');
  await until('the page', 'location.pathname === "/activity" && document.getElementById("terminal-button") !== null');
  await page.evaluate('document.getElementById("terminal-button").click()');
  await until('the shell', shell);
  const line = 'x'.repeat(98);
  // The markers are computed, so that the typed lines' echo cannot satisfy them.
  if (scenario === 'hidden') {
    // Another page brought forward hides this one. Whether the browser then slows it so much
    // that it falls behind differs from run to run; that nothing is thrown, and that output goes
    // on once it is seen, must not.
    const { targetInfos } = await cdp.send('Target.getTargets');
    const own = targetInfos.find((target) => target.type === 'page' && target.url.startsWith(base));
    const { targetId } = await cdp.send('Target.createTarget', { url: 'about:blank', browserContextId: own.browserContextId });
    await page.evaluate('document.querySelector(".xterm-helper-textarea").focus()');
    await typeLine('yes ' + line);
    await cdp.send('Target.activateTarget', { targetId });
    const start = Date.now();
    while (Date.now() - start < 10000 && !(await page.evaluate(pane + '.fellBehind !== null'))) await sleep(100);
    figures.hidden = await page.evaluate('document.hidden');
    figures.fellBehind = await page.evaluate(pane + '.fellBehind !== null');
    figures.afterMs = Date.now() - start;
    await cdp.send('Target.activateTarget', { targetId: own.targetId });
    if (figures.fellBehind)
      figures.notedMs = await until('the note', 'document.querySelector(".terminal-send-note")?.hidden === false && document.querySelector(".terminal-send-note").textContent.includes("fell behind")');
    await until('the shell again', shell);
    await interrupt();
    await typeLine('echo "stopped-$((1+1))"');
    figures.stoppedMs = await until('the prompt after Ctrl-C', tail + '.includes("stopped-2")');
  } else {
    // Whole lines of 99 bytes, so that what follows starts a line of its own.
    // Fill the replay with the page away, then come back with output still arriving.
    const at = (name) => join(root, name);
    await typeLine('cd "$HOME"; yes ' + line + ' | head -c 71280000; touch filled; i=0; while [ ! -e seen ]; do i=$((i+1)); echo "tick-$i"; sleep 0.1; done; echo "replay-$((1+1))"');
    await sleep(300);
    await page.send('Page.navigate', { url: 'about:blank' });
    await until('the replay to fill', () => existsSync(at('filled')));
    await page.send('Page.navigate', { url: base + '/activity' });
    figures.parsedMs = await until('the replay to be parsed', shell + ' && ' + pane + '.blocksLive');
    await sleep(1000);
    writeFileSync(at('seen'), '');
    await until('the end after the replay', tail + '.includes("replay-2")');
    const ticks = JSON.parse(await page.evaluate('JSON.stringify(' + tail + ')')).filter((text) => /^tick-\d+$/.test(text)).map((text) => Number(text.slice(5)));
    figures.ticks = ticks.length;
    if (ticks.length < 5 || ticks.some((tick, i) => tick !== i + 1)) throw new Error('lines after the replay are missing: ' + JSON.stringify(ticks));
  }
  const seen = JSON.parse(await errors());
  if (seen.length) throw new Error(seen.length + ' page errors, the first: ' + seen[0]);
};
await withPage(cdp, base + '/_auth/login?next=/activity', { width: 1280, height: 900, settleMs: 200, beforeNavigate: (page) => page.send('Page.addScriptToEvaluateOnNewDocument', { source: prepare }) }, check).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure, figures }));
process.exit(0);
`

// TestTerminalFastOutputInBrowser checks in a real browser that the panel loses no output
// after a replay larger than xterm.js holds, and that a hidden page a program outruns throws
// nothing, says that it fell behind, and goes on when it is seen.
func TestTerminalFastOutputInBrowser(t *testing.T) {
	covers(t, "WEB-33")
	chrome, node, helper := browserCheckTools(t)
	for _, test := range []struct {
		scenario string
		args     []string
	}{
		{"hidden", nil},
		{"replay", []string{"--terminal-replay-size", "64MiB"}},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			base := support.NewEnv(t)
			e := base.WithVar("ROTARI_WEB_AUTH_TOKEN", launchToken).WithVar("SHELL", "/bin/sh").WithVar("HOME", base.Root)
			live := e.StartWeb(append([]string{"--allow-terminal"}, test.args...)...)
			input, err := json.Marshal(map[string]string{"chrome": chrome, "base": live, "token": launchToken, "scenario": test.scenario, "root": base.Root})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(path, input, 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command(node, "--input-type=module", "-e", terminalFastOutputBrowserCheck, helper, path).CombinedOutput()
			if err != nil {
				t.Fatalf("browser check: %v\n%s", err, output)
			}
			var result struct {
				Failure string          `json:"failure"`
				Figures json.RawMessage `json:"figures"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("browser check output: %v\n%s", err, output)
			}
			t.Logf("figures: %s", result.Figures)
			if result.Failure != "" {
				t.Error(result.Failure)
			}
		})
	}
}
