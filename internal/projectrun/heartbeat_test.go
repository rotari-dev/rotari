package projectrun

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// The coordinator writes a beat at once and a new one every interval, naming
// itself, and stops when told to.
func TestHeartbeatBeatsUntilStopped(t *testing.T) {
	runner, paths := testRunner(t)
	runDir := filepath.Join(paths.RunsDir, "run-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stop := runner.startHeartbeat(paths, "run-1", 10*time.Millisecond)
	first, found, err := runner.Store.LoadHeartbeat(runDir)
	if err != nil || !found || first.Seq != 1 || first.At == "" || first.PID != os.Getpid() || first.Host == "" {
		stop()
		t.Fatalf("first beat = %+v, %t, %v; want seq 1 from this process at once", first, found, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		beat, _, err := runner.Store.LoadHeartbeat(runDir)
		if err != nil {
			stop()
			t.Fatal(err)
		}
		if beat.Seq >= 3 {
			break
		}
		if time.Now().After(deadline) {
			stop()
			t.Fatalf("heartbeat stayed at seq %d", beat.Seq)
		}
		time.Sleep(5 * time.Millisecond)
	}
	stop()
	last, _, err := runner.Store.LoadHeartbeat(runDir)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	after, _, err := runner.Store.LoadHeartbeat(runDir)
	if err != nil || after.Seq != last.Seq {
		t.Fatalf("heartbeat moved from seq %d to %d after it stopped (%v)", last.Seq, after.Seq, err)
	}
}

// A heartbeat that cannot be written is reported once when it starts to
// fail and once when it recovers, not on every beat, and never stops the
// run.
func TestHeartbeatReportsFailureOnceAndRecovery(t *testing.T) {
	runner, paths := testRunner(t)
	var mu sync.Mutex
	var messages []string
	runner.Errorf = func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, fmt.Sprintf(format, args...))
	}
	runDir := filepath.Join(paths.RunsDir, "run-1")
	// A file is where the run directory goes, so beats fail until it is gone.
	if err := os.MkdirAll(paths.RunsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stop := runner.startHeartbeat(paths, "run-1", 5*time.Millisecond)
	time.Sleep(40 * time.Millisecond)
	if err := os.Remove(runDir); err != nil {
		stop()
		t.Fatal(err)
	}
	// A beat that comes first makes the directory itself.
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		stop()
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, found, _ := runner.Store.LoadHeartbeat(runDir); found {
			break
		}
		if time.Now().After(deadline) {
			stop()
			t.Fatal("heartbeat was not written after the run directory appeared")
		}
		time.Sleep(5 * time.Millisecond)
	}
	stop()
	mu.Lock()
	defer mu.Unlock()
	if len(messages) != 2 {
		t.Fatalf("messages = %q, want one failure and one recovery", messages)
	}
}
