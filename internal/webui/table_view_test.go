package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/tableview"
)

// viewerGet requests a Table viewer route for a fixture entry, or for the
// working directory, "wd".
func (f artifactFixture) viewerGet(t *testing.T, options Options, route, name string, extra url.Values) *httptest.ResponseRecorder {
	t.Helper()
	if name == "wd" {
		extra = maps.Clone(extra)
		if extra == nil {
			extra = url.Values{}
		}
		extra.Set("entry", "wd")
	}
	return f.get(t, options, "/tableview/"+route, name, extra)
}

func TestTableViewReadsAnArtifact(t *testing.T) {
	f := newArtifactFixture(t)
	options := testOptions(f.baseDir, false)

	recorder := f.viewerGet(t, options, "api/info", "data.csv", nil)
	var info tableViewInfo
	if err := json.Unmarshal(recorder.Body.Bytes(), &info); err != nil {
		t.Fatalf("info = %d %s", recorder.Code, recorder.Body)
	}
	if info.Kind != "file" || info.Separator != "," || info.Size != int64(len("a,b\n1,2\n")) || !strings.HasPrefix(info.Download, "../api/artifact-file?") {
		t.Fatalf("info = %+v", info)
	}
	// Media paths resolve from the basedir by default, then the job's
	// working directory, then the file's own directory.
	var bases []string
	for _, base := range info.Bases {
		bases = append(bases, base.ID)
	}
	if !reflect.DeepEqual(bases, []string{"basedir", "wd", tableview.FileBase}) {
		t.Fatalf("bases = %q", bases)
	}
	// The download link is the artifact's own download.
	link, err := url.Parse("/tableview/" + info.Download)
	if err != nil {
		t.Fatal(err)
	}
	download := httptest.NewRecorder()
	Handler(options).ServeHTTP(download, httptest.NewRequest(http.MethodGet, link.ResolveReference(&url.URL{}).String(), nil))
	if download.Code != http.StatusOK || download.Body.String() != "a,b\n1,2\n" {
		t.Fatalf("download = %d %q", download.Code, download.Body)
	}

	for header, want := range map[string][][]string{"1": {{"1", "2"}}, "0": {{"a", "b"}, {"1", "2"}}} {
		recorder = f.viewerGet(t, options, "api/rows", "data.csv", url.Values{"sep": {","}, "header": {header}})
		var page tableview.Page
		if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
			t.Fatalf("rows = %d %s", recorder.Code, recorder.Body)
		}
		var rows [][]string
		for _, row := range page.Rows {
			rows = append(rows, row.Cells)
		}
		if !reflect.DeepEqual(rows, want) {
			t.Fatalf("header=%s rows = %q, want %q", header, rows, want)
		}
	}

	page := f.viewerGet(t, options, "", "data.csv", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `src="table_view.js"`) || !strings.Contains(page.Body.String(), `href="/web_tokens.css"`) {
		t.Fatalf("page = %d %s", page.Code, page.Body)
	}
	for _, asset := range []string{"table_view.js", "table_view.css"} {
		recorder := httptest.NewRecorder()
		Handler(options).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/tableview/"+asset, nil))
		if recorder.Code != http.StatusOK || recorder.Body.Len() == 0 {
			t.Fatalf("%s = %d", asset, recorder.Code)
		}
	}
}

func TestTableViewRefusesWhatContentRequestsRefuse(t *testing.T) {
	f := newArtifactFixture(t)
	options := testOptions(f.baseDir, false)
	for _, test := range []struct {
		name   string
		extra  url.Values
		status int
	}{
		{"secret.txt", url.Values{"sep": {","}}, http.StatusForbidden}, // outside every root
		{"link.csv", url.Values{"sep": {","}}, http.StatusForbidden},   // a symlink leaving the root
		{"gone.csv", url.Values{"sep": {","}}, http.StatusNotFound},    // missing
		{"data.csv", url.Values{"sep": {""}}, http.StatusBadRequest},   // no separator
		{"data.csv", url.Values{"sep": {",;"}}, http.StatusBadRequest}, // two characters
		{"data.csv", url.Values{"sep": {`"`}}, http.StatusBadRequest},  // a quote
		{"data.csv", url.Values{"sep": {","}, "header": {"yes"}}, http.StatusBadRequest},
		{"data.csv", url.Values{"sep": {","}, "limit": {"5000"}}, http.StatusBadRequest},
		{"data.csv", url.Values{"sep": {","}, "offset": {"x"}}, http.StatusBadRequest},
		{"many", url.Values{"sep": {","}}, http.StatusBadRequest}, // a directory read as a file
		{"data.csv", url.Values{"sep": {","}, "expanded": {"sub"}}, http.StatusBadRequest},
	} {
		if recorder := f.viewerGet(t, options, "api/rows", test.name, test.extra); recorder.Code != test.status {
			t.Errorf("%s %v = %d %s, want %d", test.name, test.extra, recorder.Code, recorder.Body, test.status)
		}
	}
	if recorder := f.viewerGet(t, options, "", "secret.txt", nil); recorder.Code != http.StatusForbidden {
		t.Errorf("page of an unserved entry = %d", recorder.Code)
	}
}

// The Table viewer is offered for the files it reads and any directory, on
// every live server: it runs no command, so it needs no --allow-exec.
func TestLaunchersListTheTableViewer(t *testing.T) {
	f := newArtifactFixture(t)
	for name, options := range map[string]Options{
		"read-only":         testOptions(f.baseDir, false),
		"without launch":    testOptions(f.baseDir, true),
		"with --allow-exec": launchOptions(t, f.baseDir, launchCommand("Cat", "cat", "file")),
	} {
		for query, want := range map[string]int{
			"kind=file&name=data.csv": 1, "kind=file&name=wav.scp": 1, "kind=file&name=run.log": 0,
			"kind=directory&name=many": 1, "": 2,
		} {
			recorder := httptest.NewRecorder()
			Handler(options).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/launchers?"+query, nil))
			var availability launchAvailability
			if err := json.Unmarshal(recorder.Body.Bytes(), &availability); err != nil {
				t.Fatal(err)
			}
			if len(availability.Viewers) != want || (want > 0 && (availability.Viewers[0].Name != "Table viewer" || availability.Viewers[0].Path != "/tableview/")) {
				t.Errorf("%s, %s: viewers = %+v, want %d", name, query, availability.Viewers, want)
			}
		}
	}
}

// runTableViewScript runs script in jsdom on a server, with the Table
// viewer page of entry name open.
func runTableViewScript(t *testing.T, options Options, f artifactFixture, name, fragment, script string) {
	t.Helper()
	server := httptest.NewServer(Handler(options))
	defer server.Close()
	entry := name // the working directory, "wd"
	if index, ok := f.entries[name]; ok {
		entry = strconv.Itoa(index)
	}
	query := url.Values{"project_name": {"default"}, "run_id": {f.runID}, "job_id": {"job-1"}, "entry": {entry}}
	runTableViewPage(t, server.URL+"/tableview/?"+query.Encode()+fragment, script)
}

// runTableViewPage runs script in jsdom with the Table viewer page at
// pageURL open.
func runTableViewPage(t *testing.T, pageURL, script string) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	command := exec.Command("node", "-e", `
const { JSDOM, VirtualConsole } = require('jsdom');
const pageURL = process.argv[1];
const fail = (code, detail) => { console.error(code, detail || ''); process.exit(1); };
const wait = (ms) => new Promise(resolve => setTimeout(resolve, ms));
(async () => {
  const errors = [];
  const virtualConsole = new VirtualConsole();
  virtualConsole.on('jsdomError', error => errors.push(error.stack || String(error)));
  const dom = await JSDOM.fromURL(pageURL, {
    runScripts: 'dangerously', resources: 'usable', virtualConsole,
    // jsdom has no fetch; relative URLs resolve against the page, as in a browser.
    beforeParse(window) {
      window.fetch = (input, init) => fetch(new URL(String(input), window.location.href), init);
      // Nor Response, which a static snapshot answers with.
      window.Response = Response;
    },
  });
  const window = dom.window;
  const document = window.document;
  const until = async (what, check) => {
    for (let tries = 0; tries < 100; tries++) {
      if (check()) return;
      await wait(30);
    }
    fail('timed out waiting for ' + what, document.body.innerHTML.slice(0, 2000) + '\n' + errors.join('\n'));
  };
  const cells = () => [...document.querySelectorAll('#tv-table tbody tr')].map(row => [...row.children].map(cell => cell.textContent));
  const headings = () => [...document.querySelectorAll('#tv-table thead th')].map(cell => cell.textContent);
  const fragment = () => new window.URLSearchParams(window.location.hash.slice(1));
`+script+`
  if (errors.length) fail('page errors', errors.join('\n'));
})().catch(error => fail('exception', error.stack));
`, pageURL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("browser check failed: %v\n%s", err, output)
	}
}

func TestTableViewPagesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	var content strings.Builder
	content.WriteString("n|text\n")
	for n := 1; n <= 120; n++ {
		fmt.Fprintf(&content, "%d|row %d\n", n, n)
	}
	write(t, filepath.Join(f.work, "data.csv"), content.String())
	options := testOptions(f.baseDir, false)
	// The fragment chooses the separator and the page size.
	runTableViewScript(t, options, f, "data.csv", "#sep=%7C&limit=50", `
  await until('rows', () => cells().length === 50);
  if (JSON.stringify(headings()) !== '["#","n","text"]') fail('headings', headings());
  if (JSON.stringify(cells()[0]) !== '["1","1","row 1"]') fail('first row', cells()[0]);
  const count = () => document.querySelector('.tv-pager-count').textContent;
  if (!count().includes('Rows 1–50 of 120')) fail('count', count());
  if (!document.getElementById('tv-summary').textContent.includes('120 rows · 2 columns')) fail('summary', document.getElementById('tv-summary').textContent);

  // Last goes to page 3 of 3, and the fragment keeps it.
  [...document.querySelectorAll('.tv-pager-buttons button')].find(button => button.textContent === 'Last »').click();
  await until('last page', () => cells().length === 20);
  if (JSON.stringify(cells()[19]) !== '["120","120","row 120"]') fail('last row', cells()[19]);
  if (fragment().get('page') !== '3') fail('fragment page', window.location.hash);

  // Another separator reads the file differently and starts over at page 1.
  const select = document.querySelector('.tv-toolbar select');
  select.value = ',';
  select.dispatchEvent(new window.Event('change'));
  await until('one column', () => headings().length === 2);
  if (JSON.stringify(headings()) !== '["#","n|text"]' || fragment().get('page') !== '1' || fragment().get('sep') !== ',') fail('comma', headings(), window.location.hash);

  // A separator that is not one character is refused with an explanation.
  select.value = 'other';
  select.dispatchEvent(new window.Event('change'));
  const other = document.querySelector('.tv-separator-other');
  if (other.hidden) fail('other input hidden');
  other.value = '||';
  other.dispatchEvent(new window.Event('change'));
  if (!document.getElementById('tv-error').textContent.includes('one character')) fail('two-character separator', document.getElementById('tv-error').textContent);
  other.value = '|';
  other.dispatchEvent(new window.Event('change'));
  await until('pipe again', () => headings().length === 3);
`)
}

func TestLaunchWithOpensTheTableViewerInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	runLaunchBrowserScript(t, testOptions(f.baseDir, false), f, `
  await window.showArtifacts('default', runID, 'job-1', '');
  const row = [...document.querySelectorAll('.artifact-table tbody tr')].find(row => row.children[1].textContent === 'data.csv');
  await settle(() => row.querySelector('.artifact-launchers a.launch-viewer'));
  const link = row.querySelector('.artifact-launchers a.launch-viewer');
  if (!link || link.textContent !== 'Table viewer' || link.target !== '_blank') fail('viewer link', row.innerHTML);
  const href = new URL(link.getAttribute('href'), base);
  if (href.pathname !== '/tableview/' || href.searchParams.get('entry') !== String(entries['data.csv']) || href.searchParams.get('job_id') !== 'job-1') fail('viewer href', link.getAttribute('href'));
  // Launching is off on this server: the viewer is offered, and the view
  // says why commands are not.
  const note = document.querySelector('#artifact-view .launch-unavailable-note')?.textContent || '';
  if (!note.includes('Launching is disabled')) fail('reason', note);
  const page = await fetch(href);
  if (page.status !== 200) fail('viewer page', page.status);
`)
}

func TestTableViewFiltersSortsAndDescribes(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "data.csv"), "id,score\na,3\nb,1\nc,x\nd,2\n")
	options := testOptions(f.baseDir, false)
	filters := `[{"column":1,"op":">=","value":"2"}]`
	recorder := f.viewerGet(t, options, "api/rows", "data.csv", url.Values{"sep": {","}, "filters": {filters}, "sort": {"1"}, "desc": {"1"}})
	var page tableview.Page
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("rows = %d %s", recorder.Code, recorder.Body)
	}
	var got []string
	for _, row := range page.Rows {
		got = append(got, row.Cells[0])
	}
	if !reflect.DeepEqual(got, []string{"a", "d"}) || page.Matched != 2 || page.Total != 4 {
		t.Fatalf("rows = %q matched %d of %d", got, page.Matched, page.Total)
	}
	recorder = f.viewerGet(t, options, "api/describe", "data.csv", url.Values{"sep": {","}, "filters": {filters}})
	var description tableview.Description
	if err := json.Unmarshal(recorder.Body.Bytes(), &description); err != nil {
		t.Fatalf("describe = %d %s", recorder.Code, recorder.Body)
	}
	if description.Matched != 2 || *description.Columns[1].Sum != 5 || description.Columns[0].Unique != 2 {
		t.Fatalf("describe = %s", recorder.Body)
	}
	for _, extra := range []url.Values{
		{"filters": {`{"column":1}`}},
		{"filters": {`[{"column":9,"op":"="}]`}},
		{"filters": {`[{"column":1,"op":">","value":"high"}]`}},
		{"sort": {"x"}},
		{"sort": {"1"}, "desc": {"yes"}},
		{"desc": {"1"}},
	} {
		extra.Set("sep", ",")
		if recorder := f.viewerGet(t, options, "api/rows", "data.csv", extra); recorder.Code != http.StatusBadRequest {
			t.Errorf("%v = %d %s", extra, recorder.Code, recorder.Body)
		}
	}
}

func TestTableViewFiltersInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "data.csv"), "id,score,text\na,0.9,meeting room\nb,0.75,notes\nc,,thanks\nd,10,meeting\n")
	runTableViewScript(t, testOptions(f.baseDir, false), f, "data.csv", "", `
  await until('rows', () => cells().length === 4);
  const bar = document.getElementById('tv-filterbar');
  const [column, operator] = bar.querySelectorAll('select');
  const value = bar.querySelector('.tv-filter-value');
  const add = [...bar.querySelectorAll('button')].find(button => button.textContent === '+ Add');
  column.value = '1'; column.dispatchEvent(new window.Event('change'));
  operator.value = '>='; operator.dispatchEvent(new window.Event('change'));
  value.value = '0.8'; value.dispatchEvent(new window.Event('input'));
  add.click();
  await until('filtered', () => cells().length === 2);
  if (JSON.stringify(cells().map(row => row[1])) !== '["a","d"]') fail('filtered rows', cells());
  const chip = () => document.querySelector('.tv-chip');
  if (!chip() || !chip().textContent.includes('score') || !chip().textContent.includes('≥') || !chip().textContent.includes('0.8')) fail('chip', chip() && chip().textContent);
  if (!document.querySelector('.tv-pager-count').textContent.includes('of 2 matching (4 in all)')) fail('count', document.querySelector('.tv-pager-count').textContent);
  if (JSON.parse(fragment().get('filters'))[0].value !== '0.8') fail('fragment filters', window.location.hash);

  // Describe summarizes the matching rows under the column names.
  document.querySelector('.tv-describe-toggle').click();
  await until('describe', () => document.querySelectorAll('tr.tv-describe').length === 12);
  const stat = name => [...document.querySelectorAll('tr.tv-describe')].find(row => row.children[0].textContent === name);
  if (stat('count').children[2].textContent !== '2' || stat('sum').children[2].textContent !== '10.9' || stat('mean').children[1].textContent !== '–') fail('describe', [...document.querySelectorAll('tr.tv-describe')].map(row => row.textContent));
  if (fragment().get('describe') !== '1') fail('fragment describe', window.location.hash);

  // A column heading sorts, ascending and then descending.
  const sortBy = () => [...document.querySelectorAll('#tv-table thead tr:first-child .tv-sort')][1];
  sortBy().click();
  await until('ascending', () => cells()[0] && cells()[0][1] === 'a');
  sortBy().click();
  await until('descending', () => cells()[0] && cells()[0][1] === 'd');
  if (document.querySelectorAll('#tv-table thead th')[2].getAttribute('aria-sort') !== 'descending' || fragment().get('desc') !== '1') fail('sort state', window.location.hash);

  // Removing the condition shows every row again; a bad value is explained.
  chip().querySelector('button').click();
  await until('unfiltered', () => cells().length === 4);
  const bar2 = document.getElementById('tv-filterbar');
  const [, operator2] = bar2.querySelectorAll('select');
  operator2.value = '>'; operator2.dispatchEvent(new window.Event('change'));
  const value2 = bar2.querySelector('.tv-filter-value');
  value2.value = 'high'; value2.dispatchEvent(new window.Event('input'));
  [...bar2.querySelectorAll('button')].find(button => button.textContent === '+ Add').click();
  await until('error', () => document.getElementById('tv-error').textContent.includes('needs a number'));
`)
}

func TestTableViewDescribesAndServesMedia(t *testing.T) {
	f := newArtifactFixture(t)
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(f.baseDir, "audio", "a.wav"), string(wav))
	write(t, filepath.Join(f.work, "audio", "a.wav"), string(wav[:44]))
	write(t, filepath.Join(f.work, "data.csv"), "id,audio\n1,audio/a.wav\n2,"+filepath.Join(f.outside, "secret.wav")+"\n")
	write(t, filepath.Join(f.outside, "secret.wav"), string(wav))
	options := testOptions(f.baseDir, false)
	media := func(base string) []*tableview.Media {
		t.Helper()
		recorder := f.viewerGet(t, options, "api/rows", "data.csv", url.Values{"sep": {","}, "base": {base}})
		var page tableview.Page
		if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
			t.Fatalf("rows = %d %s", recorder.Code, recorder.Body)
		}
		return []*tableview.Media{page.Rows[0].Media[1], page.Rows[1].Media[1]}
	}
	// By default relative paths resolve from the basedir.
	got := media("")
	if got[0].Properties == nil || got[0].Properties.Duration != 0.5 || got[1].Error != "Not shown: outside the allowed roots" {
		t.Fatalf("basedir media = %+v %+v", got[0], got[1])
	}
	// From the working directory the same path names a file with no samples.
	if got := media("wd"); got[0].Properties == nil || got[0].Properties.Duration != 0 || got[0].Base != "wd" {
		t.Fatalf("working directory media = %+v", got[0])
	}
	if recorder := f.viewerGet(t, options, "api/rows", "data.csv", url.Values{"sep": {","}, "base": {"elsewhere"}}); recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown base = %d", recorder.Code)
	}

	served := f.viewerGet(t, options, "api/media", "data.csv", url.Values{"ref": {"audio/a.wav"}})
	if served.Code != http.StatusOK || served.Body.Len() != len(wav) || served.Header().Get("Content-Type") != "audio/wav" ||
		served.Header().Get("Cache-Control") != "no-store" || !strings.Contains(served.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("media = %d %v", served.Code, served.Header())
	}
	for _, test := range []struct {
		ref, base string
		status    int
	}{
		{filepath.Join(f.outside, "secret.wav"), "", http.StatusForbidden}, // outside every root
		{"../../" + filepath.Base(f.outside) + "/secret.wav", "wd", http.StatusForbidden},
		{"data.csv", "file", http.StatusBadRequest}, // not media
		{"audio/gone.wav", "", http.StatusNotFound}, // missing
		{"audio/a.wav", "elsewhere", http.StatusBadRequest},
	} {
		if recorder := f.viewerGet(t, options, "api/media", "data.csv", url.Values{"ref": {test.ref}, "base": {test.base}}); recorder.Code != test.status {
			t.Errorf("%s from %q = %d %s, want %d", test.ref, test.base, recorder.Code, recorder.Body, test.status)
		}
	}
	// A directory is neither media nor text, even named like media.
	if err := os.MkdirAll(filepath.Join(f.baseDir, "audio", "frames.wav"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"api/media", "api/text"} {
		recorder := f.viewerGet(t, options, route, "data.csv", url.Values{"ref": {"audio/frames.wav"}})
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "audio/frames.wav is not a regular file") {
			t.Errorf("%s of a directory = %d %s", route, recorder.Code, recorder.Body)
		}
	}
}

func TestTableViewShowsMediaInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(f.work, "audio", "a.wav"), string(wav))
	write(t, filepath.Join(f.work, "data.csv"), "id,audio\n1,audio/a.wav\n2,audio/gone.wav\n")
	runTableViewScript(t, testOptions(f.baseDir, false), f, "data.csv", "", `
  await until('rows', () => cells().length === 2);
  // From the basedir neither file exists.
  if (!cells()[0][2].includes('File not found')) fail('basedir', cells()[0]);
  const base = document.querySelector('select[aria-label="Resolve media paths from"]');
  if ([...base.options].map(option => option.value).join() !== 'basedir,wd,file') fail('bases', base.innerHTML);
  base.value = 'wd';
  base.dispatchEvent(new window.Event('change'));
  await until('working directory', () => cells()[0][2].includes('WAV PCM 16-bit · 0:00.50 · 16 kHz · 1 ch · 256 kbps · 15.7 KiB'));
  const link = document.querySelector('#tv-table tbody a.tv-media-path');
  const href = new URL(link.href);
  if (href.pathname !== '/tableview/api/media' || href.searchParams.get('ref') !== 'audio/a.wav' || href.searchParams.get('base') !== 'wd') fail('media link', link.href);
  if ((await fetch(href)).status !== 200) fail('media fetch');
  if (!cells()[1][2].includes('File not found') || fragment().get('base') !== 'wd') fail('missing file', cells()[1], window.location.hash);
`)
}

func TestTableViewListsADirectory(t *testing.T) {
	f := newArtifactFixture(t)
	options := testOptions(f.baseDir, false)
	recorder := f.viewerGet(t, options, "api/info", "many", nil)
	var info tableViewInfo
	if err := json.Unmarshal(recorder.Body.Bytes(), &info); err != nil || info.Kind != "directory" || info.Download != "" || len(info.Bases) != 1 || info.Bases[0].ID != tableview.DirectoryBase {
		t.Fatalf("info = %d %s", recorder.Code, recorder.Body)
	}
	recorder = f.viewerGet(t, options, "api/rows", "many", url.Values{"pattern": {"f00*.txt"}})
	var page tableview.Page
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("rows = %d %s", recorder.Code, recorder.Body)
	}
	if page.Total != 11 || page.Rows[0].Cells[0] != "f000.txt" || page.Rows[0].Cells[8] != "1" {
		t.Fatalf("rows = %+v", page)
	}
	// The working directory lists its media with their properties; the
	// symlink out of it is not followed.
	recorder = f.viewerGet(t, options, "api/rows", "wd", url.Values{"pattern": {"**/*.{wav,png}"}})
	page = tableview.Page{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("working directory rows = %d %s", recorder.Code, recorder.Body)
	}
	var paths []string
	for _, row := range page.Rows {
		paths = append(paths, row.Cells[0])
	}
	if !reflect.DeepEqual(paths, []string{"clip.wav", "many/", "plot.png"}) || page.Rows[0].Media[0] == nil || page.Rows[0].Media[0].Properties == nil {
		t.Fatalf("working directory = %q %s", paths, recorder.Body)
	}
	if served := f.viewerGet(t, options, "api/media", "wd", url.Values{"ref": {"plot.png"}, "base": {tableview.DirectoryBase}}); served.Code != http.StatusOK || served.Body.String() != "\x89PNG-bytes" {
		t.Fatalf("media = %d %q", served.Code, served.Body)
	}
	for _, extra := range []url.Values{
		{"sep": {","}},     // a file's parameter
		{"view": {"tree"}}, // no such view
		{"view": {"stems"}, "group": {"color"}},
		{"view": {"files"}, "incomplete": {"1"}},
		{"pattern": {"["}},
		{"expanded": {""}},
		{"expanded": {"../outside"}},
		{"expanded": {"zz-sub/missing"}},
		{"expanded": {"escape"}},
		{"expanded": {"f000.txt"}},
		{"base": {"basedir"}}, // a file's base
	} {
		if recorder := f.viewerGet(t, options, "api/rows", "many", extra); recorder.Code != http.StatusBadRequest {
			t.Errorf("%v = %d %s", extra, recorder.Code, recorder.Body)
		}
	}
	if recorder := f.viewerGet(t, options, "api/rows", "many", url.Values{"incomplete": {"yes"}}); recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "incomplete must be 1 or 0") {
		t.Errorf("incomplete=yes = %d %s", recorder.Code, recorder.Body)
	}
	if recorder := f.viewerGet(t, options, "api/rows", "data.csv", url.Values{"sep": {","}, "view": {"files"}}); recorder.Code != http.StatusBadRequest {
		t.Errorf("a directory's parameter on a file = %d", recorder.Code)
	}
}

// rotari tableview serves one path; a directory's child names a file or
// directory under it, read under the same root.
func TestTableViewHandlerServesOnePath(t *testing.T) {
	data, outside := t.TempDir(), t.TempDir()
	write(t, filepath.Join(data, "list.csv"), "x\n1\n")
	write(t, filepath.Join(data, "sub", "more.csv"), "y\n2\n")
	write(t, filepath.Join(outside, "other.csv"), "z\n3\n")
	if _, err := TableViewHandler(filepath.Join(data, "missing.csv")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing path = %v", err)
	}
	get := func(handler http.Handler, target string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		return recorder
	}
	info := func(handler http.Handler, child string) tableViewInfo {
		t.Helper()
		recorder := get(handler, "/api/info?"+url.Values{"child": {child}}.Encode())
		var info tableViewInfo
		if err := json.Unmarshal(recorder.Body.Bytes(), &info); err != nil {
			t.Fatalf("info of %q = %d %s", child, recorder.Code, recorder.Body)
		}
		return info
	}

	directory, err := TableViewHandler(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := info(directory, ""); got.Kind != "directory" || got.Name != data {
		t.Fatalf("directory = %+v", got)
	}
	if got := info(directory, "sub"); got.Kind != "directory" || got.Name != data+"/sub" {
		t.Fatalf("child directory = %+v", got)
	}
	if got := info(directory, "sub/more.csv"); got.Kind != "file" || got.Name != data+"/sub/more.csv" || got.Separator != "," {
		t.Fatalf("child file = %+v", got)
	}
	for child, status := range map[string]int{
		"../" + filepath.Base(outside) + "/other.csv": http.StatusBadRequest,
		filepath.Join(outside, "other.csv"):           http.StatusBadRequest,
		"sub/../list.csv":                             http.StatusBadRequest,
		"missing.csv":                                 http.StatusNotFound,
	} {
		if recorder := get(directory, "/api/info?"+url.Values{"child": {child}}.Encode()); recorder.Code != status {
			t.Errorf("child %s = %d %s, want %d", child, recorder.Code, recorder.Body, status)
		}
	}
	for _, asset := range []string{"/web_tokens.css", "/web_fonts.css"} {
		if recorder := get(directory, asset); recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/css") || recorder.Body.Len() == 0 {
			t.Fatalf("%s = %d %v", asset, recorder.Code, recorder.Header())
		}
	}

	file, err := TableViewHandler(filepath.Join(data, "list.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info(file, ""); got.Kind != "file" || got.Size != int64(len("x\n1\n")) {
		t.Fatalf("file = %+v", got)
	}
	if recorder := get(file, "/api/info?child=list.csv"); recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "child applies only to a directory") {
		t.Fatalf("a file's child = %d %s", recorder.Code, recorder.Body)
	}
}

func TestTableViewExpandsDirectoriesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	for _, name := range []string{"root.tree", "tree/child.tree", "tree/deep/grand.tree"} {
		write(t, filepath.Join(f.work, filepath.FromSlash(name)), "x")
	}
	if err := os.Mkdir(filepath.Join(f.work, "tree", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=**%2F*.tree&sort=0&desc=1", `
  const button = label => document.querySelector('button[aria-label="' + label + '"]');
  const hasFile = path => cells().some(row => row[1] === path);
  await until('collapsed tree', () => cells().length === 3 && button('Expand tree'));
  if (hasFile('tree/child.tree') || hasFile('tree/deep/grand.tree')) fail('read collapsed files', cells());
  const originalFetch = window.fetch;
  let denyExpansion = true;
  window.fetch = (input, init) => {
    const url = new window.URL(input, window.location.href);
    if (denyExpansion && url.pathname.endsWith('/api/rows') && url.searchParams.has('expanded')) {
      denyExpansion = false;
      return Promise.resolve({ ok: false, status: 403, text: async () => 'Cannot read tree' });
    }
    return originalFetch(input, init);
  };
  button('Expand tree').click();
  await until('failed expansion restored', () => button('Expand tree') && document.getElementById('tv-error').textContent === 'Cannot read tree');
  if (fragment().has('expanded')) fail('failed expansion retained', window.location.hash);
  button('Expand tree').click();
  await until('expanded tree', () => cells().length === 6 && button('Collapse tree'));
  if (!hasFile('tree/child.tree') || hasFile('tree/deep/grand.tree')) fail('direct children only', cells());
  if (fragment().get('sort') !== '0' || fragment().get('desc') !== '1') fail('expansion cleared sort');
  button('Expand tree/deep').click();
  await until('expanded child', () => hasFile('tree/deep/grand.tree'));
  if (fragment().getAll('expanded').join() !== 'tree,tree/deep') fail('expanded fragment', window.location.hash);
  document.querySelector('.tv-describe-toggle').click();
  await until('expanded describe', () => [...document.querySelectorAll('tr.tv-describe')].some(row => row.textContent.includes('count7')));
  button('Expand tree/empty').click();
  await until('expanded empty directory', () => button('Collapse tree/empty'));
  if (cells().length !== 7) fail('empty directory rows', cells());
  [...document.querySelectorAll('.tv-segmented button')].find(button => button.textContent === 'Group by stem').click();
  await until('expanded stems', () => headings().includes('stem') && cells().length === 3);
  [...document.querySelectorAll('.tv-segmented button')].find(button => button.textContent === 'Files').click();
  await until('files again', () => button('Collapse tree'));
  button('Collapse tree').click();
  await until('collapsed again', () => cells().length === 3 && button('Expand tree'));
  if (fragment().has('expanded')) fail('descendant expansion retained', window.location.hash);
  button('Expand tree').click();
  await until('reopened parent', () => cells().length === 6 && button('Expand tree/deep'));
  if (hasFile('tree/deep/grand.tree')) fail('child reopened automatically');
`)
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=**%2F*.tree&expanded=tree&expanded=tree%2Fdeep", `
  await until('restored expansion', () => cells().length === 7);
  if (!document.querySelector('button[aria-label="Collapse tree/deep"]')) fail('restored child expansion');
`)
	runTableViewScript(t, testOptions(f.baseDir, false), f, "many", "#page=3", `
  const button = label => document.querySelector('button[aria-label="' + label + '"]');
  await until('third page', () => button('Expand zz-sub'));
  button('Expand zz-sub').click();
  await until('expanded on third page', () => button('Collapse zz-sub'));
  if (fragment().get('page') !== '3') fail('expansion changed page', window.location.hash);
`)
}

func TestTableViewGroupsStemsInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"eval/ref/u1.wav", "eval/ref/u2.wav", "eval/hyp/u1.wav"} {
		write(t, filepath.Join(f.work, filepath.FromSlash(name)), string(wav))
	}
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=eval%2F**%2F*.wav&expanded=eval&expanded=eval%2Fhyp&expanded=eval%2Fref", `
  await until('files', () => document.querySelectorAll('#tv-table a.tv-media-path').length === 3);
  if (JSON.stringify(headings().slice(0, 4)) !== '["#","path","format","duration"]') fail('file headings', headings());
  const wavRow = cells().find(row => row[1].includes('eval/hyp/u1.wav'));
  if (!wavRow || wavRow[3] !== '0:00.50' || !wavRow[1].includes('WAV PCM')) fail('file row', wavRow);
  if (!document.getElementById('tv-summary').textContent.includes('7 entries')) fail('summary', document.getElementById('tv-summary').textContent);

  // Group by stem lines up the first-level directories under eval/.
  [...document.querySelectorAll('.tv-segmented button')].find(button => button.textContent === 'Group by stem').click();
  // A stems view's rows start with Open row.
  await until('stems', () => headings().length === 4);
  if (JSON.stringify(headings()) !== '["#","","stem","eval/"]') fail('stem headings', headings());
  const group = document.querySelector('select[aria-label="Groups from"]');
  group.value = 'parent';
  group.dispatchEvent(new window.Event('change'));
  await until('parents', () => headings().length === 5);
  if (JSON.stringify(headings()) !== '["#","","stem","eval/hyp/","eval/ref/"]') fail('parent headings', headings());
  const u2 = cells().find(row => row[2] === 'u2');
  if (!u2 || u2[1] !== 'Open row' || u2[3] !== '— missing' || !u2[4].includes('eval/ref/u2.wav')) fail('missing cell', u2);
  if (fragment().get('listing') !== 'stems' || fragment().get('group') !== 'parent') fail('fragment', window.location.hash);

  // Only incomplete stems, and Describe of the stems view.
  const incomplete = [...document.querySelectorAll('.tv-toolbar input[type=checkbox]')][0];
  incomplete.checked = true;
  incomplete.dispatchEvent(new window.Event('change'));
  await until('incomplete', () => cells().length === 1);
  document.querySelector('.tv-describe-toggle').click();
  await until('describe', () => document.querySelectorAll('tr.tv-describe').length === 12);
`)
}

func TestTableViewOpensFilesInPanesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"eval/ref/u1.wav", "eval/hyp/u1.wav", "eval/ref/u2.wav"} {
		write(t, filepath.Join(f.work, filepath.FromSlash(name)), string(wav))
	}
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=eval%2F**%2F*.wav&expanded=eval&expanded=eval%2Fhyp&expanded=eval%2Fref", `
  await until('files', () => document.querySelectorAll('#tv-table a.tv-media-path').length === 3);
  const panesArea = document.getElementById('tv-panes');
  if (!panesArea.hidden) fail('panes shown before a file is opened');
  const link = path => [...document.querySelectorAll('#tv-table a.tv-media-path')].find(a => a.textContent === path);
  const paneList = () => [...document.querySelectorAll('.tv-pane')];
  const tabNames = pane => [...pane.querySelectorAll('.tv-tab-name')].map(button => button.textContent);
  const active = () => document.querySelector('.tv-pane-active');

  // A click opens the file in a tab of a new pane; again, it does not add
  // a second tab.
  link('eval/hyp/u1.wav').click();
  link('eval/hyp/u1.wav').click();
  if (panesArea.hidden || paneList().length !== 1 || JSON.stringify(tabNames(paneList()[0])) !== '["audiou1.wav"]') fail('first open', panesArea.innerHTML);
  if (!active().textContent.includes('Files open here')) fail('active label');
  const view = active().querySelector('.tv-tab-view');
  if (!view.textContent.includes('eval/hyp/u1.wav') || !view.textContent.includes('WAV PCM 16-bit') || !view.querySelector('audio')) fail('tab view', view.innerHTML);
  if (view.querySelector('.tv-waveform-box').hidden || view.querySelector('.tv-spectrogram-box').hidden) fail('plots hidden');
  // jsdom cannot decode audio; the view says so instead of drawing.
  await until('status', () => view.querySelector('.tv-analysis-status').textContent.includes('Cannot draw'));

  // The bar between the plots sets the waveform's height, within its bounds.
  const waveBox = () => document.querySelector('.tv-pane-active .tv-waveform-box');
  const handle = () => document.querySelector('.tv-pane-active .tv-plot-handle');
  if (waveBox().style.flexBasis !== '64px') fail('default height', waveBox().style.flex);
  const drag = (from, to) => {
    handle().dispatchEvent(new window.MouseEvent('pointerdown', { clientY: from, bubbles: true }));
    window.dispatchEvent(new window.MouseEvent('pointermove', { clientY: to }));
    window.dispatchEvent(new window.MouseEvent('pointerup', { clientY: to }));
  };
  drag(100, 200);
  if (waveBox().style.flexBasis !== '164px') fail('dragged height', waveBox().style.flex);
  drag(100, -500);
  if (waveBox().style.flexBasis !== '32px') fail('minimum height', waveBox().style.flex);
  drag(100, 5000);
  if (waveBox().style.flexBasis !== '800px') fail('maximum height', waveBox().style.flex);
  drag(100, -568);
  if (waveBox().style.flexBasis !== '132px') fail('height again', waveBox().style.flex);
  window.dispatchEvent(new window.MouseEvent('pointermove', { clientY: 900 }));
  if (waveBox().style.flexBasis !== '132px') fail('moved after release', waveBox().style.flex);

  // Waveform and Spectrogram toggle their plots.
  const toggle = name => [...view.querySelectorAll('.tv-audio-controls button')].find(button => button.textContent === name);
  toggle('Spectrogram').click();
  if (!view.querySelector('.tv-spectrogram-box').hidden || toggle('Spectrogram').getAttribute('aria-pressed') !== 'false') fail('spectrogram toggle');
  // Alone, the waveform fills the pane, with no bar to drag.
  if (!handle().hidden || waveBox().style.flexGrow !== '1') fail('waveform alone', waveBox().style.flex);

  // Split right copies the current file into a new pane, which becomes
  // active; the next file opens there.
  active().querySelector('button[aria-label="Split right"]').click();
  if (paneList().length !== 2 || active() !== paneList()[1] || JSON.stringify(tabNames(paneList()[1])) !== '["audiou1.wav"]') fail('split', panesArea.innerHTML);
  if (!document.querySelector('.tv-split-row')) fail('row split');
  link('eval/ref/u2.wav').click();
  if (JSON.stringify(tabNames(paneList()[1])) !== '["audiou1.wav","audiou2.wav"]' || JSON.stringify(tabNames(paneList()[0])) !== '["audiou1.wav"]') fail('open in active', tabNames(paneList()[0]), tabNames(paneList()[1]));

  // Clicking in the first pane makes it active; a split down nests.
  paneList()[0].dispatchEvent(new window.Event('pointerdown'));
  if (active() !== paneList()[0]) fail('activate by click');
  active().querySelector('button[aria-label="Split down"]').click();
  if (paneList().length !== 3 || !document.querySelector('.tv-split-row .tv-split-column')) fail('split down', panesArea.innerHTML);

  // Closing a pane makes its neighbour active; closing the last tab of the
  // last pane hides the panes.
  active().querySelector('button[aria-label="Close pane"]').click();
  if (paneList().length !== 2 || !active()) fail('close pane');
  for (const pane of paneList().reverse()) {
    while (pane.isConnected && pane.querySelector('.tv-tab-close')) pane.querySelector('.tv-tab-close').click();
  }
  for (let tries = 0; paneList().length && tries < 5; tries++) paneList()[0].querySelector('.tv-tab-close').click();
  if (!panesArea.hidden || paneList().length) fail('all closed', panesArea.innerHTML);

  // A stems row opens its files side by side.
  [...document.querySelectorAll('.tv-segmented button')].find(button => button.textContent === 'Group by stem').click();
  const group = await (async () => { await until('stems', () => document.querySelector('select[aria-label="Groups from"]')); return document.querySelector('select[aria-label="Groups from"]'); })();
  group.value = 'parent';
  group.dispatchEvent(new window.Event('change'));
  await until('rows', () => cells().some(row => row[2] === 'u1'));
  const u1 = [...document.querySelectorAll('#tv-table tbody tr')].find(row => row.children[2].textContent === 'u1');
  u1.querySelector('.tv-open-row').click();
  if (paneList().length !== 2 || JSON.stringify(paneList().map(pane => pane.querySelector('.tv-tab-view strong').textContent)) !== '["eval/hyp/u1.wav","eval/ref/u1.wav"]') fail('open row', panesArea.innerHTML);
`)
}

// rowNavigationHelpers are what the row navigation scripts share.
const rowNavigationHelpers = `
  const press = (key, init) => {
    const event = new window.KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init });
    (init && init.on || document.body).dispatchEvent(event);
    return event.defaultPrevented;
  };
  const current = () => {
    const rows = [...document.querySelectorAll('#tv-table tr.tv-row-current')];
    if (rows.length > 1) fail('several current rows', rows.length);
    if (rows.length && rows[0].getAttribute('aria-current') !== 'true') fail('aria-current');
    return rows.length ? rows[0].dataset.number : '';
  };
  const shown = () => [...document.querySelectorAll('.tv-pane')].map(pane => {
    const view = pane.querySelector('.tv-tab-view');
    const notice = view.querySelector('.tv-row-notice');
    return notice ? notice.textContent : view.querySelector('strong').textContent;
  });
  const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  const untilShown = (...want) => until('panes to show ' + want, () => same(shown(), want));
  const link = text => [...document.querySelectorAll('#tv-table a.tv-media-path, #tv-table .tv-text-file')].find(a => a.textContent === text);
