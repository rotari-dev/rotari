package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/stickies"
)

func stickiesRequest(handler http.Handler, method, page, body string) *httptest.ResponseRecorder {
	target := "/api/stickies?page=" + url.QueryEscape(page)
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// Sticky notes are saved per page, read back for that page only, and kept in
// the basedir where the CLI reads them.
func TestStickiesAPIRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	handler := siteFor(baseDir).handler()
	save := `{"page":"projects/demo","notes":[{"id":"a","text":"look at seed 2","color":"pink","x":10,"y":20}]}`
	if response := stickiesRequest(handler, http.MethodPost, "", save); response.Code != http.StatusOK {
		t.Fatalf("POST status = %d: %s", response.Code, response.Body)
	}
	response := stickiesRequest(handler, http.MethodGet, "projects/demo", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "look at seed 2") {
		t.Fatalf("GET = %d: %s", response.Code, response.Body)
	}
	if other := stickiesRequest(handler, http.MethodGet, "projects/other", ""); strings.Contains(other.Body.String(), "seed 2") {
		t.Fatalf("another page shows the note: %s", other.Body)
	}
	pages, err := stickies.Pages(baseDir)
	if err != nil || len(pages) != 1 || pages[0].Page != "projects/demo" {
		t.Fatalf("Pages() = %#v, %v", pages, err)
	}
}

func TestStickiesAPIRejectsInvalidAndReadOnly(t *testing.T) {
	baseDir := t.TempDir()
	handler := siteFor(baseDir).handler()
	for name, body := range map[string]string{
		"unknown color": `{"page":"p","notes":[{"id":"a","color":"red"}]}`,
		"no page":       `{"notes":[]}`,
		"bad json":      `{`,
	} {
		if response := stickiesRequest(handler, http.MethodPost, "", body); response.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, response.Code)
		}
	}
	if response := stickiesRequest(handler, http.MethodDelete, "p", ""); response.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE status = %d, want 405", response.Code)
	}
	readOnly := site{Options: testOptions(baseDir, false), notificationSession: newNotificationSession()}.handler()
	save := `{"page":"p","notes":[{"id":"a","color":"yellow"}]}`
	if response := stickiesRequest(readOnly, http.MethodPost, "", save); response.Code != http.StatusForbidden {
		t.Errorf("read-only POST status = %d, want 403", response.Code)
	}
	if response := stickiesRequest(readOnly, http.MethodGet, "p", ""); response.Code != http.StatusOK {
		t.Errorf("read-only GET status = %d, want 200", response.Code)
	}
}

// The live Recent jobs page has the same Sign out, Stickies, and Launches
// controls as the application pages; its static export has none of them,
// since it has no server behind it.
func TestRecentJobsPageHasTheSharedTopBarControls(t *testing.T) {
	live := jobsPage(livePage{logout: true}, "/", nil, nil, jobsSelection{}, true, false, "")
	for _, want := range []string{`action="/_auth/logout"`, `id="stickies-menu"`, `id="launches-button"`, "function showJobsPageLaunches", "function renderStickies"} {
		if !strings.Contains(live, want) {
			t.Errorf("live Recent jobs page lacks %s", want)
		}
	}
	if strings.Contains(live, "__ROTARI_") {
		t.Error("live Recent jobs page has an unreplaced placeholder")
	}
	static := jobsHTML("/", nil, nil, jobsSelection{}, false, false)
	for _, unwanted := range []string{`action="/_auth/logout"`, `id="stickies-menu"`, `id="launches-button"`, "renderStickies"} {
		if strings.Contains(static, unwanted) {
			t.Errorf("static Recent jobs page has %s", unwanted)
		}
	}
	if noAuth := jobsPage(livePage{}, "/", nil, nil, jobsSelection{}, true, false, ""); strings.Contains(noAuth, `action="/_auth/logout"`) {
		t.Error("Sign out is offered without a sign-in")
	}
}

