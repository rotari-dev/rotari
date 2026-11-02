// Package runwatch finds runs that stopped without finishing, such as runs
// whose coordinator was killed, and records them for the Web inbox. A run's
// own process cannot report that it was killed; rotari web looks for such
// runs instead.
package runwatch

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/rotari-dev/rotari/internal/joblist"
	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/notification"
	"github.com/rotari-dev/rotari/internal/notificationlog"
	"github.com/rotari-dev/rotari/internal/project"
	"github.com/rotari-dev/rotari/internal/state"
)

// Interval is how often rotari web looks for interrupted runs.
const Interval = 30 * time.Second

// ReportInterrupted records a run.interrupted notification for each run in
// baseDirs that project.Inspect finds interrupted and that has none yet. A
// project whose state lock is held is looked at again on the next call. It
// returns how many runs it recorded; a project it cannot inspect or record
// is an error that does not stop the others.
func ReportInterrupted(log notificationlog.Log, baseDirs []string, now time.Time) (int, error) {
	recorded := 0
	var errs []error
	for _, baseDir := range baseDirs {
		projects, err := joblist.Projects(baseDir, "")
		if err != nil {
			errs = append(errs, fmt.Errorf("list projects in %s: %w", baseDir, err))
			continue
		}
		for _, name := range projects {
			paths, err := state.ResolveProjectPaths(baseDir, name)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			run, interrupted, err := interruptedRun(paths)
			if err != nil {
				errs = append(errs, fmt.Errorf("inspect project %s in %s: %w", name, baseDir, err))
				continue
			}
			if !interrupted {
				continue
			}
			added, err := log.AppendOnce(baseDir, notification.NewInterruptedRunEvent(name, run, now))
			if err != nil {
				errs = append(errs, fmt.Errorf("record interrupted run %s: %w", run.RunID, err))
				continue
			}
			if added {
				recorded++
			}
		}
	}
	return recorded, errors.Join(errs...)
}

// interruptedRun reports the project's interrupted run, with its name and
// start time when its lock or partial summary recorded them. It inspects the
// project under the shared state lock, so a run that is starting or
// finishing is never mistaken for an interrupted one.
func interruptedRun(paths state.ProjectPaths) (model.RunSummary, bool, error) {
	release, ok, err := state.TryAcquireStateReadLock(paths.StateLockFile)
	if err != nil || !ok {
		return model.RunSummary{}, false, err
	}
	defer release()
	// Inspect removes a stale lock, so read it first.
	lock, lockErr := state.LoadLock(paths.LockFile)
	inspection, err := project.Inspect(paths)
	if err != nil || inspection.State != project.Interrupted {
		return model.RunSummary{}, false, err
	}
	run := model.RunSummary{RunID: inspection.RunID}
	if lockErr == nil && lock.RunID == run.RunID {
		run.RunName, run.StartedAt = lock.RunName, lock.StartedAt
	} else if runDir, err := state.SafeJoin(paths.RunsDir, run.RunID); err == nil {
		if summary, err := state.LoadRunSummaryHead(filepath.Join(runDir, "summary.json")); err == nil {
			run.RunName, run.StartedAt = summary.RunName, summary.StartedAt
		}
	}
	return run, true, nil
}
