package executor

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/rotari-dev/rotari/internal/state"
)

// ErrJobNotRunning reports a job directory that no executor owns.
var ErrJobNotRunning = errors.New("job is not running")

// Registry maps executor names to executors.
type Registry map[string]JobExecutor

// NewRegistry returns the built-in executors. logf receives job submission
// and completion lines.
func NewRegistry(store state.Store, logf func(string, ...any)) Registry {
	return Registry{
		"local": NewLocal(store, logf),
		"slurm": NewSlurm(store, logf),
		"pbs":   NewPBS(store, logf),
		"lsf":   NewLSF(store, logf),
		"sge":   NewSGE(store, logf),
		"ssh":   NewSSH(store),
	}
}

// Lookup returns the executor registered under name.
func (registry Registry) Lookup(name string) (JobExecutor, bool) {
	executor, ok := registry[name]
	return executor, ok
}

// Known reports whether name is a registered executor.
func (registry Registry) Known(name string) bool {
	_, ok := registry[name]
	return ok
}

// Names returns the registered executor names, sorted for stable output.
func (registry Registry) Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Owner determines which executor owns the job recorded in jobDir, based on
// the metadata each executor writes on submission. Every scheduler-style
// executor writes job.json with its own "executor" name, so job.json alone
// (not its mere existence) names the owner. New local jobs do too; legacy
// local jobs have a pid file.
func (registry Registry) Owner(store state.Store, jobDir string) (JobExecutor, error) {
	if path, err := state.ValidatedStateFile(jobDir, "job.json"); err == nil { // NOSONAR: jobDir is restricted to validated job-path boundaries
		var meta struct {
			Executor string `json:"executor"`
		}
		if err := store.ReadJSON(path, &meta); err == nil {
			if executor, ok := registry.Lookup(meta.Executor); ok {
				return executor, nil
			}
		}
	}
	if path, err := state.ValidatedStateFile(jobDir, "pid"); err == nil {
		// codeql[go/path-injection]: path is returned by ValidatedStateFile.
		if _, err := os.Stat(path); err == nil {
			if executor, ok := registry.Lookup("local"); ok {
				return executor, nil
			}
		}
	}
	return nil, ErrJobNotRunning
}

// LocalElsewhere returns a *state.ElsewhereError when executor is the local
// executor and the job in jobDir runs on another host or in another PID
// namespace (COORD-1). The local executor signals jobs by PID, which is only
// meaningful in the host and namespace that spawned the process; a failed
// PID check there would misleadingly report "job is not running", so
// callers surface this before trying. The run's recorded host
// (context.json) and the job's own record (job.json, which attempts from an
// earlier rotari lack) must both place the job here.
func LocalElsewhere(store state.Store, executor JobExecutor, runDir, jobDir string) error {
	if executor.Name() != "local" {
		return nil
	}
	safeRunDir := filepath.Join(filepath.Dir(runDir), filepath.Base(runDir))
	if context, err := state.LoadContext(store, safeRunDir); err == nil {
		run := state.ProcessIdentity{Host: context.Hostname}
		if err := state.Elsewhere(run, state.LocateProcess(run)); err != nil {
			return err
		}
	}
	// A job without a PID yet is still placed by its record.
	identity, _ := LocalJobIdentity(store, jobDir)
	return state.Elsewhere(identity, state.LocateProcess(identity))
}
