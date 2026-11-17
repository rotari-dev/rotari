package jobcontrol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/state"
)

func loadCancelRecords(t *testing.T, runDir string) []model.CancelRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(runDir, state.CancelsFileName))
	if err != nil {
		t.Fatal(err)
	}
	var records []model.CancelRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var record model.CancelRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("cancels.jsonl line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

// Every cancel request is recorded in the run's cancels.jsonl, whoever asked:
// what it asked for, what it did to each job, and why it failed when it did.
func TestCancelRecordsEachRequest(t *testing.T) {
	fixture := newActiveRunFixture(t)
	if _, err := fixture.controller.Cancel(fixture.baseDir, "demo", "", []string{"grid"}, false, CancelFromWeb); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.Cancel(fixture.baseDir, "demo", testRunID, nil, false, CancelFromWait); err != nil {
		t.Fatal(err)
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	if err := state.WriteJSON(fixture.paths.LockFile, model.LockInfo{PID: os.Getpid(), RunID: testRunID, Host: host + "-other"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.Cancel(fixture.baseDir, "demo", "", nil, false, CancelFromDisconnect); err == nil {
		t.Fatal("a cancel of another host's run succeeded")
	}

	records := loadCancelRecords(t, fixture.runDir)
	if len(records) != 3 {
		t.Fatalf("records = %+v, want one per request", records)
	}
	actions := func(record model.CancelRecord) map[string]string {
		found := map[string]string{}
		for _, target := range record.Targets {
			found[target.JobID] = target.Action
		}
		return found
	}
	byJob := records[0]
	if byJob.Source != "web" || byJob.RunWide || strings.Join(byJob.Requested, ",") != "grid" || byJob.PID != os.Getpid() || byJob.Host != host || byJob.At == "" || byJob.Error != "" {
		t.Fatalf("job cancel record = %+v", byJob)
	}
	if got := actions(byJob); got["grid-1"] != model.CancelByExecutor || got["grid-3"] != model.CancelByMarker || len(got) != 2 {
		t.Fatalf("job cancel targets = %v, want grid-1 by executor and grid-3 by marker", got)
	}
	runWide := records[1]
	if runWide.Source != "wait" || !runWide.RunWide || len(runWide.Requested) != 0 || runWide.Error != "" {
		t.Fatalf("run-wide record = %+v", runWide)
	}
	if got := actions(runWide); got["on-slurm"] != model.CancelByExecutor || got["waiting"] != model.CancelByMarker {
		t.Fatalf("run-wide targets = %v", got)
	}
	refused := records[2]
	if refused.Source != "disconnect" || !refused.RunWide || len(refused.Targets) != 0 || !strings.Contains(refused.Error, "run cancel from that host") {
		t.Fatalf("refused record = %+v", refused)
	}
}

// A request refused before its run is known, such as one for a project
// with no active run, writes no record anywhere.
func TestCancelWithoutAnActiveRunRecordsNothing(t *testing.T) {
	fixture := newActiveRunFixture(t)
	if err := os.Remove(fixture.paths.LockFile); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteJSON(fixture.paths.MetaFile, model.Meta{Phase: "finished", LastRunID: testRunID}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.Cancel(fixture.baseDir, "demo", "", nil, false, CancelFromCLI); err == nil {
		t.Fatal("a cancel without an active run succeeded")
	}
	if _, err := os.Stat(filepath.Join(fixture.runDir, state.CancelsFileName)); !os.IsNotExist(err) {
		t.Fatalf("cancels.jsonl exists after a refused request: %v", err)
	}
}
