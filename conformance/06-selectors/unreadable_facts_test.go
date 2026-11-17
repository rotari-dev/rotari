package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract SEL-11: a fact a filter condition needs that is recorded but
// cannot be read fails the command, naming the file; a fact that was never
// recorded does not match; a job the other conditions already excluded is
// not read. See contracts/06-selectors.md, "Filters".

// unreadableFactsFixture is project p of jobs ok, which succeeds, and bad,
// whose output names a diagnosis; run first runs both, and run second reruns
// bad, so ok is carried into it from first.
type unreadableFactsFixture struct {
	e             *env
	first, second string
	ok, bad       string
}

func newUnreadableFactsFixture(t *testing.T) unreadableFactsFixture {
	t.Helper()
	e := newEnv(t)
	f := unreadableFactsFixture{e: e}
	f.ok = addedJobID(t, e.mustRotari("add", "-p", "p", "--job-name", "ok", "--", "true"))
	f.bad = addedJobID(t, e.mustRotari("add", "-p", "p", "--job-name", "bad", "--", "sh", "-c", "echo cannot connect to the docker daemon; exit 1"))
	e.rotari("run", "-p", "p", "--run-name", "first", "--quiet")
	f.first = f.lastRunID()
	e.rotari("run", "-p", "p", "--run-name", "second", "--failed", "--quiet")
	f.second = f.lastRunID()
	if f.first == "" || f.second == "" || f.first == f.second {
		t.Fatalf("runs first %q and second %q did not both start", f.first, f.second)
	}
	return f
}

func (f unreadableFactsFixture) lastRunID() string {
	f.e.t.Helper()
	var meta struct {
		LastRunID string `json:"last_run_id"`
	}
	data, err := os.ReadFile(filepath.Join(f.e.base, "projects", "p", "meta.json"))
	if err != nil {
		f.e.t.Fatal(err)
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		f.e.t.Fatal(err)
	}
	return meta.LastRunID
}

// attemptFile returns the path of name in job's only attempt in run.
func (f unreadableFactsFixture) attemptFile(run, job, name string) string {
	f.e.t.Helper()
	dirs, err := filepath.Glob(filepath.Join(f.e.base, "projects", "p", "runs", run, job, "attempts", "*"))
	if err != nil || len(dirs) != 1 {
		f.e.t.Fatalf("attempt of %s in %s: %q, %v", job, run, dirs, err)
	}
	return filepath.Join(dirs[0], name)
}

func (f unreadableFactsFixture) write(path, content string) {
	f.e.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.e.t.Fatal(err)
	}
}

// unreadable puts a directory where a file is read, which fails to read
// whoever runs the test.
func (f unreadableFactsFixture) unreadable(path string) {
	f.e.t.Helper()
	if err := os.Remove(path); err != nil {
		f.e.t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		f.e.t.Fatal(err)
	}
}

func TestFiltersFailOnFactsTheyCannotRead(t *testing.T) {
	covers(t, "SEL-11")
	for _, tc := range []struct {
		name string
		// damage changes the fixture's files; it returns the path the
		// error must name, or "" when the command must succeed.
		damage func(f unreadableFactsFixture) string
		args   func(f unreadableFactsFixture) []string
		// jobs, when the command must succeed, are names show lists.
		jobs []string
	}{
		{
			name: "show of a carried job's damaged start time",
			damage: func(f unreadableFactsFixture) string {
				path := f.attemptFile(f.first, f.ok, "submitted_at")
				f.write(path, "garbage")
				return path
			},
			args: func(f unreadableFactsFixture) []string {
				return []string{"show", "-p", "p", "--filter-started-after", "2000-01-01"}
			},
		},
		{
			name: "show of a damaged status.json",
			damage: func(f unreadableFactsFixture) string {
				path := f.attemptFile(f.second, f.bad, "status.json")
				f.write(path, "{")
				return path
			},
			args: func(f unreadableFactsFixture) []string {
				return []string{"show", "-p", "p", "--filter-host", "*"}
			},
		},
		{
			name: "show of an unreadable log",
			damage: func(f unreadableFactsFixture) string {
				path := f.attemptFile(f.second, f.bad, "output")
				f.unreadable(path)
				return path
			},
			args: func(f unreadableFactsFixture) []string {
				return []string{"show", "-p", "p", "--filter-diagnosis", "docker-daemon-unavailable"}
			},
		},
		{
			name: "copy of a damaged start time",
			damage: func(f unreadableFactsFixture) string {
				path := f.attemptFile(f.first, f.ok, "submitted_at")
				f.write(path, "garbage")
				return path
			},
			args: func(f unreadableFactsFixture) []string {
				return []string{"copy", "-p", "p", "--run-id", f.first, "--filter-started-after", "2000-01-01", "--quiet"}
			},
		},
		{
			name: "run of a carried job's damaged start time",
			damage: func(f unreadableFactsFixture) string {
				path := f.attemptFile(f.first, f.ok, "submitted_at")
				f.write(path, "garbage")
				return path
			},
			args: func(f unreadableFactsFixture) []string {
				return []string{"run", "-p", "p", "--filter-started-after", "2000-01-01", "--quiet"}
			},
		},
		{
			name: "a job the result filter excludes is not read",
			damage: func(f unreadableFactsFixture) string {
				f.write(f.attemptFile(f.first, f.ok, "submitted_at"), "garbage")
				return ""
			},
			args: func(f unreadableFactsFixture) []string {
				return []string{"show", "-p", "p", "--failed", "--filter-started-after", "2000-01-01"}
			},
			jobs: []string{"bad"},
		},
		{
			name: "a missing log does not match",
			damage: func(f unreadableFactsFixture) string {
				if err := os.Remove(f.attemptFile(f.second, f.bad, "output")); err != nil {
					f.e.t.Fatal(err)
				}
				return ""
			},
			args: func(f unreadableFactsFixture) []string {
				return []string{"show", "-p", "p", "--filter-diagnosis", "docker-daemon-unavailable"}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newUnreadableFactsFixture(t)
			path := tc.damage(f)
			r := f.e.rotari(tc.args(f)...)
			if path == "" {
				if r.code != 0 {
					t.Fatalf("want success: %s", r)
				}
				for _, job := range tc.jobs {
					if !strings.Contains(r.stdout, " "+job+" ") {
						t.Fatalf("want job %s listed: %s", job, r)
					}
				}
				return
			}
			if r.code == 0 {
				t.Fatalf("want a failure naming %s: %s", path, r)
			}
			if !strings.Contains(r.stdout+r.stderr, path) {
				t.Fatalf("error does not name %s: %s", path, r)
			}
			if tc.args(f)[0] == "run" && f.lastRunID() != f.second {
				t.Fatalf("a refused run recorded run %s", f.lastRunID())
			}
		})
	}
}
