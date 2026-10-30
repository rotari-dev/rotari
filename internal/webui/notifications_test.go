package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/notification"
	"github.com/rotari-dev/rotari/internal/notificationlog"
)

func inboxFixture(t *testing.T, allowControl bool) (Options, string, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	current, other := t.TempDir(), t.TempDir()
	log := notificationlog.Open(t.TempDir())
	now := time.Now()
	record := func(baseDir, project, runID string, exitCode int) {
		t.Helper()
		event := notification.NewRunEvent(project, model.RunSummary{RunID: runID, ExitCode: exitCode, Results: []model.JobResult{{ExitCode: exitCode}}}, now)
		if err := log.Append(baseDir, []notification.Event{event}); err != nil {
			t.Fatal(err)
		}
	}
	record(current, "quiet", "run-1", 0)
	record(current, "demo", "run-2", 1)
	record(other, "demo", "run-3", 1)
	// The quiet project's browser settings select no run successes.
	projectDir := filepath.Join(current, "projects", "quiet")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, notification.FileName), []byte("[browser]\nrun_success = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	options := testOptions(current, allowControl)
	options.BaseDirs = []string{current, other}
	options.NotificationLog = &log
	return options, notification.WebBaseDirID(current), notification.WebBaseDirID(other)
}

func getInbox(t *testing.T, handler http.Handler, basedirs string) (int, inboxResponse, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/notifications?basedirs="+basedirs, nil))
	var response inboxResponse
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
	}
	return recorder.Code, response, recorder.Body.String()
}

// The inbox lists the selected basedirs' records, newest first, rendered with
// each project's current browser settings and linked to the run under its
// own basedir.
func TestNotificationsListSelectedBasedirsWithProjectSettings(t *testing.T) {
	options, current, other := inboxFixture(t, true)
	handler := Handler(options)
	code, inbox, body := getInbox(t, handler, current)
	if code != http.StatusOK || len(inbox.Entries) != 1 {
		t.Fatalf("GET = %d %s", code, body)
	}
	entry := inbox.Entries[0]
	if entry.Title != "rotari: run failed" || entry.URL != "/_basedir/"+current+"/project/demo/run/run-2" || entry.Body == "" {
		t.Fatalf("entry = %#v", entry)
	}
	if _, both, _ := getInbox(t, handler, current+","+other); len(both.Entries) != 2 || both.Entries[0].URL != "/_basedir/"+other+"/project/demo/run/run-3" {
		t.Fatalf("both basedirs = %#v", both.Entries)
	}
	if code, _, body := getInbox(t, handler, "unknown"); code != http.StatusBadRequest {
		t.Fatalf("unknown basedir = %d %s", code, body)
	}
	if code, none, _ := getInbox(t, handler, ""); code != http.StatusOK || len(none.Entries) != 0 {
		t.Fatalf("no basedir = %d %#v", code, none)
	}
}

// Clearing hides records up to the cleared cursor for every page, and the
// cursors are written only by a server with control enabled.
func TestNotificationStateIsSharedAndNeedsControl(t *testing.T) {
	options, current, other := inboxFixture(t, true)
	handler := Handler(options)
	_, inbox, _ := getInbox(t, handler, current+","+other)
	newest := inbox.Entries[0].Cursor
	post := func(handler http.Handler, state notificationlog.InboxState) *httptest.ResponseRecorder {
		body, _ := json.Marshal(state)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/notifications/state", bytes.NewReader(body)))
		return recorder
	}
	readOnly := options
	readOnly.AllowControl = false
	if recorder := post(Handler(readOnly), notificationlog.InboxState{Read: newest}); recorder.Code != http.StatusForbidden {
		t.Fatalf("read-only POST = %d %s", recorder.Code, recorder.Body.String())
	}
	if recorder := post(handler, notificationlog.InboxState{Read: newest, Cleared: inbox.Entries[1].Cursor}); recorder.Code != http.StatusOK {
		t.Fatalf("POST = %d %s", recorder.Code, recorder.Body.String())
	}
	_, after, _ := getInbox(t, Handler(readOnly), current+","+other)
	if len(after.Entries) != 1 || after.Entries[0].Cursor != newest || after.Read != newest {
		t.Fatalf("after clearing = %#v", after)
	}
	if recorder := post(handler, notificationlog.InboxState{Read: notificationlog.Cursor{CreatedAt: "now", ID: "x"}}); recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid cursor = %d", recorder.Code)
	}
}
