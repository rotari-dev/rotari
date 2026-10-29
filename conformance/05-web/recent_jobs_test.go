package web

import (
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestRecentJobsSelectsFinishedJobsByRange checks the Recent jobs page lists
// jobs that finished inside a dragged range or a preset window, leaves them
// out of a range before they finished, and refuses the since parameter it no
// longer takes.
func TestRecentJobsSelectsFinishedJobsByRange(t *testing.T) {
	covers(t, "WEB-23")
	e := support.NewEnv(t)
	run := e.CreateFinishedRun()
	web := e.StartWeb()
	now := time.Now().UTC()
	at := func(offset time.Duration) string { return now.Add(offset).Format(time.RFC3339) }

	inside := web + "/jobs/?from=" + at(-time.Hour) + "&to=" + at(time.Hour)
	if got := e.HTTPGet(inside); got.Status != 200 || !strings.Contains(got.Body, run.BadAttempt) {
		t.Fatalf("GET %s: status %d, want 200 listing %s:\n%s", inside, got.Status, run.BadAttempt, got.Body)
	} else if !strings.Contains(got.Body, "jobs finished from ") {
		t.Fatalf("GET %s does not describe the range:\n%s", inside, got.Body)
	}
	// A range long before the bar's first view, with the bar paged back.
	before := web + "/jobs/?from=" + at(-60*24*time.Hour) + "&to=" + at(-59*24*time.Hour) + "&view=30d&end=" + at(-30*24*time.Hour)
	if got := e.HTTPGet(before); got.Status != 200 || strings.Contains(got.Body, run.BadAttempt) {
		t.Fatalf("GET %s: status %d, want 200 without %s", before, got.Status, run.BadAttempt)
	}
	// With a run in the last 24 hours, the default is those 24 hours.
	if got := e.HTTPGet(web + "/jobs/"); got.Status != 200 || !strings.Contains(got.Body, run.BadAttempt) || !strings.Contains(got.Body, "jobs finished in the last 24 hours") {
		t.Fatalf("GET /jobs/: status %d, want 200 listing %s in the last 24 hours:\n%s", got.Status, run.BadAttempt, got.Body)
	}
	recent := web + "/jobs/?window=1h"
	if got := e.HTTPGet(recent); got.Status != 200 || !strings.Contains(got.Body, run.BadAttempt) || !strings.Contains(got.Body, "jobs finished in the last hour") {
		t.Fatalf("GET %s: status %d, want 200 listing %s in the last hour:\n%s", recent, got.Status, run.BadAttempt, got.Body)
	}
	for _, path := range []string{
		"/jobs/?since=7d",
		"/jobs/?window=2h",
		"/jobs/?window=6h&from=" + at(-time.Hour) + "&to=" + at(0),
		"/jobs/?from=" + at(-time.Hour),
		"/jobs/?from=" + at(time.Hour) + "&to=" + at(2*time.Hour),
		"/jobs/?from=" + at(0) + "&to=" + at(-time.Hour),
		"/jobs/?from=" + at(-31*24*time.Hour) + "&to=" + at(0),
		"/jobs/?view=14d",
		"/jobs/?end=" + at(time.Hour),
	} {
		if got := e.HTTPGet(web + path); got.Status != 400 {
			t.Errorf("GET %s: status %d, want 400", path, got.Status)
		}
	}
}
