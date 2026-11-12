package webui

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/rotari-dev/rotari/internal/resolve"
	"github.com/rotari-dev/rotari/internal/runview"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
	webprojection "github.com/rotari-dev/rotari/internal/web"
)

// terminalProjectsPath tells All shells what each project with shells is doing.
const terminalProjectsPath = terminalPrefix + "projects"

// terminalProjectResponse is a project with shells and what its last run is doing.
type terminalProjectResponse struct {
	BaseDir string `json:"basedir"`
	Project string `json:"project"`
	// Runs is how many runs the project has; with none, LastRun is absent.
	Runs    int                     `json:"runs"`
	LastRun *terminalLastRunSummary `json:"last_run,omitempty"`
	// StatusError is why the project's runs could not be read; nothing else is then set.
	StatusError string `json:"status_error,omitempty"`
}

// terminalLastRunSummary is a run as one line of All shells tells it.
type terminalLastRunSummary struct {
	RunID     string `json:"run_id"`
	RunName   string `json:"run_name,omitempty"`
	Lifecycle string `json:"lifecycle"`
	// Tone is the lifecycle's status tone, which colours the line as a pill would be.
	Tone string `json:"tone"`
	// StartedAt and FinishedAt are absent when the run's files do not say.
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	// Jobs counts the run's jobs by jobCountGroup, as the run page's Jobs card does.
	Jobs map[string]int `json:"jobs"`
}

// serveTerminalProjects lists, on GET, each project with shells and its last run.
func (s site) serveTerminalProjects(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projects := []terminalProjectResponse{}
	seen := map[string]bool{}
	for _, shell := range s.Terminal.Shells() {
		if seen[shell.Group] {
			continue
		}
		seen[shell.Group] = true
		baseDir, projectName := terminalGroupPlace(shell.Group)
		if projectName == "" {
			// No project, or a basedir's own shells, which have no runs.
			continue
		}
		entry := terminalProjectResponse{BaseDir: baseDir, Project: projectName}
		runs, last, err := s.terminalLastRun(baseDir, projectName)
		if err != nil {
			entry.StatusError = err.Error()
		} else {
			entry.Runs, entry.LastRun = runs, last
		}
		projects = append(projects, entry)
	}
	writeTerminalJSON(writer, http.StatusOK, map[string]any{"projects": projects, "now": time.Now()})
}

// terminalLastRun is how many runs a project has, and its last: the run `rotari show` shows
// when given none (resolve.RunID), which is the one going on while one is.
func (s site) terminalLastRun(baseDir, projectName string) (int, *terminalLastRunSummary, error) {
	paths, err := stateinternal.ResolveProjectPaths(baseDir, projectName)
	if err != nil {
		return 0, nil, err
	}
	ids, err := webRunIDs(paths)
	if err != nil || len(ids) == 0 {
		return 0, nil, err
	}
	runID, err := resolve.RunID(paths, "")
	if err != nil {
		return 0, nil, err
	}
	runDir, err := stateinternal.SafeJoin(paths.RunsDir, runID)
	if err != nil {
		return 0, nil, err
	}
	lifecycle, err := runview.RunLifecycleLabel(paths, runID)
	if err != nil {
		return 0, nil, err
	}
	jobs, err := webprojection.LoadRunJobs(s.Store, runDir, "")
	if err != nil {
		return 0, nil, err
	}
	last := &terminalLastRunSummary{RunID: runID, Lifecycle: lifecycle, Tone: statusTone(lifecycle), Jobs: map[string]int{}}
	for _, job := range jobs {
		last.Jobs[jobCountGroup(job.ExecutionStatus)]++
	}
	// A run that is still going may have no summary yet; its lock says when it started.
	started := ""
	if summary, summaryErr := stateinternal.LoadRunSummary(filepath.Join(runDir, "summary.json")); summaryErr == nil {
		last.RunName, started = summary.RunName, summary.StartedAt
		if lifecycle != "running" {
			last.FinishedAt = terminalTimestamp(summary.FinishedAt)
		}
	}
	if started == "" {
		if lock, lockErr := stateinternal.LoadLock(paths.LockFile); lockErr == nil && lock.RunID == runID {
			started = lock.StartedAt
		}
	}
	last.StartedAt = terminalTimestamp(started)
	return len(ids), last, nil
}

// terminalTimestamp reads a run file's time, or nil when it holds none.
func terminalTimestamp(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return &parsed
}