`

// writeRowNavigationTable writes an evaluation table of rows with ref and hyp
// columns: row 3 has a blank hyp, and row 4's names a missing file.
func writeRowNavigationTable(t *testing.T, directory string, rows int) {
	t.Helper()
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	var table strings.Builder
	table.WriteString("id,ref,hyp\n")
	for row := 1; row <= rows; row++ {
		ref, hyp := fmt.Sprintf("ref/u%d.wav", row), fmt.Sprintf("hyp/u%d.wav", row)
		write(t, filepath.Join(directory, filepath.FromSlash(ref)), string(wav))
		switch row {
		case 3:
			hyp = ""
		case 4:
			hyp = "hyp/gone.wav"
		default:
			write(t, filepath.Join(directory, filepath.FromSlash(hyp)), string(wav))
		}
		fmt.Fprintf(&table, "%d,%s,%s\n", rows-row, ref, hyp)
	}
	write(t, filepath.Join(directory, "data.csv"), table.String())
}

func TestTableViewMovesBetweenRowsInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	writeRowNavigationTable(t, f.work, 52)
	runTableViewScript(t, testOptions(f.baseDir, false), f, "data.csv", "#base=wd&limit=50", rowNavigationHelpers+`
  await until('rows', () => cells().length === 50);
  if (!document.querySelector('.tv-pager-keys').textContent.includes('Enter')) fail('key hint');
  if (current() !== '') fail('current before a move', current());

  // Without a current row, the first move takes the page's first row, and
  // a row with nothing open changes no panes.
  if (!press('ArrowDown')) fail('arrow not handled');
  if (current() !== '1' || fragment().get('row') !== '1') fail('first move', current(), window.location.hash);
  if (!document.getElementById('tv-panes').hidden) fail('panes opened by a move');

  // A click on a cell's file makes its row current and frames the cell. Focus
  // does not stay on the link, which would keep Enter and Space.
  const framed = () => {
    const found = [...document.querySelectorAll('#tv-table td.tv-cell-current')];
    if (found.length > 1) fail('several framed cells', found.length);
    return found.length ? found[0].parentElement.dataset.number + ':' + found[0].dataset.column : '';
  };
  if (framed() !== '1:1') fail('frame after a key starts at the first file', framed());
  link('ref/u2.wav').focus();
  link('ref/u2.wav').click();
  if (current() !== '2') fail('click current', current());
  if (framed() !== '2:1') fail('frame after a click', framed());
  if (document.activeElement.closest('#tv-table')) fail('focus stayed in the table', document.activeElement.outerHTML);
  await untilShown('ref/u2.wav');
  document.querySelector('.tv-pane button[aria-label="Split right"]').click();
  link('hyp/u2.wav').click();
  if (framed() !== '2:2') fail('frame on the other cell', framed());
  const tabs = [...document.querySelectorAll('.tv-pane')].map(pane => pane.querySelectorAll('.tv-tab').length);
  if (!same(tabs, [1, 2])) fail('tabs', tabs);
  await untilShown('ref/u2.wav', 'hyp/u2.wav');
  const active = document.querySelector('.tv-pane-active').dataset.pane;

  // The row moves at once, and the active pane's current file follows, keeping
  // the layout; the other pane stays. A row without the file says so instead of
  // showing the last row's.
  press('ArrowDown');
  if (current() !== '3') fail('row did not move at once', current());
  if (framed() !== '3:2') fail('frame did not move', framed());
  if (!same(shown(), ['ref/u2.wav', 'hyp/u2.wav'])) fail('tab followed at once', shown());
  await untilShown('ref/u2.wav', 'Row 3 has no file in hyp.');
  press('ArrowDown');
  await until('missing file', () => shown()[1].startsWith('Row 4: hyp/gone.wav: ') && shown()[1].includes('File not found'));
  press('ArrowDown');
  await untilShown('ref/u2.wav', 'hyp/u5.wav');
  if (document.querySelector('.tv-pane-active').dataset.pane !== active) fail('active pane changed');
  press('ArrowUp');
  await until('row 4 again', () => shown()[0] === 'ref/u2.wav' && shown()[1].startsWith('Row 4: '));

  // The tab is replaced in place, so tabs do not pile up, and a hidden tab
  // stays: split right copied row 2's ref beside the hyp opened after it.
  const paneTabs = index => [...document.querySelectorAll('.tv-pane')[index].querySelectorAll('.tv-tab-name')];
  const after = [...document.querySelectorAll('.tv-pane')].map(pane => pane.querySelectorAll('.tv-tab').length);
  if (!same(after, [1, 2])) fail('tabs after moves', after);
  paneTabs(1)[0].click();
  if (shown()[1] !== 'ref/u2.wav') fail('hidden tab followed', shown());
  paneTabs(1)[1].click();

  // Another pane follows once it is active, from whichever row it was on.
  document.querySelectorAll('.tv-pane')[0].dispatchEvent(new window.Event('pointerdown'));
  press('ArrowDown');
  await until('the other pane', () => shown()[0] === 'ref/u5.wav' && shown()[1].startsWith('Row 4: '));
  document.querySelectorAll('.tv-pane')[1].dispatchEvent(new window.Event('pointerdown'));
  press('ArrowUp');
  press('ArrowDown');
  await wait(200);
  if (shown()[0] !== 'ref/u5.wav') fail('inactive pane followed', shown());

  // A held key loads only where it stops.
  await untilShown('ref/u5.wav', 'hyp/u5.wav');
  // jsdom cannot decode audio; with a stand-in, the view asks for the file.
  window.AudioContext = class {
    decodeAudioData() { return Promise.reject(new window.Error('no decoder')); }
    close() {}
  };
  const loaded = [];
  const pageFetch = window.fetch;
  window.fetch = (input, init) => {
    const media = new window.URL(String(input), window.location.href);
    if (media.pathname.endsWith('/api/media')) loaded.push(media.searchParams.get('ref'));
    return pageFetch(input, init);
  };
  for (let times = 0; times < 5; times++) press('ArrowDown', { repeat: true });
  if (current() !== '10') fail('held key', current());
  await untilShown('ref/u5.wav', 'hyp/u10.wav');
  await wait(50);
  // The other pane's failed decode may retry on a redraw; no row passed through loads.
  if (!loaded.includes('hyp/u10.wav') || loaded.some(ref => /u[6-9]\.wav/.test(ref))) fail('held key loaded', loaded);
  window.fetch = pageFetch;
  delete window.AudioContext;

  // A clicked file opens in the active pane, and is the one that follows.
  link('ref/u20.wav').click();
  await untilShown('ref/u5.wav', 'ref/u20.wav');
  press('ArrowDown');
  await untilShown('ref/u5.wav', 'ref/u21.wav');

  // A move scrolls the table alone, up to where the page cuts it off. jsdom has
  // no layout, so the page gets one: rows 30px high in a scrolling table that
  // starts 100px down, with the page ending at 200px.
  {
    const scroller = document.querySelector('.tv-scroll');
    const page = document.querySelector('.tv-main');
    const box = window.Element.prototype.getBoundingClientRect;
    window.Element.prototype.getBoundingClientRect = function () {
      if (this === scroller) return { top: 100, bottom: 500, left: 0, right: 500 };
      if (this === page) return { top: 0, bottom: 200, left: 0, right: 500 };
      if (this.dataset.number) {
        const top = 100 + (Number(this.dataset.number) - 21) * 30 - scroller.scrollTop;
        return { top, bottom: top + 30, left: 0, right: 500 };
      }
      return { top: 0, bottom: 0, left: 0, right: 0 };
    };
    window.Element.prototype.scrollIntoView = () => fail('a move scrolled the page');
    scroller.scrollTop = 0;
    const row = number => document.querySelector('#tv-table tr[data-number="' + number + '"]');
    row(22).children[1].click(); // ends at 160px: in view
    if (scroller.scrollTop !== 0 || page.scrollTop !== 0) fail('scrolled a row in view', scroller.scrollTop);
    press('ArrowDown');
    if (scroller.scrollTop !== 0) fail('scrolled to a row in view', scroller.scrollTop);
    press('ArrowDown'); // 24: ends at 220px, past the page's end
    if (scroller.scrollTop !== 20 || page.scrollTop !== 0) fail('scrolled down', scroller.scrollTop, page.scrollTop);
    press('ArrowUp');
    press('ArrowUp');
    press('ArrowUp'); // 21: starts at 80px, above the table
    if (scroller.scrollTop !== 0 || page.scrollTop !== 0) fail('scrolled up', scroller.scrollTop, page.scrollTop);
    // With no room left for a row, as in a short window, the page scrolls just
    // enough: the table starts 195px down here, leaving 5px.
    const stub = window.Element.prototype.getBoundingClientRect;
    window.Element.prototype.getBoundingClientRect = function () {
      if (this === scroller) return { top: 195 - page.scrollTop, bottom: 595 - page.scrollTop, left: 0, right: 500 };
      return stub.call(this);
    };
    press('ArrowDown');
    if (page.scrollTop !== 25) fail('short room', page.scrollTop);
    window.Element.prototype.getBoundingClientRect = stub;
    page.scrollTop = 0;
    scroller.scrollTop = 0;
    // Near the table's end, the table cannot scroll the row into view: it stays
    // at 590px however far the table scrolls, and the page ends at 200px.
    window.Element.prototype.getBoundingClientRect = function () {
      if (this === scroller) return { top: 100 - page.scrollTop, bottom: 620 - page.scrollTop, left: 0, right: 500 };
      if (this.dataset.number) return { top: 560 - page.scrollTop, bottom: 590 - page.scrollTop, left: 0, right: 500 };
      return stub.call(this);
    };
    press('ArrowDown');
    if (page.scrollTop !== 390) fail('near the end', page.scrollTop);
    window.Element.prototype.getBoundingClientRect = stub;
    page.scrollTop = 0;
    scroller.scrollTop = 0;
    window.Element.prototype.getBoundingClientRect = box;
    delete window.Element.prototype.scrollIntoView;
    await wait(200);
  }

  // A click on a row's own text moves as a key does; a click on its controls
  // does not.
  const rowOf = number => document.querySelector('#tv-table tr[data-number="' + number + '"]');
  rowOf(30).children[1].click();
  if (current() !== '30' || framed() !== '30:0') fail('row click', current(), framed());
  await untilShown('ref/u5.wav', 'ref/u30.wav');

  // Moves cross pages both ways and stop at the ends.
  rowOf(50).children[1].click();
  await untilShown('ref/u5.wav', 'ref/u50.wav');
  press('ArrowDown');
  await until('page 2', () => current() === '51');
  if (framed() !== '51:0') fail('frame across pages', framed());
  if (fragment().get('page') !== '2' || cells().length !== 2) fail('page 2', window.location.hash);
  await untilShown('ref/u5.wav', 'ref/u51.wav');
  press('ArrowDown');
  await until('last row', () => current() === '52');
  if (!press('ArrowDown')) fail('arrow at the end not handled');
  await wait(50);
  if (current() !== '52' || fragment().get('page') !== '2') fail('past the end', current());
  press('ArrowUp');
  press('ArrowUp');
  await until('page 1', () => current() === '50');
  if (fragment().get('page') !== '1') fail('page 1', window.location.hash);
  rowOf(1).children[1].click();
  await until('row 1', () => current() === '1');
  press('ArrowUp');
  await wait(50);
  if (current() !== '1' || fragment().get('page') !== '1') fail('before the start', current());

  // A sort keeps the current row and moves in the order shown: id descends
  // with the file, so ascending id shows the file backwards.
  const sortBy = name => [...document.querySelectorAll('.tv-sort')].find(button => button.textContent.startsWith(name)).click();
  sortBy('id');
  await until('sorted', () => cells()[0][1] === '0');
  if (fragment().get('row') !== '1' || current() !== '') fail('row off the page after a sort', current(), window.location.hash);
  press('ArrowDown');
  if (current() !== '52') fail('off-page row moves to the first shown', current());
  press('ArrowDown');
  if (current() !== '51') fail('sorted move', current());
  sortBy('id');
  await until('descending', () => cells()[0][1] === '51');
  sortBy('id');
  await until('file order', () => cells()[0][1] === '51' && fragment().get('sort') === null);

  // Another separator changes what numbers mean, so the row is forgotten and
  // tabs stop following.
  rowOf(5).children[1].click();
  await until('row 5', () => current() === '5');
  await wait(200);
  const before = shown();
  const separator = document.querySelector('.tv-toolbar select');
  separator.value = ';';
  separator.dispatchEvent(new window.Event('change'));
  await until('one column', () => headings().length === 2);
  if (current() !== '' || framed() !== '' || fragment().get('row') !== null) fail('row kept', current(), window.location.hash);
  press('ArrowDown');
  await wait(200);
  if (!same(shown(), before)) fail('tabs followed another table', shown());
`)
}

// Controls keep the keys they use. The frame moves along a row and opens
// nothing; Enter opens its file, Shift+Enter the row's, and Space plays it. jsdom
// has no media playback, so the page gets a stand-in.
func TestTableViewRowKeysRespectControlsInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	writeRowNavigationTable(t, f.work, 5)
	options := testOptions(f.baseDir, false)
	runTableViewScript(t, options, f, "data.csv", "#base=wd", rowNavigationHelpers+`
  await until('rows', () => cells().length === 5);
  const framed = () => {
    const cell = document.querySelector('#tv-table td.tv-cell-current');
    return cell ? cell.parentElement.dataset.number + ':' + cell.dataset.column : '';
  };
  const played = [];
  Object.defineProperty(window.HTMLMediaElement.prototype, 'paused', { configurable: true, get() { return !this.playing; } });
  window.HTMLMediaElement.prototype.play = function () { this.playing = true; played.push('play ' + new window.URL(this.src).searchParams.get('ref')); return Promise.resolve(); };
  window.HTMLMediaElement.prototype.pause = function () { this.playing = false; played.push('pause ' + new window.URL(this.src).searchParams.get('ref')); };

  // With nothing current and no player, Enter does nothing and Space scrolls.
  if (press(' ') || press('Enter')) fail('key taken without a row');

  // Keys stay with controls, and with modifiers held.
  const filter = document.querySelector('.tv-filter-value');
  const operator = document.querySelector('select[aria-label="Operator"]');
  for (const on of [filter, operator]) {
    for (const key of ['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight', 'Enter', ' ']) {
      if (key === 'Enter' && on === filter) continue; // adds the condition
      if (press(key, { on })) fail('key taken from a control', key, on.outerHTML);
    }
  }
  for (const modifier of ['ctrlKey', 'metaKey', 'altKey', 'shiftKey']) {
    for (const key of ['ArrowDown', 'ArrowLeft']) {
      if (press(key, { [modifier]: true })) fail('key taken with', modifier, key);
    }
  }
  if (current() !== '') fail('row moved', current());
  const sort = document.querySelector('.tv-sort');
  if (press('Enter', { on: sort }) || press(' ', { on: sort })) fail('button key taken');

  // An arrow key frames the row's first file and opens nothing.
  press('ArrowDown');
  press('ArrowDown');
  if (current() !== '2' || framed() !== '2:1') fail('frame', current(), framed());
  if (!document.getElementById('tv-panes').hidden) fail('a move opened a file');

  // Left and right move the frame, stopping at the ends, and open nothing.
  if (!press('ArrowRight') || framed() !== '2:2') fail('right', framed());
  if (!press('ArrowRight') || framed() !== '2:2') fail('past the last column', framed());
  press('ArrowLeft');
  press('ArrowLeft');
  if (framed() !== '2:0') fail('left', framed());
  if (!press('ArrowLeft') || framed() !== '2:0') fail('past the first column', framed());
  if (!document.getElementById('tv-panes').hidden) fail('a frame move opened a file');
  if (current() !== '2') fail('a frame move changed the row', current());

  // A cell without a file leaves Enter alone.
  if (press('Enter') || !document.getElementById('tv-panes').hidden) fail('Enter on a cell without a file');

  // Enter opens the framed file only, and a held key does not repeat it.
  press('ArrowRight');
  if (!press('Enter')) fail('Enter not handled');
  await untilShown('ref/u2.wav');
  // Row moves keep the column.
  press('ArrowDown');
  if (framed() !== '3:1') fail('frame column after a move', framed());
  await untilShown('ref/u3.wav');
  press('ArrowUp');
  await untilShown('ref/u2.wav');

  // Space on a file already open plays and pauses it.
  if (!press(' ') || !same(played, ['play ref/u2.wav'])) fail('play', played);
  if (!press(' ', { repeat: true }) || !same(played, ['play ref/u2.wav'])) fail('repeat', played);
  if (!press(' ') || !same(played, ['play ref/u2.wav', 'pause ref/u2.wav'])) fail('pause', played);
  const audio = document.querySelector('.tv-pane-active audio');
  if (press(' ', { on: audio })) fail('key taken from the player');

  // Space on a file not open opens it, in the active pane, and plays it, pausing
  // the other one. Back on the first, it finds that tab again.
  played.length = 0;
  press(' ');
  press('ArrowRight');
  press(' ');
  if (!same(played, ['play ref/u2.wav', 'pause ref/u2.wav', 'play hyp/u2.wav'])) fail('another file', played);
  if (!same(shown(), ['hyp/u2.wav'])) fail('shown', shown());
  press('ArrowLeft');
  played.length = 0;
  press(' ');
  if (!same(played, ['pause hyp/u2.wav', 'play ref/u2.wav']) || !same(shown(), ['ref/u2.wav'])) fail('back', played, shown());
  if (document.querySelector('.tv-pane-active').querySelectorAll('.tv-tab').length !== 2) fail('a second tab for one file');

  // On a cell without a file, Space still pauses what is playing.
  press('ArrowLeft');
  played.length = 0;
  if (!press(' ') || !same(played, ['pause ref/u2.wav'])) fail('pause from another cell', played);
  press('ArrowRight');

  // A move does not start playback, and stops what was playing.
  press(' ');
  const playing = document.querySelector('.tv-pane-active audio');
  played.length = 0;
  press('ArrowDown');
  await untilShown('ref/u3.wav');
  if (playing.playing || !same(played, ['pause ref/u2.wav'])) fail('moved while playing', played);

  // Shift+Enter opens every file the row names, side by side, and only the
  // active pane's follows.
  press('ArrowUp');
  if (!press('Enter', { shiftKey: true })) fail('Shift+Enter not handled');
  await untilShown('ref/u2.wav', 'hyp/u2.wav');
  press('ArrowDown');
  await untilShown('ref/u3.wav', 'hyp/u2.wav');

  // A refused start is reported.
  press('ArrowUp');
  await untilShown('ref/u2.wav', 'hyp/u2.wav');
  window.HTMLMediaElement.prototype.play = () => Promise.reject(new window.Error('not allowed'));
  press(' ');
  await until('refusal', () => document.getElementById('tv-error').textContent === 'Cannot play the file: not allowed');
`)
	// The URL brings the row back, framed, without opening anything.
	runTableViewScript(t, options, f, "data.csv", "#base=wd&row=2", rowNavigationHelpers+`
  await until('rows', () => cells().length === 5);
  if (current() !== '2' || !document.querySelector('#tv-table tr[data-number="2"] td.tv-cell-current')) fail('row from the URL', current());
  if (!document.getElementById('tv-panes').hidden) fail('opened from the URL');
`)
	// A bad row in the URL shows nothing current.
	runTableViewScript(t, options, f, "data.csv", "#base=wd&row=abc", rowNavigationHelpers+`
  await until('rows', () => cells().length === 5);
  if (current() !== '' || document.querySelector('.tv-cell-current') || fragment().get('row') !== null) fail('bad row', current(), window.location.hash);
`)
	runTableViewScript(t, options, f, "data.csv", "#base=wd&row=99", rowNavigationHelpers+`
  await until('rows', () => cells().length === 5);
  if (current() !== '' || document.querySelector('.tv-cell-current')) fail('row past the end', current());
