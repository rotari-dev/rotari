package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

type logLinkAnswer struct {
	Text   string `json:"text"`
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Paths in a job's log that name a file are links, read against the working directory of the
// attempt whose log it is: a carried job's, an array task's, each attempt's. Without the
// terminal, only paths under the roots its artifacts are served from; with it, any.
func TestWebJobLogLinks(t *testing.T) {
	covers(t, "WEB-65")
	e := terminalEnv(t)
	outside := filepath.Join(t.TempDir(), "lib.py")
	if err := os.WriteFile(outside, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// One run, then a retry that carries the first job and the array's good task, and runs
	// the failed ones again.
	dirs := map[string]string{}
	for _, name := range []string{"ok", "bad", "array"} {
		dirs[name] = filepath.Join(e.Root, "in "+name)
		if err := os.MkdirAll(dirs[name], 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dirs[name], "train.py"), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ok := support.AddedJobID(t, e.MustRotari("add", "-p", "logs", "--working-directory", dirs["ok"], "--", "echo", "train.py:1"))
	bad := support.AddedJobID(t, e.MustRotari("add", "-p", "logs", "--working-directory", dirs["bad"], "--", "false"))
	array := support.AddedJobID(t, e.MustRotari("add", "-p", "logs", "--working-directory", dirs["array"], "--array", "1-2", "--", "sh", "-c", `test "$ROTARI_ARRAY_TASK_ID" = 1`))
	runID := func() string {
		t.Helper()
		var shown struct {
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal([]byte(e.MustRotari("show", "-p", "logs", "--json").Stdout), &shown); err != nil || shown.RunID == "" {
			t.Fatalf("show --json: %v", err)
		}
		return shown.RunID
	}
	e.RotariRun("run", "-p", "logs", "--quiet")
	first := runID()
	_ = e.Rotari("retry", "-p", "logs", "--quiet")
	second := runID()
	if first == second {
		t.Fatal("retry made no run")
	}
	ask := func(base string, headers map[string]string, run, job string, candidates ...string) []logLinkAnswer {
		t.Helper()
		body, err := json.Marshal(map[string]any{"project_name": "logs", "run_id": run, "job_id": job, "attempt_id": "", "candidates": candidates})
		if err != nil {
			t.Fatal(err)
		}
		got := e.HTTPDo(http.MethodPost, base+"/api/log-links", headers, string(body))
		var answer struct {
			Links []logLinkAnswer `json:"links"`
		}
		if err := json.Unmarshal([]byte(got.Body), &answer); got.Status != http.StatusOK || err != nil {
			t.Fatalf("log-links of %s in %s = %d %s", job, run, got.Status, got.Body)
		}
		return answer.Links
	}
	headers := map[string]string{"X-Rotari-Token": launchToken}
	plain, terminal := e.StartWeb("--allow-control=false"), e.StartWeb("--allow-terminal")
	for _, test := range []struct{ name, run, job, dir string }{
		{"a job", first, ok, dirs["ok"]}, {"the same, carried", second, ok, dirs["ok"]},
		{"a failed job", first, bad, dirs["bad"]}, {"the same, run again", second, bad, dirs["bad"]},
		{"an array task", first, array + "-1", dirs["array"]}, {"the same, carried", second, array + "-1", dirs["array"]},
		{"a failed task, run again", second, array + "-2", dirs["array"]},
	} {
		train := filepath.Join(test.dir, "train.py")
		want := []logLinkAnswer{{Text: "train.py:12:3", Path: train, Line: 12, Column: 3}, {Text: train, Path: train}}
		candidates := []string{"train.py:12:3", "train.py", train, outside + ":5", "gone.py:1", "./", "12:30"}
		if got := ask(plain, headers, test.run, test.job, candidates...); !reflect.DeepEqual(got, want) {
			t.Errorf("%s, read-only:\n got %+v\nwant %+v", test.name, got, want)
		}
		want = append(want, logLinkAnswer{Text: outside + ":5", Path: outside, Line: 5})
		if got := ask(terminal, headers, test.run, test.job, candidates...); !reflect.DeepEqual(got, want) {
			t.Errorf("%s, with the terminal:\n got %+v\nwant %+v", test.name, got, want)
		}
	}
	for name, body := range map[string]string{
		"an unknown job":   `{"project_name":"logs","run_id":"` + first + `","job_id":"nope","candidates":[]}`,
		"a path as a run":  `{"project_name":"logs","run_id":"../x","job_id":"` + ok + `","candidates":[]}`,
		"an unknown field": `{"project_name":"logs","run_id":"` + first + `","job_id":"` + ok + `","dir":"/","candidates":[]}`,
	} {
		if got := e.HTTPDo(http.MethodPost, plain+"/api/log-links", headers, body); got.Status != http.StatusBadRequest {
			t.Errorf("%s = %d %s", name, got.Status, got.Body)
		}
	}
}
