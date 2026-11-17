package jobstatus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/state"
)

func TestFilterJobFollowsCarriedOrigin(t *testing.T) {
	runsDir := t.TempDir()
	const executed, carried, later = "20260101-000000-aaaaaaaa", "20260102-000000-bbbbbbbb", "20260103-000000-cccccccc"
	attemptID := state.MakeAttemptID(executed, "job", 0)
	attemptDir := filepath.Join(runsDir, executed, "job", "attempts", attemptID)
	writeFile(t, filepath.Join(attemptDir, "status.json"), `{"phase":"finished","exit_code":3,"hosts":["node1"],"started_at":"2026-01-01T00:00:10Z"}`)
	writeFile(t, filepath.Join(attemptDir, "finished_at"), "2026-01-01T00:00:20Z\n")
	writeFile(t, filepath.Join(attemptDir, "output"), "CUDA out of memory\n")
	// Carried twice: run "carried" from "executed", and run "later" from "carried".
	writeQueue(t, filepath.Join(runsDir, carried), model.JobOrigin{RunID: executed, JobID: "job"})
	writeQueue(t, filepath.Join(runsDir, later), model.JobOrigin{RunID: carried, JobID: "job"})

	result := model.JobResult{ID: "job", ExitCode: 3}
	for name, origin := range map[string]model.JobOrigin{
		"executing run":  {RunID: executed, JobID: "job"},
		"carried once":   {RunID: carried, JobID: "job"},
		"carried twice":  {RunID: later, JobID: "job"},
		"attempt ID":     {RunID: later, JobID: "job", AttemptID: attemptID},
		"other job's ID": {RunID: later, JobID: "job", AttemptID: "att_bad"},
	} {
		t.Run(name, func(t *testing.T) {
			job := FilterJob(testStore(), runsDir, origin, "queued", result, true, time.Time{})
			attributes, err := job.ExecutionAttributes()
			if err != nil {
				t.Fatal(err)
			}
			if job.ID != "queued" || !reflect.DeepEqual(attributes.Hosts, []string{"node1"}) {
				t.Fatalf("job = %+v, attributes = %+v, want ID queued on node1", job, attributes)
			}
			if got := attributes.StartedAt.Format(time.RFC3339); got != "2026-01-01T00:00:10Z" {
				t.Fatalf("started = %s", got)
			}
			if got := attributes.FinishedAt.Format(time.RFC3339); got != "2026-01-01T00:00:20Z" {
				t.Fatalf("finished = %s", got)
			}
			if matched, err := job.Diagnosis([]string{"cuda-gpu-memory-exhausted"}); err != nil || !matched {
				t.Fatalf("diagnosis = %t, %v; it did not read the executing attempt's log", matched, err)
			}
		})
	}
}

func TestFilterJobPrefersResultHostsAndCarriedTimes(t *testing.T) {
	runsDir := t.TempDir()
	origin := model.JobOrigin{RunID: "20260101-000000-aaaaaaaa", JobID: "gone", SubmittedAt: "2026-01-01T00:00:00Z", FinishedAt: "2026-01-01T00:01:00Z"}
	job := FilterJob(testStore(), runsDir, origin, "gone", model.JobResult{ExitCode: 1, Hosts: []string{"summary-host"}}, true, time.Time{})
	attributes, err := job.ExecutionAttributes()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(attributes.Hosts, []string{"summary-host"}) || attributes.StartedAt.IsZero() || attributes.FinishedAt.IsZero() {
		t.Fatalf("attributes = %+v, want summary hosts and the carry's times", attributes)
	}
	if matched, err := job.Diagnosis([]string{"cuda-gpu-memory-exhausted"}); err != nil || matched {
		t.Fatalf("diagnosis = %t, %v; want no match and no error without a log", matched, err)
	}
}

