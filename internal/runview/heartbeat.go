package runview

import (
	"fmt"
	"time"

	"github.com/rotari-dev/rotari/internal/state"
)

// HeartbeatWarning says when runID is the project's active run, its
// coordinator is on another host or in another PID namespace that this host
// cannot check, and its heartbeat looks stopped: "no heartbeat from HOST for
// 12m", or why the heartbeat cannot be read. It is "" for any other run, for
// a run without a heartbeat, and for one that beat recently.
//
// It compares the beat's own time with now, so it waits
// state.HeartbeatClockMargin longer than runwatch, which watches the beat
// change by its own clock. A warning proves nothing about the run; it only
// says what the heartbeat shows.
func HeartbeatWarning(paths state.ProjectPaths, runID string, now time.Time) string {
	lockState, lock, err := state.InspectLock(paths.LockFile)
	if err != nil || lockState != state.LockRemote || lock.RunID != runID {
		// A lock that cannot be read is reported by the run's lifecycle.
		return ""
	}
	runDir, err := state.SafeJoin(paths.RunsDir, runID)
	if err != nil {
		return "heartbeat unreadable: " + err.Error()
	}
	beat, found, err := state.NewStore(state.DirectoryMode(), state.FileMode()).LoadHeartbeat(runDir)
	if err != nil {
		return "heartbeat unreadable: " + err.Error()
	}
	if !found {
		return ""
	}
	at, err := time.Parse(time.RFC3339Nano, beat.At)
	if err != nil {
		return fmt.Sprintf("heartbeat unreadable: %s: time %q", state.HeartbeatFileName, beat.At)
	}
	silent := now.Sub(at)
	if silent < state.HeartbeatStaleAfter+state.HeartbeatClockMargin {
		return ""
	}
	host := beat.Host
	if host == "" {
		host = lock.Host
	}
	return fmt.Sprintf("no heartbeat from %s for %dm", host, int(silent/time.Minute))
}
