package runwatch

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

// writeProject writes a project whose metadata records runID as running, held by
// a lock of pid on this host, or by no lock when pid is 0.
func writeProject(t *testing.T, baseDir, name, runID string, pid int) {
	t.Helper()
	paths, err := state.ResolveProjectPaths(baseDir, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(paths.RunsDir, runID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteJSON(paths.MetaFile, model.Meta{Phase: "running", LastRunID: runID}); err != nil {
		t.Fatal(err)
	}
	if pid == 0 {
		return
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	lock := model.LockInfo{PID: pid, RunID: runID, RunName: "nightly", StartedAt: "2026-10-31T00:00:00Z", Host: host, PIDNamespace: state.PIDNamespace()}
	if err := state.WriteJSON(paths.LockFile, lock); err != nil {
		t.Fatal(err)
	}
}

// A run whose metadata says it runs but whose local coordinator is gone is
// recorded once as interrupted, with the name and start its lock recorded. A
// run with a live coordinator, or one on another host, is not.
func TestReportInterruptedRecordsEachInterruptedRunOnce(t *testing.T) {
	baseDir := t.TempDir()
	writeProject(t, baseDir, "killed", "run-killed", 1<<30)
	writeProject(t, baseDir, "lockless", "run-lockless", 0)
	writeProject(t, baseDir, "live", "run-live", os.Getpid())
	// A coordinator on another host cannot be checked, so it counts as running.
	writeProject(t, baseDir, "remote", "run-remote", 1<<30)
	remote, err := state.ResolveProjectPaths(baseDir, "remote")
	if err != nil {
		t.Fatal(err)
	}
	if err := state.WriteJSON(remote.LockFile, model.LockInfo{PID: 1 << 30, RunID: "run-remote", Host: "elsewhere.invalid"}); err != nil {
		t.Fatal(err)
	}
	log := notificationlog.Open(t.TempDir())
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)

	recorded, err := ReportInterrupted(log, []string{baseDir}, now)
	if err != nil || recorded != 2 {
		t.Fatalf("recorded %d, %v; want 2", recorded, err)
	}
	if recorded, err := ReportInterrupted(log, []string{baseDir}, now); err != nil || recorded != 0 {
		t.Fatalf("second pass recorded %d, %v", recorded, err)
	}
	records, _, err := log.Records()
	if err != nil || len(records) != 2 {
		t.Fatalf("records = %#v, %v", records, err)
	}
	byRun := map[string]notification.Event{}
	for _, record := range records {
		if record.BaseDir != baseDir || len(record.Events) != 1 || record.Events[0].Kind != notification.RunInterrupted {
			t.Fatalf("record = %#v", record)
		}
		byRun[record.RunID] = record.Events[0]
	}
	if killed := byRun["run-killed"]; killed.Project != "killed" || killed.RunName != "nightly" || killed.StartedAt != "2026-10-31T00:00:00Z" {
		t.Fatalf("killed run = %#v", killed)
	}
	if _, ok := byRun["run-lockless"]; !ok {
		t.Fatalf("lockless run not recorded: %#v", byRun)
	}
}
