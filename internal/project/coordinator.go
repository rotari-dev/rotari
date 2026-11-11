package project

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/state"
)

type CoordinatorState string

const (
	CoordinatorLive    CoordinatorState = "live"
	CoordinatorStopped CoordinatorState = "stopped"
	CoordinatorUnknown CoordinatorState = "unknown"
)

// CoordinatorAssessment describes process ownership, not job liveness or
// permission to unlock. A management completion marker never overrides a probe.
type CoordinatorAssessment struct {
	State      CoordinatorState      `json:"state"`
	Reason     string                `json:"reason"`
	ObservedAt string                `json:"observed_at"`
	Identity   *model.RunCoordinator `json:"identity,omitempty"`
}

// AssessCoordinator reads the durable run context, independently of running.lock.
// Missing legacy identity is unknown; malformed or unreadable records are errors.
func AssessCoordinator(store state.Store, runDir string) (CoordinatorAssessment, error) {
	result := CoordinatorAssessment{State: CoordinatorUnknown, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	context, err := state.LoadContext(store, runDir)
	if errors.Is(err, os.ErrNotExist) {
		result.Reason = "run context is missing"
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("load coordinator identity: %w", err)
	}
	result.Identity = context.Coordinator
	host, err := os.Hostname()
	if err != nil {
		return result, fmt.Errorf("identify inspection host: %w", err)
	}
	identity := context.Coordinator
	result.State, result.Reason = coordinatorLocation(identity, host, state.PIDNamespace())
	if result.Reason != "" {
		return result, nil
	}
	// Probe only identities in this host and namespace. Never signal an
	// unvalidated remote PID, even with signal zero.
	observation := state.ObserveProcess(coordinatorIdentity(identity))
	result.State, result.Reason = coordinatorProcess(observation)
	return result, nil
}

func coordinatorIdentity(identity *model.RunCoordinator) state.ProcessIdentity {
	return state.ProcessIdentity{Host: identity.Host, PIDNamespace: identity.PIDNamespace, PID: identity.PID, ProcessStart: identity.ProcessStart}
}

func coordinatorLocation(identity *model.RunCoordinator, host, namespace string) (CoordinatorState, string) {
	if identity == nil {
		return CoordinatorUnknown, "coordinator identity was not recorded"
	}
	if identity.Host == "" || identity.PID <= 0 {
		return CoordinatorUnknown, "coordinator identity is incomplete"
	}
	switch verdict, verified := coordinatorIdentity(identity).Locate(host, namespace); {
	case verdict == state.ProcessOtherHost:
		return CoordinatorUnknown, "coordinator belongs to another host"
	case verdict == state.ProcessOtherNamespace:
		return CoordinatorUnknown, "coordinator belongs to another PID namespace"
	case !verified:
		return CoordinatorUnknown, "coordinator PID namespace cannot be verified"
	default:
		return CoordinatorUnknown, ""
	}
}

// coordinatorProcess words the shared process verdict (state.ObserveProcess)
// for a coordinator whose location coordinatorLocation verified. Any probe
// error, EPERM included, leaves it unknown.
func coordinatorProcess(observation state.ProcessObservation) (CoordinatorState, string) {
	switch {
	case observation.Verdict == state.ProcessGone:
		return CoordinatorStopped, "coordinator process no longer exists"
	case observation.ProbeErr != nil:
		return CoordinatorUnknown, fmt.Sprintf("cannot inspect coordinator process: %v", observation.ProbeErr)
	case observation.Verdict == state.ProcessUnverifiable:
		return CoordinatorUnknown, "coordinator process start time cannot be verified; process may still be alive"
	case observation.Verdict == state.ProcessReused:
		return CoordinatorStopped, "PID belongs to a different process"
	case observation.Verdict == state.ProcessSame:
		return CoordinatorLive, "coordinator process identity matches"
	default:
		return CoordinatorUnknown, fmt.Sprintf("coordinator process is %s", observation.Verdict)
	}
}
