package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// The shell asks for the cursor's position as a program does, with no line editor and no echo,
// and keeps what it is told in the answers file: ESC [ row ; column R for each.
const terminalAnswersShell = `#!/bin/sh
stty raw -echo
printf '\033[c'
{
  sleep 0.5
  printf '\033[6n\033[6n\033[6n'
  while [ ! -e "$ROTARI_TEST_ANSWERS.away" ]; do sleep 0.05; done
  printf '\033[?1004h\033[?1004l\033[6n'
  touch "$ROTARI_TEST_ANSWERS.asked"
} &
exec cat >> "$ROTARI_TEST_ANSWERS"
`

const terminalAnswersBrowserCheck = `
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, base, token, answers } = JSON.parse(readFileSync(input, 'utf8'));
const { cdp, close } = await launchChrome(chrome);
let failure = '';
const check = async (page) => {
  const pane = 'document.querySelector(".terminal-pane").terminalPane';
  const said = () => (existsSync(answers) ? readFileSync(answers, 'latin1') : '');
  const count = (pattern) => (said().match(pattern) || []).length;
  const positions = () => count(/\x1b\[\d+;\d+R/g);
  const until = async (what, expression) => {
    for (const start = Date.now(); Date.now() - start < 30000; await sleep(50)) {
      try { if (typeof expression === 'function' ? expression() : await page.evaluate(expression)) return; } catch {}
    }
    throw new Error('timed out waiting for ' + what + '; answers: ' + JSON.stringify(said()));
  };
  const shown = 'document.querySelector(".terminal-pane")?.terminalPane?.blocksLive === true';
  const answered = async (want, when) => {
    await until(want + ' answers ' + when, () => positions() >= want);
    // Any answer too many is on its way by now.
    await sleep(700);
    if (positions() !== want || count(/\x1b\[\?1;2c/g) !== 1)
      throw new Error(when + ': want ' + want + ' positions and one device answer, got ' + JSON.stringify(said()));
  };
  await until('sign-in', 'document.querySelector("#password") !== null');
  await page.evaluate('document.querySelector("#password").value = ' + JSON.stringify(token) + '; document.querySelector("form").submit()');
  await until('the page', 'location.pathname === "/activity" && document.getElementById("terminal-button") !== null');
  await page.evaluate('document.getElementById("terminal-button").click()');
  await answered(3, 'at first, the shell having asked before the page connected');
  // Following a link, and connecting again in the same page, replay what was answered.
  await page.send('Page.navigate', { url: base + '/activity?again=1' });
  await until('the replay', 'location.search === "?again=1" && ' + shown);
  await answered(3, 'after a page change');
  await page.send('Page.navigate', { url: 'about:blank' });
  await sleep(300);
  writeFileSync(answers + '.away', '');
  await until('a question with no page open', () => existsSync(answers + '.asked'));
  await page.send('Page.navigate', { url: base + '/activity?back=1' });
  await until('the replay', 'location.search === "?back=1" && ' + shown);
  await answered(4, 'after a question asked with no page open');
  await page.send('Page.navigate', { url: base + '/activity?last=1' });
  await until('the replay', 'location.search === "?last=1" && ' + shown);
  await answered(4, 'after the last page change');
  // What is typed still arrives.
  await page.evaluate('document.querySelector(".xterm-helper-textarea").focus()');
  await page.send('Input.insertText', { text: 'typed' });
  await until('what was typed', () => said().includes('typed'));
};
await withPage(cdp, base + '/_auth/login?next=/activity', { width: 1280, height: 900, settleMs: 200 }, check).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure }));
process.exit(0);
`

// TestTerminalAnswersOnceInBrowser checks in a real browser that the terminal answers a
// program's question, such as ipython's for the cursor's position, once: not again for each
// replay, where the answer would be typed into whatever runs by then, and still for one that
// the shell asked before a page first showed it or while none did.
func TestTerminalAnswersOnceInBrowser(t *testing.T) {
	covers(t, "WEB-33")
	chrome, node, helper := browserCheckTools(t)
	base := support.NewEnv(t)
	shell := filepath.Join(base.Root, "asking-shell")
	if err := os.WriteFile(shell, []byte(terminalAnswersShell), 0o700); err != nil {
		t.Fatal(err)
	}
	answers := filepath.Join(base.Root, "answers")
	e := base.WithVar("ROTARI_WEB_AUTH_TOKEN", launchToken).WithVar("HOME", base.Root).WithVar("ROTARI_TEST_ANSWERS", answers)
	live := e.StartWeb("--allow-terminal", "--terminal-shell", shell)
	input, err := json.Marshal(map[string]string{"chrome": chrome, "base": live, "token": launchToken, "answers": answers})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, "--input-type=module", "-e", terminalAnswersBrowserCheck, helper, path).CombinedOutput()
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
