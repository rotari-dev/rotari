package coordination

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

// unshareArgs runs a command in a new PID namespace, with a user namespace
// so that no privilege is needed, on the same host.
var unshareArgs = []string{"unshare", "--user", "--map-root-user", "--pid", "--fork", "--mount-proc"}

// TestControlFromAnotherPIDNamespace checks that controlling a local job
// from another PID namespace on its host, such as a sandbox or container,
// fails naming both namespaces, without recording a cancel or signalling
// anything, and that the job still reads as running there (COORD-1).
func TestControlFromAnotherPIDNamespace(t *testing.T) {
	covers(t, "COORD-1")
	if out, err := exec.Command(unshareArgs[0], append(unshareArgs[1:], "true")...).CombinedOutput(); err != nil {
		t.Skipf("cannot create a PID namespace here (%v: %s); COORD-1's namespace case was verified by hand, see the shared-rule-violations work log", err, bytes.TrimSpace(out))
	}
	here, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		t.Skipf("PID namespace unavailable: %v", err)
	}
	e := support.NewEnv(t)
	run := e.StartRun("live", 1, false)
	job := run.Jobs[0]
	support.WaitUntil(t, 15*time.Second, func() (bool, string) {
		return support.JobProcesses(t, e.Root, job) == 1, "the job did not start"
	})
	inside := func(args ...string) support.Result {
		t.Helper()
		cmd := e.Command(args...)
		wrapped := exec.Command(unshareArgs[0], append(append(unshareArgs[1:], cmd.Path), args...)...)
		wrapped.Env, wrapped.Dir = cmd.Env, cmd.Dir
		var stdout, stderr bytes.Buffer
		wrapped.Stdout, wrapped.Stderr = &stdout, &stderr
		code := 0
		if err := wrapped.Run(); err != nil {
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			code = exitErr.ExitCode()
		}
		return support.Result{Args: args, Code: code, Stdout: stdout.String(), Stderr: stderr.String()}
	}
	for _, command := range []string{"suspend", "resume", "cancel"} {
		got := inside(command, "-p", "live", job)
		if got.Code == 0 || !strings.Contains(got.Stderr, "runs in PID namespace "+here+", not pid:[") {
			t.Errorf("%s from another PID namespace: %s; want a refusal naming %s and the namespace it ran in", command, got, here)
		}
	}
	attempts, err := filepath.Glob(filepath.Join(e.Base, "projects", "live", "runs", run.RunID, job, "attempts", "*"))
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempts of %s: %v %v", job, attempts, err)
	}
	if _, err := os.Stat(filepath.Join(attempts[0], "cancelled")); err == nil {
		t.Errorf("a refused cancel recorded a cancellation in %s", attempts[0])
	}
	if alive := support.JobProcesses(t, e.Root, job); alive != 1 {
		t.Fatalf("a refused request stopped the job: %d processes", alive)
	}
	if got := inside("jobs", "-p", "live", "--format", "%a %s"); !strings.Contains(got.Stdout, " running") {
		t.Errorf("the job must read as running from another PID namespace: %s", got)
	}
	// From its own namespace the job is still controlled as before.
	e.MustRotari("suspend", "-p", "live", job)
	e.MustRotari("resume", "-p", "live", job)
}
