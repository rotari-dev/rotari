package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInteractiveShell(t *testing.T) {
	for line, want := range map[string]bool{
		"zsh":                                 true,
		"-zsh":                                true,
		"/usr/bin/bash -l":                    true,
		"bash --login":                        true,
		"zsh -i":                              true,
		"bash --init-file /tmp/r/rotari.bash": true,
		"zsh -o vi":                           true,
		"bash -O globstar":                    true,
		"fish --init-command cd":              true,
		"bash -i x.sh":                        true,
		"sh":                                  true,
		"bash x.sh":                           false,
		"sh -c make":                          false,
		"bash -lc make":                       false,
		"fish --command ls":                   false,
		"zsh -- x.zsh":                        false,
		"python3":                             false,
		"vim":                                 false,
		"ssh host":                            false,
	} {
		if got := interactiveShell(strings.Fields(line)); got != want {
			t.Errorf("interactiveShell(%q) = %v, want %v", line, got, want)
		}
	}
	if interactiveShell(nil) {
		t.Error("no command line is a shell")
	}
}

// eventually waits for a condition, failing with what it last said.
func eventually(t *testing.T, what string, check func() (bool, string)) {
	t.Helper()
	var last string
	for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var ok bool
		if ok, last = check(); ok {
			return
		}
	}
	t.Fatalf("%s: %s", what, last)
}

// A shell started inside the session's shell is the current shell: its directory is the
// shell's, its prompt is a prompt, and a command or script it runs is busy and named.
func TestAShellStartedInsideIsTheCurrentShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh here")
	}
	shell := startShell(t)
	home, err := filepath.EvalSymlinks(shell.run(t, "pwd -P"))
	if err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(home, "inner")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	state := func() (string, bool, string) {
		dir, _ := shell.Dir()
		dir, _ = filepath.EvalSymlinks(dir)
		busy, _ := shell.busy()
		command, _ := shell.Foreground()
		return dir, busy, command
	}
	expect := func(what, dir string, busy bool, command string) {
		t.Helper()
		eventually(t, what, func() (bool, string) {
			gotDir, gotBusy, gotCommand := state()
			return gotDir == dir && gotBusy == busy && strings.Contains(gotCommand, command) && (command != "" || gotCommand == ""),
				"dir " + gotDir + ", busy " + map[bool]string{true: "yes", false: "no"}[gotBusy] + ", command " + gotCommand
		})
	}
	expect("the session's shell", home, false, "")
	if _, err := shell.Write([]byte("PS1='inner$ ' sh -i\n")); err != nil {
		t.Fatal(err)
	}
	expect("at the inner shell's prompt", home, false, "")
	if _, err := shell.Write([]byte("cd inner\n")); err != nil {
		t.Fatal(err)
	}
	expect("the inner shell's cd", inner, false, "")
	if _, err := shell.Write([]byte("sleep 30\n")); err != nil {
		t.Fatal(err)
	}
	expect("a command in the inner shell", inner, true, "sleep 30")
	if _, err := shell.Write([]byte("\x03")); err != nil {
		t.Fatal(err)
	}
	expect("the inner shell's prompt again", inner, false, "")
	// A shell running a script is a command, not a shell waiting for the user.
	if _, err := shell.Write([]byte("sh -c 'sleep 30; :'\n")); err != nil {
		t.Fatal(err)
	}
	expect("a script in the inner shell", inner, true, "sh -c sleep 30; :")
	if _, err := shell.Write([]byte("\x03")); err != nil {
		t.Fatal(err)
	}
	expect("the inner shell after the script", inner, false, "")
	if _, err := shell.Write([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	expect("the session's shell after the inner one", home, false, "")
}
