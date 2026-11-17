package runview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/state"
)

func TestHeartbeatWarningOnlyForAStaleRemoteRun(t *testing.T) {
	now := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	localHost, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	stale := state.HeartbeatStaleAfter + state.HeartbeatClockMargin
	for _, test := range []struct {
		name string
		// lockHost is "" for no lock.
		lockHost, lockRun string
		// beat is the heartbeat file's content; "" for none.
		beat string
		want string
	}{
		{name: "no lock", beat: beatAt(now.Add(-time.Hour))},
		{name: "local coordinator", lockHost: localHost, lockRun: "run-1", beat: beatAt(now.Add(-time.Hour))},
		{name: "another run's lock", lockHost: "login2", lockRun: "run-2", beat: beatAt(now.Add(-time.Hour))},
		{name: "no heartbeat", lockHost: "login2", lockRun: "run-1"},
		{name: "recent beat", lockHost: "login2", lockRun: "run-1", beat: beatAt(now.Add(-time.Minute))},
		{name: "just within the margin", lockHost: "login2", lockRun: "run-1", beat: beatAt(now.Add(-stale + time.Second))},
		{name: "stale beat", lockHost: "login2", lockRun: "run-1", beat: beatAt(now.Add(-stale - 90*time.Second)), want: "no heartbeat from beat-host for 7m"},
		{name: "damaged heartbeat", lockHost: "login2", lockRun: "run-1", beat: "{", want: "heartbeat unreadable: "},
		{name: "unparsable time", lockHost: "login2", lockRun: "run-1", beat: `{"seq":1,"at":"soon"}`, want: `heartbeat unreadable: heartbeat.json: time "soon"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			paths, err := state.ResolveProjectPaths(t.TempDir(), "p")
			if err != nil {
				t.Fatal(err)
			}
			runDir := filepath.Join(paths.RunsDir, "run-1")
			if err := os.MkdirAll(runDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if test.lockHost != "" {
				lock := model.LockInfo{PID: 1 << 30, RunID: test.lockRun, Host: test.lockHost, PIDNamespace: state.PIDNamespace()}
				if err := state.WriteJSON(paths.LockFile, lock); err != nil {
					t.Fatal(err)
				}
			}
			if test.beat != "" {
				if err := os.WriteFile(filepath.Join(runDir, state.HeartbeatFileName), []byte(test.beat), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := HeartbeatWarning(paths, "run-1", now)
			if test.want == "" && got != "" || test.want != "" && !strings.HasPrefix(got, test.want) {
				t.Fatalf("HeartbeatWarning = %q, want %q", got, test.want)
			}
		})
	}
}

func beatAt(at time.Time) string {
	return `{"seq":3,"at":"` + at.Format(time.RFC3339Nano) + `","host":"beat-host"}`
}
