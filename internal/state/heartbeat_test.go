package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeartbeatRoundTripAndAbsence(t *testing.T) {
	store := NewStore(0o755, 0o644)
	runDir := t.TempDir()
	if _, found, err := store.LoadHeartbeat(runDir); err != nil || found {
		t.Fatalf("LoadHeartbeat of a run without one = %t, %v; want not found and no error", found, err)
	}
	beat := Heartbeat{Seq: 7, At: "2026-11-17T00:00:00.000000001Z", Host: "login1", PID: 42, PIDNamespace: "pid:[1]"}
	if err := store.WriteHeartbeat(runDir, beat); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.LoadHeartbeat(runDir)
	if err != nil || !found || got != beat {
		t.Fatalf("LoadHeartbeat = %+v, %t, %v; want %+v", got, found, err, beat)
	}
	path := filepath.Join(runDir, HeartbeatFileName)
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.LoadHeartbeat(runDir); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("LoadHeartbeat of a damaged heartbeat = %v, want an error naming %s", err, path)
	}
}
