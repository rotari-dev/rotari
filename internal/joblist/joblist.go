package joblist

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rotari-dev/rotari/internal/jobstatus"
	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/project"
	"github.com/rotari-dev/rotari/internal/runlineage"
	"github.com/rotari-dev/rotari/internal/state"
)

const (
	// DefaultSince is the listing window when none is given.
	DefaultSince = 24 * time.Hour
	// DefaultSinceText is DefaultSince as users write it.
	DefaultSinceText = "1d"
)

// Interval is a span of finish times from Start up to, but not including,
// End. A zero End leaves the span open.
type Interval struct {
	Start time.Time
	End   time.Time
}

func (interval Interval) contains(value time.Time) bool {
	return !value.Before(interval.Start) && (interval.End.IsZero() || value.Before(interval.End))
}

// intervals is the set of spans a finished job must fall in to be listed.
type intervals []Interval

func (spans intervals) contains(value time.Time) bool {
	for _, span := range spans {
		if span.contains(value) {
			return true
		}
	}
	return false
}

// latestEnd is where the last span ends, or zero when a span is open.
func (spans intervals) latestEnd() time.Time {
	var latest time.Time
	for _, span := range spans {
		if span.End.IsZero() {
			return time.Time{}
		}
		if span.End.After(latest) {
			latest = span.End
		}
	}
	return latest
}

// earliest is the first moment any span covers; with no spans nothing
// finished is listed, so it is now.
func (spans intervals) earliest(now time.Time) time.Time {
	earliest := now
	for index, span := range spans {
		if index == 0 || span.Start.Before(earliest) {
			earliest = span.Start
		}
	}
	return earliest
}

// Row is one job in the listing: an unfinished attempt of an active or
// interrupted run, a job of such a run that has not started (with no
// AttemptID), or a job that finished inside the window.
type Row struct {
	State       string
	BaseDir     string
	Project     string
	RunID       string
	JobID       string
	AttemptID   string
	JobName     string
	Command     string
	FullCommand string
	StartedAt   time.Time
	FinishedAt  time.Time
	jobstatus.RunTime
}

// ParseSince parses a listing window: a Go duration such as "24h" or "90m",
// or a whole number of days such as "7d", which Go durations lack. Empty
// means DefaultSince.
func ParseSince(value string) (time.Duration, error) {
	if value == "" {
		return DefaultSince, nil
	}
	if days, ok := strings.CutSuffix(value, "d"); ok {
		count, err := strconv.Atoi(days)
		if err != nil || count < 0 {
			return 0, fmt.Errorf("invalid duration")
		}
		return time.Duration(count) * 24 * time.Hour, nil
	}
	window, err := time.ParseDuration(value)
	if err != nil || window < 0 {
		return 0, fmt.Errorf("invalid duration")
	}
	return window, nil
}

func sortTime(row Row) time.Time {
	if !row.FinishedAt.IsZero() {
		return row.FinishedAt
	}
	return row.StartedAt
}

// Sort orders rows by finish time, or start time for unfinished jobs,
// newest first. A row without either, such as a pending job, is listed right
// after the newest row of its run, so a run's jobs stay together.
func Sort(rows []Row) {
	runKey := func(row Row) string { return row.BaseDir + "\x00" + row.Project + "\x00" + row.RunID }
	newest := make(map[string]time.Time)
	for _, row := range rows {
		if at := sortTime(row); at.After(newest[runKey(row)]) {
			newest[runKey(row)] = at
		}
	}
	sort.SliceStable(rows, func(left, right int) bool {
		leftAt, rightAt := sortTime(rows[left]), sortTime(rows[right])
		leftOwn, rightOwn := !leftAt.IsZero(), !rightAt.IsZero()
		if !leftOwn {
			leftAt = newest[runKey(rows[left])]
		}
		if !rightOwn {
			rightAt = newest[runKey(rows[right])]
		}
		if !leftAt.Equal(rightAt) {
			return leftAt.After(rightAt)
		}
		return leftOwn && !rightOwn
	})
}

// Projects returns requested, or every project in baseDir sorted by name, or
// the default project when baseDir has none.
func Projects(baseDir, requested string) ([]string, error) {
	if requested != "" {
		if !state.IsValidPathElement(requested) {
			return nil, fmt.Errorf("invalid project name %q", requested)
		}
		return []string{requested}, nil
	}
	entries, err := os.ReadDir(filepath.Join(baseDir, "projects"))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{state.DefaultProjectName}, nil
		}
		return nil, fmt.Errorf("failed to read projects: %w", err)
	}
	projects := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && state.IsValidPathElement(entry.Name()) {
			projects = append(projects, entry.Name())
		}
	}
	sort.Strings(projects)
	return projects, nil
}

