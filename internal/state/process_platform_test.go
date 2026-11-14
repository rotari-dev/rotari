package state

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// processIdentityPlatforms are those where rotari verifies a recorded process
// by its PID namespace and start time; CI runs both.
var processIdentityPlatforms = map[string]bool{"linux": true, "darwin": true}

func TestPIDNamespaceIsKnownWhereProcessesAreVerified(t *testing.T) {
	if !processIdentityPlatforms[runtime.GOOS] {
		t.Skipf("rotari does not verify recorded processes on %s", runtime.GOOS)
	}
	namespace := PIDNamespace()
	if namespace == "" {
		t.Fatal("PIDNamespace is empty, so no recorded process can be located")
	}
	if runtime.GOOS == "darwin" && namespace != NoPIDNamespaces {
		t.Fatalf("PIDNamespace = %q on macOS, want %q", namespace, NoPIDNamespaces)
	}
	if PIDNamespace() != namespace {
		t.Fatal("PIDNamespace changed between calls")
	}
}

// TestProcessStartTellsProcessesApart checks what the reuse check rests on: a
// running process's start time is known and does not change while it runs, a
// process started later has another, and a process that is gone has none. A
// start time that drifted would make a live coordinator read as reused.
func TestProcessStartTellsProcessesApart(t *testing.T) {
	if !processIdentityPlatforms[runtime.GOOS] {
		t.Skipf("rotari does not verify recorded processes on %s", runtime.GOOS)
	}
	own := ProcessStart(os.Getpid())
	if own == "" {
		t.Fatal("this process has no start time")
	}
	// Linux counts in clock ticks (usually 10 ms), so wait past one.
	time.Sleep(50 * time.Millisecond)
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	pid := child.Process.Pid
	defer func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	}()
	childStart := ProcessStart(pid)
	if childStart == "" {
		t.Fatalf("child %d has no start time", pid)
	}
	if childStart == own {
		t.Fatalf("child %d has this process's start time %q", pid, own)
	}
	for range 5 {
		time.Sleep(20 * time.Millisecond)
		if again := ProcessStart(pid); again != childStart {
			t.Fatalf("child %d start time changed from %q to %q while it ran", pid, childStart, again)
		}
	}
	if again := ProcessStart(os.Getpid()); again != own {
		t.Fatalf("this process's start time changed from %q to %q", own, again)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	if gone := ProcessStart(pid); gone != "" {
		t.Fatalf("reaped child %d still has start time %q", pid, gone)
	}
	if none := ProcessStart(0); none != "" {
		t.Fatalf("PID 0 has start time %q", none)
	}
}
