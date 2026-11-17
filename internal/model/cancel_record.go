package model

// CancelRecord is one line of a run's cancels.jsonl: one cancel request, as
// the process that handled it saw it. It records no command lines or
// environment.
type CancelRecord struct {
	// At is when the request was handled, with nanoseconds.
	At string `json:"at"`
	// Source is who asked: cli, wait, disconnect, web, or mcp.
	Source       string `json:"source"`
	PID          int    `json:"pid"`
	Host         string `json:"host,omitempty"`
	PIDNamespace string `json:"pid_namespace,omitempty"`
	// RunWide is a request to cancel the whole run; otherwise Requested
	// lists the job or attempt IDs asked for.
	RunWide   bool     `json:"run_wide"`
	Requested []string `json:"requested,omitempty"`
	// Targets are the jobs the request acted on, in the order it acted.
	Targets []CancelTarget `json:"targets,omitempty"`
	// Error is why the request failed, when it did.
	Error string `json:"error,omitempty"`
}

// CancelTarget is what one cancel request did to one job.
type CancelTarget struct {
	JobID string `json:"job_id"`
	// Action is CancelByExecutor, CancelByMarker, or CancelByMarkerAndExecutor.
	Action string `json:"action,omitempty"`
	// Error is why acting on this job failed; the request stopped there.
	Error string `json:"error,omitempty"`
}

// Cancel actions a CancelTarget records.
const (
	// CancelByExecutor cancelled a submitted job through its executor.
	CancelByExecutor = "executor"
	// CancelByMarker wrote the job's cancelled marker before it was
	// submitted, so the run skips it.
	CancelByMarker = "marker"
	// CancelByMarkerAndExecutor wrote the marker and then found the job
	// submitted meanwhile, and cancelled it through its executor too.
	CancelByMarkerAndExecutor = "marker+executor"
)
