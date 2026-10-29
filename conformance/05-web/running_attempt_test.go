package web

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestRunPageShowsRunningAttemptID checks /api/run gives a running job the
// attempt ID that `rotari jobs` and the Recent jobs page show, before the run
// has a summary.
func TestRunPageShowsRunningAttemptID(t *testing.T) {
	covers(t, "WEB-24")
	e := support.NewEnv(t)
	run := e.StartRun("live", 1, true)
	lines := strings.Fields(e.MustRotari("jobs", "--basedir", e.Base, "live", "--format", "%a").Stdout)
	attemptID := lines[len(lines)-1]
	if !strings.HasPrefix(attemptID, "att_") {
		t.Fatalf("rotari jobs attempt ID = %q, want one running attempt", attemptID)
	}
	web := e.StartWeb()
	response := e.HTTPGet(web + "/api/run?project_name=live&run_id=" + url.QueryEscape(run.RunID))
	var detail struct {
		Jobs []struct {
			ID        string `json:"id"`
			AttemptID string `json:"attempt_id"`
		} `json:"jobs"`
	}
	if response.Status != 200 || json.Unmarshal([]byte(response.Body), &detail) != nil || len(detail.Jobs) != 1 {
		t.Fatalf("/api/run: status %d: %s", response.Status, response.Body)
	}
	if job := detail.Jobs[0]; job.ID != run.Jobs[0] || job.AttemptID != attemptID {
		t.Errorf("/api/run job = %+v, want %s with attempt ID %s", job, run.Jobs[0], attemptID)
	}
	if recent := e.HTTPGet(web + "/jobs/"); recent.Status != 200 || !strings.Contains(recent.Body, attemptID) {
		t.Errorf("Recent jobs: status %d, want it listing %s:\n%s", recent.Status, attemptID, recent.Body)
	}
}
