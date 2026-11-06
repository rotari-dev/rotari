package weblog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/webui"
)

func readRecords(t *testing.T, path string) []map[string]any {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var records []map[string]any
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("invalid JSONL record %q: %v", scanner.Text(), err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}

func TestRecorderWritesBoundedAllowListedRecords(t *testing.T) {
	dir := t.TempDir()
	recorder := New(dir, "web", nil)
	recorder.RecordAuthEvent(webui.AuthEvent{
		Event: "sign_in", Outcome: "wrong_credentials", Transport: "https",
		UserAgent: strings.Repeat("a", 300) + "\nforged",
	})
	recorder.RecordServerStart(Start{Version: "rotari dev", Host: "localhost", Auth: "none"})
	recorder.Close()

	records := readRecords(t, Path(dir))
	if len(records) != 2 {
		t.Fatalf("records = %#v", records)
	}
	if got := records[0]["user_agent"].(string); len(got) != 256 || strings.Contains(got, "forged") {
		t.Fatalf("user_agent = %q, want 256 bounded bytes", got)
	}
	for _, key := range []string{"port", "allow_exec", "allow_terminal", "allow_control", "notifications", "artifact_root_count", "registered_basedir_count"} {
		if _, ok := records[1][key]; !ok {
			t.Errorf("server_start lacks %q: %#v", key, records[1])
		}
	}
	if info, err := os.Stat(Path(dir)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("log mode = %v, %v, want 0600", info, err)
	}
}

func TestRecorderRateLimitsRotatesAndKeepsConcurrentLinesWhole(t *testing.T) {
	dir := t.TempDir()
	recorder := New(dir, "web", nil)
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	recorder.now = func() time.Time { return now }
	for range rateLimit + 5 {
		recorder.RecordAuthEvent(webui.AuthEvent{Event: "sign_in", Outcome: "wrong_credentials"})
	}
	now = now.Add(rateWindow)
	recorder.RecordAuthEvent(webui.AuthEvent{Event: "sign_in", Outcome: "accepted"})
	recorder.Close()
	records := readRecords(t, Path(dir))
	if len(records) != rateLimit+1 || records[len(records)-1]["suppressed"] != float64(5) {
		t.Fatalf("rate-limited records = %#v", records)
	}

	if err := os.WriteFile(Path(dir), bytes.Repeat([]byte("x"), maxFileBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	first := New(dir, "web", nil)
	second := New(dir, "tableview", nil)
	var wait sync.WaitGroup
	for index := range 20 {
		wait.Add(2)
		go func() { defer wait.Done(); first.RecordServerStart(Start{Version: "first"}) }()
		go func() { defer wait.Done(); second.RecordServerStart(Start{Version: "second"}) }()
		_ = index
	}
	wait.Wait()
	first.Close()
	second.Close()
	if info, err := os.Stat(filepath.Join(dir, RotatedFileName)); err != nil || info.Size() != maxFileBytes {
		t.Fatalf("rotated log = %v, %v", info, err)
	}
	if got := len(readRecords(t, Path(dir))); got != 40 {
		t.Fatalf("current log has %d whole records, want 40", got)
	}
}

func TestRecorderFailureWarnsOnceAndDoesNotBlockCallers(t *testing.T) {
	parent := t.TempDir()
	blocked := filepath.Join(parent, "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var warning bytes.Buffer
	recorder := New(filepath.Join(blocked, "master"), "web", &warning)
	for range 3 {
		recorder.RecordAuthEvent(webui.AuthEvent{Event: "sign_in", Outcome: "accepted"})
	}
	recorder.Close()
	if got := strings.Count(warning.String(), "cannot record Web events"); got != 1 {
		t.Fatalf("warning = %q, want one failure", warning.String())
	}
}
