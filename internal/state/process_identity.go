package state

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

// ProcessIdentity is a recorded process: the host and PID namespace it ran
// in, its PID there, and its start time (ProcessStart). A PID names a process
// only within its host and namespace, and the start time tells it from a
// later process that reuses the PID. Records from an earlier rotari may lack
// the namespace, the start, or even the host.
type ProcessIdentity struct {
	Host         string
	PIDNamespace string
	PID          int
	ProcessStart string
}

// ProcessVerdict is what can be said, from here, about a recorded process.
type ProcessVerdict string

const (
	// ProcessHere is a location verdict: the record does not place the
	// process on another host or in another namespace.
	ProcessHere ProcessVerdict = "here"
	// ProcessSame is the recorded process, still running.
	ProcessSame ProcessVerdict = "same"
	// ProcessGone is no process with the PID.
	ProcessGone ProcessVerdict = "gone"
	// ProcessReused is another process that now has the PID.
	ProcessReused ProcessVerdict = "reused"
	// ProcessOtherHost and ProcessOtherNamespace are processes whose PID
	// means nothing here; they must not be probed or signalled.
	ProcessOtherHost      ProcessVerdict = "other host"
	ProcessOtherNamespace ProcessVerdict = "other namespace"
	// ProcessUnverifiable is a process with the PID whose identity cannot be
	// checked, or a PID that cannot be probed.
	ProcessUnverifiable ProcessVerdict = "unverifiable"
)

// ProcessObservation is the one answer to whether a recorded process can be
// observed from here, used by local job control and liveness, client
// attachments, the coordinator check, and the execution probe.
type ProcessObservation struct {
	Verdict ProcessVerdict
	// LocationVerified is true when the record names this host and PID
	// namespace and this platform exposes its own namespace. A record without
	// them, from an earlier rotari, is probed as if it were here, as before;
	// callers that require a verified location check this.
	LocationVerified bool
	// ProbeErr is the signal-zero probe's error when it was neither nil nor
	// ESRCH, such as EPERM, for a process that exists but is not ours.
	ProbeErr error
	// Host and PIDNamespace are this process's, for messages.
	Host         string
	PIDNamespace string
}

// Locate places identity relative to host and namespace without probing:
// ProcessOtherHost, ProcessOtherNamespace, or ProcessHere, and whether the
// record names both so that ProcessHere is verified.
func (identity ProcessIdentity) Locate(host, namespace string) (ProcessVerdict, bool) {
	if identity.Host != "" && !strings.EqualFold(identity.Host, host) {
		return ProcessOtherHost, false
	}
	if identity.PIDNamespace != "" && namespace != "" && identity.PIDNamespace != namespace {
		return ProcessOtherNamespace, false
	}
	return ProcessHere, identity.Host != "" && identity.PIDNamespace != "" && namespace != ""
}

// LocateProcess is Locate from this process's host and PID namespace.
func LocateProcess(identity ProcessIdentity) ProcessObservation {
	host, err := os.Hostname()
	if err != nil {
		return ProcessObservation{Verdict: ProcessUnverifiable, ProbeErr: fmt.Errorf("identify this host: %w", err), PIDNamespace: PIDNamespace()}
	}
	observation := ProcessObservation{Host: host, PIDNamespace: PIDNamespace()}
	observation.Verdict, observation.LocationVerified = identity.Locate(host, observation.PIDNamespace)
	return observation
}

// ObserveProcess says whether the recorded process is the one running here
// under its PID.
func ObserveProcess(identity ProcessIdentity) ProcessObservation {
	return observe(identity, identity.PID)
}

// ObserveProcessGroup says the same of the process group the recorded
// process leads, which outlives its leader while its other members run.
func ObserveProcessGroup(identity ProcessIdentity) ProcessObservation {
	return observe(identity, -identity.PID)
}

func observe(identity ProcessIdentity, target int) ProcessObservation {
	observation := LocateProcess(identity)
	if observation.Verdict != ProcessHere {
		return observation
	}
	if identity.PID <= 1 {
		observation.Verdict = ProcessUnverifiable
		observation.ProbeErr = fmt.Errorf("recorded PID %d cannot be probed", identity.PID)
		return observation
	}
	outcome := ProbeOutcome(identity.ProcessStart, ProcessStart(identity.PID), syscall.Kill(target, 0))
	observation.Verdict, observation.ProbeErr = outcome.Verdict, outcome.ProbeErr
	return observation
}

// ProbeOutcome is the verdict and probe error of a process probed here; see
// ClassifyProcess.
func ProbeOutcome(recordedStart, currentStart string, probeErr error) ProcessObservation {
	outcome := ProcessObservation{Verdict: ClassifyProcess(recordedStart, currentStart, probeErr)}
	if probeErr != nil && !errors.Is(probeErr, syscall.ESRCH) {
		outcome.ProbeErr = probeErr
	}
	return outcome
}

// ClassifyProcess decides a probed process from the signal-zero probe's
// error and the recorded and current start times of its PID. EPERM means
// the process exists; a start time unknown on either side cannot be
// checked.
func ClassifyProcess(recordedStart, currentStart string, probeErr error) ProcessVerdict {
	switch {
	case errors.Is(probeErr, syscall.ESRCH):
		return ProcessGone
	case probeErr != nil && !errors.Is(probeErr, syscall.EPERM):
		return ProcessUnverifiable
	case recordedStart == "" || currentStart == "":
		return ProcessUnverifiable
	case recordedStart != currentStart:
		return ProcessReused
	default:
		return ProcessSame
	}
}

// ElsewhereError is a refusal to control a process that runs on another
// host or in another PID namespace, where its PID names another process or
// none (COORD-1).
type ElsewhereError struct {
	Identity    ProcessIdentity
	Observation ProcessObservation
}

func (err *ElsewhereError) Error() string {
	if err.Observation.Verdict == ProcessOtherHost {
		return fmt.Sprintf("runs on host %q", err.Identity.Host)
	}
	return fmt.Sprintf("runs in PID namespace %s, not %s", err.Identity.PIDNamespace, err.Observation.PIDNamespace)
}

// Elsewhere returns an *ElsewhereError when observation places the process
// on another host or in another namespace, and nil otherwise.
func Elsewhere(identity ProcessIdentity, observation ProcessObservation) error {
	switch observation.Verdict {
	case ProcessOtherHost, ProcessOtherNamespace:
		return &ElsewhereError{Identity: identity, Observation: observation}
	}
	return nil
}
