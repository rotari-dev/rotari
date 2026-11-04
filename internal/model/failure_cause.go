package model

import (
	"slices"
	"strings"
)

// FailureCauseDiagnosis is the kind of a failure cause that a saved rule
// diagnosis names; the other causes are FailureKind* values.
const FailureCauseDiagnosis = "diagnosis"

// FailureCause returns the one cause a failed result is grouped under, as
// its kind and name, and empty strings for a success. Causes rotari records
// itself (blocked, cancelled, timeout) come first because they are certain;
// then the latest saved rule diagnosis, named by its rule; then the result's
// first remaining failure kind. Failure summaries group jobs by it and
// --filter-failure-cause selects by it.
//
// The cause timeout is only rotari's own, which records TimeoutExitCode and a
// timed-out error. Any other result of kind timeout, such as a scheduler's time
// limit, a scheduler command that timed out, or a job that exited 124 by itself
// as GNU timeout does, was not stopped by rotari, so its suggestion to raise
// rotari's timeout would not apply.
func FailureCause(result JobResult) (kind, name string) {
	kinds := FailureKinds(result)
	if len(kinds) == 0 {
		return "", ""
	}
	for _, recorded := range []string{FailureKindBlocked, FailureKindCancelled} {
		if slices.Contains(kinds, recorded) {
			return recorded, recorded
		}
	}
	if result.ExitCode == TimeoutExitCode && isTimedOutError(result.Error) {
		return FailureKindTimeout, FailureKindTimeout
	}
	if len(result.Diagnoses) > 0 {
		return FailureCauseDiagnosis, result.Diagnoses[0].Name
	}
	for _, kind := range kinds {
		if kind != FailureKindTimeout {
			return kind, kind
		}
	}
	return FailureKindError, FailureKindError
}

// isTimedOutError reports whether a result's error text is the one a job stopped
// by rotari's timeout records.
func isTimedOutError(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "timed out after ")
}

// FailureCauseID returns the selector ID of a failure cause or diagnosis
// name: lowercase letters and digits, with each other run of characters
// replaced by one hyphen. A failure kind is its own ID.
func FailureCauseID(name string) string {
	var builder strings.Builder
	separator := false
	for _, character := range strings.ToLower(name) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(character)
			separator = false
			continue
		}
		separator = true
	}
	return builder.String()
}