`)
}

// A directory's rows move too: a file follows, as media or text, and a row
// that has none says so. Expanding renumbers rows, so it forgets the row.
func TestTableViewMovesBetweenDirectoryRowsInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(f.work, "set", "a.wav"), string(wav))
	write(t, filepath.Join(f.work, "set", "b.txt"), "text\n")
	write(t, filepath.Join(f.work, "set", "sub", "c.wav"), string(wav))
	write(t, filepath.Join(f.work, "pair", "ref", "u1.wav"), string(wav))
	write(t, filepath.Join(f.work, "pair", "hyp", "u1.wav"), string(wav))
	write(t, filepath.Join(f.work, "pair", "ref", "u2.wav"), string(wav))
	options := testOptions(f.baseDir, false)
	runTableViewScript(t, options, f, "wd", "#pattern=set%2F**&expanded=set", rowNavigationHelpers+`
  await until('files', () => link('set/a.wav'));
  link('set/a.wav').click();
  await untilShown('set/a.wav');
  press('ArrowDown');
  await until('text file', () => document.querySelector('.tv-pane .tv-tab-kind').textContent === 'text');
  press('ArrowDown');
  await until('directory row', () => (document.querySelector('.tv-row-notice') || {}).textContent === 'Row ' + current() + ' has no file in ' + headings()[1] + '.');
  const number = current();
  [...document.querySelectorAll('.tv-directory')].find(button => button.getAttribute('aria-label') === 'Expand set/sub').click();
  await until('expanded', () => link('set/sub/c.wav'));
  if (current() !== '' || fragment().get('row') !== null) fail('row kept after expanding', number, window.location.hash);
`)
	runTableViewScript(t, options, f, "wd", "#listing=stems&pattern=pair%2F**%2F*.wav&expanded=pair&expanded=pair%2Fhyp&expanded=pair%2Fref&group=parent", rowNavigationHelpers+`
  await until('stems', () => document.querySelector('.tv-open-row'));
  document.querySelector('.tv-open-row').click();
  await untilShown('pair/hyp/u1.wav', 'pair/ref/u1.wav');
  press('ArrowDown');
  await until('missing member', () => shown()[1] === 'pair/ref/u1.wav' && shown()[0].startsWith('Row 2 has no file in '));
`)
}

// Opening Describe scrolls the page to the table, and the table to its head;
// closing it moves nothing. jsdom has no layout, so the page gets one.
func TestTableViewDescribeShowsItsRowsInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	runTableViewScript(t, testOptions(f.baseDir, false), f, "data.csv", "", `
  await until('rows', () => cells().length === 1);
  const page = document.querySelector('.tv-main');
  const area = document.querySelector('.tv-table-area');
  const scroller = document.querySelector('.tv-scroll');
  window.Element.prototype.getBoundingClientRect = function () {
    if (this === page) return { top: 50 };
    if (this === area) return { top: 450 - page.scrollTop };
    return { top: 0 };
  };
  scroller.scrollTop = 300;
  const toggle = () => document.querySelector('.tv-describe-toggle');
  toggle().click();
  await until('describe', () => document.querySelector('#tv-table tr.tv-describe'));
  await until('scroll', () => page.scrollTop === 400);
  if (scroller.scrollTop !== 0) fail('table scroll', scroller.scrollTop);
  page.scrollTop = 100;
  scroller.scrollTop = 200;
  toggle().click();
  await until('closed', () => !document.querySelector('#tv-table tr.tv-describe'));
  await wait(50);
  if (page.scrollTop !== 100 || scroller.scrollTop !== 200) fail('closing scrolled', page.scrollTop, scroller.scrollTop);
`)
}

// A refused filter must leave a way out, and a typo's text to correct.
func TestTableViewRecoversFromARefusedFilterInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "data.csv"), "id,name\n1,alpha\n2,beta\n3,Gamma\n")
	options := testOptions(f.baseDir, false)
	runTableViewScript(t, options, f, "data.csv", "", `
  await until('rows', () => cells().length === 3);
  const errorText = () => document.getElementById('tv-error').textContent;
  const add = (column, op, text) => {
    const columns = document.querySelector('select[aria-label="Column"]');
    columns.value = String(column);
    columns.dispatchEvent(new window.Event('change'));
    const operator = document.querySelector('select[aria-label="Operator"]');
    operator.value = op;
    operator.dispatchEvent(new window.Event('change'));
    const value = document.querySelector('.tv-filter-value');
    value.value = text;
    value.dispatchEvent(new window.Event('input'));
    [...document.querySelectorAll('.tv-filter-form button')].find(button => button.textContent === '+ Add').click();
  };
  // A bad pattern says why, keeps the rows, and puts it back to correct.
  add(1, 'regex', '(unclosed');
  await until('refusal', () => errorText().includes('regex'));
  if (cells().length !== 3 || document.querySelector('.tv-chip')) fail('rows after a refusal', cells());
  if (document.querySelector('.tv-filter-value').value !== '(unclosed' || document.querySelector('select[aria-label="Operator"]').value !== 'regex') fail('form after a refusal');
  if (fragment().has('filters')) fail('filter kept in the URL', window.location.hash);
  // The same goes for other operators.
  add(1, '>', 'high');
  await until('text ordering refused', () => errorText().includes('needs a number'));
  if (cells().length !== 3 || document.querySelector('.tv-filter-value').value !== 'high') fail('form after an ordering', cells());
  // The options list offers both, and they filter.
  add(1, 'regex', '^(alpha|beta)$');
  await until('alternation', () => cells().length === 2);
  if (errorText() !== '') fail('error stayed', errorText());
  if (!document.querySelector('.tv-chip').textContent.includes('matches regex')) fail('chip', document.querySelector('.tv-chip').textContent);
  document.querySelector('.tv-chip button').click();
  await until('removed', () => cells().length === 3);
  add(1, 'not_regex', '(?i)gamma');
  await until('negative', () => cells().length === 2);
  if (cells().some(row => row[2] === 'Gamma')) fail('negative', cells());
  // Reloading gives the same view.
  if (JSON.parse(fragment().get('filters'))[0].value !== '(?i)gamma') fail('fragment', window.location.hash);
`)
	// A bad filter in the URL has no earlier rows: its chip can still be removed.
	bad := url.QueryEscape(`[{"column":1,"op":"regex","value":"(unclosed"}]`)
	runTableViewScript(t, options, f, "data.csv", "#filters="+bad, `
  await until('refusal', () => document.getElementById('tv-error').textContent.includes('regex'));
  if (document.querySelector('.tv-chip code').textContent !== '(unclosed') fail('chip', document.getElementById('tv-filterbar').innerHTML);
  document.querySelector('.tv-chip button').click();
  await until('rows', () => cells().length === 3);
  if (document.getElementById('tv-error').textContent !== '' || fragment().has('filters')) fail('after removing', window.location.hash);
`)
}

func TestTableViewDescribesDistributionsInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "data.csv"), "n,label\n1,<b data-x>\n2,<b data-x>\n3,other\n4,<b data-x>\n")
	runTableViewScript(t, testOptions(f.baseDir, false), f, "data.csv", "", `
  await until('rows', () => cells().length === 4);
  document.querySelector('.tv-describe-toggle').click();
  await until('describe', () => document.querySelector('#tv-table tr.tv-describe'));
  const lines = [...document.querySelectorAll('#tv-table tr.tv-describe')];
  const names = lines.map(line => line.children[0].textContent);
  const want = ['histogram', 'count', 'unique', 'top', 'mean', 'std', 'min', '25%', '50%', '75%', 'max', 'sum'];
  if (JSON.stringify(names) !== JSON.stringify(want)) fail('rows', names);
  const cell = (name, column) => lines[names.indexOf(name)].children[column + 1];
  if (cell('25%', 0).textContent !== '1.75' || cell('50%', 0).textContent !== '2.5' || cell('75%', 0).textContent !== '3.25') fail('quartiles', cell('25%', 0).textContent);
  if (cell('50%', 1).textContent !== '–' || cell('histogram', 1).textContent !== '–') fail('text column', cell('50%', 1).textContent);
  // A value holding markup stays text.
  const top = cell('top', 1);
  if (top.querySelector('[data-x]') || !top.textContent.includes('<b data-x>3') || !top.textContent.includes('other1')) fail('top', top.innerHTML);
  if (cell('top', 0).textContent !== '–') fail('all unique', cell('top', 0).textContent);

  // The table holds a button, and the bars open in a pane.
  if (cell('histogram', 0).querySelector('.tv-histogram')) fail('bars in the table');
  const open = () => document.querySelector('#tv-table tr.tv-describe .tv-open-histogram');
  if (open().textContent !== 'Histogram') fail('button', open().outerHTML);
  if (!document.getElementById('tv-panes').hidden) fail('opened before a click');
  open().click();
  const pane = () => document.querySelector('.tv-pane-active');
  const bins = () => pane().querySelectorAll('.tv-histogram-bin');
  if (bins().length !== 20 || pane().querySelectorAll('.tv-histogram-bar').length !== 4) fail('bars', bins().length);
  if (bins()[0].title !== '1 – 1.15: 1') fail('bar title', bins()[0].title);
  if (!pane().querySelector('[role="img"]').getAttribute('aria-label').includes('4 numbers from 1 to 4')) fail('label');
  if (!pane().querySelector('.tv-tab-view strong').textContent.includes('n') || !pane().querySelector('.tv-histogram-axis').textContent.includes('14')) fail('titles', pane().innerHTML);
  if (document.activeElement.closest('#tv-table')) fail('focus stayed in the table');

  // The number of bars can change, merging the same counts.
  const bars = pane().querySelector('select[aria-label="Number of bars"]');
  if ([...bars.options].map(option => option.value).join() !== '10,20,50,100,200' || bars.value !== '20') fail('choices', bars.outerHTML);
  const choose = number => {
    const select = pane().querySelector('select[aria-label="Number of bars"]');
    select.value = String(number);
    select.dispatchEvent(new window.Event('change'));
  };
  const total = () => [...bins()].reduce((sum, bin) => sum + Number(bin.title.split(': ')[1]), 0);
  for (const number of [10, 50, 100, 200, 20]) {
    choose(number);
    if (bins().length !== number || total() !== 4 || pane().querySelector('select[aria-label="Number of bars"]').value !== String(number)) fail('bars', number, bins().length, total());
  }
  choose(10);
  if (bins()[0].title !== '1 – 1.3: 1' || bins()[9].title !== '3.7 – 4: 1') fail('merged bars', bins()[0].title, bins()[9].title);
  choose(20);

  // The same selection shows its tab again.
  open().click();
  if (pane().querySelectorAll('.tv-tab').length !== 1) fail('a second tab', pane().querySelectorAll('.tv-tab').length);

  // A new selection counts different rows, so it opens its own tab, and the first
  // stays as it was. A row move changes neither.
  const operator = document.querySelector('select[aria-label="Operator"]');
  operator.value = '>';
  operator.dispatchEvent(new window.Event('change'));
  const value = document.querySelector('.tv-filter-value');
  value.value = '2';
  value.dispatchEvent(new window.Event('input'));
  [...document.querySelectorAll('.tv-filter-form button')].find(button => button.textContent === '+ Add').click();
  await until('filtered describe', () => cells().length === 2 && [...document.querySelectorAll('tr.tv-describe')].some(row => row.textContent.startsWith('count2')));
  document.querySelector('.tv-open-histogram').click();
  if (pane().querySelectorAll('.tv-tab').length !== 2) fail('filtered tab', pane().querySelectorAll('.tv-tab').length);
  if (!pane().querySelector('.tv-media-properties').textContent.includes('2 numbers in the rows selected')) fail('filtered summary', pane().querySelector('.tv-media-properties').textContent);
  document.body.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }));
  await wait(200);
  if (!pane().querySelector('.tv-histogram')) fail('a row move replaced the histogram');
  if (document.body.dispatchEvent(new window.KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true })) !== true) fail('Space taken without a player');

`)
}

// A pane's file saves through /api/download, whatever its type.
func TestTableViewDownloadsAFile(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "notes", "評価.log"), "line\n")
	options := testOptions(f.baseDir, false)
	recorder := f.viewerGet(t, options, "api/download", "wd", url.Values{"ref": {"notes/評価.log"}})
	if recorder.Code != http.StatusOK || recorder.Body.String() != "line\n" {
		t.Fatalf("download = (%d, %q)", recorder.Code, recorder.Body.String())
	}
	_, params, err := mime.ParseMediaType(recorder.Header().Get("Content-Disposition"))
	if err != nil || params["filename"] != "評価.log" {
		t.Fatalf("disposition = %q, %v", recorder.Header().Get("Content-Disposition"), err)
	}
	if recorder.Header().Get(headerContentType) != "application/octet-stream" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers = %v", recorder.Header())
	}
	for name, ref := range map[string]string{
		"nothing":       "",
		"missing":       "notes/gone.log",
		"directory":     "notes",
		"above":         "../secret",
		"outside":       filepath.Join(f.outside, "secret.txt"),
		"out by a link": "link.csv",
	} {
		if recorder := f.viewerGet(t, options, "api/download", "wd", url.Values{"ref": {ref}}); recorder.Code < 400 {
			t.Errorf("%s = (%d, %q)", name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestTableViewPanesOfferDownloadsInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	writeRowNavigationTable(t, f.work, 3)
	write(t, filepath.Join(f.work, "set", "notes.txt"), "text\n")
	options := testOptions(f.baseDir, false)
	runTableViewScript(t, options, f, "data.csv", "#base=wd", rowNavigationHelpers+`
  await until('rows', () => cells().length === 3);
  const download = () => document.querySelector('.tv-pane-active a[download]');
  link('ref/u1.wav').click();
  await untilShown('ref/u1.wav');
  const url = new window.URL(download().href);
  if (url.pathname !== '/tableview/api/download' || url.searchParams.get('ref') !== 'ref/u1.wav' || url.searchParams.get('base') !== 'wd') fail('url', download().href);
  if (download().getAttribute('download') !== 'u1.wav' || download().title !== 'Download u1.wav') fail('name', download().outerHTML);
  const response = await window.fetch(download().href);
  if (response.status !== 200 || !(response.headers.get('content-disposition') || '').includes('u1.wav')) fail('fetch', response.status);

  // It follows the file showing.
  press('ArrowDown');
  await untilShown('ref/u2.wav');
  if (download().getAttribute('download') !== 'u2.wav') fail('after a move', download().outerHTML);

  // A row without the file has nothing to save.
  press('ArrowRight');
  link('hyp/u1.wav').click();
  press('ArrowDown');
  press('ArrowDown');
  await until('no file', () => document.querySelector('.tv-row-notice'));
  if (download()) fail('download for a notice', download().outerHTML);

  // Nor has a histogram.
  document.querySelector('.tv-describe-toggle').click();
  await until('describe', () => document.querySelector('.tv-open-histogram'));
  document.querySelector('.tv-open-histogram').click();
  if (!document.querySelector('.tv-pane-active .tv-histogram') || download()) fail('download for a histogram');
`)
	runTableViewScript(t, options, f, "wd", "#pattern=set%2F*&expanded=set", rowNavigationHelpers+`
  await until('text file', () => link('set/notes.txt'));
  link('set/notes.txt').click();
  await until('pane', () => document.querySelector('.tv-pane-active a[download]'));
  const response = await window.fetch(document.querySelector('.tv-pane-active a[download]').href);
  if ((await response.text()) !== 'text\n') fail('text download');
