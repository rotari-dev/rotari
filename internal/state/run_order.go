package state

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// runIDTimeLayout is the start second a run ID begins with: rotari names a
// run YYYYMMDD-HHMMSS-XXXXXXXX, its UTC start and then a random tail.
const runIDTimeLayout = "20060102-150405"

// runIDLength is the length of a run ID rotari makes.
const runIDLength = len(runIDTimeLayout) + len("-00000000")

// NewRunID makes the ID of a run that starts at now.
func NewRunID(now time.Time) string {
	var value [4]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("failed to generate run id: %v", err))
	}
	return fmt.Sprintf("%s-%08x", now.UTC().Format(runIDTimeLayout), value)
}

// RunIDStart is the start second a run ID records. ok is false for an ID in
// another form.
func RunIDStart(runID string) (time.Time, bool) {
	if len(runID) < len(runIDTimeLayout) {
		return time.Time{}, false
	}
	started, err := time.Parse(runIDTimeLayout, runID[:len(runIDTimeLayout)])
	return started, err == nil
}

// RunsNewestFirst lists the runs of a project, newest first. This is the one
// run order:
//
//   - The newest run is meta.json's last run when it names a run directory.
//   - The others are ordered by the start second in the run ID. Runs that
//     share a second are ordered by the coordinator's recorded_at in their
//     context.json, read only for those runs; a run without it sorts by its
//     ID among them, after the runs that have it.
//   - Runs whose ID records no start are the oldest, ordered by ID.
//
// A project without a runs directory has no runs.
func RunsNewestFirst(paths ProjectPaths) ([]string, error) {
	meta, err := LoadMeta(paths.MetaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}
	entries, err := os.ReadDir(paths.RunsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read runs: %w", err)
	}
	last := ""
	runIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !IsValidPathElement(entry.Name()) {
			continue
		}
		if entry.Name() == meta.LastRunID {
			last = entry.Name()
			continue
		}
		runIDs = append(runIDs, entry.Name())
	}
	if err := sortRunsNewestFirst(paths.RunsDir, runIDs); err != nil {
		return nil, err
	}
	if last != "" {
		runIDs = append([]string{last}, runIDs...)
	}
	return runIDs, nil
}

// NewestRunID is the first run of RunsNewestFirst, empty when the project
// has none.
func NewestRunID(paths ProjectPaths) (string, error) {
	// meta.json's last run, when it names a run directory, is first in the
	// order; finding it needs no listing.
	meta, err := LoadMeta(paths.MetaFile)
	if err != nil {
		return "", fmt.Errorf("failed to load metadata: %w", err)
	}
	if IsValidPathElement(meta.LastRunID) {
		// codeql[go/path-injection]: LastRunID is validated by IsValidPathElement.
		if info, err := os.Stat(filepath.Join(paths.RunsDir, meta.LastRunID)); err == nil && info.IsDir() {
			return meta.LastRunID, nil
		}
	}
	runIDs, err := RunsNewestFirst(paths)
	if err != nil || len(runIDs) == 0 {
		return "", err
	}
	return runIDs[0], nil
}

type runOrderKey struct {
	runID    string
	start    time.Time
	hasStart bool
	// recorded is the coordinator's recorded_at, read only for a run that
	// shares its start second with another.
	recorded    time.Time
	hasRecorded bool
}

func sortRunsNewestFirst(runsDir string, runIDs []string) error {
	keys := make([]runOrderKey, len(runIDs))
	seconds := make(map[time.Time]int, len(runIDs))
	for index, runID := range runIDs {
		keys[index].runID = runID
		keys[index].start, keys[index].hasStart = RunIDStart(runID)
		if keys[index].hasStart {
			seconds[keys[index].start]++
		}
	}
	for index := range keys {
		key := &keys[index]
		if !key.hasStart || seconds[key.start] < 2 {
			continue
		}
		recorded, ok, err := coordinatorRecordedAt(filepath.Join(runsDir, key.runID))
		if err != nil {
			return fmt.Errorf("failed to order run %q: %w", key.runID, err)
		}
		key.recorded, key.hasRecorded = recorded, ok
	}
	sort.SliceStable(keys, func(i, j int) bool { return newerRun(keys[i], keys[j]) })
	for index := range keys {
		runIDs[index] = keys[index].runID
	}
	return nil
}

func newerRun(left, right runOrderKey) bool {
	if left.hasStart != right.hasStart {
		return left.hasStart
	}
	if left.hasStart && !left.start.Equal(right.start) {
		return left.start.After(right.start)
	}
	if left.hasRecorded != right.hasRecorded {
		return left.hasRecorded
	}
	if left.hasRecorded && !left.recorded.Equal(right.recorded) {
		return left.recorded.After(right.recorded)
	}
	return left.runID > right.runID
}

// coordinatorRecordedAt reads when the run's coordinator recorded itself;
// ok is false for a run whose context, from an earlier rotari, has none.
func coordinatorRecordedAt(runDir string) (time.Time, bool, error) {
	context, err := LoadContext(NewStore(DirectoryMode(), FileMode()), runDir)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	if context.Coordinator == nil || context.Coordinator.RecordedAt == "" {
		return time.Time{}, false, nil
	}
	recorded, err := time.Parse(time.RFC3339Nano, context.Coordinator.RecordedAt)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid coordinator recorded_at %q in %s: %w", context.Coordinator.RecordedAt, ContextPath(runDir), err)
	}
	return recorded, true, nil
}
