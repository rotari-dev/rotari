package notification

import (
	"fmt"
	"strings"

	"github.com/rotari-dev/rotari/internal/model"
)

// browserBodyLimit bounds a browser notification's body, in characters.
const browserBodyLimit = 1000

// Recordable returns event with only the job and result facts that the
// shared field vocabulary reads, so a kept record carries no environment,
// executor options, or other job settings.
func (event Event) Recordable() Event {
	if event.Job != nil {
		job := *event.Job
		event.Job = &model.JobSpec{ID: job.ID, AttemptID: job.AttemptID, Command: job.Command, WorkingDirectory: job.WorkingDirectory,
			Executor: job.Executor, Name: job.Name, Stage: job.Stage, ArrayTaskID: job.ArrayTaskID}
	}
	if event.Result != nil {
		result := *event.Result
		event.Result = &model.JobResult{ID: result.ID, AttemptID: result.AttemptID, ExitCode: result.ExitCode, Error: result.Error,
			Command: result.Command, Hosts: result.Hosts, Diagnoses: result.Diagnoses, DiagnosisStatus: result.DiagnosisStatus,
			DiagnosisRules: result.DiagnosisRules}
	}
	return event
}

// BrowserMessage is one browser notification: an inbox entry and, for a page
// that sees it arrive, a desktop notification.
type BrowserMessage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	// Link reports whether the message opens its run, which the "link" field
	// selects.
	Link bool `json:"link"`
}

// NewBrowserMessage renders one run's events with browser settings. Selected
// job failures, then job successes, each up to settings.MaxJobs, and the run's
// completion become sections of "field name: value" lines in settings.Fields
// order. It reports false when settings select none of the events.
func NewBrowserMessage(events []Event, settings ChannelSettings) (BrowserMessage, bool) {
	var failed, succeeded []Event
	var run *Event
	for index, event := range events {
		if !settings.Includes(event) {
			continue
		}
		switch {
		case event.Kind == RunFinished || event.Kind == RunInterrupted:
			run = &events[index]
		case event.JobStatus == model.StatusSuccess:
			if len(succeeded) < settings.MaxJobs {
				succeeded = append(succeeded, event)
			}
		default:
			if len(failed) < settings.MaxJobs {
				failed = append(failed, event)
			}
		}
	}
	jobs := func(count int, outcome string) string {
		if count == 1 {
			return "1 job " + outcome
		}
		return fmt.Sprintf("%d jobs %s", count, outcome)
	}
	var title string
	switch {
	case run != nil:
		outcome := "succeeded"
		if run.Kind == RunInterrupted {
			outcome = "interrupted"
		} else if run.ExitCode != 0 {
			outcome = "failed"
		}
		title = "rotari: run " + outcome
		if len(failed) > 0 {
			title += " (" + jobs(len(failed), "failed") + ")"
		}
	case len(failed) > 0:
		title = "rotari: " + jobs(len(failed), "failed")
	case len(succeeded) > 0:
		title = "rotari: " + jobs(len(succeeded), "succeeded")
	default:
		return BrowserMessage{}, false
	}
	sections := make([]string, 0, len(failed)+len(succeeded)+1)
	for _, event := range append(append([]Event(nil), failed...), succeeded...) {
		if lines := browserLines(event, settings.Fields); len(lines) > 0 {
			sections = append(sections, strings.Join(lines, "\n"))
		}
	}
	if run != nil {
		if lines := browserLines(*run, settings.Fields); len(lines) > 0 {
			sections = append(sections, strings.Join(lines, "\n"))
		}
	}
	body := []rune(strings.Join(sections, "\n\n"))
	if len(body) > browserBodyLimit {
		body = append(body[:browserBodyLimit-3], []rune("...")...)
	}
	link := false
	for _, name := range settings.Fields {
		link = link || name == "link"
	}
	return BrowserMessage{Title: title, Body: string(body), Link: link}, true
}

// browserLines writes event's selected fields as "field name: value" lines.
// A run's status is its outcome, as in the title, and a command is one line
// of space-separated arguments.
func browserLines(event Event, fields []string) []string {
	lines := make([]string, 0, len(fields))
	for _, name := range fields {
		if name == "link" {
			continue
		}
		label := strings.ReplaceAll(name, "_", " ") + ": "
		if name == "run_status" && event.Kind == RunFinished {
			status := "success"
			if event.ExitCode != 0 {
				status = "failed"
			}
			lines = append(lines, label+status)
			continue
		}
		for _, field := range eventFields(event, name, event.Successes, event.Failures, "", "") {
			value := FormatValue(field.Value)
			if command, ok := field.Value.([]string); ok && name == "command" {
				value = strings.Join(command, " ")
			}
			lines = append(lines, label+value)
		}
	}
	return lines
}
