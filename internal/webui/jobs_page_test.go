package webui

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/joblist"
)

// A Recent jobs row shows the command under the job name, as the run page
// does, and the attempt ID by its end with the whole ID kept in its text,
// title, and copy button.
func TestJobsHTMLShowsCommandUnderJobAndAttemptByItsEnd(t *testing.T) {
	attempt := "att_20261010-205222-3185eff0-c18f6264e-1-0"
	page := jobsHTML("/", nil, []joblist.Row{{State: "success", Project: "p", RunID: "r", JobName: "decode[1]", Command: "sh -c echo…", FullCommand: "sh -c echo hi", AttemptID: attempt}}, jobsSelection{}, false, false)
	for _, want := range []string{
		`decode[1]</a><div class="jobs-command"><code title="sh -c echo hi">sh -c echo…</code>`,
		`data-copy-value="sh -c echo hi"`,
		`<td class="jobs-attempt" data-sort-value="` + attempt + `"><span class="id-tail" title="` + attempt + `"><bdi>` + attempt + `</bdi></span>`,
		`data-copy-value="` + attempt + `"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("jobs page lacks %q", want)
		}
	}
	if strings.Contains(page, `data-sort="command"`) {
		t.Error("jobs page still has a separate Command column")
	}
}

// The Recent jobs subtitle names what the page lists: the dragged range,
// or the window before now.
func TestJobsHTMLDescribesSelection(t *testing.T) {
	from := time.Date(2026, 10, 23, 14, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		selection jobsSelection
		want      string
	}{
		{jobsSelection{}, "Unfinished jobs and jobs finished in the last 24 hours"},
		{jobsSelection{Window: 6 * time.Hour}, "Unfinished jobs and jobs finished in the last 6 hours"},
		{jobsSelection{Window: time.Hour}, "Unfinished jobs and jobs finished in the last hour"},
		{jobsSelection{From: from, To: to}, "Unfinished jobs and jobs finished from " + joblist.FormatTimestamp(from) + " to " + joblist.FormatTimestamp(to)},
	} {
		page := jobsHTML("/", nil, nil, test.selection, true, false)
		if !strings.Contains(page, `<div class="facts">`+test.want+`</div>`) {
			t.Errorf("selection %+v: jobs page lacks subtitle %q", test.selection, test.want)
		}
	}
}

// Only the live page has the range bar, given this basedir, the Activity
// API under its prefix, and the selection; the static export keeps the
// default window.
func TestJobsHTMLHasRangeBarOnlyWhenLive(t *testing.T) {
	basedirs := []webBaseDir{{ID: "b1", Path: "/data/one"}, {ID: "b2", Path: "/data/two"}}
	from := time.Date(2026, 10, 23, 14, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		selection jobsSelection
		want      string
	}{
		{jobsSelection{}, `<div class="jobs-range" data-activity-url="/_basedir/b2/api/activity" data-basedir-id="b2" data-window="24h"></div>`},
		{jobsSelection{Window: time.Hour}, `data-window="1h"></div>`},
		{jobsSelection{From: from, To: from.Add(time.Hour)}, `data-window="" data-from="2026-10-23T14:00:00Z" data-to="2026-10-23T15:00:00Z"></div>`},
	} {
		if page := jobsHTML("/_basedir/b2/", nil, nil, test.selection, true, false, basedirs); !strings.Contains(page, test.want) {
			t.Errorf("selection %+v: live jobs page lacks %q", test.selection, test.want)
		}
	}
	if page := jobsHTML("/", nil, nil, jobsSelection{}, false, false); strings.Contains(page, `class="jobs-range"`) {
		t.Error("static jobs page has a range bar it cannot use")
	}
}

func TestParseJobsSelection(t *testing.T) {
	now := time.Date(2026, 10, 29, 8, 30, 0, 0, time.UTC)
	selection, err := parseJobsSelection(url.Values{"view": {"1d"}}, now)
	if err != nil || !selection.Default || len(selection.Spans) != 0 {
		t.Fatalf("no choice = %#v, %v; want the default, left to resolveJobsDefault", selection, err)
	}
	for text, want := range map[string]time.Duration{"24h": 24 * time.Hour, "6h": 6 * time.Hour, "1h": time.Hour} {
		selection, err := parseJobsSelection(url.Values{"window": {text}, "view": {"30d"}, "end": {"2026-09-30T00:00:00Z"}}, now)
		if err != nil || selection.Window != want || len(selection.Spans) != 1 || !selection.Spans[0].Start.Equal(now.Add(-want)) {
			t.Fatalf("window %s = %#v, %v; want the open span of %s", text, selection, err, want)
		}
	}
	// A range may end after now, and lie long before the bar's first view.
	from := time.Date(2025, 1, 16, 9, 0, 0, 0, time.FixedZone("JST", 9*3600))
	selection, err = parseJobsSelection(url.Values{"from": {from.Format(time.RFC3339)}, "to": {"2025-01-17T00:00:00Z"}}, now)
	if err != nil || !selection.From.Equal(from) || len(selection.Spans) != 1 || !selection.Spans[0].Start.Equal(from) || !selection.Spans[0].End.Equal(time.Date(2025, 1, 17, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("range = %#v, %v; want the span from %v to 2025-01-17T00:00Z", selection, err, from)
	}
	if _, err := parseJobsSelection(url.Values{"from": {"2026-10-29T00:00:00Z"}, "to": {"2026-10-30T00:00:00Z"}}, now); err != nil {
		t.Fatalf("range ending after now: %v", err)
	}
	for _, query := range []url.Values{
		{"since": {"7d"}},
		{"window": {"2h"}},
		{"window": {"6h", "1h"}},
		{"window": {"6h"}, "from": {"2026-10-28T00:00:00Z"}, "to": {"2026-10-29T00:00:00Z"}},
		{"from": {"2026-10-28T00:00:00Z"}},
		{"to": {"2026-10-29T00:00:00Z"}},
		{"from": {"2026-10-28"}, "to": {"2026-10-29T00:00:00Z"}},
		{"from": {"2026-10-28T00:00:00Z"}, "to": {"tomorrow"}},
		{"from": {"2026-10-28T00:00:00Z"}, "to": {"2026-10-28T00:00:00Z"}},
		{"from": {"2026-10-29T00:00:00Z"}, "to": {"2026-10-28T00:00:00Z"}},
		{"from": {"2026-10-29T09:00:00Z"}, "to": {"2026-10-29T10:00:00Z"}},
		{"from": {"2026-09-28T00:00:00Z"}, "to": {"2026-10-28T00:00:01Z"}},
		{"view": {"14d"}},
		{"end": {"2026-10-29"}},
		{"end": {"2026-10-30T00:00:00Z"}},
	} {
		if selection, err := parseJobsSelection(query, now); err == nil {
			t.Errorf("parseJobsSelection(%v) = %#v, want an error", query, selection)
		}
	}
}

// With nothing chosen, the page lists the 24 hours before now unless the
// latest run ended before them; then it lists the 24 hours up to that end,
// rounded up to the minute, taking the latest end even when a run that
// started earlier ended later.
func TestResolveJobsDefaultFollowsLatestRun(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	runID := func(started time.Time, n int) string {
		return started.Format("20060102-150405") + fmt.Sprintf("-%08d", n)
	}
	recent := func(t *testing.T, selection jobsSelection) {
		t.Helper()
		if selection.Latest || selection.Window != 24*time.Hour || len(selection.Spans) != 1 || !selection.Spans[0].Start.Equal(now.Add(-24*time.Hour)) || !selection.Spans[0].End.IsZero() {
			t.Fatalf("selection = %#v, want the 24 hours before now", selection)
		}
	}

	empty := t.TempDir()
	selection, err := resolveJobsDefault(jobsSelection{Default: true}, empty, now)
	if err != nil {
		t.Fatal(err)
	}
	recent(t, selection)

	fresh := t.TempDir()
	started := now.Add(-30 * time.Hour)
	writeTestJobsRun(t, fresh, "demo", runID(started, 1), "job", started, now.Add(-20*time.Hour), 0)
	if selection, err = resolveJobsDefault(jobsSelection{Default: true}, fresh, now); err != nil {
		t.Fatal(err)
	}
	recent(t, selection)

	old := t.TempDir()
	// The earlier run ends last, 5 days ago at 18:03:20.
	longStart := now.Add(-8 * 24 * time.Hour)
	longEnd := time.Date(2026, 10, 24, 18, 3, 20, 0, time.UTC)
	writeTestJobsRun(t, old, "demo", runID(longStart, 1), "long-job", longStart, longEnd, 0)
	shortStart := now.Add(-6 * 24 * time.Hour)
	writeTestJobsRun(t, old, "other", runID(shortStart, 2), "short-job", shortStart, shortStart.Add(time.Hour), 0)
	if selection, err = resolveJobsDefault(jobsSelection{Default: true}, old, now); err != nil {
		t.Fatal(err)
	}
	to := time.Date(2026, 10, 24, 18, 4, 0, 0, time.UTC)
	if !selection.Latest || !selection.To.Equal(to) || !selection.From.Equal(to.Add(-24*time.Hour)) || len(selection.Spans) != 1 || !selection.Spans[0].End.Equal(to) {
		t.Fatalf("selection = %#v, want the 24 hours up to %v", selection, to)
	}
	if text := jobsScopeText(selection); text != "Unfinished jobs and jobs finished in the 24 hours up to "+joblist.FormatTimestamp(to)+", around the latest run" {
		t.Fatalf("subtitle = %q", text)
	}

	chosen := jobsSelection{Window: time.Hour, Spans: []joblist.Interval{{Start: now.Add(-time.Hour)}}}
	if selection, err = resolveJobsDefault(chosen, old, now); err != nil || selection.Latest || selection.Window != time.Hour {
		t.Fatalf("a chosen window = %#v, %v; want it kept", selection, err)
	}
}

// The bar draws this basedir's runs only, ticks, and the selected window;
// dragging selects a range snapped to the hour in the default 7d view, a
// click selects the local day under it, dragging the selection moves it,
// presets go back to now while view buttons keep the bar's page, and ‹ and
// › page by the view's width, › disabled at now.
func TestJobsRangeBarInBrowser(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	htmlPath := filepath.Join(t.TempDir(), "jobs.html")
	basedirs := []webBaseDir{{ID: "b1", Path: "/data/one", Current: true}, {ID: "b2", Path: "/data/two"}}
	if err := os.WriteFile(htmlPath, []byte(jobsHTML("/", nil, nil, jobsSelection{Window: 6 * time.Hour}, true, false, basedirs)), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
const fs = require('fs');
const { JSDOM } = require('jsdom');
const hour = 3600e3, day = 24 * hour;
const fail = (...args) => { console.error(...args); process.exit(2); };
const now = Date.now();
const iso = time => new Date(time).toISOString().replace(/\.\d{3}Z$/, 'Z');
async function open(search) {
  const fetched = [];
  const dom = new JSDOM(fs.readFileSync(process.argv[1], 'utf8'), {
    url: 'http://localhost/jobs/' + search,
    runScripts: 'dangerously',
    beforeParse(window) {
      window.fetch = async url => {
        fetched.push(url);
        return { ok: true, json: async () => ({ runs: [
          { basedir_id: 'b1', project_name: 'p', run_id: 'r1', lifecycle: 'finished', exit_code: 1, started_at: iso(now - 2 * day), ends_at: iso(now - 2 * day + hour) },
          { basedir_id: 'b2', project_name: 'q', run_id: 'r2', lifecycle: 'running', exit_code: null, started_at: iso(now - day), ends_at: iso(now) },
        ] }) };
      };
      window.HTMLElement.prototype.getBoundingClientRect = function () { return { left: 0, top: 0, width: 1000, height: 48 }; };
    },
  });
  await new Promise(resolve => setTimeout(resolve, 20));
  const { window } = dom;
  const visited = [];
  window.jobsNavigateTo = url => visited.push(decodeURIComponent(url));
  const document = window.document;
  const buttons = label => document.querySelector('.jobs-range-group[aria-label="' + label + '"]').querySelectorAll('button');
  const bar = document.querySelector('.jobs-range-bar');
  const pointer = (target, type, x) => target.dispatchEvent(new window.MouseEvent(type, { bubbles: true, button: 0, clientX: x }));
  return { window, document, visited, fetched, buttons, bar, pointer };
}
const param = (url, name) => new URL('http://localhost' + url).searchParams.get(name);
(async () => {
  let page = await open('');
  const presets = page.buttons('Window'), views = page.buttons('Bar width');
  if ([...presets].map(b => b.getAttribute('aria-pressed')).join() !== 'false,true,false') fail('presets', [...presets].map(b => b.textContent));
  if ([...views].map(b => b.getAttribute('aria-pressed')).join() !== 'false,true,false') fail('views');
  if (page.fetched.length !== 1 || param(page.fetched[0].replace(/^[^?]*/, '/x'), 'to') === null) fail('fetched', page.fetched);
  const runs = page.document.querySelectorAll('.jobs-range-run');
  if (runs.length !== 1 || !runs[0].classList.contains('tone-bad')) fail('runs', [...runs].map(r => r.outerHTML));
  if (page.document.querySelectorAll('.jobs-range-tick').length < 6) fail('ticks');
  const selection = page.document.querySelector('.jobs-range-selection');
  if (selection.hidden || Math.abs(parseFloat(selection.style.width) - 100 / 28) > 0.01) fail('window selection', selection.style.width);
  const later = page.document.querySelector('[aria-label="Later"]');
  if (!later.disabled) fail('later enabled at now');

  // Drag from 1/2 to 3/4 of the 7d bar: 3.5 to 5.25 days after its start.
  page.pointer(page.bar, 'pointerdown', 500);
  page.pointer(page.bar, 'pointermove', 750);
  page.pointer(page.bar, 'pointerup', 750);
  let url = page.visited.pop();
  let from = Date.parse(param(url, 'from')), to = Date.parse(param(url, 'to'));
  const start = now - 7 * day;
  if (from % hour || to % hour || Math.abs(from - (start + 3.5 * day)) > hour || Math.abs(to - (start + 5.25 * day)) > hour || param(url, 'window') !== null) fail('drag', url);
  // A click selects the local (here UTC) day under it.
  page.pointer(page.bar, 'pointerdown', 500);
  page.pointer(page.bar, 'pointerup', 501);
  url = page.visited.pop();
  from = Date.parse(param(url, 'from'));
  to = Date.parse(param(url, 'to'));
  if (from % day || to - from !== day || from > start + 3.5 * day || to <= start + 3.5 * day) fail('click', url);
  presets[0].click();
  views[2].click();
  page.document.querySelector('[aria-label="Earlier"]').click();
  const want = ['/jobs/?window=1h', '/jobs/?window=6h&view=30d'];
  if (JSON.stringify(page.visited.slice(0, 2)) !== JSON.stringify(want)) fail('buttons', page.visited, want);
  url = page.visited[2];
  if (param(url, 'window') !== '6h' || Math.abs(Date.parse(param(url, 'end')) - (now - 7 * day)) > 1000) fail('earlier', url);

  // A range opened from a link before the view is shown; moving it keeps its length.
  const linkFrom = now - 20 * day - (now % hour), linkTo = linkFrom + 2 * day;
  fs.writeFileSync(process.argv[1], fs.readFileSync(process.argv[1], 'utf8').replace('data-window="6h"', 'data-window="" data-from="' + iso(linkFrom) + '" data-to="' + iso(linkTo) + '"'));
  page = await open('?from=' + iso(linkFrom) + '&to=' + iso(linkTo));
  const shown = page.document.querySelector('.jobs-range-selection');
  if (shown.hidden || Math.abs(parseFloat(shown.style.left) - (5 / 7) * 100) > 0.01) fail('linked range', shown.hidden, shown.style.left);
  if (page.document.querySelector('[aria-label="Later"]').disabled) fail('later disabled before now');
  page.pointer(shown, 'pointerdown', 800);
  page.pointer(page.bar, 'pointermove', 700);
  page.pointer(page.bar, 'pointerup', 700);
  url = page.visited.pop();
  from = Date.parse(param(url, 'from'));
  to = Date.parse(param(url, 'to'));
  if (to - from !== 2 * day || Math.abs(from - (linkFrom - 0.7 * day)) > hour || Math.abs(Date.parse(param(url, 'end')) - linkTo) > 1000) fail('move', url);
  page.document.querySelector('[aria-label="Later"]').click();
  url = page.visited.pop();
  if (param(url, 'from') !== iso(linkFrom) || param(url, 'end') !== iso(linkTo + 7 * day)) fail('later', url);
  process.exit(0);
})().catch(error => fail(error));
`
	command := exec.Command("node", "-e", script, htmlPath)
	command.Env = append(os.Environ(), "TZ=UTC")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("range bar check failed: %v\n%s", err, output)
	}
}
