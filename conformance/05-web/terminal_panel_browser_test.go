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
  await until('the Terminal button to count two', 'document.getElementById("terminal-button").textContent === "Terminal (2)"');
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
  await page.evaluate('[...document.querySelectorAll(".terminal-bar button")].find((item) => item.getAttribute("aria-label") === "Maximize").click()');
  // Says what it found, since nothing but CI runs this.
  const measured = 'JSON.stringify((() => {' +
    'const box = document.getElementById("terminal-panel").getBoundingClientRect();' +
    'const top = document.elementFromPoint(innerWidth / 2, 2);' +
    'return { classes: document.documentElement.className, panelTop: box.top, panelBottom: box.bottom, innerHeight, atTop: top && (top.id || top.className || top.tagName) };' +
    '})())';
  await until('the panel to cover the window', covers).catch(async (error) => {
    throw new Error(error.message + '; measured ' + (await page.evaluate(measured)));
  });
  await page.evaluate('[...document.querySelectorAll(".terminal-bar button")].find((item) => item.getAttribute("aria-label") === "Restore").click()');
  await until('the panel to give the window back', '!' + covers);
  await collect();

  // The Settings menu's copy and send buttons beside the path of this rotari show their pictures.
  await page.evaluate('document.getElementById("settings-button").click()');
  const aboutIcons = JSON.parse(await page.evaluate('JSON.stringify([...document.querySelectorAll(".settings-about button")].map((button) => { const icon = button.querySelector("svg").getBoundingClientRect(); return [button.getAttribute("aria-label"), Math.round(icon.width), Math.round(icon.height)]; }))'));
  if (aboutIcons.length !== 2 || aboutIcons.some(([, width, height]) => width < 10 || height < 10)) throw new Error('the Settings menu\'s path buttons show no picture: ' + JSON.stringify(aboutIcons));
  await page.evaluate('document.getElementById("settings-button").click()');

  // Split shows two shells side by side, each typed into and sized on its own.
  const barButton = (text) => '[...document.querySelectorAll(".terminal-bar button")].find((item) => item.textContent === ' + JSON.stringify(text) + ' || item.getAttribute("aria-label") === ' + JSON.stringify(text) + ')';
  const paneHas = (index, text) => 'document.querySelectorAll(".terminal-pane")[' + index + ']?.querySelector(".xterm-rows")?.textContent.includes(' + JSON.stringify(text) + ')';
  const bothSettled = '[...document.querySelectorAll(".terminal-status")].every((item) => item.hidden)';
  // The splits are pictures with no frame, in the middle of their buttons and of the bar.
  const icons = await page.evaluate('JSON.stringify(["Split right", "Split down"].map((label) => {' +
    'const button = document.querySelector(".terminal-bar button[aria-label=\\"" + label + "\\"]");' +
    'const box = button.getBoundingClientRect();' +
    'const icon = button.querySelector("svg").getBoundingClientRect();' +
    'const bar = document.querySelector(".terminal-bar").getBoundingClientRect();' +
    'const style = getComputedStyle(button);' +
    'return { border: style.borderTopWidth, off: Math.abs(box.top + box.height / 2 - icon.top - icon.height / 2), offBar: Math.abs(bar.top + bar.height / 2 - icon.top - icon.height / 2), hit: document.elementFromPoint(icon.left + icon.width / 2, icon.top + icon.height / 2) === button, title: button.title };' +
    '}))');
  for (const icon of JSON.parse(icons)) {
    if (icon.border !== '0px' || icon.off > 1 || icon.offBar > 2) throw new Error('a split icon is framed or off center: ' + icons);
    // The pointer over the picture is over the button, whose title says what it does.
    if (!icon.hit || !icon.title) throw new Error('a split icon\'s picture hides its button\'s title: ' + icons);
  }
  await page.evaluate(barButton('Split right') + '.click()');
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
    'return Math.abs(first.left - box.left) <= 1 && Math.abs(second.right - box.right) <= 1 && first.right <= second.left + 1 && second.left - first.right <= 8 &&' +
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

  // A row under the right-hand pane, then a column beside it: four panes, as columns of rows.
  const paneCount = (count) => 'document.querySelectorAll(".xterm-helper-textarea").length === ' + count;
  await page.evaluate('document.querySelectorAll(".xterm-helper-textarea")[1].focus()');
  await page.evaluate(barButton('Split down') + '.click()');
  await until('a third pane', paneCount(3));
  await until('three panes to settle', bothSettled);
  await page.evaluate(barButton('Split right') + '.click()');
  await until('a fourth pane', paneCount(4));
  await until('four panes to settle', bothSettled);
  await typeLine('echo "third-$((3+3))"', 2);
  await until('the third pane to run a command', paneHas(2, 'third-6'));
  await typeLine('echo "fourth-$((4+4))"', 3);
  await until('the fourth pane to run a command', paneHas(3, 'fourth-8'));
  const fourBoxes = 'JSON.stringify((() => {' +
    'const box = document.querySelector(".terminal-panes").getBoundingClientRect();' +
    'return { box: [box.left, box.top, box.width, box.height], panes: [...document.querySelectorAll(".terminal-pane")].map((pane) => {' +
    'const item = pane.getBoundingClientRect();' +
    'const screen = pane.querySelector(".xterm-screen").getBoundingClientRect();' +
    'return { left: item.left, top: item.top, right: item.right, bottom: item.bottom, screen: [screen.width, screen.height] };' +
    '}) };' +
    '})())';
  const checkFour = async () => {
    const { box, panes } = JSON.parse(await page.evaluate(fourBoxes));
    const area = panes.reduce((sum, item) => sum + (item.right - item.left) * (item.bottom - item.top), 0);
    // Handles take the rest, so most of the room is in the panes.
    if (area > box[2] * box[3] + 1 || area < box[2] * box[3] * 0.9) throw new Error('four panes take the wrong room: ' + JSON.stringify({ box, panes }));
    panes.forEach((one, i) => panes.slice(i + 1).forEach((other) => {
      const apart = one.right <= other.left + 1 || other.right <= one.left + 1 || one.bottom <= other.top + 1 || other.bottom <= one.top + 1;
      if (!apart) throw new Error('panes overlap: ' + JSON.stringify({ box, panes }));
    }));
    for (const item of panes) {
      if (item.screen[0] > item.right - item.left || item.screen[1] > item.bottom - item.top || item.screen[0] < (item.right - item.left) - 60) throw new Error('a terminal does not fit its pane: ' + JSON.stringify({ box, panes }));
    }
  };
  await sleep(300);
  await checkFour();
  await collect();

  // Following a link finds all four.
  await page.evaluate('location.href = "/project/p1"');
  await until('four panes on the next page', paneCount(4));
  await until('the fourth pane replayed', paneHas(3, 'fourth-8'));
  await until('the third pane replayed', paneHas(2, 'third-6'));
  await until('four panes to settle again', bothSettled);
  await sleep(300);
  await checkFour();
  await collect();

  // Closing panes leaves their shells, with their tabs.
  const tabCount = await page.evaluate('document.querySelectorAll(".terminal-tab").length');
  for (const left of [3, 2, 1]) {
    await page.evaluate(barButton('Close pane') + '.click()');
    await until(left + ' panes', 'document.querySelectorAll(".terminal-pane").length === ' + left + ' && ' + paneCount(left));
  }
  if (await page.evaluate('document.querySelectorAll(".terminal-tab").length !== ' + tabCount)) throw new Error('closing a pane ended its shell');
  await collect();

  // Down either side, the panel has the window's height, and the page, its top bar, and a dialog lie clear of it.
  const choosePlace = (label) => page.evaluate(barButton('Panel position') + '.click(); [...document.querySelectorAll(".terminal-tab-menu button")].find((item) => item.textContent === ' + JSON.stringify(label) + ').click()');
  const sideFacts = 'JSON.stringify((() => {' +
    'const panel = document.getElementById("terminal-panel").getBoundingClientRect();' +
    'const content = document.querySelector(".content").getBoundingClientRect();' +
    'const topbar = document.querySelector(".content > .topbar").getBoundingClientRect();' +
    'const sidebar = document.querySelector(".sidebar").getBoundingClientRect();' +
    'const screen = document.querySelector(".terminal-pane .xterm-screen").getBoundingClientRect();' +
    'const pane = document.querySelector(".terminal-pane").getBoundingClientRect();' +
    'window.showPath("/p");' +
    'const dialog = document.querySelector(".output-panel").getBoundingClientRect();' +
    'window.closeOutputModal();' +
    'return { panel: [panel.left, panel.top, panel.right, panel.bottom], content: [content.left, content.right], topbar: [topbar.left, topbar.right], sidebar: [sidebar.left, sidebar.right], dialog: [dialog.left, dialog.right], screen: screen.width, pane: pane.width, innerWidth, innerHeight };' +
    '})())';
  const checkSide = async (where) => {
    const facts = JSON.parse(await page.evaluate(sideFacts));
    const { panel, content, topbar, sidebar, dialog, screen, pane } = facts;
    const fail = (what) => { throw new Error('at the ' + where + ', ' + what + ': ' + JSON.stringify(facts)); };
    if (panel[1] !== 0 || panel[3] !== facts.innerHeight) fail('the panel does not have the window\'s height');
    if (Math.abs(panel[2] - panel[0] - 480) > 1) fail('the panel is not its usual width');
    if (where === 'right') {
      if (panel[2] !== facts.innerWidth) fail('the panel is not at the edge');
      if (content[1] > panel[0] + 1 || topbar[1] > panel[0] + 1 || dialog[1] > panel[0] + 1) fail('the page is under the panel');
    } else {
      if (panel[0] !== 0) fail('the panel is not at the edge');
      if (sidebar[0] < panel[2] - 1 || content[0] < panel[2] - 1 || topbar[0] < panel[2] - 1 || dialog[0] < panel[2] - 1) fail('the page is under the panel');
    }
    if (screen > pane || screen < pane - 60) fail('the terminal does not fit its pane');
  };
  for (const [label, where] of [['Right', 'right'], ['Left', 'left']]) {
    await choosePlace(label);
    await until('the panel at the ' + where, 'document.documentElement.dataset.terminalPlace === ' + JSON.stringify(where));
    await sleep(300);
    await checkSide(where);
    await collect();
  }
  // As wide as it goes, in a window as narrow as allows a side, nothing in the top bar runs under the panel.
  const topbarClear = 'JSON.stringify((() => {' +
    'const panel = document.getElementById("terminal-panel").getBoundingClientRect();' +
    'const items = [...document.querySelectorAll(".content > .topbar button, .content > .topbar .crumbs")].filter((item) => item.getClientRects().length).map((item) => item.getBoundingClientRect());' +
    'return { panel: [panel.left, panel.right], furthest: Math.max(...items.map((item) => item.right)), nearest: Math.min(...items.map((item) => item.left)), scroll: document.documentElement.scrollWidth, innerWidth };' +
    '})())';
  const growPanel = 'for (let press = 0; press < 80; press++) document.querySelector(".terminal-resizer").dispatchEvent(new KeyboardEvent("keydown", { key: document.documentElement.dataset.terminalPlace === "right" ? "ArrowLeft" : "ArrowRight", bubbles: true }))';
  for (const [label, where] of [['Right', 'right'], ['Left', 'left']]) {
    for (const width of [1280, 1000]) {
      await page.send('Emulation.setDeviceMetricsOverride', { width, height: 900, deviceScaleFactor: 1, mobile: false });
      await choosePlace(label);
      await until('the panel at the ' + where + ' in ' + width, 'document.documentElement.dataset.terminalPlace === ' + JSON.stringify(where) + ' && innerWidth === ' + width);
      await page.evaluate(growPanel);
      await sleep(200);
      const facts = JSON.parse(await page.evaluate(topbarClear));
      const clear = where === 'right' ? facts.furthest <= facts.panel[0] + 1 : facts.nearest >= facts.panel[1] - 1;
      if (!clear || facts.scroll > facts.innerWidth + 1) throw new Error('at the ' + where + ' in ' + width + ', the top bar runs under the panel or the page scrolls sideways: ' + JSON.stringify(facts));
    }
  }
  await page.send('Emulation.setDeviceMetricsOverride', { width: 1280, height: 900, deviceScaleFactor: 1, mobile: false });
  await page.evaluate('localStorage.removeItem("rotari-terminal-width"); document.documentElement.style.removeProperty("--terminal-saved-width")');
  await choosePlace('Left');
  await until('the panel at the left again', 'document.documentElement.dataset.terminalPlace === "left"');

  // It stays there with the next page, and keeps the shell.
  await page.evaluate('location.href = "/project/p1"');
  await until('the next page with the panel at the left', 'document.documentElement.dataset.terminalPlace === "left" && document.querySelector(".xterm-helper-textarea") !== null');
  await until('the shell at the left to settle', bothSettled);
  await sleep(300);
  await checkSide('left');
  // A narrow window has it at the bottom, and a wider one gives it back.
  await page.send('Emulation.setDeviceMetricsOverride', { width: 900, height: 900, deviceScaleFactor: 1, mobile: false });
  await until('the panel at the bottom of a narrow window', 'document.documentElement.dataset.terminalPlace === undefined && document.getElementById("terminal-panel").getBoundingClientRect().top > 0');
  await page.send('Emulation.setDeviceMetricsOverride', { width: 1280, height: 900, deviceScaleFactor: 1, mobile: false });
  await until('the panel back at the left', 'document.documentElement.dataset.terminalPlace === "left"');
  await choosePlace('Bottom');
  await until('the panel at the bottom', 'document.documentElement.dataset.terminalPlace === undefined');
  // Its height stays where it is put, though the sidebar's follows it and is watched.
  const panelHeight = 'document.getElementById("terminal-panel").getBoundingClientRect().height';
  const heightBefore = await page.evaluate(panelHeight);
  await page.evaluate('document.querySelector(".terminal-resizer").dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp", bubbles: true }))');
  await sleep(300);
  const heightAfter = await page.evaluate(panelHeight);
  if (Math.abs(heightAfter - heightBefore - 24) > 1) throw new Error('the panel did not keep its height: ' + heightBefore + ' then ' + heightAfter);
  await collect();

  // In a window of its own, the panel takes the page's shell, what the page sends reaches it
  // there, and closing the window puts the panel back on the same shell.
  await page.evaluate('window.__popups = []; const opening = window.open; window.open = (...args) => { const opened = opening.apply(window, args); window.__popups.push(opened); return opened; }; true');
  await page.evaluate(barButton('Panel position') + '.click()');
  // A browser opens a window only from a click.
  await page.send('Runtime.evaluate', { expression: '[...document.querySelectorAll(".terminal-tab-menu button")].find((item) => item.textContent === "Window").click()', userGesture: true });
  await until('the panel to leave the page', '!document.documentElement.classList.contains("terminal-open") && window.__popups.length === 1 && !window.__popups[0].closed');
  const inPopup = (expression) => '(() => { const doc = window.__popups[0].document; return ' + expression + '; })()';
  await until('the terminal in its window', inPopup('doc.documentElement.hasAttribute("data-terminal-window") && doc.querySelector(".xterm-rows") !== null && doc.querySelector(".terminal-pane-status").hidden'));
  if (await page.evaluate(inPopup('doc.querySelector(".terminal-resizer") !== null'))) throw new Error('the window has a resizer');
  const popupProblem = await page.evaluate('window.sendToTerminal("echo popped-$((2+3))").then((value) => value || "")');
  if (popupProblem) throw new Error('sending to the window: ' + popupProblem);
  await until('the sent line in the window', inPopup('doc.querySelector(".xterm-rows").textContent.includes("popped-$((2+3))")'));
  if (await page.evaluate('[...document.querySelectorAll(".terminal-pane-status")].some((status) => /Another tab/.test(status.textContent))')) throw new Error('the page said another tab took its shell');
  await page.evaluate('window.__popups[0].close(); true');
  await until('the panel back in the page', 'document.documentElement.classList.contains("terminal-open") && !document.getElementById("terminal-panel").hidden');
  await until('the same shell in the page', screenHas('popped-$((2+3))'));
  await collect();

  // All shells shows a card for each shell in the page's dialog, as a log is, saying where
  // it is now, and choosing one closes it.
  // The send above typed its line without Enter; run it first, so that cd starts a line of its own.
  await typeLine('');
  await typeLine('cd / && echo "rooted-$((3+4))"');
  await until('the shell to move', screenHas('rooted-7'));
  await page.evaluate(barButton('All shells') + '.click()');
  const cardDirs = '[...document.querySelectorAll(".terminal-overview-card .terminal-overview-dir")].map((item) => item.textContent)';
  await until('the cards to say where their shells are', cardDirs + '.length > 0 && ' + cardDirs + '.every((text) => text !== "\u2026")');
  const dirs = JSON.parse(await page.evaluate('JSON.stringify(' + cardDirs + ')'));
  if (!dirs.includes('/') || dirs.some((text) => text.startsWith('started in'))) throw new Error('the cards say ' + JSON.stringify(dirs));
  // Over the page and inside the window, with the panel's room left to it, as a log is.
  const placed = JSON.parse(await page.evaluate('JSON.stringify((() => { const box = document.querySelector(".terminal-overview").getBoundingClientRect(); const panel = document.getElementById("terminal-panel").getBoundingClientRect(); return { box: [box.left, box.top, box.right, box.bottom], panelTop: panel.top, width: innerWidth, height: innerHeight, top: document.elementFromPoint((box.left + box.right) / 2, box.top + 8)?.closest(".terminal-overview") !== null }; })())'));
  if (!placed.top || placed.box[0] < 0 || placed.box[1] < 0 || placed.box[2] > placed.width || placed.box[3] > placed.panelTop + 1) throw new Error('the overview is not over the page above the panel: ' + JSON.stringify(placed));
  await page.evaluate('document.querySelector(".terminal-overview-current").click()');
  await until('the overview to close on a choice', 'document.querySelector(".terminal-overview") === null');
  await collect();
  // On the Recent jobs page, whose styles are not the application page's, it is a dialog
  // over the page too, not a block under the jobs.
  await page.evaluate('location.href = "/jobs/"');
  await until('the panel on Recent jobs', 'document.querySelector(".terminal-bar") !== null && document.querySelector(".xterm-rows") !== null');
  await page.evaluate(barButton('All shells') + '.click()');
  await until('the overview on Recent jobs', 'document.querySelector(".terminal-overview-card") !== null');
  const jobsDialog = JSON.parse(await page.evaluate('JSON.stringify((() => { const dialog = document.querySelector(".terminal-overview-modal"); const box = document.querySelector(".terminal-overview").getBoundingClientRect(); return { position: getComputedStyle(dialog).position, top: box.top, bottom: box.bottom, height: innerHeight }; })())'));
  if (jobsDialog.position !== 'fixed' || jobsDialog.top < 0 || jobsDialog.bottom > jobsDialog.height) throw new Error('the overview on Recent jobs is not a dialog in view: ' + JSON.stringify(jobsDialog));
  await page.evaluate('document.activeElement.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }))');
  await until('the overview to close on Recent jobs', 'document.querySelector(".terminal-overview") === null');
  await collect();
}).catch((error) => { failure = error.stack || String(error); });
await close();
console.log(JSON.stringify({ failure, violations: [...violations] }));
process.exit(0);
`

// TestTerminalPanelInBrowser uses the real xterm.js in Chrome, signed in as a browser
// is: opening the panel, typing, following a link with the shell still there, a second
// tab, and up to four panes.
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
