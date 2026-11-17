package runwatch

import (
	"os"
	"path/filepath"
	"strings"
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

	recorded, err := NewWatcher().Report(log, []string{baseDir}, now)
	if err != nil || recorded != 2 {
		t.Fatalf("recorded %d, %v; want 2", recorded, err)
	}
	if recorded, err := NewWatcher().Report(log, []string{baseDir}, now); err != nil || recorded != 0 {
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

// A basedir that has no projects yet, or whose directory is gone, has no
// interrupted run: a pass records nothing, fails nothing, and creates no
// project directory.
func TestReportInterruptedSkipsBaseDirsWithoutProjects(t *testing.T) {
	fresh := t.TempDir()
	gone := filepath.Join(t.TempDir(), "gone")
	log := notificationlog.Open(t.TempDir())

	recorded, err := NewWatcher().Report(log, []string{fresh, gone}, time.Now())
	if err != nil || recorded != 0 {
		t.Fatalf("recorded %d, %v; want 0, nil", recorded, err)
	}
	for _, baseDir := range []string{fresh, gone} {
		if _, err := os.Stat(filepath.Join(baseDir, "projects")); !os.IsNotExist(err) {
			t.Fatalf("%s/projects exists after a pass: %v", baseDir, err)
		}
	}
}

// writeRemoteRun writes a project whose run is held by a coordinator on
// another host, with a heartbeat of seq unless seq is 0.
func writeRemoteRun(t *testing.T, baseDir, name, runID string, seq uint64) string {
	t.Helper()
	writeProject(t, baseDir, name, runID, 1<<30)
	paths, err := state.ResolveProjectPaths(baseDir, name)
	if err != nil {
		t.Fatal(err)
	}
	lock := model.LockInfo{PID: 1 << 30, RunID: runID, RunName: "nightly", StartedAt: "2026-11-17T00:00:00Z", Host: "login2.invalid"}
	if err := state.WriteJSON(paths.LockFile, lock); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(paths.RunsDir, runID)
	if seq > 0 {
		writeBeat(t, runDir, seq)
	}
	return runDir
}

func writeBeat(t *testing.T, runDir string, seq uint64) {
	t.Helper()
	beat := state.Heartbeat{Seq: seq, At: "2026-11-17T08:00:00Z", Host: "login2.invalid", PID: 1 << 30}
	if err := state.NewStore(0o755, 0o644).WriteHeartbeat(runDir, beat); err != nil {
		t.Fatal(err)
	}
}

// A remote run whose heartbeat stays unchanged for UnresponsiveAfter, by
// the watcher's clock, is recorded once as unresponsive; a beat that
// changes starts the wait again, and a run without a heartbeat is never
// reported.
func TestWatcherReportsARemoteRunWhoseHeartbeatStopped(t *testing.T) {
	baseDir := t.TempDir()
	stopped := writeRemoteRun(t, baseDir, "stopped", "run-stopped", 4)
	beating := writeRemoteRun(t, baseDir, "beating", "run-beating", 1)
	writeRemoteRun(t, baseDir, "old", "run-old", 0)
	log := notificationlog.Open(t.TempDir())
	watcher := NewWatcher()
	start := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)

	pass := func(at time.Duration, want int) {
		t.Helper()
		recorded, err := watcher.Report(log, []string{baseDir}, start.Add(at))
		if err != nil || recorded != want {
			t.Fatalf("pass at +%s recorded %d, %v; want %d", at, recorded, err, want)
		}
	}
	pass(0, 0)
	writeBeat(t, beating, 2)
	pass(UnresponsiveAfter-time.Second, 0)
	writeBeat(t, beating, 3)
	pass(UnresponsiveAfter, 1)
	writeBeat(t, beating, 4)
	pass(2*UnresponsiveAfter, 0)

	records, _, err := log.Records()
	if err != nil || len(records) != 1 || len(records[0].Events) != 1 {
		t.Fatalf("records = %#v, %v; want one", records, err)
	}
	event := records[0].Events[0]
	if event.Kind != notification.RunUnresponsive || event.RunID != "run-stopped" || event.Project != "stopped" || event.RunName != "nightly" ||
		event.Host != "login2.invalid" || event.LastHeartbeatAt != "2026-11-17T08:00:00Z" || event.SilentSeconds != int64(UnresponsiveAfter/time.Second) {
		t.Fatalf("event = %#v", event)
	}

	// A heartbeat that moves, even backwards as when another coordinator
	// takes the run over, is a live run.
	other := NewWatcher()
	if _, err := other.Report(log, []string{baseDir}, start); err != nil {
		t.Fatal(err)
	}
	writeBeat(t, stopped, 1)
	if _, err := other.Report(log, []string{baseDir}, start.Add(UnresponsiveAfter)); err != nil {
		t.Fatal(err)
	}
	if silent, stale := other.observe(runKey{baseDir, "stopped", "run-stopped"}, state.Heartbeat{Seq: 1}, start.Add(UnresponsiveAfter+time.Second)); stale || silent != time.Second {
		t.Fatalf("observe after a changed beat = %s, %t; want 1s and not stale", silent, stale)
	}
}

// A remote run's heartbeat that cannot be decoded is an error naming it, and
// does not stop the other projects.
func TestWatcherReportsADamagedHeartbeat(t *testing.T) {
	baseDir := t.TempDir()
	runDir := writeRemoteRun(t, baseDir, "damaged", "run-damaged", 1)
	path := filepath.Join(runDir, state.HeartbeatFileName)
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProject(t, baseDir, "killed", "run-killed", 1<<30)
	recorded, err := NewWatcher().Report(notificationlog.Open(t.TempDir()), []string{baseDir}, time.Now())
	if err == nil || !strings.Contains(err.Error(), path) || recorded != 1 {
		t.Fatalf("Report = %d, %v; want the killed run recorded and an error naming %s", recorded, err, path)
	}
}
