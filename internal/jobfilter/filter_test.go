package jobfilter

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
)

func TestFilterExcludesNegatedStagesAndMatrices(t *testing.T) {
	filter := Filter{NotStages: []string{"prepare", "report"}, NotMatrices: []string{"sweep"}}
	for _, test := range []struct {
		name    string
		command model.QueuedCommand
		want    bool
	}{
		{name: "excluded stage", command: model.QueuedCommand{Stage: "prepare"}, want: false},
		{name: "second excluded stage", command: model.QueuedCommand{Stage: "report"}, want: false},
		{name: "other stage", command: model.QueuedCommand{Stage: "train"}, want: true},
		{name: "no stage", command: model.QueuedCommand{}, want: true},
		{name: "excluded matrix", command: model.QueuedCommand{Matrix: &model.MatrixSpec{BaseName: "sweep"}}, want: false},
		{name: "other matrix", command: model.QueuedCommand{Matrix: &model.MatrixSpec{BaseName: "grid"}}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := filter.MatchesCommand(test.command); got != test.want {
				t.Fatalf("MatchesCommand(%+v) = %t, want %t", test.command, got, test.want)
			}
		})
	}
}

func TestFilterMatchesCommandPattern(t *testing.T) {
	filter := Filter{Command: `python .*\.py`}
	if !filter.MatchesCommand(model.QueuedCommand{Command: []string{"python", "train.py"}}) {
		t.Fatal("filter command pattern did not match python job")
	}
	if filter.MatchesCommand(model.QueuedCommand{Command: []string{"bash", "train.sh"}}) {
		t.Fatal("filter command pattern matched a different command")
	}
}

func TestFilterMatchesExitCodesAndFailureKinds(t *testing.T) {
	filter := Filter{ExitCodes: []int{124}, FailureKinds: []string{"timeout"}}
	if !filter.matchesResult(model.JobResult{ExitCode: 124, Error: "timed out after 1s"}, true) {
		t.Fatal("exit code filter did not match a selected result")
	}
	if filter.matchesResult(model.JobResult{ExitCode: 3, Error: "timed out after 1s"}, true) {
		t.Fatal("exit code filter matched an excluded result")
	}
	if !filter.matchesResult(model.JobResult{ExitCode: 124, Error: "timed out after 1s"}, true) {
		t.Fatal("failure kind filter did not match a timeout result")
	}
	if filter.matchesResult(model.JobResult{ExitCode: 0}, true) {
		t.Fatal("failure kind filter matched a success result")
	}
	if filter.matchesResult(model.JobResult{ExitCode: 1}, false) {
		t.Fatal("unfinished result matched a result filter")
	}
}

func TestSelectsCombinesSelectionAndEveryJobCondition(t *testing.T) {
	failed := Job{ID: "a", Result: model.JobResult{ExitCode: 3}, Finished: true, Attributes: Attributes{Hosts: []string{"worker-01"}}}
	matchDiagnosis := func([]string) (bool, error) { return true, nil }
	for _, test := range []struct {
		name      string
		filter    Filter
		selection string
		job       Job
		want      bool
	}{
		{name: "no conditions", selection: "", job: Job{}, want: true},
		{name: "all", selection: "all", job: Job{}, want: true},
		{name: "selection matches", selection: "failed", job: failed, want: true},
		{name: "selection excludes", selection: "success", job: failed, want: false},
		{name: "exit code with selection", filter: Filter{ExitCodes: []int{1}}, selection: "failed", job: failed, want: false},
		{name: "exit code without selection", filter: Filter{ExitCodes: []int{3}}, job: failed, want: true},
		{name: "unfinished never matches exit code", filter: Filter{ExitCodes: []int{0}}, job: Job{}, want: false},
		{name: "definition", filter: Filter{Changed: true, ChangedIDs: map[string]bool{"b": true}}, job: failed, want: false},
		{name: "host", filter: Filter{Hosts: []string{"other"}}, selection: "failed", job: failed, want: false},
		{name: "diagnosis without provider", filter: Filter{Diagnoses: []string{"oom"}}, job: failed, want: false},
		{name: "diagnosis", filter: Filter{Diagnoses: []string{"oom"}}, job: Job{ID: "a", Result: failed.Result, Finished: true, Diagnosis: matchDiagnosis}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := test.filter.Selects(test.selection, test.job); err != nil || got != test.want {
				t.Fatalf("Selects(%q, %+v) = %t, %v, want %t", test.selection, test.job, got, err, test.want)
			}
		})
	}
}

func TestSelectsArrayUsesAggregateSelectionAndAnyTaskCondition(t *testing.T) {
	tasks := []Job{
		{ID: "a-1", Result: model.JobResult{ExitCode: 1}, Finished: true},
		{ID: "a-2", Result: model.JobResult{ExitCode: 2}, Finished: true},
		{ID: "a-3", Finished: true},
	}
	selects := func(filter Filter, selection string, tasks []Job) bool {
		t.Helper()
		selected, err := filter.SelectsArray(selection, tasks)
		if err != nil {
			t.Fatal(err)
		}
		return selected
	}
	if !selects(Filter{ExitCodes: []int{2}}, "failed", tasks) {
		t.Fatal("exit code of a later failed task did not select the array")
	}
	if selects(Filter{ExitCodes: []int{4}}, "", tasks) {
		t.Fatal("exit code no task has selected the array")
	}
	if selects(Filter{}, "success", tasks) {
		t.Fatal("an array with failed tasks matched success")
	}
	unfinished := append(tasks[:2:2], Job{ID: "a-3"})
	if selects(Filter{}, "failed", unfinished) || !selects(Filter{}, "unfinished", unfinished) {
		t.Fatal("an array with an unfinished task is not unfinished as a whole")
	}
}