// A fact recorded in a file that cannot be read or decoded is an error
// naming the file; a file that does not exist records nothing.
func TestFilterJobReportsFactsItCannotRead(t *testing.T) {
	const executed, carried = "20260101-000000-aaaaaaaa", "20260102-000000-bbbbbbbb"
	attemptID := state.MakeAttemptID(executed, "job", 0)
	for _, test := range []struct {
		name string
		// damage changes the attempt directory, or runsDir, and returns
		// the path the error must name, or "" for no error.
		damage    func(t *testing.T, runsDir, attemptDir string) string
		diagnosis bool
	}{
		{name: "intact", damage: func(*testing.T, string, string) string { return "" }},
		{name: "missing status.json", damage: func(t *testing.T, _, dir string) string {
			removeFile(t, filepath.Join(dir, "status.json"))
			return ""
		}},
		{name: "damaged status.json", damage: func(t *testing.T, _, dir string) string {
			writeFile(t, filepath.Join(dir, "status.json"), "{")
			return filepath.Join(dir, "status.json")
		}},
		{name: "damaged started_at", damage: func(t *testing.T, _, dir string) string {
			writeFile(t, filepath.Join(dir, "status.json"), `{"phase":"finished","started_at":"soon"}`)
			return filepath.Join(dir, "status.json")
		}},
		{name: "damaged submitted_at", damage: func(t *testing.T, _, dir string) string {
			writeFile(t, filepath.Join(dir, "status.json"), `{"phase":"finished"}`)
			writeFile(t, filepath.Join(dir, "submitted_at"), "garbage")
			return filepath.Join(dir, "submitted_at")
		}},
		{name: "damaged job.json behind a missing submitted_at", damage: func(t *testing.T, _, dir string) string {
			writeFile(t, filepath.Join(dir, "status.json"), `{"phase":"finished"}`)
			writeFile(t, filepath.Join(dir, "job.json"), "{")
			return filepath.Join(dir, "job.json")
		}},
		{name: "unreadable finished_at", damage: func(t *testing.T, _, dir string) string {
			replaceWithDir(t, filepath.Join(dir, "finished_at"))
			return filepath.Join(dir, "finished_at")
		}},
		{name: "missing log", diagnosis: true, damage: func(t *testing.T, _, dir string) string {
			removeFile(t, filepath.Join(dir, "output"))
			return ""
		}},
		{name: "unreadable log", diagnosis: true, damage: func(t *testing.T, _, dir string) string {
			replaceWithDir(t, filepath.Join(dir, "output"))
			return filepath.Join(dir, "output")
		}},
		{name: "damaged carry", damage: func(t *testing.T, runsDir, _ string) string {
			writeFile(t, filepath.Join(runsDir, carried, "commands.json"), "{")
			return "commands.json"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runsDir := t.TempDir()
			attemptDir := filepath.Join(runsDir, executed, "job", "attempts", attemptID)
			writeFile(t, filepath.Join(attemptDir, "status.json"), `{"phase":"finished","exit_code":3,"hosts":["node1"],"started_at":"2026-01-01T00:00:10Z"}`)
			writeFile(t, filepath.Join(attemptDir, "finished_at"), "2026-01-01T00:00:20Z\n")
			writeFile(t, filepath.Join(attemptDir, "output"), "CUDA out of memory\n")
			writeQueue(t, filepath.Join(runsDir, carried), model.JobOrigin{RunID: executed, JobID: "job"})
			want := test.damage(t, runsDir, attemptDir)

			job := FilterJob(testStore(), runsDir, model.JobOrigin{RunID: carried, JobID: "job"}, "job", model.JobResult{ExitCode: 3}, true, time.Time{})
			var err error
			if test.diagnosis {
				_, err = job.Diagnosis([]string{"cuda-gpu-memory-exhausted"})
			} else {
				_, err = job.ExecutionAttributes()
			}
			if want == "" {
				if err != nil {
					t.Fatalf("error = %v, want none", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "job job of run "+carried) {
				t.Fatalf("error = %v, want one naming the job and %s", err, want)
			}
		})
	}
}

// A carry whose recorded time does not parse is an error naming the carry.
func TestFilterJobReportsADamagedCarryTime(t *testing.T) {
	origin := model.JobOrigin{RunID: "20260101-000000-aaaaaaaa", JobID: "gone", SubmittedAt: "yesterday"}
	_, err := FilterJob(testStore(), t.TempDir(), origin, "gone", model.JobResult{ExitCode: 1}, true, time.Time{}).ExecutionAttributes()
	if err == nil || !strings.Contains(err.Error(), "submitted_at") {
		t.Fatalf("error = %v, want one naming the carry's submitted_at", err)
	}
}

// A successful job has no diagnosis, so its log is not read.
func TestFilterJobDoesNotReadASuccessfulJobsLog(t *testing.T) {
	runsDir := t.TempDir()
	const run = "20260101-000000-aaaaaaaa"
	attemptDir := filepath.Join(runsDir, run, "job", "attempts", state.MakeAttemptID(run, "job", 0))
	writeFile(t, filepath.Join(attemptDir, "status.json"), `{"phase":"finished","exit_code":0}`)
	replaceWithDir(t, filepath.Join(attemptDir, "output"))
	matched, err := FilterJob(testStore(), runsDir, model.JobOrigin{RunID: run, JobID: "job"}, "job", model.JobResult{}, true, time.Time{}).Diagnosis([]string{"cuda-gpu-memory-exhausted"})
	if err != nil || matched {
		t.Fatalf("diagnosis = %t, %v; want no match and no read", matched, err)
	}
}

func removeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// replaceWithDir puts a directory where a file is read, which fails to read
// whoever runs the test.
func replaceWithDir(t *testing.T, path string) {
	t.Helper()
	_ = os.Remove(path)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeQueue(t *testing.T, runDir string, origin model.JobOrigin) {
	t.Helper()
	queue := model.Queue{Commands: []model.QueuedCommand{{ID: "job", Command: []string{"true"}, Origin: &origin}}}
	data, err := json.Marshal(queue)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(runDir, "commands.json"), string(data))
}
