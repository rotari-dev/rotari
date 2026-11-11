package state

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestProcessLocation(t *testing.T) {
	for _, test := range []struct {
		name     string
		identity ProcessIdentity
		want     ProcessVerdict
		verified bool
	}{
		{"same host and namespace", ProcessIdentity{Host: "Here", PIDNamespace: "ns"}, ProcessHere, true},
		{"other host", ProcessIdentity{Host: "there", PIDNamespace: "ns"}, ProcessOtherHost, false},
		{"other namespace", ProcessIdentity{Host: "here", PIDNamespace: "other"}, ProcessOtherNamespace, false},
		{"no namespace, from an earlier rotari", ProcessIdentity{Host: "here"}, ProcessHere, false},
		{"no host, from an earlier rotari", ProcessIdentity{}, ProcessHere, false},
		{"namespace only", ProcessIdentity{PIDNamespace: "other"}, ProcessOtherNamespace, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, verified := test.identity.Locate("here", "ns")
			if got != test.want || verified != test.verified {
				t.Fatalf("Locate = %s, %t; want %s, %t", got, verified, test.want, test.verified)
			}
		})
	}
	if got, verified := (ProcessIdentity{Host: "here", PIDNamespace: "ns"}).Locate("here", ""); got != ProcessHere || verified {
		t.Fatalf("without a namespace here nothing is verified: %s, %t", got, verified)
	}
}

func TestClassifyProcess(t *testing.T) {
	for _, test := range []struct {
		name, recorded, current string
		probeErr                error
		want                    ProcessVerdict
		probeErrKept            bool
	}{
		{"same", "1", "1", nil, ProcessSame, false},
		{"same, not ours", "1", "1", syscall.EPERM, ProcessSame, true},
		{"reused", "1", "2", nil, ProcessReused, false},
		{"gone", "1", "", syscall.ESRCH, ProcessGone, false},
		{"start unknown", "", "1", nil, ProcessUnverifiable, false},
		{"leader gone, group left", "1", "", nil, ProcessUnverifiable, false},
		{"probe failed", "1", "1", syscall.EIO, ProcessUnverifiable, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := ProbeOutcome(test.recorded, test.current, test.probeErr)
			if got.Verdict != test.want || (got.ProbeErr != nil) != test.probeErrKept {
				t.Fatalf("ProbeOutcome = %+v, want %s (probe error kept %t)", got, test.want, test.probeErrKept)
			}
		})
	}
}

func TestObserveProcess(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	self := ProcessIdentity{Host: host, PIDNamespace: PIDNamespace(), PID: os.Getpid(), ProcessStart: ProcessStart(os.Getpid())}
	if self.ProcessStart == "" || self.PIDNamespace == "" {
		t.Skip("process start or PID namespace is not available on this platform")
	}
	if got := ObserveProcess(self); got.Verdict != ProcessSame || !got.LocationVerified {
		t.Fatalf("this process = %+v", got)
	}
	reused := self
	reused.ProcessStart = "0"
	if got := ObserveProcess(reused).Verdict; got != ProcessReused {
		t.Fatalf("a reused PID = %s", got)
	}
	legacy := ProcessIdentity{PID: os.Getpid()}
	if got := ObserveProcess(legacy); got.Verdict != ProcessUnverifiable || got.LocationVerified {
		t.Fatalf("a record from an earlier rotari is probed as before: %+v", got)
	}
	// A PID in another namespace is never probed: whatever it names here,
	// the verdict is the namespace.
	other := self
	other.PIDNamespace = "pid:[1]"
	got := ObserveProcess(other)
	if got.Verdict != ProcessOtherNamespace {
		t.Fatalf("another namespace = %+v", got)
	}
	var elsewhere *ElsewhereError
	if err := Elsewhere(other, got); !errors.As(err, &elsewhere) || !strings.Contains(err.Error(), "pid:[1]") || !strings.Contains(err.Error(), self.PIDNamespace) {
		t.Fatalf("Elsewhere = %v; want both namespaces", err)
	}
	if err := Elsewhere(self, ObserveProcess(self)); err != nil {
		t.Fatalf("this process is not elsewhere: %v", err)
	}
	if got := ObserveProcess(ProcessIdentity{Host: "elsewhere.invalid", PID: os.Getpid()}).Verdict; got != ProcessOtherHost {
		t.Fatalf("another host = %s", got)
	}
}
