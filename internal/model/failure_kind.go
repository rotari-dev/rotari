package model

import (
	"regexp"
	"strings"
)

// TimeoutExitCode is the exit code recorded for a job stopped by its timeout,
// matching GNU timeout.
const TimeoutExitCode = 124

// Failure kinds that --filter-failure-kind selects.
const (
	FailureKindTimeout   = "timeout"
	FailureKindCancelled = "cancelled"
	FailureKindBlocked   = "blocked"
	FailureKindOOM       = "oom"
	FailureKindSignal    = "signal"
	FailureKindError     = "error"
)

// oomWord matches OOM as a word, or the OOM killer's forms such as oom-kill and
// OOMKilled, and not words that contain the letters, such as room or zoom.
var oomWord = regexp.MustCompile(`\boom(?:[-_ ]?kill(?:ed|er)?)?\b`)

var failureKinds = []string{FailureKindTimeout, FailureKindCancelled, FailureKindBlocked, FailureKindOOM, FailureKindSignal, FailureKindError}

// FailureKindValues returns the failure kinds in the order the CLI lists them.
func FailureKindValues() []string {
	return append([]string(nil), failureKinds...)
}

// FailureKinds returns the kinds a failed result falls into, in
// FailureKindValues order: those its recorded error and exit code show, and
// those its saved diagnoses' rules imply, such as oom for a memory rule that
// matched only the log; error when no other applies; none for a success.
func FailureKinds(result JobResult) []string {
	if result.ExitCode == 0 {
		return nil
	}
	found := map[string]bool{}
	text := strings.ToLower(strings.TrimSpace(result.Error))
	if result.ExitCode == TimeoutExitCode || strings.Contains(text, "timed out") || strings.Contains(text, "timeout") {
		found[FailureKindTimeout] = true
	}
	if IsCancelledError(result.Error) {
		found[FailureKindCancelled] = true
	}
	if strings.HasPrefix(text, "blocked") {
		found[FailureKindBlocked] = true
	}
	if strings.Contains(text, "out of memory") || strings.Contains(text, "out_of_memory") || oomWord.MatchString(text) || strings.Contains(text, "memory limit") {
		found[FailureKindOOM] = true
	}
	if result.ExitCode > 128 {
		found[FailureKindSignal] = true
	}
	for _, diagnosis := range result.Diagnoses {
		for _, kind := range diagnosis.Kinds {
			found[kind] = true
		}
	}
	kinds := make([]string, 0, len(found)+1)
	for _, kind := range failureKinds {
		if found[kind] && kind != FailureKindError {
			kinds = append(kinds, kind)
		}
	}
	if len(kinds) == 0 {
		kinds = append(kinds, FailureKindError)
	}
	return kinds
}
