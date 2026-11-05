package web

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// writeRowsTable writes eval.csv beside its ref and hyp files. Row 2 has no hyp,
// and row 3's is missing.
func writeRowsTable(t *testing.T, directory string, rows int) string {
	t.Helper()
	// One second of 16 kHz mono 16-bit silence.
	header := []byte("RIFF\x24\x7d\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x80\x3e\x00\x00\x00\x7d\x00\x00\x02\x00\x10\x00data\x00\x7d\x00\x00")
	wav := append(header, make([]byte, 32000)...)
	var table strings.Builder
	table.WriteString("id,ref,hyp\n")
	for row := 1; row <= rows; row++ {
		ref, hyp := fmt.Sprintf("ref-%d.wav", row), fmt.Sprintf("hyp-%d.wav", row)
		names := []string{ref, hyp}
		switch row {
		case 2:
			hyp, names = "", []string{ref}
		case 3:
			hyp, names = "gone.wav", []string{ref}
		}
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(directory, name), wav, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		fmt.Fprintf(&table, "%d,%s,%s\n", row, ref, hyp)
	}
	path := filepath.Join(directory, "eval.csv")
	if err := os.WriteFile(path, []byte(table.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestTableViewerMovesBetweenRows runs the binary's page in jsdom. Real key
// defaults and playback are TestTableViewerRowKeysInBrowser.
func TestTableViewerMovesBetweenRows(t *testing.T) {
	covers(t, "WEB-30")
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	e := support.NewEnv(t)
	address := e.StartTableView(writeRowsTable(t, e.Root, 52))
	script := `
const { JSDOM, VirtualConsole } = require('jsdom');
const errors = [];
const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', error => errors.push(error.stack || String(error)));
const fail = (...detail) => { console.error(...detail, errors.join('\n')); process.exit(1); };
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
(async () => {
  const dom = await JSDOM.fromURL(process.argv[1] + '#limit=50', {
    runScripts: 'dangerously', resources: 'usable', virtualConsole,
    beforeParse(window) {
      window.fetch = (input, init) => fetch(new URL(String(input), window.location.href), init);
    },
  });
  const window = dom.window, document = window.document;
  const until = async (what, check) => {
    for (const deadline = Date.now() + 15000; Date.now() < deadline; await wait(30)) {
      if (check()) return;
    }
    fail('timed out waiting for', what, document.getElementById('tv-panes').textContent);
  };
  const press = key => document.body.dispatchEvent(new window.KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  const current = () => (document.querySelector('#tv-table tr[aria-current="true"]') || { dataset: {} }).dataset.number;
  const fragment = () => new window.URLSearchParams(window.location.hash.slice(1));
  const shown = () => {
    const view = document.querySelector('.tv-pane .tv-tab-view');
    return view && (view.querySelector('strong') || view).textContent;
  };
  await until('rows', () => document.querySelector('#tv-table a.tv-media-path'));

  // A cell's file makes its row current, and the tab follows the keys.
  document.querySelector('#tv-table tr[data-number="1"] a.tv-media-path').click();
  if (current() !== '1' || fragment().get('row') !== '1') fail('current', current(), window.location.hash);
  await until('ref of row 1', () => shown() === 'ref-1.wav');
  press('ArrowDown');
  if (current() !== '2') fail('move', current());
  await until('ref of row 2', () => shown() === 'ref-2.wav');

  // A row without the file says so, instead of showing the last row's.
  document.querySelector('#tv-table tr[data-number="1"] a.tv-media-path:not([href*="ref-"])').click();
  await until('hyp of row 1', () => shown() === 'hyp-1.wav');
  press('ArrowDown');
  await until('blank hyp', () => shown() === 'Row 2 has no file in hyp.');
  press('ArrowDown');
  await until('missing hyp', () => /^Row 3: gone\.wav: /.test(shown()));
  press('ArrowDown');
  await until('hyp of row 4', () => shown() === 'hyp-4.wav');

  // Moves cross a page and stop at the end.
  document.querySelector('#tv-table tr[data-number="50"] td:nth-child(2)').click();
  await until('hyp of row 50', () => shown() === 'hyp-50.wav');
  press('ArrowDown');
  await until('row 51', () => current() === '51');
  if (fragment().get('page') !== '2') fail('page', window.location.hash);
  await until('hyp of row 51', () => shown() === 'hyp-51.wav');
  press('ArrowDown');
  press('ArrowDown');
  await wait(50);
  if (current() !== '52') fail('past the end', current());
  await until('hyp of row 52', () => shown() === 'hyp-52.wav');
  if (errors.length) fail('page errors');
  window.close();
})().catch(error => fail(error.stack));
`
	if output, err := exec.Command("node", "-e", script, address).CombinedOutput(); err != nil {
		t.Fatalf("browser check: %v\n%s", err, output)
	}
}

// rowKeysBrowserCheck presses real keys, so the browser's own defaults apply:
// typing in an input, activating a button, and playing audio.
const rowKeysBrowserCheck = `
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [helper, input] = process.argv.slice(1);
const { launchChrome, withPage, sleep } = await import(pathToFileURL(helper));
const { chrome, address } = JSON.parse(readFileSync(input, 'utf8'));
const keys = {
  ArrowDown: { key: 'ArrowDown', code: 'ArrowDown', windowsVirtualKeyCode: 40 },
  ArrowUp: { key: 'ArrowUp', code: 'ArrowUp', windowsVirtualKeyCode: 38 },
  ArrowRight: { key: 'ArrowRight', code: 'ArrowRight', windowsVirtualKeyCode: 39 },
  Space: { key: ' ', code: 'Space', windowsVirtualKeyCode: 32, text: ' ' },
  ShiftEnter: { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, text: '\r', modifiers: 8 },
};
const browser = await launchChrome(chrome);
try {
  await withPage(browser.cdp, address, { width: 1000, height: 700 }, async (page) => {
    const value = (expression) => page.evaluate(expression);
    const press = async (name) => {
      await page.send('Input.dispatchKeyEvent', { type: 'keyDown', ...keys[name] });
      await page.send('Input.dispatchKeyEvent', { type: 'keyUp', ...keys[name], text: undefined });
    };
    const until = async (what, expression) => {
      for (const deadline = Date.now() + 10000; Date.now() < deadline; await sleep(50)) {
        if (await value(expression)) return;
      }
      throw new Error('timed out waiting for ' + what + ': ' + await value('document.body.innerText.slice(0, 500)'));
    };
    const current = 'document.querySelector("#tv-table tr[aria-current=true]")?.dataset.number';
    await until('rows', 'document.querySelector("#tv-table a.tv-media-path")');

    // A real click frames the cell and leaves the keys to the table, and the
    // frame moves with the row.
    const framed = 'document.querySelector("#tv-table td.tv-cell-current")?.parentElement.dataset.number';
    const box = await value('(() => { const box = document.querySelector("#tv-table tr[data-number=\'4\'] a.tv-media-path").getBoundingClientRect(); return { x: box.x + 5, y: box.y + box.height / 2 }; })()');
    await page.send('Input.dispatchMouseEvent', { type: 'mousePressed', ...box, button: 'left', clickCount: 1 });
    await page.send('Input.dispatchMouseEvent', { type: 'mouseReleased', ...box, button: 'left', clickCount: 1 });
    await until('the file', 'document.querySelector(".tv-pane audio")?.src.includes("ref-4.wav")');
    if ((await value(framed)) !== '4') throw new Error('the clicked cell is not framed');
    if (await value('!!document.activeElement.closest("#tv-table")')) throw new Error('focus stayed in the table');
    await press('ArrowDown');
    await until('row 5', current + ' === "5"');
    if ((await value(framed)) !== '5') throw new Error('the frame did not move');
    await until('the next file', 'document.querySelector(".tv-pane audio")?.src.includes("ref-5.wav")');

    // Moves scroll the table, not the page: the row ends up in view with the
    // page where it was. The table is tall enough only with a short window.
    await page.send('Emulation.setDeviceMetricsOverride', { width: 1000, height: 420, deviceScaleFactor: 1, mobile: false });
    await sleep(100);
    const pageTop = 'document.querySelector(".tv-main").scrollTop';
    if ((await value(pageTop)) !== 0) throw new Error('the page started scrolled');
    for (let times = 0; times < 20; times++) await press('ArrowDown');
    await until('row 25', current + ' === "25"');
    if ((await value(pageTop)) !== 0) throw new Error('a move scrolled the page');
    if ((await value('document.querySelector(".tv-scroll").scrollTop')) <= 0) throw new Error('a move did not scroll the table');
    const inView = await value('(() => { const box = document.querySelector("#tv-table tr.tv-row-current").getBoundingClientRect(); return box.top >= 0 && box.bottom <= document.querySelector(".tv-main").getBoundingClientRect().bottom + 1; })()');
    if (!inView) throw new Error('the current row is out of view');
    for (let times = 0; times < 20; times++) await press('ArrowUp');
    await until('row 5', current + ' === "5"');
    await page.send('Emulation.setDeviceMetricsOverride', { width: 1000, height: 700, deviceScaleFactor: 1, mobile: false });
    await sleep(100);

    // An input keeps its keys: the arrow stays and a space is typed.
    await value('document.querySelector(".tv-filter-value").focus()');
    await press('ArrowDown');
    await press('Space');
    await sleep(100);
    if ((await value(current)) !== '5') throw new Error('an arrow in an input moved the row');
    if ((await value('document.querySelector(".tv-filter-value").value')) !== ' ') throw new Error('a space was not typed in the input');
    await value('document.activeElement.blur()');

    // Space plays and pauses the framed file, which is open.
    if (!(await value('document.querySelector(".tv-pane audio").paused'))) throw new Error('a move started playback');
    await press('Space');
    await until('playback', '!document.querySelector(".tv-pane audio").paused');
    await press('Space');
    await until('pause', 'document.querySelector(".tv-pane audio").paused');

    // Right moves the frame, opening nothing, and Space plays the framed file.
    await press('ArrowRight');
    if ((await value('document.querySelector(".tv-cell-current").dataset.column')) !== '2') throw new Error('the frame did not move right');
    if (!(await value('document.querySelector(".tv-pane audio").src.includes("ref-5.wav")'))) throw new Error('moving the frame changed the pane');
    await press('Space');
    await until('the framed file', '[...document.querySelectorAll(".tv-pane audio")].some((audio) => audio.src.includes("hyp-5.wav") && !audio.paused)');
    await press('Space');
    await until('its pause', '[...document.querySelectorAll(".tv-pane audio")].every((audio) => audio.paused)');

    // A pane too short for its plots scrolls, and keeps the spectrogram.
    await value('document.querySelector(".tv-pane-active button[aria-label=\'Split down\']").click()');
    await value('document.querySelector(".tv-pane-active button[aria-label=\'Split down\']").click()');
    await until('the split panes', 'document.querySelectorAll(".tv-pane").length === 3');
    const short = await value('(() => { const pane = document.querySelector(".tv-pane-active"); const body = pane.querySelector(".tv-pane-body"); const box = pane.querySelector(".tv-spectrogram-box"); return { height: box.clientHeight, canScroll: body.scrollHeight > body.clientHeight }; })()');
    if (short.height < 120 || !short.canScroll) throw new Error('a short pane lost its spectrogram: ' + JSON.stringify(short));
    await value('document.querySelector(".tv-pane-active button[aria-label=\'Close pane\']").click()');
    await value('document.querySelector(".tv-pane-active button[aria-label=\'Close pane\']").click()');
    await until('one pane', 'document.querySelectorAll(".tv-pane").length === 1');

    // Shift+Enter opens the whole row.
    await press('ShiftEnter');
    await until('both files', 'document.querySelectorAll(".tv-pane audio").length === 2');

    // Opening Describe scrolls the page to the table.
    await value('document.querySelector(".tv-describe-toggle").click()');
    await until('describe', 'document.querySelector("#tv-table tr.tv-describe")');
    await until('the table at the top', 'Math.abs(document.querySelector(".tv-table-area").getBoundingClientRect().top - document.querySelector(".tv-main").getBoundingClientRect().top) <= 1 || document.querySelector(".tv-main").scrollTop >= document.querySelector(".tv-main").scrollHeight - document.querySelector(".tv-main").clientHeight - 1');
    if ((await value('document.querySelector(".tv-main").scrollTop')) <= 0) throw new Error('Describe did not scroll past the panes');

    // A button keeps Space, and the player stays paused.
    await value('document.querySelector(".tv-sort").focus()');
    await press('Space');
    await until('the sort', 'document.querySelector("#tv-table th[aria-sort=ascending]")');
    if (!(await value('document.querySelector(".tv-pane audio").paused'))) throw new Error('Space on a button played the file');
  });
} finally {
  await browser.close();
}
`

// TestTableViewerRowKeysInBrowser checks what jsdom does not model.
func TestTableViewerRowKeysInBrowser(t *testing.T) {
	covers(t, "WEB-30")
	chrome, node, helper := browserCheckTools(t)
	e := support.NewEnv(t)
	address := e.StartTableView(writeRowsTable(t, e.Root, 30))
	input, err := json.Marshal(map[string]string{"chrome": chrome, "address": address})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(node, "--input-type=module", "-e", rowKeysBrowserCheck, helper, path).CombinedOutput(); err != nil {
		t.Fatalf("browser check: %v\n%s", err, output)
	}
}
