package model

import (
	"slices"
	"testing"
)

func TestFailureKindsReadOOMAsAWord(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{text: "OOM", want: true},
		{text: "job hit OOM on node01", want: true},
		{text: "oom-kill event in step 0", want: true},
		{text: "container OOMKilled", want: true},
		{text: "oom_killer invoked", want: true},
		{text: "out of memory", want: true},
		{text: "out_of_memory", want: true},
		{text: "exceeded memory limit", want: true},
		{text: "no room left in the queue"},
		{text: "zoom client failed"},
		{text: "bloom filter is corrupt"},
	}
	for _, test := range tests {
		kinds := FailureKinds(JobResult{ExitCode: 1, Error: test.text})
		if got := slices.Contains(kinds, FailureKindOOM); got != test.want {
			t.Errorf("FailureKinds(%q) = %v, oom = %v, want %v", test.text, kinds, got, test.want)
		}
	}
}

// A saved diagnosis adds the failure kinds its rule implies, in kind order and
// once each, so an OOM that only the log showed is kind oom and not error. A
// diagnosis saved before rules declared kinds adds none.
func TestFailureKindsAddSavedDiagnosisKinds(t *testing.T) {
	oom := RuleDiagnosis{Name: "Python memory error", Kinds: []string{FailureKindOOM}}
	for _, test := range []struct {
		name   string
		result JobResult
		want   []string
	}{
		{name: "log-only memory error", result: JobResult{ExitCode: 1, Diagnoses: []RuleDiagnosis{oom}}, want: []string{FailureKindOOM}},
		{name: "with a signal", result: JobResult{ExitCode: 137, Diagnoses: []RuleDiagnosis{oom}}, want: []string{FailureKindOOM, FailureKindSignal}},
		{name: "error text and diagnosis agree", result: JobResult{ExitCode: 1, Error: "out of memory", Diagnoses: []RuleDiagnosis{oom, oom}}, want: []string{FailureKindOOM}},
		{name: "timeout before oom", result: JobResult{ExitCode: TimeoutExitCode, Diagnoses: []RuleDiagnosis{oom}}, want: []string{FailureKindTimeout, FailureKindOOM}},
		{name: "saved before kinds", result: JobResult{ExitCode: 1, Diagnoses: []RuleDiagnosis{{Name: "Python memory error"}}}, want: []string{FailureKindError}},
		{name: "a success has none", result: JobResult{Diagnoses: []RuleDiagnosis{oom}}, want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := FailureKinds(test.result); !slices.Equal(got, test.want) {
				t.Fatalf("FailureKinds = %q, want %q", got, test.want)
			}
		})
	}
}
