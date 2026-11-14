package executor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCancelWatchdogHandlesSignalBeforeTrap(t *testing.T) {
	var commandReady, commandStopping, commandRelease, timerReady, timerRelease, timerSignalled, graceStarted string
	cmd, jobDir := startCancelTestWrapperWithScript(t, func(jobDir string) string {
		commandReady = filepath.Join(jobDir, "command-ready")
		commandStopping = filepath.Join(jobDir, "command-stopping")
		commandRelease = filepath.Join(jobDir, "command-release")
		timerReady = filepath.Join(jobDir, "timer-ready")
		timerRelease = filepath.Join(jobDir, "timer-release")
		timerSignalled = filepath.Join(jobDir, "timer-signalled")
		graceStarted = filepath.Join(jobDir, "grace-started")
		command := []string{"sh", "-c",
			`trap ': > "$2"; while [ ! -f "$3" ]; do sleep 0.01; done; exit 143' TERM; : > "$1"; while :; do sleep 0.01; done`,
			"sh", commandReady, commandStopping, commandRelease}
		script := StatusWrapperScript(command, jobDir, nil, "", "")
		replace := func(before, after string) {
			t.Helper()
			script = replaceCancelWatchdogStep(t, script, before, after)
		}

		// Hold the timer before its USR1 trap, so the wrapper's stop signal is
		// lost exactly as it can be when the command exits immediately.
		timerStart := "            trap '' TERM INT QUIT\n"
		replace(timerStart, timerStart+
			"            trap '' USR1\n"+
			"            : > "+ShellQuote(timerReady)+"\n"+
			"            while [ ! -f "+ShellQuote(timerRelease)+" ]; do sleep 0.01; done\n")
		signalTimer := "            kill -USR1 \"$cancel_killer\" 2>/dev/null || break\n"
		replace(signalTimer,
			"            while [ ! -f "+ShellQuote(timerReady)+" ]; do sleep 0.01; done\n"+
				signalTimer+
				"            : > "+ShellQuote(timerSignalled)+"\n")
		graceSleep := fmt.Sprintf("            sleep %d &\n", timeoutGraceSeconds)
		replace(graceSleep, "            : > "+ShellQuote(graceStarted)+"\n"+graceSleep)
		return script
	})

	waitForCancelWatchdogFile(t, commandReady)
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitForCancelWatchdogFile(t, timerReady)
	waitForCancelWatchdogFile(t, commandStopping)
	if err := os.WriteFile(commandRelease, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	waitForCancelWatchdogFile(t, timerSignalled)
	if err := os.WriteFile(timerRelease, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 143 {
				t.Fatalf("wrapper exited with %v, want the cancelled exit code 143", err)
			}
			status, err := os.ReadFile(filepath.Join(jobDir, "status.json"))
			if err != nil || !strings.Contains(string(status), `"phase":"cancelled"`) {
				t.Fatalf("status.json = %s, %v; want a cancelled phase", status, err)
			}
			return
		case <-ticker.C:
			if _, err := os.Stat(graceStarted); err == nil {
				t.Fatal("the watchdog began its grace sleep after the command had exited")
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("read watchdog marker: %v", err)
			}
		case <-deadline.C:
			t.Fatal("the wrapper did not finish after its command exited")
		}
	}
}

func TestCancelWatchdogRetriesLostStopSignal(t *testing.T) {
	for _, test := range []struct {
		name   string
		signal syscall.Signal
		code   int
	}{
		{"TERM", syscall.SIGTERM, 143},
		{"INT", syscall.SIGINT, 130},
		{"QUIT", syscall.SIGQUIT, 131},
	} {
		t.Run(test.name, func(t *testing.T) {
			testCancelWatchdogRetriesLostStopSignal(t, test.signal, test.code)
		})
	}
}

func testCancelWatchdogRetriesLostStopSignal(t *testing.T, signal syscall.Signal, code int) {
	t.Helper()
	var commandReady, commandStopping, commandRelease, timerReady, signalLost string
	cmd, jobDir := startCancelTestWrapperWithScript(t, func(jobDir string) string {
		commandReady = filepath.Join(jobDir, "command-ready")
		commandStopping = filepath.Join(jobDir, "command-stopping")
		commandRelease = filepath.Join(jobDir, "command-release")
		timerReady = filepath.Join(jobDir, "timer-ready")
		signalLost = filepath.Join(jobDir, "signal-lost")
		command := []string{"sh", "-c",
			`trap ': > "$2"; while [ ! -f "$3" ]; do sleep 0.01; done; exit 143' TERM; : > "$1"; while :; do sleep 0.01; done`,
			"sh", commandReady, commandStopping, commandRelease}
		script := StatusWrapperScript(command, jobDir, nil, "", "")
		graceSleep := fmt.Sprintf("            sleep %d &\n", timeoutGraceSeconds)
		script = replaceCancelWatchdogStep(t, script, graceSleep, "            : > "+ShellQuote(timerReady)+"\n"+graceSleep)
		signalTimer := `kill -USR1 "$cancel_killer" 2>/dev/null`
		// Drop the first stop signal after the timer's initial liveness check.
		script = replaceCancelWatchdogStep(t, script, signalTimer,
			`if [ -z "${test_signal_lost:-}" ]; then test_signal_lost=1; : > `+
				ShellQuote(signalLost)+`; else `+signalTimer+`; fi`)
		return script
	})

	waitForCancelWatchdogFile(t, commandReady)
	if err := syscall.Kill(cmd.Process.Pid, signal); err != nil {
		t.Fatal(err)
	}
	waitForCancelWatchdogFile(t, timerReady)
	waitForCancelWatchdogFile(t, commandStopping)
	if err := os.WriteFile(commandRelease, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	waitForCancelWatchdogFile(t, signalLost)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != code {
			t.Fatalf("wrapper exited with %v, want the cancelled exit code %d", err, code)
		}
		status, err := os.ReadFile(filepath.Join(jobDir, "status.json"))
		if err != nil || !strings.Contains(string(status), `"phase":"cancelled"`) {
			t.Fatalf("status.json = %s, %v; want a cancelled phase", status, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the wrapper did not stop after the watchdog's first stop signal was lost")
	}
}

func replaceCancelWatchdogStep(t *testing.T, script, before, after string) string {
	t.Helper()
	if count := strings.Count(script, before); count != 1 {
		t.Fatalf("wrapper contains %d copies of %q, want one", count, before)
	}
	return strings.Replace(script, before, after, 1)
}

func waitForCancelWatchdogFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read %s: %v", path, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
