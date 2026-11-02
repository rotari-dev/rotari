package notificationlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/notification"
)

func testLog(t *testing.T, now *time.Time) Log {
	t.Helper()
	log := Open(t.TempDir())
	log.now = func() time.Time { return *now }
	return log
}

func jobEvent(runID, jobID string, exitCode int) notification.Event {
	return notification.NewJobEvent("demo", runID, "", model.JobSpec{ID: jobID, Name: jobID, Environment: []string{"TOKEN=secret"}},
		model.JobResult{ID: jobID, ExitCode: exitCode}, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))
}

func TestRecordsListEveryRunNewestFirst(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	log := testLog(t, &now)
	for _, step := range []struct {
		runID, jobID string
	}{{"run-a", "first"}, {"run-b", "second"}, {"run-a", "third"}} {
		now = now.Add(time.Second)
		if err := log.Append("/state", []notification.Event{jobEvent(step.runID, step.jobID, 1)}); err != nil {
			t.Fatal(err)
		}
	}
	records, problems, err := log.Records()
	if err != nil || len(problems) != 0 {
		t.Fatalf("records: %v, problems %v", err, problems)
	}
	var jobs []string
	for _, record := range records {
		if record.BaseDir != "/state" || record.Project != "demo" {
			t.Fatalf("record location = %#v", record)
		}
		jobs = append(jobs, record.Events[0].Job.ID)
		if record.Events[0].Job.Environment != nil {
			t.Fatalf("record keeps the job environment: %#v", record.Events[0].Job)
		}
	}
	if strings.Join(jobs, ",") != "third,second,first" {
		t.Fatalf("records in order %v", jobs)
	}
	if _, err := os.Stat(filepath.Join(log.dir, "run-a.jsonl")); err != nil {
		t.Fatalf("run-a has no file of its own: %v", err)
	}
}

