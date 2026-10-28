package webui

import (
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// TestWebArtifactPreviewInBrowser loads the real page in jsdom against the
// real handler and opens each kind of preview the way a user would: the
// listing, an image, a log from its end, a table, a directory and its next
// page, a child whose name has a quote, and a file without a preview.
func TestWebArtifactPreviewInBrowser(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "many", "it's.txt"), "quoted name")
	write(t, filepath.Join(f.work, "NOTES"), "no extension\n")
	server := httptest.NewServer(Handler(testOptions(f.baseDir, false)))
	defer server.Close()
	script := `
const { JSDOM, VirtualConsole } = require('jsdom');
const base = process.argv[1];
const runID = process.argv[2];
const entries = JSON.parse(process.argv[3]);
const fail = (code, detail) => { console.error(code, detail || ''); process.exit(1); };
const wait = (ms) => new Promise(resolve => setTimeout(resolve, ms));
// settle waits until check passes, or for nine seconds, rather than for a
// fixed time that a loaded host can exceed; the check that follows it then
// reports the details of a timeout.
const settle = async (check) => {
  const passes = () => { try { return !!check(); } catch { return false; } };
  for (let tries = 0; tries < 300 && !passes(); tries++) await wait(30);
};
(async () => {
  const html = await (await fetch(base + '/')).text();
  const dom = new JSDOM(html, {
    runScripts: 'dangerously', url: base + '/', virtualConsole: new VirtualConsole(),
    beforeParse(window) {
      window.fetch = (input, init) => fetch(new URL(String(input), base), init);
      window.setInterval = () => 1;
    },
  });
  const window = dom.window;
  const document = window.document;
  await settle(() => window.eval('state') !== undefined);
  // A job without a record shows its text in the log box, like a log, so
  // the copy button sits where it does for logs.
  await window.showArtifacts('default', runID, 'job-2', '');
  const log = document.getElementById('modal-log');
  if (log.hidden || log.textContent !== 'Artifacts: (not recorded)' || !document.getElementById('artifact-view').hidden) fail('not recorded', log.textContent);
  await window.showArtifacts('default', runID, 'missing-job', '');
  if (log.hidden || !log.textContent.startsWith('Failed to load artifacts')) fail('failure', log.textContent);
  await window.showArtifacts('default', runID, 'job-1', '');
  const view = document.getElementById('artifact-view');
  if (view.hidden || document.getElementById('modal-log').hidden !== true) fail('view toggling');
  const rows = view.querySelectorAll('.artifact-table tbody tr');
  if (rows.length !== 14) fail('rows', rows.length);
  const buttons = [...view.querySelectorAll('.artifact-table .artifact-open')].map(button => button.textContent);
  if (buttons.some(text => text.includes('secret.txt'))) fail('outside entry is openable');
  if (buttons.length !== 11) fail('buttons', buttons);
  const workingDirectoryRow = rows[0];
  if (JSON.stringify([...workingDirectoryRow.children].slice(0, 3).map(cell => cell.textContent)) !== '["directory",".","working directory"]') fail('working directory row', workingDirectoryRow.textContent);
  // Each row has Download for a file it can open and launch buttons for a
  // file or directory it can open; other rows have empty action cells.
  const listingHeaders = [...view.querySelectorAll('.artifact-table thead th')].map(cell => cell.textContent);
  if (JSON.stringify(listingHeaders) !== '["Type","Path","Found in","Download","Launch"]') fail('listing headers', listingHeaders);
  const rowActions = (name) => {
    const row = [...rows].find(row => row.children[1].textContent.endsWith(name));
    return [!!row.querySelector('.artifact-download'), !!row.querySelector('.artifact-launchers')];
  };
  for (const [name, want] of [['.', '[false,true]'], ['plot.png', '[true,true]'], ['many', '[false,true]'], ['secret.txt', '[false,false]'], ['gone.csv', '[false,false]']]) {
    if (JSON.stringify(rowActions(name)) !== want) fail('row actions of ' + name, rowActions(name));
  }
  const plotDownload = [...rows].find(row => row.children[1].textContent === 'plot.png').querySelector('.artifact-download');
  if (!plotDownload.getAttribute('href').includes('download=1') || !plotDownload.hasAttribute('download')) fail('row download link', plotDownload.outerHTML);

  const preview = document.getElementById('artifact-preview');
  let scrolls = 0;
  preview.scrollIntoView = options => {
    if (options.block !== 'start' || options.inline !== 'nearest') fail('preview scroll alignment', options);
    if (preview.textContent.includes('Loading...')) fail('preview scrolled before rendering');
    scrolls++;
  };
  const openArtifact = window.openArtifact;
  window.openArtifact = async (...args) => {
    const before = scrolls;
    await openArtifact(...args);
    if (scrolls !== before + 1) fail('preview not scrolled into view', args);
  };

  await window.openArtifact('wd', '', 'directory');
  if (![...preview.querySelectorAll('.artifact-open')].some(button => button.textContent === 'plot.png')) fail('working directory preview', preview.textContent);
  await window.openArtifact('wd', 'data.csv', 'file');
  if ([...preview.querySelectorAll('tbody td')].map(cell => cell.textContent).join() !== '1,2') fail('working directory child', preview.textContent);

  await window.openArtifact(entries['plot.png'], '', 'file');
  const image = view.querySelector('.artifact-image');
  if (!image) fail('image');
  const imageResponse = await fetch(new URL(image.getAttribute('src'), base));
  if (imageResponse.status !== 200 || imageResponse.headers.get('content-type') !== 'image/png') fail('image response', imageResponse.status);
  if (!view.querySelector('.artifact-download').getAttribute('href').includes('download=1')) fail('download link');

  await window.openArtifact(entries['run.log'], '', 'file');
  let text = view.querySelector('.artifact-text').textContent;
  if (!text.endsWith('line 19999\n') || text.startsWith('line 00000')) fail('log tail', text.slice(0, 20));
  const earlier = [...view.querySelectorAll('button')].find(button => button.textContent === 'Load earlier');
  if (!earlier) fail('load earlier');
  const beforeEarlier = scrolls;
  await window.moreArtifactText();
  if (scrolls !== beforeEarlier) fail('pagination moved preview');
  const longer = view.querySelector('.artifact-text').textContent;
  if (longer.length <= text.length || !longer.endsWith(text)) fail('earlier page');

  await window.openArtifact(entries['data.csv'], '', 'file');
  const headers = [...view.querySelectorAll('.artifact-preview thead th')].map(cell => cell.textContent);
  const cells = [...view.querySelectorAll('.artifact-preview tbody td')].map(cell => cell.textContent);
  if (JSON.stringify(headers) !== '["a","b"]' || JSON.stringify(cells) !== '["1","2"]') fail('table', JSON.stringify({ headers, cells }));

  await window.openArtifact(entries['many'], '', 'directory');
  let children = view.querySelectorAll('.artifact-preview tbody tr');
  if (children.length !== 200) fail('directory page', children.length);
  const childHeaders = [...view.querySelectorAll('.artifact-preview thead th')].map(cell => cell.textContent);
  if (JSON.stringify(childHeaders) !== '["Type","Name","Size","Modified","Download","Launch"]') fail('directory headers', childHeaders);
  // Directories come first; a directory child launches but does not download.
  const fileChild = [...children].find(row => row.children[0].textContent === 'file');
  const dirChild = [...children].find(row => row.children[0].textContent === 'directory');
  if (!fileChild.querySelector('.artifact-download') || !fileChild.querySelector('.artifact-launchers')) fail('file child actions', fileChild.innerHTML);
  if (dirChild.querySelector('.artifact-download') || !dirChild.querySelector('.artifact-launchers')) fail('directory child actions', dirChild.innerHTML);
  const more = [...view.querySelectorAll('.artifact-preview button')].find(button => button.textContent === 'More');
  const beforeMore = scrolls;
  more.click();
  await settle(() => view.querySelectorAll('.artifact-preview tbody tr').length === 53);
  if (scrolls !== beforeMore) fail('directory pagination moved preview');
  children = view.querySelectorAll('.artifact-preview tbody tr');
  if (children.length !== 53) fail('second directory page', children.length);
  const quoted = [...view.querySelectorAll('.artifact-preview .artifact-open')].find(button => button.textContent === "it's.txt");
  if (!quoted) fail('quoted child');
  // A click starts an open that is not awaited; let it finish, scroll
  // included, before the next open, or the two scrolls overlap.
  const beforeQuoted = scrolls;
  quoted.click();
  await settle(() => scrolls === beforeQuoted + 1);
  if (!view.querySelector('.artifact-preview-title').textContent.includes("many/it's.txt")) fail('quoted child title', view.querySelector('.artifact-preview').textContent);

  await window.openArtifact(entries['weights.npy'], '', 'file');
  const arrayText = view.querySelector('.artifact-preview').textContent;
  if (!arrayText.includes('<f4 shape (3,), 3 values') || view.querySelector('.artifact-values').textContent !== '[1, 2, 3]') fail('array', arrayText);

  for (const [name, tag, type] of [['clip.wav', 'audio', 'audio/wav'], ['clip.mp4', 'video', 'video/mp4']]) {
    await window.openArtifact(entries[name], '', 'file');
    const media = view.querySelector('.artifact-preview ' + tag + '[controls]');
    if (!media) fail(tag, view.querySelector('.artifact-preview').innerHTML);
    const mediaResponse = await fetch(new URL(media.getAttribute('src'), base));
    if (mediaResponse.status !== 200 || mediaResponse.headers.get('content-type') !== type) fail(tag + ' response', mediaResponse.status);
  }

  await window.openArtifact(entries['model.pt'], '', 'file');
  if (!view.querySelector('.artifact-preview').textContent.includes('No preview for this file type. 9 B')) fail('no preview', view.querySelector('.artifact-preview').textContent);
  // A file of no listed type opens as text on request; a binary one is
  // refused.
  view.querySelector('.artifact-open-text').click();
  await settle(() => view.querySelector('.artifact-preview').textContent.includes('Failed to open'));
  if (!view.querySelector('.artifact-preview').textContent.includes('Failed to open: binary file')) fail('binary as text', view.querySelector('.artifact-preview').textContent);
  await window.openArtifact('wd', 'NOTES', 'file');
  view.querySelector('.artifact-open-text').click();
  await settle(() => view.querySelector('.artifact-text'));
  if (view.querySelector('.artifact-text')?.textContent !== 'no extension\n') fail('text without an extension', view.querySelector('.artifact-preview').innerHTML);
  process.exit(0);
})().catch(error => fail('error', error.stack));
`
	entries := "{"
	for name, index := range f.entries {
		if len(entries) > 1 {
			entries += ","
		}
		entries += strconv.Quote(name) + ":" + strconv.Itoa(index)
	}
	entries += "}"
	command := exec.Command("node", "-e", script, server.URL, f.runID, entries)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("browser preview: %v\n%s", err, output)
	}
}
