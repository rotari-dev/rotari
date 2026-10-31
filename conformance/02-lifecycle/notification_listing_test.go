package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

type listedNotification struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
	BaseDir   string `json:"basedir"`
	Project   string `json:"project"`
	RunID     string `json:"run_id"`
	Title     string `json:"title"`
	Unread    bool   `json:"unread"`
}

func listNotifications(t *testing.T, e *support.Env, args ...string) []listedNotification {
	t.Helper()
	var listed []listedNotification
	result := e.MustRotari(append([]string{"notifications", "--json"}, args...)...)
	if err := json.Unmarshal([]byte(result.Stdout), &listed); err != nil {
		t.Fatalf("notifications --json: %v\n%s", err, result.Stdout)
	}
	return listed
}

func notificationTitles(listed []listedNotification) []string {
	titles := make([]string, 0, len(listed))
	for _, item := range listed {
		titles = append(titles, item.Project+": "+item.Title)
	}
	return titles
}

// `rotari notifications` lists what the Web inbox shows, oldest first, across
// the known state directories or the selected basedir and project. It reads
// the inbox's shared read cursor for --unread and never moves it.
func TestNotificationsListRecordedNotificationsWithoutReadingThem(t *testing.T) {
	covers(t, "NOT-3")
	e := support.NewEnv(t)
	e.CreateFinishedRun()
	e.MustRotari("add", "-p", "p2", "--", "true")
	e.MustRotari("run", "-p", "p2", "--quiet")
	other := filepath.Join(e.Root, "other")
	e.MustRotari("add", "-b", other, "-p", "p3", "--", "true")
	e.MustRotari("run", "-b", other, "-p", "p3", "--quiet")

	all := listNotifications(t, e)
	want := []string{"p1: rotari: run failed (1 job failed)", "p2: rotari: run succeeded", "p3: rotari: run succeeded"}
	if got := notificationTitles(all); !reflect.DeepEqual(got, want) {
		t.Fatalf("notifications = %v, want %v", got, want)
	}
	for _, item := range all {
		if !item.Unread || item.RunID == "" || !filepath.IsAbs(item.BaseDir) {
			t.Fatalf("notification = %+v", item)
		}
	}
	for name, test := range map[string]struct {
		args []string
		want []string
	}{
		"positional project": {[]string{"p1"}, want[:1]},
		"project option":     {[]string{"-p", "p2"}, want[1:2]},
		"basedir":            {[]string{"--basedir", other}, want[2:]},
	} {
		if got := notificationTitles(listNotifications(t, e, test.args...)); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s: notifications = %v, want %v", name, got, test.want)
		}
	}

	// The Web inbox reads the first two; the listing then marks only the
	// last one unread, and --unread lists only it.
	base := e.StartWeb()
	read := map[string]string{"created_at": all[1].CreatedAt, "id": all[1].ID}
	if response := e.HTTPPostJSON(base+"/api/notifications/state", map[string]any{"read": read, "cleared": map[string]string{"created_at": "", "id": ""}}); response.Status != 200 {
		t.Fatalf("POST = %d: %s", response.Status, response.Body)
	}
	statePath := filepath.Join(e.Master, "notifications", "inbox-state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := notificationTitles(listNotifications(t, e, "--unread")); !reflect.DeepEqual(got, want[2:]) {
		t.Fatalf("--unread = %v, want %v", got, want[2:])
	}
	text := e.MustRotari("notifications").Stdout
	if strings.Count(text, "(unread)") != 1 || !strings.Contains(text, "rotari: run failed (1 job failed)") || !strings.Contains(text, "job name: bad") {
		t.Fatalf("text listing:\n%s", text)
	}
	if after, err := os.ReadFile(statePath); err != nil || string(after) != string(before) {
		t.Fatalf("listing changed the inbox state: %s, %v", after, err)
	}

	for name, args := range map[string][]string{
		"bad window":      {"notifications", "--since", "1x"},
		"project twice":   {"notifications", "p1", "-p", "p2"},
		"invalid project": {"notifications", "a/b"},
		"backslash":       {"notifications", `a\b`},
		"extra argument":  {"notifications", "p1", "p2"},
	} {
		if result := e.Rotari(args...); result.Code == 0 {
			t.Fatalf("%s: accepted: %s", name, result)
		}
	}
}