// A job's attributes and log are read only once every earlier condition
// holds, and an error reading them is returned rather than taken as a
// non-match.
func TestSelectsReadsFactsLastAndReturnsTheirErrors(t *testing.T) {
	unreadable := errors.New("status.json: unreadable")
	after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var reads []string
	job := func(id string, exitCode int) Job {
		return Job{
			ID: id, Result: model.JobResult{ExitCode: exitCode}, Finished: true,
			LoadAttributes: func() (Attributes, error) {
				reads = append(reads, id+" attributes")
				return Attributes{}, unreadable
			},
			Diagnosis: func([]string) (bool, error) {
				reads = append(reads, id+" log")
				return false, unreadable
			},
		}
	}
	for _, test := range []struct {
		name      string
		filter    Filter
		selection string
		exitCode  int
		reads     []string
		selected  bool
		err       bool
	}{
		{name: "result selection excludes first", filter: Filter{StartedAfter: &after}, selection: "failed", reads: nil},
		{name: "exit code excludes first", filter: Filter{ExitCodes: []int{2}, Diagnoses: []string{"oom"}}, exitCode: 1, reads: nil},
		{name: "definition excludes first", filter: Filter{New: true, StartedAfter: &after}, exitCode: 1, reads: nil},
		{name: "no fact condition reads nothing", filter: Filter{ExitCodes: []int{1}}, exitCode: 1, reads: nil, selected: true},
		{name: "unreadable attributes", filter: Filter{StartedAfter: &after, Diagnoses: []string{"oom"}}, exitCode: 1, reads: []string{"a attributes"}, err: true},
		{name: "unreadable log", filter: Filter{Diagnoses: []string{"oom"}}, exitCode: 1, reads: []string{"a log"}, err: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			reads = nil
			selected, err := test.filter.Selects(test.selection, job("a", test.exitCode))
			if !slices.Equal(reads, test.reads) {
				t.Fatalf("reads = %q, want %q", reads, test.reads)
			}
			if test.err != errors.Is(err, unreadable) || selected != test.selected {
				t.Fatalf("Selects = %t, %v, want %t and error %t", selected, err, test.selected, test.err)
			}
		})
	}

	// An array decided as a whole stops at the first matching task; a task
	// tried before it that cannot be read fails the decision.
	matching := Job{ID: "b-1", Result: model.JobResult{ExitCode: 1}, Finished: true, LoadAttributes: func() (Attributes, error) {
		reads = append(reads, "b-1 attributes")
		return Attributes{StartedAt: after}, nil
	}}
	reads = nil
	if selected, err := (Filter{StartedAfter: &after}).SelectsArray("", []Job{matching, job("b-2", 1)}); err != nil || !selected {
		t.Fatalf("SelectsArray = %t, %v, want the first task to select the array", selected, err)
	}
	if !slices.Equal(reads, []string{"b-1 attributes"}) {
		t.Fatalf("reads = %q, want only the first task's", reads)
	}
	if selected, err := (Filter{StartedAfter: &after}).SelectsArray("", []Job{job("b-2", 1), matching}); !errors.Is(err, unreadable) || selected {
		t.Fatalf("SelectsArray = %t, %v, want the unreadable task's error", selected, err)
	}
}

func TestHasRunConditions(t *testing.T) {
	if (Filter{NotStages: []string{"a"}, Command: "x", Changed: true}).HasRunConditions() {
		t.Fatal("definition conditions reported as run conditions")
	}
	for _, filter := range []Filter{{ExitCodes: []int{1}}, {FailureKinds: []string{"oom"}}, {Diagnoses: []string{"x"}}, {Hosts: []string{"h"}}, {LongerThan: time.Second}} {
		if !filter.HasRunConditions() {
			t.Fatalf("%+v has no run conditions", filter)
		}
	}
}

func TestEmptyFilterMatchesEveryCommand(t *testing.T) {
	filter := Filter{}
	if !filter.Empty() {
		t.Fatal("zero Filter is not empty")
	}
	if !filter.MatchesCommand(model.QueuedCommand{Stage: "prepare"}) {
		t.Fatal("empty filter excluded a command")
	}
	if (Filter{NotStages: []string{"prepare"}}).Empty() {
		t.Fatal("filter with a negated stage is empty")
	}
}

func TestFilterMatchesExecutionAttributes(t *testing.T) {
	started := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	finished := started.Add(2 * time.Hour)
	filter := Filter{
		Hosts:          []string{"worker-*"},
		StartedAfter:   timePtr(started),
		FinishedBefore: timePtr(finished.Add(time.Minute)),
		LongerThan:     2 * time.Hour,
		ShorterThan:    3 * time.Hour,
	}
	attributes := Attributes{Hosts: []string{"worker-01"}, StartedAt: started, FinishedAt: finished, Now: finished}
	if !filter.MatchesAttributes(attributes) {
		t.Fatal("execution attribute filter did not match")
	}
	if filter.MatchesAttributes(Attributes{Hosts: []string{"other"}, StartedAt: started, FinishedAt: finished, Now: finished}) {
		t.Fatal("host filter matched an unrelated host")
	}
	if filter.MatchesAttributes(Attributes{Hosts: []string{"worker-01"}, StartedAt: started, Now: finished}) {
		t.Fatal("missing finished time matched a finished-time filter")
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
