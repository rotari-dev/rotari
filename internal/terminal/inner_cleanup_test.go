package terminal

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/state"
)

func TestInnerShellExitsBeforeFixtureCleanup(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("no %s here", name)
		}
	}
	done := filepath.Join(t.TempDir(), "inner-exited")
	var innerPID int
	var innerStart string
	t.Cleanup(func() {
		if innerPID == 0 || innerStart == "" {
			return
		}
		isInnerShell := func() bool {
			if err := syscall.Kill(innerPID, 0); errors.Is(err, syscall.ESRCH) {
				return false
			} else if err != nil {
				t.Errorf("probe inner shell %d: %v", innerPID, err)
				return false
			}
			current := state.ProcessStart(innerPID)
			if current == "" {
				if err := syscall.Kill(innerPID, 0); err == nil {
					t.Errorf("cannot verify inner shell %d before cleanup", innerPID)
				} else if !errors.Is(err, syscall.ESRCH) {
					t.Errorf("probe inner shell %d: %v", innerPID, err)
				}
				return false
			}
			return current == innerStart
		}
		for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if !isInnerShell() {
				return
			}
		}
		if !isInnerShell() {
			return
		}
		if err := syscall.Kill(innerPID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("stop inner shell %d: %v", innerPID, err)
		}
		t.Error("inner shell did not exit")
	})
	if !t.Run("fixture", func(t *testing.T) {
		home := t.TempDir()
		writeFiles(t, home, map[string]string{
			".zshenv": "unsetopt GLOBAL_RCS\n",
			".zshrc":  "PS1='zsh%# '\n",
			".bashrc": "PS1='bash$ '\ntrap 'sleep 0.5; : > \"$HOME/late-history\"; : > \"$TRACE_EXIT_FILE\"' EXIT\ntrap 'exit' HUP\n",
		})
		outer := integrated(t, "zsh", home, "TRACE_EXIT_FILE="+done)
		innerMarks(t, outer, "bash", "bash$")
		var err error
		innerPID, err = strconv.Atoi(outer.run(t, `printf '%s' "$$"`))
		if err != nil {
			t.Fatal(err)
		}
		innerStart = state.ProcessStart(innerPID)
		if innerStart == "" {
			t.Fatalf("cannot identify inner shell %d", innerPID)
		}
	}) {
		t.Fatal("fixture failed")
	}
	if _, err := os.Stat(done); err != nil {
		t.Fatalf("the inner shell's exit cleanup did not finish before fixture cleanup: %v", err)
	}
}
