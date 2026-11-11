package state

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
)

func TestRunsNewestFirst(t *testing.T) {
	paths, err := ResolveProjectPaths(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if runIDs, err := RunsNewestFirst(paths); err != nil || runIDs != nil {
		t.Fatalf("a project without runs: %v, %v", runIDs, err)
	}
	makeRun := func(runID, recordedAt string) {
		t.Helper()
		runDir := filepath.Join(paths.RunsDir, runID)
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if recordedAt != "" {
			if err := WriteJSON(ContextPath(runDir), model.RunContext{Coordinator: &model.RunCoordinator{RecordedAt: recordedAt}}); err != nil {
				t.Fatal(err)
			}
		}
		// Directory times run opposite to start order; they are not used.
		modified := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Duration(len(runID)) * time.Hour)
		if err := os.Chtimes(runDir, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	// Three runs share a second; name order disagrees with start order.
	makeRun("20260101-000000-ffffffff", "2026-01-01T00:00:00.1Z")
	makeRun("20260101-000000-00000000", "2026-01-01T00:00:00.9Z")
	makeRun("20260101-000000-88888888", "") // from an earlier rotari
	makeRun("20260101-000001-aaaaaaaa", "")
	makeRun("20251231-235959-99999999", "")
	makeRun("renamed", "")
	if err := os.WriteFile(filepath.Join(paths.RunsDir, "not-a-run"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"20260101-000001-aaaaaaaa",
		"20260101-000000-00000000",
		"20260101-000000-ffffffff",
		"20260101-000000-88888888",
		"20251231-235959-99999999",
		"renamed",
	}
	runIDs, err := RunsNewestFirst(paths)
	if err != nil || !reflect.DeepEqual(runIDs, want) {
		t.Fatalf("runs = %v, %v; want %v", runIDs, err, want)
	}

	// meta.json's last run is the newest when it names a run directory.
	meta := DefaultMeta()
	meta.LastRunID = "20260101-000000-ffffffff"
	if err := WriteJSON(paths.MetaFile, meta); err != nil {
		t.Fatal(err)
	}
	want = append([]string{"20260101-000000-ffffffff"}, want[0], want[1], want[3], want[4], want[5])
	if runIDs, err = RunsNewestFirst(paths); err != nil || !reflect.DeepEqual(runIDs, want) {
		t.Fatalf("runs with a last run = %v, %v; want %v", runIDs, err, want)
	}
	meta.LastRunID = "deleted"
	if err := WriteJSON(paths.MetaFile, meta); err != nil {
		t.Fatal(err)
	}
	if newest, err := NewestRunID(paths); err != nil || newest != "20260101-000001-aaaaaaaa" {
		t.Fatalf("newest with a missing last run = %q, %v", newest, err)
	}

	// A context that cannot be read, for a run whose order needs it, is an
	// error, not a silent guess.
	if err := os.WriteFile(ContextPath(filepath.Join(paths.RunsDir, "20260101-000000-88888888")), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunsNewestFirst(paths); err == nil || !strings.Contains(err.Error(), "20260101-000000-88888888") {
		t.Fatalf("an unreadable context must fail naming the run: %v", err)
	}
}

func TestRunIDStart(t *testing.T) {
	now := time.Date(2026, 11, 12, 3, 4, 5, 600, time.UTC)
	runID := NewRunID(now)
	if len(runID) != runIDLength {
		t.Fatalf("run ID %q has length %d, want %d", runID, len(runID), runIDLength)
	}
	if started, ok := RunIDStart(runID); !ok || !started.Equal(now.Truncate(time.Second)) {
		t.Fatalf("start of %q = %v, %v", runID, started, ok)
	}
	if _, ok := RunIDStart("renamed"); ok {
		t.Fatal("a run ID in another form has no start")
	}
}
