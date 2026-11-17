package jobstatus

import (
	"errors"
	"fmt"
	"io/fs"
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
//
// A fact that was never recorded is left out; one recorded in a file that
// cannot be read or decoded, including the carry that locates the attempt,
// is an error naming the job and the file.
func FilterJob(store state.Store, runsDir string, origin model.JobOrigin, id string, result model.JobResult, finished bool, now time.Time) jobfilter.Job {
	// The attempt is located, and its files read, only for a filter that
	// asks for them.
	var located bool
	var attemptDir string
	var carries []model.JobOrigin
	var locateErr error
	locate := func() (string, error) {
		if !located {
			attemptDir, carries, locateErr = locateAttempt(runsDir, origin, 0)
			located = true
		}
		return attemptDir, locateErr
	}
	fail := func(err error) error {
		return fmt.Errorf("job %s of run %s: %w", id, origin.RunID, err)
	}
	return jobfilter.Job{
		ID: id, Result: result, Finished: finished,
		LoadAttributes: func() (jobfilter.Attributes, error) {
			dir, err := locate()
			if err != nil {
				return jobfilter.Attributes{}, fail(err)
			}
			attributes, err := AttemptAttributes(store, dir, now)
			if err != nil {
				return jobfilter.Attributes{}, fail(err)
			}
			if len(result.Hosts) > 0 {
				attributes.Hosts = result.Hosts
			}
			for _, carry := range append([]model.JobOrigin{origin}, carries...) {
				if attributes.StartedAt.IsZero() {
					if attributes.StartedAt, err = parseRecordedTime(carry.SubmittedAt); err != nil {
						return jobfilter.Attributes{}, fail(fmt.Errorf("carry from run %s: submitted_at: %w", carry.RunID, err))
					}
				}
				if attributes.FinishedAt.IsZero() {
					if attributes.FinishedAt, err = parseRecordedTime(carry.FinishedAt); err != nil {
						return jobfilter.Attributes{}, fail(fmt.Errorf("carry from run %s: finished_at: %w", carry.RunID, err))
					}
				}
			}
			return attributes, nil
		},
		Diagnosis: func(selectors []string) (bool, error) {
			if result.ExitCode == 0 {
				// Only a failed job has a diagnosis; its log is not read.
				return false, nil
			}
			dir, err := locate()
			if err != nil {
				return false, fail(err)
			}
			log, found, err := ReadLog(dir)
			if err != nil {
				return false, fail(err)
			}
			return found && diagnose.MatchesResultSelectors(selectors, result, log), nil
		},
	}
}

// AttemptAttributes reads the hosts and times an attempt directory records:
// the wrapper's start, or the submission when it has not started. A file
// that does not exist records nothing; one that cannot be read or decoded is
// an error naming it.
func AttemptAttributes(store state.Store, attemptDir string, now time.Time) (jobfilter.Attributes, error) {
	attributes := jobfilter.Attributes{Now: now}
	if attemptDir == "" {
		return attributes, nil
	}
	path, err := state.ValidatedStateFile(attemptDir, "status.json")
	if err != nil {
		return attributes, err
	}
	var status executor.WrapperStatus
	switch err := store.ReadJSON(path, &status); {
	case errors.Is(err, fs.ErrNotExist):
	case errors.Is(err, state.ErrInvalidJSON):
		return attributes, fmt.Errorf("%s: %w", path, err)
	case err != nil:
		return attributes, err
	default:
		attributes.Hosts = status.Hosts
		if attributes.StartedAt, err = parseRecordedTime(status.StartedAt); err != nil {
			return attributes, fmt.Errorf("%s: started_at: %w", path, err)
		}
	}
	if attributes.StartedAt.IsZero() {
		if attributes.StartedAt, err = loadAttemptTime(attemptDir, "submitted_at"); err != nil {
			return attributes, err
		}
	}
	if attributes.FinishedAt, err = loadAttemptTime(attemptDir, "finished_at"); err != nil {
		return attributes, err
	}
	return attributes, nil
}

// loadAttemptTime reads one of an attempt's recorded times; zero when none
// is recorded.
func loadAttemptTime(attemptDir, name string) (time.Time, error) {
	value, path, err := state.LoadAttemptTimestamp(attemptDir, name)
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := parseRecordedTime(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %w", path, err)
	}
	return parsed, nil
}

// parseRecordedTime parses a time rotari recorded: zero for "", which
// records nothing, and an error for anything else that does not parse.
func parseRecordedTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return jobfilter.ParseTimestamp(value)
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
// else its stdout, or else its stderr. found is false when the attempt has
// none of them; a log file that exists but cannot be read is an error.
func ReadLog(attemptDir string) (log string, found bool, err error) {
	if attemptDir == "" {
		return "", false, nil
	}
	for _, name := range []string{"output", state.StdoutFileName, state.StderrFileName} {
		data, err := os.ReadFile(filepath.Join(attemptDir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	}
	return "", false, nil
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
