package jobstatus

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rotari-dev/rotari/internal/diagnose"
	"github.com/rotari-dev/rotari/internal/executor"
	"github.com/rotari-dev/rotari/internal/jobfilter"
	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/state"
)

// maxCarryDepth bounds how many carried origins FilterJob follows.
const maxCarryDepth = 16

// FilterJob returns what job filters need to know about the job that origin
// names under runsDir: result and finished as the caller resolved them, and
// the hosts, times, and log of the attempt that produced them. A job carried
// into origin's run is read from the run it was carried from. id is the job's
// ID for definition conditions. Hosts recorded on result take precedence over
// the attempt's, and times the carry records fill in ones the attempt lacks.
func FilterJob(store state.Store, runsDir string, origin model.JobOrigin, id string, result model.JobResult, finished bool, now time.Time) jobfilter.Job {
	// The attempt is located, and its files read, only for a filter that
	// asks for them.
	var located bool
	var attemptDir string
	var carries []model.JobOrigin
	locate := func() string {
		if !located {
			// Filters cannot report an error; an attempt whose carry
			// cannot be read reads as not found, like its other files.
			attemptDir, carries, _ = locateAttempt(runsDir, origin, 0)
			located = true
		}
		return attemptDir
	}
	return jobfilter.Job{
		ID: id, Result: result, Finished: finished,
		LoadAttributes: func() jobfilter.Attributes {
			attributes := AttemptAttributes(store, locate(), now)
			if len(result.Hosts) > 0 {
				attributes.Hosts = result.Hosts
			}
			for _, carry := range append([]model.JobOrigin{origin}, carries...) {
				if attributes.StartedAt.IsZero() {
					attributes.StartedAt, _ = jobfilter.ParseTimestamp(carry.SubmittedAt)
				}
				if attributes.FinishedAt.IsZero() {
					attributes.FinishedAt, _ = jobfilter.ParseTimestamp(carry.FinishedAt)
				}
			}
			return attributes
		},
		Diagnosis: func(selectors []string) bool {
			log, err := ReadLog(locate())
			return err == nil && diagnose.MatchesResultSelectors(selectors, result, log)
		},
	}
}

// AttemptAttributes reads the hosts and times an attempt directory records:
// the wrapper's start, or the submission when it has not started.
func AttemptAttributes(store state.Store, attemptDir string, now time.Time) jobfilter.Attributes {
	attributes := jobfilter.Attributes{Now: now}
	if attemptDir == "" {
		return attributes
	}
	if path, err := state.ValidatedStateFile(attemptDir, "status.json"); err == nil {
		if status, ok := executor.LoadWrapperStatus(store, path); ok {
			attributes.Hosts = status.Hosts
			attributes.StartedAt, _ = jobfilter.ParseTimestamp(status.StartedAt)
		}
	}
	if attributes.StartedAt.IsZero() {
		attributes.StartedAt, _ = jobfilter.ParseTimestamp(state.ResolveAttemptTimestamp(attemptDir, "submitted_at"))
	}
	attributes.FinishedAt, _ = jobfilter.ParseTimestamp(state.ResolveAttemptTimestamp(attemptDir, "finished_at"))
	return attributes
}

// ResultAttemptDir returns the directory of the attempt that produced the
// result origin names: that attempt, or for a job carried into origin's run,
// the attempt it was carried from, as FilterJob reads it. It is "" when no
// such directory exists, and an error when a carry cannot be read.
func ResultAttemptDir(runsDir string, origin model.JobOrigin) (string, error) {
	dir, _, err := locateAttempt(runsDir, origin, 0)
	return dir, err
}

// ReadLog returns the log of an attempt for diagnosis: its merged output, or
// else its stdout, or else its stderr.
func ReadLog(attemptDir string) (string, error) {
	if attemptDir == "" {
		return "", os.ErrNotExist
	}
	for _, name := range []string{"output", state.StdoutFileName, state.StderrFileName} {
		data, err := os.ReadFile(filepath.Join(attemptDir, name))
		if err == nil {
			return string(data), nil
		}
	}
	return "", os.ErrNotExist
}

// locateAttempt returns the directory of the attempt origin names, following
// the origins of carried jobs, and the carries it followed; "" when none is
// found, and an error when a run's commands.json, which records a carry,
// cannot be read.
func locateAttempt(runsDir string, origin model.JobOrigin, depth int) (string, []model.JobOrigin, error) {
	if origin.AttemptID != "" {
		if payload, err := state.DecodeAttemptID(origin.AttemptID); err == nil {
			if runDir, err := state.SafeJoin(runsDir, payload.RunID); err == nil {
				if dir, err := state.SpecificAttemptJobDir(runDir, payload.JobID, origin.AttemptID); err == nil && isDir(dir) {
					return dir, nil, nil
				}
			}
		}
	}
	runDir, err := state.SafeJoin(runsDir, origin.RunID)
	if err != nil {
		return "", nil, nil
	}
	if dir, err := state.LatestAttemptJobDir(runDir, origin.JobID); err == nil && isDir(dir) {
		return dir, nil, nil
	}
	if !isDir(runDir) {
		// The run was deleted: its attempt is gone, which is not an error.
		return "", nil, nil
	}
	carried, err := state.LoadRunOrigin(runDir, origin.JobID)
	if err != nil {
		return "", nil, fmt.Errorf("run %q: %w", origin.RunID, err)
	}
	if carried == nil || depth >= maxCarryDepth {
		return "", nil, nil
	}
	dir, carries, err := locateAttempt(runsDir, *carried, depth+1)
	return dir, append([]model.JobOrigin{*carried}, carries...), err
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
