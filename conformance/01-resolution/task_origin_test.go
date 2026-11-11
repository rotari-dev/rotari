package resolution

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestArrayTasksInheritTheArrayOrigin checks that a task of an array whose
// run records only the array's origin, as runs may, reads as carried from
// the same task of the source array in show, jobs, and the Web run view, and
// that a carry whose commands.json cannot be read is an error, not a job
// without an origin (RES-30).
func TestArrayTasksInheritTheArrayOrigin(t *testing.T) {
	covers(t, "RES-30")
	e := support.NewEnv(t)
	const project = "arr"
	array := support.AddedJobID(t, e.MustRotari("add", "-p", project, "--array", "1-3", "--", "sh", "-c", `test "$ROTARI_ARRAY_TASK_ID" != 2 || exit 5`))
	e.RotariRun("run", "-p", project, "--quiet")
	first := showRunID(t, e.MustRotari("show", "-p", project, "--json").Stdout)
	e.RotariRun("retry", "-p", project, "--quiet")
	retry := showRunID(t, e.MustRotari("show", "-p", project, "--json").Stdout)
	runsDir := filepath.Join(e.Base, "projects", project, "runs")
	dropTaskOrigins(t, filepath.Join(runsDir, retry, "commands.json"))
	task := array + "-1"

	var job struct {
		Jobs []struct {
			Status  string `json:"status"`
			Carried bool   `json:"carried"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(e.MustRotari("show", "-p", project, "-r", retry, "-j", task, "--json").Stdout), &job); err != nil || len(job.Jobs) != 1 ||
		job.Jobs[0].Status != "success (carried)" || !job.Jobs[0].Carried {
		t.Errorf("show -j %s --json: %+v (%v); want success (carried)", task, job, err)
	}
	if out := e.MustRotari("show", "-p", project, "-r", retry, "-j", task, "--no-pager").Stdout; !strings.Contains(out, "carried forward from run "+first) {
		t.Errorf("show -j %s does not say where the task was carried from:\n%s", task, out)
	}
	if out := e.MustRotari("jobs", "-p", project).Stdout; strings.Count(out, "success (carried)") != 2 {
		t.Errorf("jobs must show tasks 1 and 3 carried:\n%s", out)
	}
	web := e.StartWeb()
	var run struct {
		Jobs []struct {
			ID      string `json:"id"`
			Carried bool   `json:"carried"`
			Origin  *struct {
				RunID string `json:"run_id"`
				JobID string `json:"job_id"`
			} `json:"origin"`
		} `json:"jobs"`
	}
	got := e.HTTPGet(web + "/api/run?" + url.Values{"project_name": {project}, "run_id": {retry}}.Encode())
	if err := json.Unmarshal([]byte(got.Body), &run); got.Status != 200 || err != nil {
		t.Fatalf("/api/run: status %d: %v: %s", got.Status, err, got.Body)
	}
	found := false
	for _, job := range run.Jobs {
		if job.ID == task {
			found = true
			if !job.Carried || job.Origin == nil || job.Origin.RunID != first || job.Origin.JobID != task {
				t.Errorf("/api/run %s: carried %v, origin %+v; want task 1 of run %s", task, job.Carried, job.Origin, first)
			}
		}
	}
	if !found {
		t.Errorf("/api/run has no %s: %s", task, got.Body)
	}

	// When the commands.json that records the carry cannot be read, the log
	// cannot be found, and that is said.
	if err := os.WriteFile(filepath.Join(runsDir, retry, "commands.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := e.Rotari("show", "-p", project, "-r", retry, "-j", task, "--follow", "--stream", "stdout"); got.Code == 0 || !strings.Contains(got.Stderr, "commands.json") {
		t.Errorf("show --follow through an unreadable carry must fail naming the file: %s", got)
	}
	query := url.Values{"project_name": {project}, "run_id": {retry}, "job_id": {task}, "stream": {"stdout"}}
	if got := e.HTTPGet(web + "/api/log?" + query.Encode()); got.Status == 200 || !strings.Contains(got.Body, "commands.json") {
		t.Errorf("/api/log through an unreadable carry: status %d: %s", got.Status, got.Body)
	}
}

// dropTaskOrigins leaves a run's array commands with their array-level
// origin only.
func dropTaskOrigins(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var queue map[string]any
	if err := json.Unmarshal(data, &queue); err != nil {
		t.Fatal(err)
	}
	dropped := false
	for _, command := range queue["commands"].([]any) {
		command := command.(map[string]any)
		if _, ok := command["task_origins"]; ok && command["origin"] != nil {
			delete(command, "task_origins")
			dropped = true
		}
	}
	if !dropped {
		t.Fatalf("no array command with task origins in %s", path)
	}
	if data, err = json.Marshal(queue); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