func TestRecordsReportUnreadableRecordsAndSkipOneBeingWritten(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	log := testLog(t, &now)
	if err := log.Append("/state", []notification.Event{jobEvent("run-a", "first", 1)}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(log.dir, "run-a.jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	// A writer killed mid-record leaves a line without its newline.
	if _, err := file.WriteString(`{"schema_version":1,"id":`); err != nil {
		t.Fatal(err)
	}
	file.Close()
	records, problems, err := log.Records()
	if err != nil || len(records) != 1 || len(problems) != 0 {
		t.Fatalf("records %d, problems %v, err %v", len(records), problems, err)
	}
	// A later record still starts on its own line; the cut record is then
	// reported, not dropped silently.
	now = now.Add(time.Second)
	if err := log.Append("/state", []notification.Event{jobEvent("run-a", "second", 1)}); err != nil {
		t.Fatal(err)
	}
	records, problems, err = log.Records()
	if err != nil || len(records) != 2 || len(problems) != 1 || !strings.Contains(problems[0], "run-a.jsonl line 2") {
		t.Fatalf("records %d, problems %v, err %v", len(records), problems, err)
	}
}

func TestRecordsExpireAfterRetention(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	log := testLog(t, &now)
	if err := log.Append("/state", []notification.Event{jobEvent("run-old", "old", 1)}); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(log.dir, "run-old.jsonl")
	if err := os.Chtimes(oldFile, now, now); err != nil {
		t.Fatal(err)
	}
	now = now.Add(Retention + time.Minute)
	records, _, err := log.Records()
	if err != nil || len(records) != 0 {
		t.Fatalf("expired records are listed: %d, %v", len(records), err)
	}
	if _, err := os.Stat(oldFile); err != nil {
		t.Fatalf("reading removed a file: %v", err)
	}
	if err := log.Append("/state", []notification.Event{jobEvent("run-new", "new", 1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expired run file kept after an append: %v", err)
	}
	records, _, err = log.Records()
	if err != nil || len(records) != 1 || records[0].RunID != "run-new" {
		t.Fatalf("records = %#v, %v", records, err)
	}
}

func TestAppendRejectsUnsafeOrMixedRuns(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	log := testLog(t, &now)
	for name, test := range map[string]struct {
		baseDir string
		events  []notification.Event
	}{
		"slash in run ID":     {"/state", []notification.Event{jobEvent("../escape", "job", 1)}},
		"backslash in run ID": {"/state", []notification.Event{jobEvent(`..\escape`, "job", 1)}},
		"relative basedir":    {"state", []notification.Event{jobEvent("run-a", "job", 1)}},
		"two runs":            {"/state", []notification.Event{jobEvent("run-a", "job", 1), jobEvent("run-b", "job", 1)}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := log.Append(test.baseDir, test.events); err == nil {
				t.Fatal("Append accepted the record")
			}
		})
	}
	if entries, _ := os.ReadDir(log.dir); len(entries) != 0 {
		t.Fatalf("rejected records left files: %v", entries)
	}
}

func TestInboxCursorsOnlyAdvanceAndStayPerBasedir(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	log := testLog(t, &now)
	cursor := func(second int, id string) Cursor {
		return Cursor{CreatedAt: now.Add(time.Duration(second) * time.Second).Format(timeLayout), ID: id}
	}
	if inbox, err := log.State(); err != nil || len(inbox.BaseDirs) != 0 || inbox.For("/one") != (Cursors{}) {
		t.Fatalf("initial state = %#v, %v", inbox, err)
	}
	inbox, err := log.Advance(map[string]Cursors{"/one": {Read: cursor(2, "b")}})
	if err != nil || inbox.For("/one").Read != cursor(2, "b") {
		t.Fatalf("read = %#v, %v", inbox, err)
	}
	// Reading one basedir leaves the others unread.
	if inbox.For("/two") != (Cursors{}) {
		t.Fatalf("another basedir was read: %#v", inbox)
	}
	// Another page that saw less does not move the cursor back.
	if inbox, err = log.Advance(map[string]Cursors{"/one": {Read: cursor(1, "a")}}); err != nil || inbox.For("/one").Read != cursor(2, "b") {
		t.Fatalf("read moved back: %#v, %v", inbox, err)
	}
	// Clearing also reads, and only in its basedir.
	inbox, err = log.Advance(map[string]Cursors{"/two": {Cleared: cursor(3, "c")}})
	if err != nil || inbox.For("/two") != (Cursors{Read: cursor(3, "c"), Cleared: cursor(3, "c")}) || inbox.For("/one") != (Cursors{Read: cursor(2, "b")}) {
		t.Fatalf("clear = %#v, %v", inbox, err)
	}
	if stored, err := log.State(); err != nil || stored.For("/one") != inbox.For("/one") || stored.For("/two") != inbox.For("/two") {
		t.Fatalf("stored state = %#v, %v", stored, err)
	}
	for name, changes := range map[string]map[string]Cursors{
		"invalid cursor":   {"/one": {Read: Cursor{CreatedAt: "yesterday", ID: "x"}}},
		"relative basedir": {"one": {Read: cursor(4, "d")}},
	} {
		if _, err := log.Advance(changes); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// AppendOnce records an event once per run and kind, even when several
// callers race, and leaves other kinds and runs alone.
func TestAppendOnceRecordsAnEventOncePerRun(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	log := Open(t.TempDir())
	interrupted := func(runID string) notification.Event {
		return notification.NewInterruptedRunEvent("demo", model.RunSummary{RunID: runID}, now)
	}
	if err := log.Append("/state", []notification.Event{jobEvent("run-a", "job", 1)}); err != nil {
		t.Fatal(err)
	}
	added := make(chan bool, 8)
	for range 8 {
		go func() {
			ok, err := log.AppendOnce("/state", interrupted("run-a"))
			if err != nil {
				t.Error(err)
			}
			added <- ok
		}()
	}
	count := 0
	for range 8 {
		if <-added {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("appended %d times, want once", count)
	}
	if ok, err := log.AppendOnce("/state", interrupted("run-b")); err != nil || !ok {
		t.Fatalf("another run = %v, %v", ok, err)
	}
	records, problems, err := log.Records()
	if err != nil || len(problems) != 0 || len(records) != 3 {
		t.Fatalf("records = %d, problems %v, err %v", len(records), problems, err)
	}
}