`)
}

func TestTableViewExportsRows(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "data.csv"), "id,score\n1,3\n2,1\n3,2\n")
	options := testOptions(f.baseDir, false)
	selected := url.Values{
		"sep":     {","},
		"filters": {`[{"column":1,"op":">","value":"1"}]`},
		"sort":    {"1"},
		"desc":    {"1"},
	}
	recorder := f.viewerGet(t, options, "api/export", "data.csv", selected)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "id,score\n1,3\n3,2\n" {
		t.Fatalf("export = (%d, %q)", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Disposition"); got != "attachment; filename=data.export.csv" {
		t.Fatalf("disposition = %q", got)
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %v", recorder.Header())
	}
	for name, extra := range map[string]url.Values{
		"offset":  {"sep": {","}, "offset": {"1"}},
		"limit":   {"sep": {","}, "limit": {"1"}},
		"base":    {"sep": {","}, "base": {"wd"}},
		"filters": {"sep": {","}, "filters": {`[{"column":9,"op":"="}]`}},
		"sort":    {"sep": {","}, "sort": {"9"}},
		"sep":     {"sep": {",,"}},
	} {
		if recorder := f.viewerGet(t, options, "api/export", "data.csv", extra); recorder.Code != http.StatusBadRequest {
			t.Errorf("%s = (%d, %q)", name, recorder.Code, recorder.Body.String())
		}
	}
	if recorder := f.viewerGet(t, options, "api/export", "link.csv", url.Values{"sep": {","}}); recorder.Code < 400 {
		t.Fatalf("outside the roots = (%d, %q)", recorder.Code, recorder.Body.String())
	}
	if recorder := f.viewerGet(t, options, "api/export", "wd", url.Values{"pattern": {"data.csv"}}); recorder.Code != http.StatusOK ||
		!strings.HasPrefix(recorder.Body.String(), "path,") || !strings.Contains(recorder.Body.String(), "data.csv,") ||
		!strings.HasSuffix(recorder.Header().Get("Content-Disposition"), `.export.csv`) {
		t.Fatalf("directory = (%d, %q, %v)", recorder.Code, recorder.Body.String(), recorder.Header())
	}
}

func TestTableViewExportNamesEncodeTheSource(t *testing.T) {
	for _, test := range []struct {
		path      string
		directory bool
		want      string
	}{
		{"/a/eval.tsv", false, "eval.export.tsv"},
		{"/a/noext", false, "noext.export"},
		{"/a/results", true, "results.export.csv"},
		{"/a/評価 \"x\".csv", false, "評価 \"x\".export.csv"},
	} {
		source := tableViewSource{target: tableview.Target{Path: test.path}, directory: test.directory}
		_, params, err := mime.ParseMediaType(tableViewExportDisposition(source))
		if err != nil || params["filename"] != test.want {
			t.Errorf("%s = %v, %v; want %s", test.path, params, err, test.want)
		}
	}
}

// failingExport writes some of its rows, then fails.
type failingExport struct{}

func (failingExport) Stream(_ context.Context, writer io.Writer) error {
	if _, err := writer.Write([]byte("id\n1\n")); err != nil {
		return err
	}
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return errors.New("read failed")
}

// A read failing after the status has gone must not pass for a whole file.
func TestTableViewExportFailureAbortsTheResponse(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		streamTableViewExport(writer, request, failingExport{})
	}))
	defer server.Close()
	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if body, err := io.ReadAll(response.Body); err == nil {
		t.Fatalf("read whole body %q without an error", body)
	}
}

func TestTableViewExportLinkInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "data.csv"), "id,score\n1,3\n2,1\n3,2\n")
	runTableViewScript(t, testOptions(f.baseDir, false), f, "data.csv", "#base=wd", `
  await until('rows', () => cells().length === 3);
  const link = () => document.querySelector('.tv-export');
  if (!link() || link().textContent !== 'Export rows' || !link().hasAttribute('download')) fail('link', document.getElementById('tv-pager').innerHTML);
  const url = () => new window.URL(link().href);
  if (url().pathname !== '/tableview/api/export' || url().searchParams.has('base') || url().searchParams.get('sep') !== ',') fail('url', link().href);
  if (!link().title.includes('all 3 ')) fail('title', link().title);

  // It follows the sort and the filters, and fetching it gives those rows.
  [...document.querySelectorAll('.tv-sort')].find(button => button.textContent.startsWith('score')).click();
  await until('sort', () => cells()[0][1] === '2');
  document.querySelector('select[aria-label="Column"]').value = '1';
  document.querySelector('select[aria-label="Column"]').dispatchEvent(new window.Event('change'));
  const operator = document.querySelector('select[aria-label="Operator"]');
  operator.value = '>';
  operator.dispatchEvent(new window.Event('change'));
  const value = document.querySelector('.tv-filter-value');
  value.value = '1';
  value.dispatchEvent(new window.Event('input'));
  [...document.querySelectorAll('.tv-filter-form button')].find(button => button.textContent === '+ Add').click();
  await until('filter', () => cells().length === 2);
  if (!link().title.includes('all 2 ')) fail('filtered title', link().title);
  const response = await window.fetch(link().href);
  if ((await response.text()) !== 'id,score\n3,2\n1,3\n') fail('fetched', response.status);

  // With no rows, or a query refused, there is nothing to offer.
  value.value = '';
  const form = document.querySelector('select[aria-label="Column"]');
  form.value = '1';
  form.dispatchEvent(new window.Event('change'));
  document.querySelector('.tv-filter-value').value = '99';
  document.querySelector('.tv-filter-value').dispatchEvent(new window.Event('input'));
  [...document.querySelectorAll('.tv-filter-form button')].find(button => button.textContent === '+ Add').click();
  await until('no rows', () => cells().length === 1 && cells()[0][0] === 'No rows.');
  if (link()) fail('link with no rows');
`)
}

// Opening a file on request scrolls the page to the panes; a file following a row
// move, or one Space opens to play, leaves it where it is. jsdom has no layout, so
// the page gets one.
func TestTableViewOpeningScrollsToThePanesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	writeRowNavigationTable(t, f.work, 4)
	runTableViewScript(t, testOptions(f.baseDir, false), f, "data.csv", "#base=wd", rowNavigationHelpers+`
  await until('rows', () => cells().length === 4);
  const page = document.querySelector('.tv-main');
  const area = document.getElementById('tv-panes');
  window.Element.prototype.getBoundingClientRect = function () {
    if (this === page) return { top: 50, bottom: 500, left: 0, right: 500 };
    if (this === area) return { top: 50 - page.scrollTop, left: 0, right: 500 };
    return { top: 0, bottom: 0, left: 0, right: 0 };
  };
  window.HTMLMediaElement.prototype.play = function () { return Promise.resolve(); };
  window.HTMLMediaElement.prototype.pause = function () {};
  const scrolled = () => { page.scrollTop = 200; };
  const check = (what, want) => { if (page.scrollTop !== want) fail(what, page.scrollTop); };

  // A click on a file.
  scrolled();
  link('ref/u1.wav').click();
  check('click', 0);
  await untilShown('ref/u1.wav');

  // A row move, and the file that follows it.
  scrolled();
  press('ArrowDown');
  await untilShown('ref/u2.wav');
  check('row move', 200);

  // A frame move.
  press('ArrowRight');
  check('frame move', 200);

  // Space on a file not open yet, which opens it to play.
  press(' ');
  await untilShown('hyp/u2.wav');
  check('Space', 200);

  // Enter on the framed file, and Shift+Enter on the row.
  press('Enter');
  check('Enter', 0);
  scrolled();
  press('Enter', { shiftKey: true });
  check('Shift+Enter', 0);

  // A click on a row's own text moves, and is not a request to open.
  scrolled();
  document.querySelector('#tv-table tr[data-number="4"]').children[1].click();
  await wait(200);
  check('row click', 200);

  // A histogram.
  document.querySelector('.tv-describe-toggle').click();
  await until('describe', () => document.querySelector('.tv-open-histogram'));
  scrolled();
  document.querySelector('.tv-open-histogram').click();
  check('histogram', 0);
`)
}

// A tab dragged out of the window, or its pane's button, moves the file to a
// window of its own. jsdom has no native drag or windows, so the page gets stand-ins.
func TestTableViewPopsOutATabInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	writeRowNavigationTable(t, f.work, 2)
	options := testOptions(f.baseDir, false)
	runTableViewScript(t, options, f, "data.csv", "#base=wd", rowNavigationHelpers+`
  await until('rows', () => cells().length === 2);
  const opened = [];
  let allow = true;
  window.open = (url, name, features) => { opened.push({ url, name, features }); return allow ? {} : null; };
  const tab = () => document.querySelector('.tv-pane .tv-tab');
  const end = (clientX, clientY, dropEffect) => {
    const event = new window.MouseEvent('dragend', { bubbles: true, clientX, clientY, screenX: 700, screenY: 300 });
    Object.defineProperty(event, 'dataTransfer', { value: { dropEffect } });
    tab().dispatchEvent(event);
  };
  link('ref/u1.wav').click();
  await untilShown('ref/u1.wav');

  // Dropped inside the window, or on something that took it: nothing happens.
  end(10, 10, 'none');
  end(-5, 10, 'move');
  if (opened.length) fail('opened', opened);

  // A blocked window leaves the tab, and says so.
  allow = false;
  end(-5, 10, 'none');
  if (opened.length !== 1 || !tab() || !document.getElementById('tv-error').textContent.includes('blocked')) fail('blocked', opened.length);
  allow = true;

  // Past each edge, it moves out where it was dropped, keeping the file.
  document.getElementById('tv-error').hidden = true;
  end(window.innerWidth + 5, 10, 'none');
  if (opened.length !== 2 || tab()) fail('moved out', opened.length);
  if (!opened[1].features.includes('left=700') || !opened[1].features.includes('top=300') || !opened[1].features.startsWith('popup')) fail('place', opened[1].features);
  const url = new window.URL(opened[1].url, window.location.href);
  if (url.pathname !== window.location.pathname || url.search !== window.location.search) fail('page', url);
  const media = JSON.parse(new window.URLSearchParams(url.hash.slice(1)).get('popout'));
  if (media.ref !== 'ref/u1.wav' || media.base !== 'wd' || !media.source) fail('media', media);

  // A row move goes to that window, with the row's files in its origin's terms.
  const posted = [];
  window.eval('windowChannel = { postMessage(message) { window.__posted.push(message); } }');
  window.__posted = posted;
  press('ArrowDown');
  await until('follow', () => posted.some(message => message.type === 'follow'));
  const follow = posted.find(message => message.type === 'follow');
  if (follow.to !== opened[1].name || follow.row !== 2 || follow.files[1].ref !== 'ref/u2.wav' || !follow.files[1].source || follow.table !== media.origin.table) fail('follow', JSON.stringify(follow));
  // Space plays it there.
  posted.length = 0;
  if (!press(' ') || posted[0].type !== 'toggle' || posted[0].to !== opened[1].name) fail('toggle', JSON.stringify(posted));

  // Clicking here brings row moves back, unless that window reports later.
  posted.length = 0;
  link('ref/u1.wav').click();
  await untilShown('ref/u1.wav');
  press('ArrowDown');
  await untilShown('ref/u2.wav');
  if (posted.some(message => message.type === 'follow')) fail('followed elsewhere', JSON.stringify(posted));
  const receive = message => window.eval('handleWindowMessage')(Object.assign({ to: window.name, from: 'rotari-window-x' }, message));
  receive({ type: 'active' });
  press('ArrowUp');
  await until('follow again', () => posted.some(message => message.type === 'follow' && message.to === 'rotari-window-x'));
  // Messages for another window, or from this one, do nothing.
  window.eval('handleWindowMessage')({ to: 'rotari-other', from: 'rotari-window-x', type: 'closed' });
  window.eval('handleWindowMessage')({ to: window.name, from: window.name, type: 'closed' });
  posted.length = 0;
  press(' ');
  if (posted[0].to !== 'rotari-window-x') fail('active window changed', JSON.stringify(posted));
  // Once it closes, they come back here.
  receive({ type: 'closed' });
  posted.length = 0;
  press('ArrowDown');
  await wait(200);
  if (posted.some(message => message.type === 'follow')) fail('followed a closed window', JSON.stringify(posted));

  // Arrow keys from the window move the table.
  const before = current();
  receive({ type: 'key', key: 'ArrowUp' });
  if (current() === before) fail('key from a window', current());
  await wait(200);

  // A file given back opens here and lets the other window know.
  window.focus = () => {};
  posted.length = 0;
  const tabs = () => document.querySelectorAll('.tv-pane .tv-tab').length;
  const count = tabs();
  receive({ type: 'dock', tab: 't9', media: { kind: 'image', ref: 'back.png', path: 'back.png', base: 'wd', source: media.source } });
  if (tabs() !== count + 1 || posted[0].type !== 'tab-moved' || posted[0].tab !== 't9' || posted[0].to !== 'rotari-window-x') fail('dock', JSON.stringify(posted));

  // A tab dragged in from another window lands in the pane, split by zone.
  const drag = (over, type, text, init) => {
    const event = new window.MouseEvent(type, Object.assign({ bubbles: true, cancelable: true }, init));
    Object.defineProperty(event, 'dataTransfer', { value: { types: ['application/x-rotari-tab'], getData: () => text, dropEffect: '' } });
    over.dispatchEvent(event);
    return event;
  };
  const dropped = JSON.stringify({ window: 'rotari-window-y', tab: 't5', media: { kind: 'image', ref: 'in.png', path: 'in.png', base: 'wd', source: media.source } });
  posted.length = 0;
  if (!drag(document.querySelector('.tv-pane .tv-tabs'), 'dragover', dropped).defaultPrevented) fail('strip dragover');
  drag(document.querySelector('.tv-pane .tv-tabs'), 'drop', dropped);
  if (!document.querySelector('.tv-pane-active .tv-tab-view strong').textContent.includes('in.png')) fail('dropped tab');
  if (posted[0].type !== 'tab-moved' || posted[0].to !== 'rotari-window-y' || posted[0].tab !== 't5') fail('dropped message', JSON.stringify(posted));

  // Anywhere else, it opens in the active pane; bad data, and a drag from this
  // window, do nothing.
  const table = document.querySelector('.tv-table-area');
  const after = tabs();
  posted.length = 0;
  drag(table, 'drop', 'not json');
  drag(table, 'drop', JSON.stringify({ window: window.name, tab: 't5', media: {} }));
  if (tabs() !== after || posted.length) fail('bad drop', tabs(), JSON.stringify(posted));
  drag(table, 'drop', dropped.replace('in.png', 'other.png').replace('in.png', 'other.png'));
  if (tabs() !== after + 1) fail('drop outside panes', tabs());
  window.eval('windowChannel = null');

  // So does the button, at no particular place.
  for (const close of document.querySelectorAll('.tv-pane button[aria-label="Close pane"]')) close.click();
  link('ref/u2.wav').click();
  await untilShown('ref/u2.wav');
  document.querySelector('.tv-pane button[aria-label="Open in a new window"]').click();
  if (opened.length !== 3 || opened[2].features.includes('left=') || document.querySelector('.tv-pane')) fail('button', opened[2].features);
  if (new window.URLSearchParams(new window.URL(opened[2].url, window.location.href).hash.slice(1)).get('home') !== window.name) fail('home', opened[2].url);
