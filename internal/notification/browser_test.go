package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
)

func TestNewBrowserMessageMergesARunsEvents(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	failed := NewJobEvent("demo", "run-1", "nightly",
		model.JobSpec{ID: "job-1", Name: "train", Command: []string{"python", "train.py"}},
		model.JobResult{ID: "job-1", ExitCode: 1, Diagnoses: []model.RuleDiagnosis{{Name: "First"}, {Name: "Second"}}}, now)
	succeeded := NewJobEvent("demo", "run-1", "nightly", model.JobSpec{ID: "job-2", Name: "eval"}, model.JobResult{ID: "job-2"}, now)
	run := NewRunEvent("demo", model.RunSummary{RunID: "run-1", RunName: "nightly", Status: "failed", ExitCode: 1,
		Results: []model.JobResult{{ExitCode: 1}, {}}}, now)
	settings := ChannelSettings{JobFailure: true, JobSuccess: true, RunFailure: true, RunSuccess: true, MaxJobs: 10,
		Fields: []string{"job_name", "command", "diagnosis_name", "run_status", "failure_count", "link"}}

	message, ok := NewBrowserMessage([]Event{succeeded, failed, run}, settings)
	if !ok {
		t.Fatal("message not rendered")
	}
	want := BrowserMessage{
		Title: "rotari: run failed (1 job failed)",
		Body: "job name: train\ncommand: python train.py\ndiagnosis name: First\ndiagnosis name: Second\n\n" +
			"job name: eval\n\nrun status: failed\nfailure count: 1",
		Link: true,
	}
	if message != want {
		t.Fatalf("message = %#v, want %#v", message, want)
	}
}

func TestNewBrowserMessageAppliesSelection(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	job := func(id string, exitCode int) Event {
		return NewJobEvent("demo", "run-1", "", model.JobSpec{ID: id, Name: id}, model.JobResult{ID: id, ExitCode: exitCode}, now)
	}
	run := func(exitCode int) Event {
		return NewRunEvent("demo", model.RunSummary{RunID: "run-1", ExitCode: exitCode}, now)
	}
	defaults := Defaults().Browser
	for _, test := range []struct {
		name     string
		events   []Event
		settings func(*ChannelSettings)
		title    string
		body     string
	}{
		{name: "job successes are off by default", events: []Event{job("ok", 0)}},
		{name: "job successes", events: []Event{job("a", 0), job("b", 0)}, settings: func(s *ChannelSettings) { s.JobSuccess = true; s.Fields = []string{"job_name"} },
			title: "rotari: 2 jobs succeeded", body: "job name: a\n\njob name: b"},
		{name: "max jobs bounds each list", events: []Event{job("a", 1), job("b", 1), job("c", 1)}, settings: func(s *ChannelSettings) { s.MaxJobs = 2; s.Fields = []string{"job_name"} },
			title: "rotari: 2 jobs failed", body: "job name: a\n\njob name: b"},
		{name: "a successful run", events: []Event{run(0)}, settings: func(s *ChannelSettings) { s.Fields = []string{"run_status"} },
			title: "rotari: run succeeded", body: "run status: success"},
		{name: "a run's exit code is its outcome", events: []Event{run(2)}, settings: func(s *ChannelSettings) { s.RunFailure = false }},
		{name: "a run without a selected completion", events: []Event{job("bad", 1), run(0)}, settings: func(s *ChannelSettings) { s.RunSuccess = false; s.Fields = nil },
			title: "rotari: 1 job failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := defaults
			settings.Fields = append([]string(nil), defaults.Fields...)
			if test.settings != nil {
				test.settings(&settings)
			}
			message, ok := NewBrowserMessage(test.events, settings)
			if ok != (test.title != "") || message.Title != test.title || message.Body != test.body {
				t.Fatalf("message = %#v, %v; want title %q, body %q", message, ok, test.title, test.body)
			}
		})
	}
}

func TestNewBrowserMessageTruncatesTheBody(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	event := NewJobEvent("demo", "run-1", "", model.JobSpec{ID: "job", Name: strings.Repeat("é", 1200)}, model.JobResult{ID: "job", ExitCode: 1}, now)
	message, ok := NewBrowserMessage([]Event{event}, ChannelSettings{JobFailure: true, MaxJobs: 1, Fields: []string{"job_name"}})
	body := []rune(message.Body)
	if !ok || len(body) != browserBodyLimit || !strings.HasSuffix(message.Body, "é...") {
		t.Fatalf("body has %d characters: %q", len(body), message.Body[len(message.Body)-10:])
	}
}

func TestRecordableKeepsOnlyVocabularyFacts(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	task := 3
	event := NewJobEvent("demo", "run-1", "", model.JobSpec{ID: "job", Name: "train", Command: []string{"run"}, ArrayTaskID: &task,
		Environment: []string{"TOKEN=secret"}, ExecutorOptions: []string{"--account=lab"}}, model.JobResult{ID: "job", ExitCode: 1, Hosts: []string{"gpu1"}}, now)
	recorded := event.Recordable()
	if recorded.Job.Environment != nil || recorded.Job.ExecutorOptions != nil {
		t.Fatalf("recorded job keeps its environment: %#v", recorded.Job)
	}
	all := AvailableFields()
	original, kept := NewPayload(Batch{Events: []Event{event}}, all, "", ""), NewPayload(Batch{Events: []Event{recorded}}, all, "", "")
	if len(original.Events[0].Fields) != len(kept.Events[0].Fields) {
		t.Fatalf("recorded fields = %#v, want %#v", kept.Events[0].Fields, original.Events[0].Fields)
	}
	if event.Job.Environment == nil {
		t.Fatal("Recordable changed the original event")
	}
}
