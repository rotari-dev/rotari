// Package runwatch finds runs that stopped without finishing, such as runs
// whose coordinator was killed, and records them for the Web inbox. A run's
// own process cannot report that it was killed; rotari web looks for such
// runs instead. A run whose coordinator is on another host is judged by its
// heartbeat, which only suggests that it stopped.
package runwatch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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

// UnresponsiveAfter is how long a remote run's heartbeat must stay unchanged,
// by the watcher's own clock, before the run is reported unresponsive.
const UnresponsiveAfter = state.HeartbeatStaleAfter

// Watcher finds runs that stopped without finishing and records them for
// the Web inbox. A local run whose coordinator is dead is interrupted. An
// active run whose coordinator is on another host or in another PID
// namespace, which this host cannot check, is unresponsive once its
// heartbeat has not changed for UnresponsiveAfter. A Watcher remembers the
// heartbeats it saw between calls of Report, so one watcher must be kept for
// the life of the process; it is not safe for concurrent use.
type Watcher struct {
	seen map[runKey]observation
}

type runKey struct {
	baseDir, project, runID string
}

// observation is the heartbeat a watcher last saw change, and when it saw
// it, by its own clock.
type observation struct {
	seq    uint64
	seenAt time.Time
}

// NewWatcher returns a Watcher that has seen no heartbeat yet, so it calls
// no run unresponsive before UnresponsiveAfter has passed.
func NewWatcher() *Watcher {
	return &Watcher{seen: map[runKey]observation{}}
}

// Report records a run.interrupted notification for each run in baseDirs
// that project.Inspect finds interrupted, and a run.unresponsive one for
// each remote run whose heartbeat has stopped, unless the run already has
// one. A project whose state lock is held is looked at again on the next
// call. It returns how many notifications it recorded; a project it cannot
// inspect or record is an error that does not stop the others.
func (watcher *Watcher) Report(log notificationlog.Log, baseDirs []string, now time.Time) (int, error) {
	recorded := 0
	var errs []error
	active := map[runKey]bool{}
	for _, baseDir := range baseDirs {
		projects, err := joblist.Projects(baseDir, "")
		if errors.Is(err, state.ErrBaseDirMissing) {
			// A basedir that is gone has no runs to report, and the Web UI
			// marks it missing; one that has not run yet has none either.
			continue
		}
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
			found, err := inspectRun(paths)
			if err != nil {
				errs = append(errs, fmt.Errorf("inspect project %s in %s: %w", name, baseDir, err))
				continue
			}
			var event *notification.Event
			switch {
			case found.interrupted:
				interrupted := notification.NewInterruptedRunEvent(name, found.run, now)
				event = &interrupted
			case found.remote:
				key := runKey{baseDir, name, found.run.RunID}
				active[key] = true
				if silent, stale := watcher.observe(key, found.beat, now); stale {
					unresponsive := notification.NewUnresponsiveRunEvent(name, found.run, found.beat.Host, found.beat.At, silent, now)
					event = &unresponsive
				}
			}
			if event == nil {
				continue
			}
			added, err := log.AppendOnce(baseDir, *event)
			if err != nil {
				errs = append(errs, fmt.Errorf("record %s run %s: %w", event.RunStatus, event.RunID, err))
				continue
			}
			if added {
				recorded++
			}
		}
	}
	// Forget runs that are no longer active remote runs, so the map does not
	// grow without bound. A run whose project could not be inspected this
	// time is forgotten too, and waits UnresponsiveAfter again.
	for key := range watcher.seen {
		if !active[key] {
			delete(watcher.seen, key)
		}
	}
	return recorded, errors.Join(errs...)
}

// observe notes beat for key at now, and reports how long the beat has been
// unchanged and whether that reaches UnresponsiveAfter. A beat that changed,
// in either direction, since the last call starts the wait again.
func (watcher *Watcher) observe(key runKey, beat state.Heartbeat, now time.Time) (time.Duration, bool) {
	last, ok := watcher.seen[key]
	if !ok || last.seq != beat.Seq {
		watcher.seen[key] = observation{seq: beat.Seq, seenAt: now}
		return 0, false
	}
	silent := now.Sub(last.seenAt)
	return silent, silent >= UnresponsiveAfter
}

// foundRun is what inspectRun found about a project's run.
type foundRun struct {
	run model.RunSummary
	// interrupted is a run whose local coordinator is gone.
	interrupted bool
	// remote is an active run whose coordinator this host cannot check and
	// that writes a heartbeat, beat.
	remote bool
	beat   state.Heartbeat
}

// inspectRun reports the project's interrupted run, or its active remote run
// with the heartbeat it last wrote, with the run's name and start time when
// its lock or partial summary recorded them. It inspects the project under
// the shared state lock, so a run that is starting or finishing is never
// mistaken for an interrupted one. A project without a directory, such as
// the default project of a basedir that has no state yet or one deleted
// meanwhile, has no run. A remote run without a heartbeat, started by a
// rotari that wrote none, is not reported, since nothing can be told about
// it; one whose heartbeat cannot be read is an error.
func inspectRun(paths state.ProjectPaths) (foundRun, error) {
	release, ok, err := state.TryAcquireStateReadLock(paths.StateLockFile)
	if errors.Is(err, fs.ErrNotExist) {
		if _, statErr := os.Stat(paths.ProjectDir); errors.Is(statErr, fs.ErrNotExist) {
			return foundRun{}, nil
		}
	}
	if err != nil || !ok {
		return foundRun{}, err
	}
	defer release()
	// Inspect removes a stale lock, so read it first.
	lock, lockErr := state.LoadLock(paths.LockFile)
	inspection, err := project.Inspect(paths)
	if err != nil {
		return foundRun{}, err
	}
	var found foundRun
	switch {
	case inspection.State == project.Interrupted:
		found.interrupted = true
	case inspection.State == project.Running && inspection.Lock == state.LockRemote:
		runDir, err := state.SafeJoin(paths.RunsDir, inspection.RunID)
		if err != nil {
			return foundRun{}, err
		}
		beat, ok, err := state.NewStore(state.DirectoryMode(), state.FileMode()).LoadHeartbeat(runDir)
		if err != nil {
			return foundRun{}, fmt.Errorf("run %s: %w", inspection.RunID, err)
		}
		if !ok {
			return foundRun{}, nil
		}
		found.remote, found.beat = true, beat
	default:
		return foundRun{}, nil
	}
	found.run = model.RunSummary{RunID: inspection.RunID}
	if lockErr == nil && lock.RunID == found.run.RunID {
		found.run.RunName, found.run.StartedAt = lock.RunName, lock.StartedAt
	} else if runDir, err := state.SafeJoin(paths.RunsDir, found.run.RunID); err == nil {
		if summary, err := state.LoadRunSummaryHead(filepath.Join(runDir, "summary.json")); err == nil {
			found.run.RunName, found.run.StartedAt = summary.RunName, summary.StartedAt
		}
	}
	return found, nil
}
