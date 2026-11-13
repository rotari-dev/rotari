package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ourShell is the activity of shell among ServerShells of this process, which started it.
func ourShell(t *testing.T, shell *startedShell) (ShellActivity, bool) {
	t.Helper()
	shells, err := ServerShells(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	for _, activity := range shells {
		if activity.PID == shell.command.Process.Pid {
			return activity, true
		}
	}
	return ShellActivity{}, false
}

func awaitActivity(t *testing.T, shell *startedShell, what string, ok func(ShellActivity) bool) ShellActivity {
	t.Helper()
	for deadline := time.Now().Add(testWait); ; time.Sleep(20 * time.Millisecond) {
		activity, found := ourShell(t, shell)
		if found && ok(activity) {
			return activity
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: last seen %+v (found %v)\n%s", what, activity, found, shell.text())
		}
	}
}

// TestServerShellsSeesWhatRunsFromOutside checks that a shell's foreground command and
// background processes are seen from the process table alone, and that a child that is
// not a shell is left out.
func TestServerShellsSeesWhatRunsFromOutside(t *testing.T) {
	other := exec.Command("sleep", "30")
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Process.Kill(); _ = other.Wait() })
	shell := startShell(t)
	idle := awaitActivity(t, shell, "an idle shell", func(activity ShellActivity) bool { return !activity.Busy() })
	if idle.Foreground != "" {
		t.Errorf("idle shell has foreground %q", idle.Foreground)
	}
	shells, err := ServerShells(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	for _, activity := range shells {
		if activity.PID == other.Process.Pid {
			t.Errorf("a child without a terminal of its own counted as a shell: %+v", activity)
		}
	}

	if _, err := fmt.Fprintf(shell, "sleep 31 &\n"); err != nil {
		t.Fatal(err)
	}
	// Until it execs, the forked child still runs the shell.
	background := awaitActivity(t, shell, "a background sleep", func(activity ShellActivity) bool {
		return len(activity.Background) == 1 && strings.Contains(activity.Background[0].Command, "sleep 31")
	})
	if background.Foreground != "" {
		t.Errorf("with a background sleep: %+v", background)
	}

	if _, err := fmt.Fprintf(shell, "sleep 32\n"); err != nil {
		t.Fatal(err)
	}
	// The shell hands the terminal to the command's group as it forks it, so for a moment
	// the group has no process to name ("a command"), or one still running the shell.
	busy := awaitActivity(t, shell, "a foreground sleep", func(activity ShellActivity) bool { return activity.Foreground == "sleep 32" })
	if len(busy.Background) != 1 {
		t.Errorf("with a foreground sleep: %+v", busy)
	}
}
