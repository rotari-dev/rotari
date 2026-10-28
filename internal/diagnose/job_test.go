package diagnose

import "testing"

func TestTailLog(t *testing.T) {
	if got := TailLog("short", 10); got != "short" {
		t.Errorf("TailLog(short) = %q", got)
	}
	if got := TailLog("0123456789", 4); got != "[earlier log output omitted]\n6789" {
		t.Errorf("TailLog(long) = %q", got)
	}
}

func TestLastOutputLine(t *testing.T) {
	tests := []struct {
		name, log, want string
	}{
		{name: "empty", log: "", want: ""},
		{name: "trailing blank lines", log: "step 1\nloss became NaN\n\n  \n", want: "loss became NaN"},
		{name: "ANSI escapes and spaces", log: "\x1b[31m  loss became NaN \x1b[0m\n", want: "loss became NaN"},
		{name: "carriage return overwrites", log: "epoch 1: 10%\repoch 1: 50%\r\n", want: "epoch 1: 50%"},
		{name: "separate streams end with stderr", log: "--- stdout ---\nstep 1\n\n--- stderr ---\nloss became NaN\n", want: "loss became NaN"},
		{name: "empty stderr after its header", log: "--- stdout ---\nstep 1\n--- stderr ---\n", want: "step 1"},
		{name: "only the tail marker", log: TailLog("01", 1)[:len(omittedLogMarker)+1], want: ""},
	}
	for _, test := range tests {
		if got := LastOutputLine(test.log); got != test.want {
			t.Errorf("%s: LastOutputLine(%q) = %q, want %q", test.name, test.log, got, test.want)
		}
	}
}

func TestDiagnoseDefaultMatchesSchedulerErrorAndLog(t *testing.T) {
	diagnoses := DiagnoseDefault(Job{
		Error: "CUDA error: out of memory",
		Log:   "Traceback (most recent call last):\nValueError: invalid batch size",
	})
	if len(diagnoses) != 2 {
		t.Fatalf("DiagnoseDefault() returned %d diagnoses: %#v", len(diagnoses), diagnoses)
	}
	if diagnoses[0].Name != "CUDA/GPU memory exhausted" {
		t.Errorf("first diagnosis = %#v", diagnoses[0])
	}
	if diagnoses[1].Name != "Python type or value error" || diagnoses[1].Evidence != "ValueError: invalid batch size" {
		t.Errorf("second diagnosis = %#v", diagnoses[1])
	}
}

func TestDefaultRulesReturnsIndependentSlice(t *testing.T) {
	rules := DefaultRules()
	rules[0].Name = "changed"
	if DefaultRules()[0].Name == "changed" {
		t.Fatal("DefaultRules() returned a slice backed by mutable package state")
	}
}