// Collect lists unfinished attempts and jobs of projects in baseDir that
// finished within window before now. Each job's status follows the
// internal/jobstatus fallback chain.
func Collect(store state.Store, baseDir string, projects []string, now time.Time, window time.Duration) ([]Row, error) {
	return CollectFinishedIn(store, baseDir, projects, now, []Interval{{Start: now.Add(-window)}})
}

// CollectFinishedIn is Collect for jobs that finished inside any of spans
// rather than one window before now. Unfinished jobs are listed regardless.
func CollectFinishedIn(store state.Store, baseDir string, projects []string, now time.Time, spans []Interval) ([]Row, error) {
	finished := intervals(spans)
	rows := make([]Row, 0)
	for _, projectName := range projects {
		paths, err := state.ResolveProjectPaths(baseDir, projectName)
		if err != nil {
			return nil, err
		}
		runEntries, err := os.ReadDir(paths.RunsDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read runs for project %q: %w", projectName, err)
		}
		runs := make([]os.DirEntry, 0, len(runEntries))
		for _, entry := range runEntries {
			if entry.IsDir() && state.IsValidPathElement(entry.Name()) {
				runs = append(runs, entry)
			}
		}
		sort.SliceStable(runs, func(left, right int) bool { return runs[left].Name() > runs[right].Name() })
		activeRunID := ""
		if lockState, lock, err := project.InspectRunLock(paths); err == nil && (lockState == state.LockActive || lockState == state.LockRemote) {
			activeRunID = lock.RunID
		}
		type runRows struct {
			rows    []Row
			include bool
			err     error
		}
		collected := make([]runRows, len(runs))
		state.ReadEach(len(runs), func(index int) {
			name := runs[index].Name()
			rows, include, _, err := collectRun(store, paths, name, name == activeRunID, now, finished)
			collected[index] = runRows{rows: rows, include: include, err: err}
		})
		// An older interrupted run can retain an unfinished attempt even
		// after recovery and a later completed run, so every run is read.
		for _, run := range collected {
			if run.err != nil {
				return nil, run.err
			}
			if run.include {
				rows = append(rows, run.rows...)
			}
		}
	}
	return rows, nil
}