`)
	// The new window shows only that file, and closes with its last tab.
	media := url.QueryEscape(fmt.Sprintf(`{"kind":"image","ref":"gone.png","path":"gone.png","base":"wd","source":"project_name=default&run_id=%s&job_id=job-1&entry=wd","time":2}`, f.runID))
	runTableViewScript(t, options, f, "data.csv", "#home=rotari-main-1&popout="+media, rowNavigationHelpers+`
  await until('pane', () => document.querySelector('.tv-pane .tv-tab-view'));
  if (!document.body.classList.contains('tv-popout') || shown()[0] !== 'gone.png' || document.title !== 'gone.png · rotari') fail('window', document.title);
  if (document.querySelector('#tv-table tbody tr')) fail('table loaded', cells());
  const posted = [];
  window.__posted = posted;
  window.eval('windowChannel = { postMessage(message) { window.__posted.push(message); } }');

  // Arrow keys go to the table, and the table's follow changes the file.
  if (!press('ArrowDown') || posted[0].type !== 'active' || posted[1].type !== 'key' || posted[1].key !== 'ArrowDown' || posted[1].to !== 'rotari-main-1') fail('key', JSON.stringify(posted));
  if (press('ArrowDown', { ctrlKey: true })) fail('modified key taken');
  const tab = window.eval('panes').layout.tabs[0];
  tab.media.origin = { table: 'T', row: 1, column: 2 };
  const receive = message => window.eval('handleWindowMessage')(Object.assign({ to: window.name, from: 'rotari-main-1' }, message));
  receive({ type: 'follow', table: 'other', row: 2, files: [] });
  if (shown()[0] !== 'gone.png') fail('followed another table', shown());
  receive({ type: 'follow', table: 'T', row: 2, files: [null, null, { kind: 'image', ref: 'next.png', path: 'next.png', source: 'x', origin: { table: 'T', row: 2, column: 2 } }] });
  if (shown()[0] !== 'next.png') fail('follow', shown());

  // The button gives the file back, and says so when nothing answers.
  posted.length = 0;
  document.querySelector('button[aria-label="Back to the table"]').click();
  if (posted[0].type !== 'dock' || posted[0].to !== 'rotari-main-1' || posted[0].media.ref !== 'next.png') fail('dock', JSON.stringify(posted));
  await until('no answer', () => document.getElementById('tv-error').textContent.includes('did not answer'));
  if (shown()[0] !== 'next.png') fail('kept', shown());
  document.getElementById('tv-error').textContent = '';
  document.querySelector('button[aria-label="Back to the table"]').click();

  let closed = false;
  window.close = () => { closed = true; };
  receive({ type: 'tab-moved', tab: tab.id });
  if (!closed) fail('not closed');
  await wait(600);
  if (document.getElementById('tv-error').textContent !== '') fail('error after an answer');
`)
	runTableViewScript(t, options, f, "data.csv", "#popout=not-json", `
  await until('error', () => document.getElementById('tv-error').textContent.includes('could not be read'));
`)
}

// A document has no rows, so the keys leave the page alone.
func TestTableViewRowKeysIgnoreDocumentsInBrowser(t *testing.T) {
	data := t.TempDir()
	write(t, filepath.Join(data, "notes.md"), "# Notes\n")
	handler, err := TableViewHandler(filepath.Join(data, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	runTableViewPage(t, server.URL+"/", rowNavigationHelpers+`
  await until('document', () => document.querySelector('.tv-tab-view') && document.querySelector('.tv-table-area').hidden);
  for (const key of ['ArrowDown', 'ArrowUp', 'Enter', ' ']) {
    if (press(key)) fail('key taken', key);
  }
`)
}

// A plot computes its spectra once for what it shows: drawing again at the
// same size reuses them, a new size is drawn stretched until it has held,
// and a new height alone needs no new spectra; only a sideways or Shift
// wheel scrolls a plot. jsdom has no canvas or Web Audio, so the page gets
// stand-ins that count the work.
func TestTableViewPlotsComputeOnceInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(f.work, "u1.wav"), string(wav))
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=*.wav", `
  await until('files', () => document.querySelector('#tv-table a.tv-media-path'));
  window.devicePixelRatio = 1;
  const plot = { width: 100, height: 40 };
  for (const name of ['clientWidth', 'clientHeight']) {
    const inherited = Object.getOwnPropertyDescriptor(window.Element.prototype, name);
    Object.defineProperty(window.HTMLCanvasElement.prototype, name, {
      configurable: true,
      get() { return this.isConnected ? plot[name === 'clientWidth' ? 'width' : 'height'] : inherited.get.call(this); },
    });
  }
  const drawn = [];
  window.HTMLCanvasElement.prototype.getContext = function () {
    const canvas = this;
    return {
      createImageData: (width, height) => ({ width, height, data: new Uint8ClampedArray(width * height * 4) }),
      putImageData() {},
      drawImage: (source, x, y, width, height) => { if (canvas.classList.contains('tv-spectrogram')) drawn.push([source.width, source.height, width, height]); },
      clearRect() {},
      fillRect: (x, y, width, height) => { if (canvas.classList.contains('tv-spectrogram')) rects.push([x, y, width, height]); },
      fillText: (text) => { (canvas.classList.contains('tv-spectrogram') ? spectrogramLabels : waveLabels).push(text); },
      measureText: (text) => ({ width: text.length * 6 }),
    };
  };
  const rects = [], spectrogramLabels = [], waveLabels = [];
  window.AudioContext = class {
    async decodeAudioData() {
      const data = new Float32Array(8000).map((_, index) => Math.sin(index / 5));
      return { length: data.length, sampleRate: 16000, duration: 0.5, numberOfChannels: 1, getChannelData: () => data };
    }
    close() {}
  };
  window.spectra = 0;
  window.eval('const computed = magnitudes; magnitudes = function (...args) { window.spectra++; return computed(...args); };');
  const last = () => JSON.stringify(drawn[drawn.length - 1]);

  document.querySelector('#tv-table a.tv-media-path').click();
  await until('first drawing', () => window.spectra === 100 && drawn.length);
  if (last() !== '[100,40,100,40]') fail('first drawing', last());
  const view = window.eval('tabViews').values().next().value;

  // The plots have a grid and a vertical scale, and the time axis labels the
  // grid's round times where they fall. 8 kHz is the top of 16 kHz audio.
  plot.height = 200;
  rects.length = spectrogramLabels.length = waveLabels.length = 0;
  view.redraw();
  await until('tall plot', () => last() === '[100,200,100,200]');
  rects.length = spectrogramLabels.length = waveLabels.length = 0;
  view.redraw();
  if (JSON.stringify(spectrogramLabels) !== '["2 kHz","4 kHz","6 kHz","8 kHz"]') fail('frequency scale', spectrogramLabels);
  if (JSON.stringify(waveLabels) !== '["0.5","0","-0.5"]') fail('amplitude scale', waveLabels);
  const across = y => rects.some(rect => rect[1] === y && rect[2] === 100 && rect[3] === 1);
  if (!across(150) || !across(100) || !across(50)) fail('frequency grid', JSON.stringify(rects));
  const axis = [...view.axis.children];
  if (JSON.stringify(axis.map(label => label.textContent)) !== '["0.0 s","0.2 s","0.4 s"]') fail('time axis', view.axis.textContent);
  const down = x => rects.some(rect => rect[0] === x && rect[1] === 0 && rect[2] === 1 && rect[3] === 200);
  if (!down(0) || !down(40) || !down(80)) fail('time grid', JSON.stringify(rects));
  if (axis[1].style.left !== '40%') fail('label position', axis[1].style.left);
  plot.height = 40;
  view.redraw();
  await until('short plot', () => last() === '[100,40,100,40]');
  const spectraBefore = window.spectra;
  view.redraw();
  if (window.spectra !== spectraBefore) fail('grid recomputed spectra');
  window.spectra = 100;
  view.redraw();
  if (window.spectra !== 100 || last() !== '[100,40,100,40]') fail('same size computed again', window.spectra, last());

  // A new width is stretched until it holds, then computed once.
  plot.width = 120;
  view.redraw();
  plot.width = 150;
  view.redraw();
  view.redraw();
  if (window.spectra !== 100 || last() !== '[100,40,150,40]') fail('resizing computed', window.spectra, last());
  await until('settled width', () => window.spectra === 250);
  if (last() !== '[150,40,150,40]') fail('settled width drawing', last());
  await wait(300);
  if (window.spectra !== 250) fail('settled more than once', window.spectra);

  // A new height redraws the colours from the same spectra.
  plot.height = 60;
  view.redraw();
  await until('settled height', () => last() === '[150,60,150,60]');
  if (window.spectra !== 250) fail('height computed spectra', window.spectra);

  // A new range in view is computed at once.
  view.zoom(0.5);
  if (window.spectra !== 400 || last() !== '[150,60,150,60]') fail('zoom', window.spectra, last());

  // A plain wheel over a plot is left to scroll the page; a sideways or
  // Shift wheel scrolls the plot's range instead.
  const canvas = document.querySelector('canvas.tv-spectrogram');
  const roll = init => { const event = new window.WheelEvent('wheel', { cancelable: true, bubbles: true, ...init }); canvas.dispatchEvent(event); return event.defaultPrevented; };
  const start = () => view.range().start;
  const before = start();
  if (roll({ deltaY: 100 }) || start() !== before) fail('plain wheel taken', start());
  if (!roll({ deltaY: 100, shiftKey: true }) || !(start() > before)) fail('shift wheel', start());
  const shifted = start();
  if (!roll({ deltaX: -100, deltaY: 10 }) || !(start() < shifted)) fail('sideways wheel', start());
`)
}

// The spectrogram's FFT finds a tone in its bin, and its colour scale runs
// from viridis' first stop to its last; jsdom has no canvas to draw them.
// A histogram of equal values has one bar, and no choice of how many.
func TestTableViewHistogramOfEqualValuesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "data.csv"), "n\n7\n7\n")
	runTableViewScript(t, testOptions(f.baseDir, false), f, "data.csv", "", `
  await until('rows', () => cells().length === 2);
  document.querySelector('.tv-describe-toggle').click();
  await until('describe', () => document.querySelector('.tv-open-histogram'));
  document.querySelector('.tv-open-histogram').click();
  const pane = document.querySelector('.tv-pane-active');
  if (pane.querySelectorAll('.tv-histogram-bin').length !== 1 || pane.querySelector('select[aria-label="Number of bars"]')) fail('one bar', pane.innerHTML);
`)
}

func TestPlotTicksAreRound(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	command := exec.Command("node", "-e", `
const vm = require('vm');
const context = vm.createContext({ Math, Float32Array, Float64Array, Uint32Array, Uint8Array, Map, Number, String });
vm.runInContext(process.argv[1] + '; this.niceTicks = niceTicks; this.formatFrequency = formatFrequency;', context);
const check = (start, end, count, want) => {
  const got = context.niceTicks(start, end, count).values.map(value => Number(value.toFixed(6)));
  if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(start + '..' + end + ': ' + JSON.stringify(got));
};
check(0, 8000, 4, [0, 2000, 4000, 6000, 8000]);
check(0, 22050, 4, [0, 10000, 20000]);
check(0, 22050, 10, [0, 5000, 10000, 15000, 20000]);
check(0, 0.5, 5, [0, 0.1, 0.2, 0.3, 0.4, 0.5]);
check(1.23, 1.27, 4, [1.23, 1.24, 1.25, 1.26, 1.27]);
check(61, 125, 3, [100]);
check(5, 5, 4, []);
check(0, 1, 0, []);
for (const [hertz, want] of [[500, '500 Hz'], [1000, '1 kHz'], [2500, '2.5 kHz'], [22050, '22.05 kHz']]) {
  if (context.formatFrequency(hertz) !== want) throw new Error(context.formatFrequency(hertz));
}
`, tableViewPanesJS)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("ticks: %v\n%s", err, output)
	}
}

func TestSpectrogramFindsATone(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	command := exec.Command("node", "-e", `
const vm = require('vm');
const context = vm.createContext({ Math, Float32Array, Float64Array, Uint32Array, Uint8Array, Map });
vm.runInContext(process.argv[1] + '; this.magnitudes = magnitudes; this.viridisTable = viridisTable;', context);
const rate = 16000, size = 512, samples = new Float32Array(4096);
for (let index = 0; index < samples.length; index++) samples[index] = Math.sin(2 * Math.PI * 1000 * index / rate);
const out = new Float32Array(size / 2);
context.magnitudes(samples, 1000, size, out);
const peak = out.indexOf(Math.max(...out));
if (peak !== 1000 / rate * size) { console.error('peak bin', peak); process.exit(1); }
// Every bin matches the windowed DFT worked out directly.
const noise = new Float32Array(64).map((_, index) => Math.sin(index * index * 0.37));
const fast = new Float32Array(32);
context.magnitudes(noise, 0, 64, fast);
for (let bin = 0; bin < 32; bin++) {
  let real = 0, imag = 0;
  for (let index = 0; index < 64; index++) {
    const sample = noise[index] * (0.5 - 0.5 * Math.cos(2 * Math.PI * index / 63));
    real += sample * Math.cos(-2 * Math.PI * bin * index / 64);
    imag += sample * Math.sin(-2 * Math.PI * bin * index / 64);
  }
  if (Math.abs(fast[bin] - Math.hypot(real, imag)) > 1e-4) { console.error('bin', bin, fast[bin], Math.hypot(real, imag)); process.exit(1); }
}
// Past the end of the samples the window reads silence.
context.magnitudes(samples, samples.length, size, out);
if (Math.max(...out) !== 0) { console.error('silence', Math.max(...out)); process.exit(1); }
const table = context.viridisTable;
if ([...table.slice(0, 3)].join() !== '68,1,84' || [...table.slice(255 * 3)].join() !== '253,231,37') { console.error('colours', table.slice(0, 3), table.slice(255 * 3)); process.exit(1); }
`, tableViewPanesJS)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("spectrogram check failed: %v\n%s", err, output)
	}
}

// A directory's table file opens in a Table viewer of its own, as the
// directory's child, in a new tab.
func TestTableViewOpensTableFilesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "eval", "scores.csv"), "id,score\nu1,0.5\n")
	write(t, filepath.Join(f.work, "eval", "u1.wav"), "RIFF")
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=eval%2F*&expanded=eval", `
  await until('files', () => document.querySelector('#tv-table a.tv-open-table'));
  const links = [...document.querySelectorAll('#tv-table a.tv-open-table')];
  if (links.length !== 1 || links[0].target !== '_blank') fail('links', document.getElementById('tv-table').innerHTML);
  const opened = new window.URL(links[0].href);
  if (opened.pathname !== window.location.pathname || opened.searchParams.get('child') !== 'eval/scores.csv' || opened.searchParams.get('entry') !== 'wd' || opened.hash) fail('href', links[0].href);
  const info = await (await window.fetch('api/info' + opened.search)).json();
  if (info.kind !== 'file' || !info.name.endsWith('/eval/scores.csv')) fail('opened info', JSON.stringify(info));
