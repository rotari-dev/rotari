package web

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestRetriedJobsShowTheSameAttemptEverywhere checks show, /api/run, and
// `rotari jobs` agree on a retry's attempts: a running job shows its new
// attempt, a carried job the attempt that produced its result, and a job the
// retry has not started none, although it has an earlier attempt.
func TestRetriedJobsShowTheSameAttemptEverywhere(t *testing.T) {
	covers(t, "WEB-24")
	e := support.NewEnv(t)
	project := "retried"
	marker := filepath.Join(e.Root, "hold")
	carried := support.AddedJobID(t, e.MustRotari("add", "-p", project, "--job-name", "carried", "--", "true"))
	running := support.AddedJobID(t, e.MustRotari("add", "-p", project, "--job-name", "running", "--", "sh", "-c", `test -e "$0" && sleep 300; exit 1`, marker))
	waiting := support.AddedJobID(t, e.MustRotari("add", "-p", project, "--job-name", "waiting", "--depends-on-finished", "running", "--", "false"))
	if result := e.Rotari("run", "-p", project, "--quiet"); result.Code == 0 {
		t.Fatalf("first run succeeded: %s", result)
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	e.MustRotari("retry", "-p", project, "--quiet", "--async")
	t.Cleanup(func() { _ = e.Rotari("cancel", "-p", project, "--wait") })

	jobsAttempts := map[string]string{}
	support.WaitUntil(t, 30*time.Second, func() (bool, string) {
		output := e.MustRotari("jobs", "--basedir", e.Base, project, "--since", "0", "--format", "%n %a %s").Stdout
		for _, line := range strings.Split(output, "\n") {
			if fields := strings.Fields(line); len(fields) >= 3 {
				jobsAttempts[fields[0]] = fields[1]
			}
		}
		return strings.Contains(output, "running (recorded)"), output
	})
	var shown struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(e.MustRotari("show", "-p", project, "--json").Stdout), &shown); err != nil || shown.RunID == "" {
		t.Fatalf("show --json did not describe the retry: %v", err)
	}
	showAttempts := map[string]string{}
	for _, line := range strings.Split(e.MustRotari("show", "-p", project, "--wide", "--no-pager").Stdout, "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 {
			showAttempts[fields[0]] = fields[1]
		}
	}
	response := e.HTTPGet(e.StartWeb() + "/api/run?project_name=" + project + "&run_id=" + url.QueryEscape(shown.RunID))
	var detail struct {
		Jobs []struct {
			ID        string `json:"id"`
			AttemptID string `json:"attempt_id"`
		} `json:"jobs"`
	}
	if response.Status != 200 || json.Unmarshal([]byte(response.Body), &detail) != nil {
		t.Fatalf("/api/run: status %d: %s", response.Status, response.Body)
	}
	apiAttempts := map[string]string{}
	for _, job := range detail.Jobs {
		apiAttempts[job.ID] = job.AttemptID
	}
	if !strings.HasPrefix(apiAttempts[carried], "att_") || !strings.HasPrefix(apiAttempts[running], "att_") || apiAttempts[waiting] != "" {
		t.Errorf("/api/run attempts = %v, want carried %s and running %s with one, waiting %s without", apiAttempts, carried, running, waiting)
	}
	for name, jobID := range map[string]string{"carried": carried, "running": running, "waiting": waiting} {
		want := apiAttempts[jobID]
		if want == "" {
			want = "-"
		}
		if got := showAttempts[jobID]; got != want {
			t.Errorf("show attempt of %s = %q, want %q as /api/run", name, got, want)
		}
		// `rotari jobs --since 0` lists only unfinished jobs.
		if got := jobsAttempts[name]; name != "carried" && got != want {
			t.Errorf("rotari jobs attempt of %s = %q, want %q as /api/run", name, got, want)
		}
	}
}
