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
