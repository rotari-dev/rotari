package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/notification"
	"github.com/rotari-dev/rotari/internal/notificationlog"
	"github.com/rotari-dev/rotari/internal/state"
)

func TestRunNotificationManagerBatchesJobsUntilRunFinishes(t *testing.T) {
	paths := testWebhookPaths(t)
	if err := os.WriteFile(filepath.Join(paths.BaseDir, notification.FileName), []byte(`[webhook]
url = "https://example.invalid/hook"
job_success = true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	runID := "run-batch"
	runDir := filepath.Join(paths.RunsDir, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteJSON(filepath.Join(runDir, "summary.json"), model.RunSummary{RunID: runID, Status: "failed", ExitCode: 1}); err != nil {
		t.Fatal(err)
	}

	manager := newRunNotificationManager(t.TempDir())
	var sent []notification.Batch
	manager.send = func(_ notification.WebhookSettings, _ string, batch notification.Batch) {
		sent = append(sent, batch)
	}
	manager.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	manager.JobFinished(paths, runID, "", model.JobSpec{ID: "ok"}, model.JobResult{ID: "ok"})
	manager.JobFinished(paths, runID, "", model.JobSpec{ID: "bad"}, model.JobResult{ID: "bad", ExitCode: 1})
	manager.RunFinished(paths, runID, 1)

	if len(sent) != 1 || len(sent[0].Events) != 3 {
		t.Fatalf("sent batches = %#v", sent)
	}
}

func TestRunNotificationManagerAppliesJobSuccessSetting(t *testing.T) {
	paths := testWebhookPaths(t)
	if err := os.WriteFile(filepath.Join(paths.BaseDir, notification.FileName), []byte(`[webhook]
url = "https://example.invalid/hook"
job_success = false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := newRunNotificationManager(t.TempDir())
	var sent []notification.Batch
	manager.send = func(_ notification.WebhookSettings, _ string, batch notification.Batch) { sent = append(sent, batch) }
	manager.JobFinished(paths, "run-filter", "", model.JobSpec{ID: "ok"}, model.JobResult{ID: "ok"})

	manager.mu.Lock()
	for key := range manager.pending {
		manager.mu.Unlock()
		manager.flush(key)
		manager.mu.Lock()
	}
	manager.mu.Unlock()
	if len(sent) != 0 {
		t.Fatalf("sent batches = %#v", sent)
	}
}

// Every batch is recorded for the Web inbox before any channel's selection,
// whether or not a webhook is configured or its settings can be read; only
// the webhook depends on them.
func TestRunNotificationManagerRecordsEveryBatch(t *testing.T) {
	for name, config := range map[string]string{
		"no webhook":       "",
		"webhook selected": "[webhook]\nurl = \"https://example.invalid/hook\"\njob_success = false\n",
		"unreadable":       "[webhook\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(envWebhookURL, "")
			paths := testWebhookPaths(t)
			if config != "" {
				if err := os.WriteFile(filepath.Join(paths.BaseDir, notification.FileName), []byte(config), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			runID := "run-record"
			runDir := filepath.Join(paths.RunsDir, runID)
			if err := os.MkdirAll(runDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := state.WriteJSON(filepath.Join(runDir, "summary.json"), model.RunSummary{RunID: runID, Status: "finished"}); err != nil {
				t.Fatal(err)
			}
			masterDir := t.TempDir()
			manager := newRunNotificationManager(masterDir)
			var sent []notification.Batch
			manager.send = func(_ notification.WebhookSettings, _ string, batch notification.Batch) { sent = append(sent, batch) }
			manager.JobFinished(paths, runID, "", model.JobSpec{ID: "ok"}, model.JobResult{ID: "ok"})
			manager.RunFinished(paths, runID, 0)

			records, problems, err := notificationlog.Open(masterDir).Records()
			if err != nil || len(problems) != 0 || len(records) != 1 || len(records[0].Events) != 2 {
				t.Fatalf("records = %#v, problems %v, err %v", records, problems, err)
			}
			if records[0].BaseDir != paths.BaseDir || records[0].RunID != runID {
				t.Fatalf("record location = %#v", records[0])
			}
			wantSent := 0
			if name == "webhook selected" {
				wantSent = 1
			}
			if len(sent) != wantSent || (wantSent == 1 && len(sent[0].Events) != 1) {
				t.Fatalf("sent batches = %#v", sent)
			}
		})
	}
}

// A run that unlock releases unfinished is recorded and sent at once.
func TestRunNotificationManagerReportsAnInterruptedRunAtOnce(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(envWebhookURL, "")
	paths := testWebhookPaths(t)
	if err := os.WriteFile(filepath.Join(paths.BaseDir, notification.FileName), []byte("[webhook]\nurl = \"https://example.invalid/hook\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	masterDir := t.TempDir()
	manager := newRunNotificationManager(masterDir)
	var sent []notification.Batch
	manager.send = func(_ notification.WebhookSettings, _ string, batch notification.Batch) { sent = append(sent, batch) }
	manager.RunInterrupted(paths, model.RunSummary{RunID: "run-stopped", RunName: "nightly", StartedAt: "2026-01-02T03:00:00Z"})
	if len(sent) != 1 || len(sent[0].Events) != 1 || sent[0].Events[0].Kind != notification.RunInterrupted || sent[0].Events[0].RunName != "nightly" {
		t.Fatalf("sent batches = %#v", sent)
	}
	records, _, err := notificationlog.Open(masterDir).Records()
	if err != nil || len(records) != 1 || records[0].Events[0].Kind != notification.RunInterrupted {
		t.Fatalf("records = %#v, %v", records, err)
	}
}
