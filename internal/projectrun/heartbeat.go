package projectrun

import (
	"os"
	"time"

	"github.com/rotari-dev/rotari/internal/state"
)

// startHeartbeat writes the run's heartbeat now and every interval until
// the returned function is called. A failed write does not stop the run; it
// is reported when writing starts to fail and again when it recovers, not on
// every beat.
func (runner Runner) startHeartbeat(paths state.ProjectPaths, runID string, interval time.Duration) func() {
	runDir, err := state.SafeJoin(paths.RunsDir, runID)
	if err != nil {
		runner.errorf("run %s writes no heartbeat: %v", runID, err)
		return func() {}
	}
	host, err := os.Hostname()
	if err != nil {
		runner.errorf("run %s heartbeat names no host: %v", runID, err)
	}
	beat := state.Heartbeat{Host: host, PID: os.Getpid(), PIDNamespace: state.PIDNamespace()}
	failing := false
	write := func() {
		beat.Seq++
		beat.At = runner.timestampNano()
		err := runner.Store.WriteHeartbeat(runDir, beat)
		switch {
		case err != nil && !failing:
			runner.errorf("run %s failed to write its heartbeat: %v", runID, err)
		case err == nil && failing:
			runner.errorf("run %s writes its heartbeat again", runID)
		}
		failing = err != nil
	}
	write()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				write()
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}