// The application page keeps the sidebar's sticky-note marks through every
// drawing of the sidebar, not only through a render of the page.
func TestApplicationPageMarksStickiesInTheSidebar(t *testing.T) {
	page := testSite().webHTML()
	for _, want := range []string{"function markSidebarStickies", "renderSidebar = function"} {
		if !strings.Contains(page, want) {
			t.Errorf("application page lacks %s", want)
		}
	}
}

// On the Recent jobs page, sidebar links of pages with notes carry a dot, the
// links of their parents a faint one, and the logo none; and the Stickies and
// Settings menus open independently of each other.
func TestRecentJobsPageMarksStickiesAndKeepsSettingsMenuWorking(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	htmlPath := filepath.Join(t.TempDir(), "jobs.html")
	page := jobsPage(livePage{}, "/", []string{"alpha", "beta"}, nil, jobsSelection{}, true, false, "")
	if err := os.WriteFile(htmlPath, []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
const fs = require('fs');
const { JSDOM, VirtualConsole } = require('jsdom');
const errors = [];
const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', error => errors.push(error.stack || String(error)));
const pages = [
  { page: 'project/alpha', notes: [{ id: 'a', text: 'check the seed 2 result carefully today', color: 'pink' }, { id: 'b', text: 'two', color: 'blue' }] },
  { page: 'project/beta/run/r1', notes: [{ id: 'c', text: 'deep', color: 'green' }] },
  { page: '/', notes: [{ id: 'd', text: 'home', color: 'orange' }] },
];
const dom = new JSDOM(fs.readFileSync(process.argv[1], 'utf8'), {
  url: 'http://localhost/jobs/',
  runScripts: 'dangerously',
  virtualConsole,
  beforeParse(window) {
    window.setInterval = () => 1;
    window.fetch = async url => {
      url = String(url);
      if (url === '/api/stickies') return { ok: true, json: async () => ({ pages }) };
      if (url.startsWith('/api/stickies?page=')) return { ok: true, json: async () => ({ page: 'jobs', notes: [] }) };
      if (url.startsWith('/api/notifications')) return { ok: true, json: async () => ({ entries: [], basedirs: {} }) };
      return { ok: false, json: async () => ({}), text: async () => '' };
    };
  },
});
const fail = message => { console.error(message); process.exit(1); };
setTimeout(() => {
  const document = dom.window.document;
  const link = text => [...document.querySelectorAll('.sidebar a')].find(a => a.textContent.trim().startsWith(text));
  const pin = a => a && a.querySelector('.sticky-pin');
  if (!pin(link('alpha')) || !pin(link('alpha')).classList.contains('sticky-pin-pink')) fail('alpha has no pink dot');
  if (!pin(link('alpha')).title.includes('check the seed 2 result') || !pin(link('alpha')).title.includes('+1 more')) fail('alpha dot title: ' + pin(link('alpha')).title);
  if (!pin(link('beta')) || !pin(link('beta')).classList.contains('sticky-pin-below')) fail('beta has no faint dot for the note below it');
  if (!pin(link('All projects')) || !pin(link('All projects')).classList.contains('sticky-pin-orange')) fail('All projects has no dot');
  if (pin(document.querySelector('.sidebar-brand'))) fail('the logo carries a dot');
  const settings = document.getElementById('settings-panel');
  const stickies = document.getElementById('stickies-panel');
  document.getElementById('settings-button').click();
  if (settings.hidden) fail('Settings does not open beside Stickies');
  document.getElementById('stickies-button').click();
  if (stickies.hidden) fail('Stickies does not open');
  if (!settings.hidden) fail('opening Stickies leaves Settings open');
  document.getElementById('settings-button').click();
  if (settings.hidden) fail('Settings does not open after Stickies');
  if (errors.length) fail(errors.join('\n'));
  process.exit(0);
}, 100);
`
	if output, err := exec.Command("node", "-e", script, htmlPath).CombinedOutput(); err != nil {
		t.Fatalf("Recent jobs page stickies check failed: %v\n%s", err, output)
	}
}