func collectRun(store state.Store, paths state.ProjectPaths, runID string, active bool, now time.Time, finished intervals) ([]Row, bool, bool, error) {
	runDir := filepath.Join(paths.RunsDir, runID)
	cutoff := finished.earliest(now)
	// startedAfterSpans is set for a finished run that started after every
	// span ended. A job it ran and recorded a result for finished later
	// still and is skipped unread; jobs carried from an earlier run keep
	// their origin's times, and jobs without a result may be unfinished,
	// so those are still read.
	startedAfterSpans := false
	if !active {
		// A run that finished before every span is decided from its
		// summary's head, before its other files are read.
		head, err := state.LoadRunSummaryHead(filepath.Join(runDir, "summary.json"))
		if errors.Is(err, state.ErrNewerStateVersion) {
			return nil, false, false, err
		}
		if finishedAt, parseErr := parseTimestamp(head.FinishedAt); err == nil && parseErr == nil {
			if finishedAt.Before(cutoff) {
				return nil, false, true, nil
			}
			end := finished.latestEnd()
			if startedAt, startErr := parseTimestamp(head.StartedAt); startErr == nil && !end.IsZero() && !startedAt.Before(end) {
				startedAfterSpans = true
			}
		}
	}
	run, err := state.LoadRunSnapshot(runDir)
	if err != nil {
		return nil, false, false, err
	}
	summary := run.Summary
	runQueue, err := run.Queue()
	if err != nil {
		return nil, false, false, nil
	}
	resultByID := jobstatus.RecordedResults(run)
	rows := make([]Row, 0)
	for _, job := range model.QueueToJobs(runQueue.Commands) {
		origin := runQueue.OriginOf(job.ID)
		summaryResult, hasSummary := resultByID[job.ID]
		if startedAfterSpans && origin == nil && hasSummary {
			continue
		}
		latestAttemptID, jobDir, err := state.LatestAttempt(runDir, job.ID)
		if err != nil {
			continue
		}
		resolved := jobstatus.ReadJob(store, jobDir, summaryResult, hasSummary)
		statusOK := resolved.Finished()
		displayStatus := resolved.DisplayStatus(model.JobSpec{ID: job.ID, Command: job.Command})
		carried := runlineage.IsCarried(origin, latestAttemptID, resolved.Blocked(), hasSummary)
		jobState := jobstatus.DisplayLabel(displayStatus, resolved.Accepted(), carried)
		pending := displayStatus == jobstatus.StatusPending
		submittedText, finishedText := jobstatus.Timestamps(runDir, jobDir, origin, carried)
		startedAt, err := parseTimestamp(submittedText)
		if err != nil && resolved.Attempt.HasWrapper && resolved.Attempt.Wrapper.StartedAt != "" {
			startedAt, err = parseTimestamp(resolved.Attempt.Wrapper.StartedAt)
		}
		if err != nil && summary.StartedAt != "" && !pending {
			startedAt, err = parseTimestamp(summary.StartedAt)
		}
		if err != nil && statusOK {
			continue
		}
		finishedAt, finishedErr := parseTimestamp(finishedText)
		if finishedErr != nil && summary.FinishedAt != "" && statusOK {
			finishedAt, finishedErr = parseTimestamp(summary.FinishedAt)
		}
		if !statusOK {
			finishedAt = time.Time{}
		}
		if statusOK {
			if finishedErr == nil {
				if !finished.contains(finishedAt) {
					continue
				}
			} else if !finished.contains(startedAt) {
				continue
			}
		}
		attemptID := latestAttemptID
		if attemptID == "" {
			if result, ok := resultByID[job.ID]; ok {
				attemptID = result.AttemptID
			}
		}
		if attemptID == "" && !pending {
			continue
		}
		jobName := state.ReadJobName(jobDir)
		if jobName == "" {
			jobName = job.Name
		}
		if jobName == "" {
			jobName = "-"
		}
		fullCommand := strings.Join(job.Command, " ")
		command := ShortenText(fullCommand, 40)
		finished := finishedAt
		if finishedErr != nil {
			finished = time.Time{}
		}
		runTime := jobstatus.MeasureRunTime(jobDir, startedAt, finished, statusOK, now)
		rows = append(rows, Row{State: jobState, BaseDir: paths.BaseDir, Project: paths.ProjectName, RunID: runID, JobID: job.ID, AttemptID: attemptID, JobName: jobName, Command: command, FullCommand: fullCommand, StartedAt: startedAt, FinishedAt: finishedAt, RunTime: runTime})
	}
	return rows, true, false, nil
}

// ShortenText cuts value to maxLength bytes, ending with "..." when
// maxLength leaves room for it.
func ShortenText(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	if maxLength <= 3 {
		return value[:maxLength]
	}
	return value[:maxLength-3] + "..."
}

func parseTimestamp(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
}

// FormatTimestamp formats a listing time in time.Local with the zone name,
// matching the other web pages.
func FormatTimestamp(value time.Time) string {
	return value.In(time.Local).Format("2006-01-02 15:04:05 MST")
}

// FormatLocalTimestamp is FormatTimestamp without the time zone.
func FormatLocalTimestamp(value time.Time) string {
	return value.In(time.Local).Format("2006-01-02 15:04:05")
}

// FormatRunTime formats a job's elapsed time and, while it runs, how long
// ago it last wrote output: "12m 03s, quiet 11m 58s", or "12m 03s, no
// output" when it has written none.
func FormatRunTime(runTime jobstatus.RunTime) string {
	elapsed := FormatElapsed(runTime.Elapsed)
	if !runTime.Running {
		return elapsed
	}
	if runTime.Quiet < 0 {
		return elapsed + ", no output"
	}
	return elapsed + ", quiet " + FormatElapsed(runTime.Quiet)
}

// FormatElapsed formats a duration as "42s", "3m 05s", or "2h 07m", and a
// negative one as "-".
func FormatElapsed(value time.Duration) string {
	if value < 0 {
		return "-"
	}
	seconds := int64(value / time.Second)
	if seconds < 60 {
		return strconv.FormatInt(seconds, 10) + "s"
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds%60)
	}
	hours := minutes / 60
	return fmt.Sprintf("%dh %02dm", hours, minutes%60)
}
