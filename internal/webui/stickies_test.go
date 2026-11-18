package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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
	for _, unwanted := range []string{`action="/_auth/logout"`, "stickies-menu", "launches-button", "renderStickies"} {
		if strings.Contains(static, unwanted) {
			t.Errorf("static Recent jobs page has %s", unwanted)
		}
	}
	if noAuth := jobsPage(livePage{}, "/", nil, nil, jobsSelection{}, true, false, ""); strings.Contains(noAuth, `action="/_auth/logout"`) {
		t.Error("Sign out is offered without a sign-in")
	}
}
