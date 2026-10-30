package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

type inboxCursor struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

type inboxResponse struct {
	Entries []struct {
		inboxCursor
		Title string `json:"title"`
		Body  string `json:"body"`
		URL   string `json:"url"`
	} `json:"entries"`
	Read    inboxCursor `json:"read"`
	Cleared inboxCursor `json:"cleared"`
}

func currentBasedirID(t *testing.T, e *support.Env, base string) string {
	t.Helper()
	var listed struct {
		Basedirs []struct {
			ID      string `json:"id"`
			Current bool   `json:"current"`
		} `json:"basedirs"`
	}
	if err := json.Unmarshal([]byte(e.HTTPGet(base+"/api/basedirs").Body), &listed); err != nil {
		t.Fatal(err)
	}
	for _, entry := range listed.Basedirs {
		if entry.Current {
			return entry.ID
		}
	}
	t.Fatal("no current basedir listed")
	return ""
}

func getInbox(t *testing.T, e *support.Env, base, basedirID string) inboxResponse {
	t.Helper()
	response := e.HTTPGet(base + "/api/notifications?basedirs=" + basedirID)
	var inbox inboxResponse
	if response.Status != 200 {
		t.Fatalf("GET /api/notifications = %d: %s", response.Status, response.Body)
	}
	if err := json.Unmarshal([]byte(response.Body), &inbox); err != nil {
		t.Fatal(err)
	}
	return inbox
}

// A run records its notifications whether or not a Web page is open, so the
// inbox of a server started afterwards shows them. The read and cleared
// cursors outlive the server, only a server with control may move them, and
// records expire after 10 days.
func TestWebInboxShowsNotificationsRecordedWhileClosed(t *testing.T) {
	covers(t, "NOT-2", "WEB-21")
	e := support.NewEnv(t)
	run := e.CreateFinishedRun()
	base := e.StartWeb()
	id := currentBasedirID(t, e, base)
	inbox := getInbox(t, e, base, id)
	if len(inbox.Entries) != 1 {
		t.Fatalf("entries = %+v", inbox.Entries)
	}
	entry := inbox.Entries[0]
	if entry.Title != "rotari: run failed (1 job failed)" || !strings.Contains(entry.Body, "job name: bad") ||
		entry.URL != "/_basedir/"+id+"/project/"+run.Project+"/run/"+run.RunID {
		t.Fatalf("entry = %+v", entry)
	}
	if inbox.Read != (inboxCursor{}) {
		t.Fatalf("history is already read: %+v", inbox.Read)
	}

	readOnly := e.StartWeb("--allow-control=false")
	if response := e.HTTPPostJSON(readOnly+"/api/notifications/state", map[string]any{"read": entry.inboxCursor, "cleared": inboxCursor{}}); response.Status != 403 {
		t.Fatalf("read-only POST = %d: %s", response.Status, response.Body)
	}
	if response := e.HTTPPostJSON(base+"/api/notifications/state", map[string]any{"read": entry.inboxCursor, "cleared": inboxCursor{}}); response.Status != 200 {
		t.Fatalf("POST = %d: %s", response.Status, response.Body)
	}
	if restarted := getInbox(t, e, e.StartWeb(), id); restarted.Read != entry.inboxCursor || len(restarted.Entries) != 1 {
		t.Fatalf("read state after a restart = %+v", restarted)
	}

	old := time.Now().Add(-11 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(e.Master, "notifications", run.RunID+".jsonl"), old, old); err != nil {
		t.Fatal(err)
	}
	if expired := getInbox(t, e, base, id); len(expired.Entries) != 0 {
		t.Fatalf("an 11-day-old record is listed: %+v", expired.Entries)
	}
}

// Both live pages show recorded history in the inbox, unread and without the
// desktop API, and opening the inbox marks it read on the server.
func TestWebNotificationInboxWithoutDesktopPermission(t *testing.T) {
	covers(t, "WEB-21")
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	script := `
const {JSDOM, VirtualConsole} = require('jsdom');
const fs = require('fs');
const base = process.argv[2];
const errors = [];
const console = new VirtualConsole();
console.on('jsdomError', error => errors.push(String(error)));
const dom = new JSDOM(fs.readFileSync(process.argv[1], 'utf8'), {
  url: 'http://example.test/', runScripts: 'dangerously', virtualConsole: console,
  beforeParse(w) {
    w.Notification = undefined;
    w.isSecureContext = false;
    w.setInterval = () => 1;
    w.fetch = (url, init) => String(url).startsWith('/api/notifications')
      ? fetch(base + url, init)
      : Promise.reject(Error('offline'));
  },
});
const settle = () => new Promise(resolve => setTimeout(resolve, 200));
(async () => {
  try {
    await settle();
    const w = dom.window, d = w.document;
    const button = d.querySelector('.notification-inbox > button');
    if (!button || button.textContent !== 'Inbox (1)') throw Error('recorded history is not unread: ' + button?.textContent);
    button.click();
    await settle();
    const panel = d.getElementById('notification-inbox-panel');
    if (panel.hidden || button.textContent !== 'Inbox (0)' || !panel.querySelector('a'))
      throw Error('inbox not readable');
    const shared = await (await fetch(base + '/api/notifications?basedirs=' + process.argv[3])).json();
    if (!shared.read.id) throw Error('opening the inbox did not mark it read on the server');
    if (errors.length) throw Error(errors.join('\n'));
    w.close();
  } catch (error) { process.stderr.write(String(error)); process.exit(1); }
})();
`
	for _, route := range []string{"/", "/jobs/"} {
		t.Run(route, func(t *testing.T) {
			e := support.NewEnv(t)
			e.CreateFinishedRun()
			base := e.StartWeb()
			response := e.HTTPGet(base + route)
			if response.Status != 200 {
				t.Fatalf("page status = %d: %s", response.Status, response.Body)
			}
			path := filepath.Join(t.TempDir(), "index.html")
			if err := os.WriteFile(path, []byte(response.Body), 0o600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("node", "-e", script, path, base, currentBasedirID(t, e, base)).CombinedOutput(); err != nil {
				t.Fatalf("inbox: %v\n%s", err, output)
			}
		})
	}
}