`)
}

// A directory row's arrow shows that directory in the same window, the path
// above links back to each directory up to the source, Back returns, and
// files open in panes stay open, read from where they were opened.
func TestTableViewMovesBetweenDirectoriesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	write(t, filepath.Join(f.work, "eval", "ref", "u1.txt"), "ref one\n")
	write(t, filepath.Join(f.work, "eval", "hyp", "u1.txt"), "hyp one\n")
	write(t, filepath.Join(f.work, "eval", "scores.csv"), "id\nu1\n")
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=eval%2F**&sort=0&desc=1", `
  const enter = path => document.querySelector('#tv-table a.tv-enter[aria-label="Open ' + path + '"]');
  const crumbs = () => [...document.querySelectorAll('#tv-name a.tv-crumb')].map(a => a.textContent);
  const current = () => document.querySelector('#tv-name [aria-current="page"]').textContent;
  const up = () => document.querySelector('#tv-up .tv-up');
  const paths = () => cells().map(row => row[1].replace(/^[▸▾] /, '').replace(/(→|Open table)$/, ''));
  await until('source', () => enter('eval'));
  if (up().getAttribute('aria-disabled') !== 'true' || crumbs().length) fail('top directory up', document.getElementById('tv-up').innerHTML);
  // A modifier leaves the link to the browser: a new tab.
  const modified = new window.MouseEvent('click', { bubbles: true, cancelable: true, ctrlKey: true });
  enter('eval').dispatchEvent(modified);
  if (window.location.search.includes('child=')) fail('ctrl-click moved', window.location.href);
  enter('eval').click();
  await until('eval', () => current() === 'eval' && enter('ref'));
  const query = new window.URLSearchParams(window.location.search);
  if (query.get('child') !== 'eval' || query.get('entry') !== 'wd') fail('query', window.location.search);
  if (fragment().has('sort') || fragment().get('pattern') !== '**/*') fail('view not reset', window.location.hash);
  if (paths().join() !== 'hyp/,ref/,scores.csv') fail('eval rows', paths());
  enter('ref').click();
  await until('eval/ref', () => current() === 'ref' && paths().join() === 'u1.txt');
  if (crumbs().length !== 2 || crumbs()[1] !== 'eval') fail('crumbs', crumbs());
  document.querySelector('#tv-table .tv-text-file').click();
  const text = () => document.querySelector('.tv-pane-active .tv-text')?.textContent;
  await until('ref text', () => text() === 'ref one\n');
  if (!document.querySelector('.tv-pane-active .tv-tab-title').textContent.includes('eval/ref/u1.txt')) fail('pane title');
  up().click();
  await until('up to eval', () => current() === 'eval' && enter('hyp'));
  enter('hyp').click();
  await until('eval/hyp', () => current() === 'hyp' && paths().join() === 'u1.txt');
  if (text() !== 'ref one\n') fail('pane changed', text());
  document.querySelector('#tv-table .tv-text-file').click();
  await until('hyp text', () => text() === 'hyp one\n');
  if (document.querySelectorAll('.tv-pane-active .tv-tab').length !== 2) fail('tabs', document.querySelectorAll('.tv-tab').length);
  // The other tab still reads the file from eval/ref.
  [...document.querySelectorAll('.tv-tab')].find(tab => tab.title === 'eval/ref/u1.txt').querySelector('.tv-tab-name').click();
  await until('ref tab', () => text() === 'ref one\n');
  window.history.back();
  await until('back to eval', () => current() === 'eval' && enter('ref'));
  document.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'ArrowUp', altKey: true }));
  await until('Alt+Up to the source', () => !crumbs().length && enter('eval') && !window.location.search.includes('child='));
  enter('eval').click();
  // The name changes before the rows do, so wait for eval's own rows, and take its table
  // file's link by name rather than the first one, which the source's rows may still hold.
  await until('eval again', () => current() === 'eval' && enter('ref'));
  const scoresLink = [...document.querySelectorAll('#tv-table a.tv-open-table')].find(a => new window.URL(a.href).searchParams.get('child') === 'eval/scores.csv');
  if (!scoresLink) fail('no link to eval/scores.csv', document.getElementById('tv-table').innerHTML.slice(0, 2000));
  window.history.pushState(null, '', scoresLink.getAttribute('href'));
  window.dispatchEvent(new window.PopStateEvent('popstate'));
  await until('table file', () => current() === 'scores.csv' && headings().includes('id'));
  document.querySelector('#tv-name a.tv-crumb:last-of-type').click();
  await until('table file up', () => current() === 'eval' && enter('ref'));
  window.history.pushState(null, '', '?' + new window.URLSearchParams({ ...Object.fromEntries(query), child: 'eval/missing' }));
  window.dispatchEvent(new window.PopStateEvent('popstate'));
  await until('missing directory', () => document.getElementById('tv-error').textContent && !document.querySelector('#tv-table tbody'));
  if (current() !== 'missing') fail('missing name', current());
`)
}

// A directory's file that is not media opens as text in the active pane,
// whatever its name, a page at a time; a binary file says so.
func TestTableViewShowsTextFilesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	long := strings.Repeat("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde\n", 1500)
	write(t, filepath.Join(f.work, "notes", "README"), "no extension\n")
	write(t, filepath.Join(f.work, "notes", "config.yaml"), "lr: 0.1\n")
	write(t, filepath.Join(f.work, "notes", "long.log"), long)
	write(t, filepath.Join(f.work, "notes", "model.pt"), "PK\x00\x01torch")
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=notes%2F*&expanded=notes", `
  await until('files', () => document.querySelectorAll('#tv-table .tv-text-file').length === 4);
  const open = name => [...document.querySelectorAll('#tv-table .tv-text-file')].find(button => button.textContent === 'notes/' + name).click();
  const view = () => document.querySelector('.tv-pane-active .tv-tab-view');
  const shown = async (what, check) => { await until(what, () => view() && check(view())); return view(); };
  open('README');
  await shown('README', v => v.querySelector('.tv-text')?.textContent === 'no extension\n');
  if (document.querySelector('.tv-pane-active .tv-tab-kind').textContent !== 'text') fail('tab kind');
  open('config.yaml');
  await shown('yaml', v => v.textContent.includes('notes/config.yaml') && v.querySelector('.tv-text')?.textContent === 'lr: 0.1\n');
  // A long file loads more on demand.
  open('long.log');
  let v = await shown('first page', v => v.querySelector('.tv-text')?.textContent.length > 0);
  const more = [...v.querySelectorAll('button')].find(button => button.textContent === 'Load more');
  if (!more || more.hidden || v.querySelector('.tv-text').textContent.length >= `+strconv.Itoa(len(long))+`) fail('first page', v.querySelector('.tv-analysis-status').textContent);
  more.click();
  await shown('second page', v => v.querySelector('.tv-text').textContent.length === `+strconv.Itoa(len(long))+`);
  if (!more.hidden || v.querySelector('.tv-analysis-status').textContent) fail('whole file');
  open('model.pt');
  await shown('binary', v => v.querySelector('.tv-analysis-status').textContent === 'Cannot show as text: binary file: download it instead');
`)
}

// A file of more than one channel offers its channels and their mix for the
// plots; jsdom cannot decode audio, so the page gets a stand-in decoder.
func TestTableViewChoosesAChannelInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mono.wav", "stereo.wav", "array.wav"} {
		write(t, filepath.Join(f.work, "set", name), string(wav))
	}
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=set%2F*.wav&expanded=set", `
  await until('files', () => document.querySelectorAll('#tv-table a.tv-media-path').length === 3);
  // The stand-in decodes a file into as many channels as its name says.
  const counts = { mono: 1, stereo: 2, array: 10 };
  let decoding = 0;
  // It also records how the players' sound is routed.
  const contexts = [];
  const node = name => ({ name, links: [], connect(to, output) { this.links.push(to.name + (output === undefined ? '' : ':' + output)); }, disconnect() { this.links = []; } });
  window.AudioContext = class {
    constructor() { this.state = 'suspended'; this.destination = node('speakers'); this.nodes = []; contexts.push(this); }
    resume() { this.state = 'running'; return Promise.resolve(); }
    createMediaElementSource(player) { const made = node('player'); made.player = player; this.nodes.push(made); return made; }
    createChannelSplitter(count) { const made = node('splitter' + count); this.nodes.push(made); return made; }
    createGain() { const made = node('single'); this.nodes.push(made); return made; }
    decodeAudioData() {
      const count = counts[decoding];
      return Promise.resolve({
        numberOfChannels: count, sampleRate: 8000, length: 8000, duration: 1,
        getChannelData: channel => new window.Float32Array(8000).fill(channel / 10),
      });
    }
    close() {}
  };
  window.HTMLCanvasElement.prototype.getContext = () => null;
  const open = async name => {
    decoding = name;
    [...document.querySelectorAll('#tv-table a.tv-media-path')].find(a => a.textContent === 'set/' + name + '.wav').click();
    const view = document.querySelector('.tv-pane-active .tv-tab-view');
    await until(name + ' decoded', () => view.querySelector('.tv-analysis-status').textContent === '');
    return view;
  };

  // One channel has nothing to choose.
  let view = await open('mono');
  if (!view.querySelector('.tv-channel-choice').hidden) fail('mono choice', view.innerHTML);

  // Two channels are buttons, Mix first and pressed.
  view = await open('stereo');
  const choice = view.querySelector('.tv-channel-choice');
  const pressed = () => [...choice.querySelectorAll('button[aria-pressed="true"]')].map(button => button.textContent);
  if (choice.hidden || JSON.stringify([...choice.querySelectorAll('button')].map(button => button.textContent)) !== '["Mix","1","2"]' || JSON.stringify(pressed()) !== '["Mix"]') fail('stereo choice', choice.outerHTML);
  // Mix plays the player as it is, without the audio context.
  const playing = () => contexts.find(context => context.nodes.length);
  if (playing()) fail('routed on mix');
  choice.querySelector('button[aria-label="Channel 2"]').click();
  if (JSON.stringify(pressed()) !== '["2"]') fail('channel 2', choice.outerHTML);
  // A chosen channel plays alone, from every speaker, in a resumed context.
  const graph = () => JSON.stringify(playing().nodes.map(made => made.name + '>' + made.links.join(',')));
  if (graph() !== '["player>splitter2","splitter2>single:1","single>speakers"]' || playing().state !== 'running' || playing().nodes[0].player !== view.querySelector('audio')) fail('channel 2 graph', graph());
  // Back to Mix, the player goes straight to the speakers.
  choice.querySelector('button[aria-label="Mix"]').click();
  if (graph() !== '["player>speakers","splitter2>","single>speakers"]') fail('mix graph', graph());
  choice.querySelector('button[aria-label="Channel 2"]').click();

  // The channel stays with its file when another tab is shown.
  view = await open('array');
  const list = view.querySelector('.tv-channel-choice select[aria-label="Channel"]');
  if (!list || list.options.length !== 11 || list.value !== 'mix') fail('many channels', view.innerHTML);
  list.value = '9';
  list.dispatchEvent(new window.Event('change'));
  if (view.querySelector('.tv-channel-choice select').value !== '9') fail('channel 10');
  // Every player shares one playback context.
  if (contexts.filter(context => context.nodes.length).length !== 1 || !playing().nodes.some(made => made.name === 'splitter10')) fail('one context', contexts.length);
  [...document.querySelectorAll('.tv-tab-name')].find(button => button.textContent.endsWith('stereo.wav')).click();
  if (JSON.stringify([...document.querySelectorAll('.tv-pane-active .tv-channel-choice button[aria-pressed="true"]')].map(button => button.textContent)) !== '["2"]') fail('kept channel');
`)
}

// channelSamples is a channel as it is, or the mean of them all.
func TestChannelSamplesMixesChannels(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	command := exec.Command("node", "-e", `
const vm = require('vm');
const context = vm.createContext({ Math, Float32Array, Float64Array, Uint32Array, Uint8Array, Map });
vm.runInContext(process.argv[1] + '; this.channelSamples = channelSamples;', context);
const left = Float32Array.from([1, 0.5, 0]), right = Float32Array.from([0, 0.5, -1]);
const audio = { channels: [left, right] };
if (context.channelSamples(audio, 1) !== right) { console.error('channel'); process.exit(1); }
const mix = context.channelSamples(audio, 'mix');
if ([...mix].join() !== '0.5,0.5,-0.5') { console.error('mix', mix); process.exit(1); }
if (context.channelSamples(audio, 'mix') !== mix) { console.error('mix not kept'); process.exit(1); }
const mono = { channels: [left] };
if (context.channelSamples(mono, 'mix') !== left) { console.error('mono'); process.exit(1); }
`, tableViewPanesJS)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("channel check failed: %v\n%s", err, output)
	}
}

func TestTableViewMovesTabsAndLinksPanesInBrowser(t *testing.T) {
	f := newArtifactFixture(t)
	wav, err := os.ReadFile(filepath.Join("..", "tableview", "testdata", "media", "pcm16.wav"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.wav", "b.wav", "c.wav"} {
		write(t, filepath.Join(f.work, "set", name), string(wav))
	}
	runTableViewScript(t, testOptions(f.baseDir, false), f, "wd", "#pattern=set%2F*.wav&expanded=set", `
  await until('files', () => document.querySelectorAll('#tv-table a.tv-media-path').length === 3);
  const link = path => [...document.querySelectorAll('#tv-table a.tv-media-path')].find(a => a.textContent === path);
  const paneList = () => [...document.querySelectorAll('.tv-pane')];
  const tabs = pane => [...pane.querySelectorAll('.tv-tab')].map(tab => tab.title);
  const layout = () => JSON.stringify(paneList().map(tabs));
  for (const path of ['set/a.wav', 'set/b.wav', 'set/c.wav']) link(path).click();
  const first = () => paneList()[0];
  const source = (pane, ref) => ({ pane: pane.dataset.pane, tab: [...pane.querySelectorAll('.tv-tab')].find(tab => tab.title === ref).dataset.tab });

  // Tabs are draggable; a drop in the middle of the strip reorders them.
  if (!first().querySelector('.tv-tab').draggable) fail('not draggable');
  window.dropTab(source(first(), 'set/c.wav'), first().dataset.pane, 'center', 0);
  if (layout() !== '[["set/c.wav","set/a.wav","set/b.wav"]]') fail('reorder', layout());

  // A drop near the right edge splits the pane and moves the tab into the
  // new pane, which becomes active.
  window.dropTab(source(first(), 'set/b.wav'), first().dataset.pane, 'right', 0);
  if (layout() !== '[["set/c.wav","set/a.wav"],["set/b.wav"]]' || !paneList()[1].classList.contains('tv-pane-active')) fail('split right', layout());
  window.dropTab(source(first(), 'set/a.wav'), paneList()[1].dataset.pane, 'top', 0);
  if (layout() !== '[["set/c.wav"],["set/a.wav"],["set/b.wav"]]' || !document.querySelector('.tv-split-row .tv-split-column')) fail('split top', layout(), document.getElementById('tv-pane-root').innerHTML);

  // Moving the last tab out of a pane closes it; dropping a pane's only tab
  // on its own edge changes nothing.
  window.dropTab(source(first(), 'set/c.wav'), paneList()[2].dataset.pane, 'center', 1);
  if (layout() !== '[["set/a.wav"],["set/b.wav","set/c.wav"]]') fail('close emptied', layout());
  window.dropTab(source(first(), 'set/a.wav'), first().dataset.pane, 'left', 0);
  if (layout() !== '[["set/a.wav"],["set/b.wav","set/c.wav"]]') fail('own edge', layout());

  // Link time & zoom is one toggle for every pane.
  const toggle = () => document.querySelector('.tv-link-toggle');
  if (!toggle() || toggle().getAttribute('aria-pressed') !== 'false') fail('link toggle', document.getElementById('tv-pane-toolbar').innerHTML);
  toggle().click();
  if (toggle().getAttribute('aria-pressed') !== 'true') fail('linked');
  toggle().click();
  if (toggle().getAttribute('aria-pressed') !== 'false') fail('unlinked');
`)
}
