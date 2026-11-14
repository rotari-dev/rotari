package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// integrated starts shell with the shell integration, in home, with env besides HOME and
// PATH.
func integrated(t *testing.T, shell, home string, env ...string) *startedShell {
	t.Helper()
	path, err := exec.LookPath(shell)
	if err != nil {
		t.Skipf("no %s here", shell)
	}
	dir, err := WriteIntegration()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return startShellWith(t, Options{Shell: path, Dir: home, Integration: dir, Rows: 24, Cols: 80,
		Env: append([]string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}, env...)})
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

var oscMark = regexp.MustCompile("\x1b\\]133;([^\a]*)\a")

// marksAfter types false, and returns the OSC 133 marks the shell printed by the prompt after it.
func marksAfter(t *testing.T, shell *startedShell) []string {
	t.Helper()
	if _, err := shell.Write([]byte("false\n")); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		text := shell.text()
		if at := strings.Index(text, "\x1b]133;D;1\a"); at >= 0 && strings.Contains(text[at:], "133;B\a") {
			var marks []string
			for _, match := range oscMark.FindAllStringSubmatch(text, -1) {
				marks = append(marks, match[1])
			}
			return marks
		}
	}
	t.Fatalf("no marks for false; the terminal shows:\n%q", shell.text())
	return nil
}

// endsWith says whether marks end with want.
func endsWith(marks []string, want ...string) bool {
	return len(marks) >= len(want) && slices.Equal(marks[len(marks)-len(want):], want)
}

// Bash reads a login shell's files, keeps $? for the user's PROMPT_COMMAND, and marks a
// prompt that a theme rebuilds each time.
func TestBashIntegrationReadsTheLoginFilesAndMarks(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		".bash_profile": "export FROM_PROFILE=yes\nPROMPT_COMMAND='LAST=$?; PS1=\"theme> \";'\n",
		// Only the first of these a login shell would read.
		".profile": "export FROM_PROFILE=no\n",
	})
	shell := integrated(t, "bash", home)
	// Bash before 4.4, as macOS has, prints no PS0, and so no C. The run clears what was
	// shown, so the marks are checked from false's output on; the last B is the one put back
	// in the theme's prompt.
	want := []string{"C", "D;1", "A", "B"}
	if version := shell.run(t, `echo $((BASH_VERSINFO[0] * 100 + BASH_VERSINFO[1]))`); version < "404" && len(version) == 3 {
		want = want[1:]
	}
	if marks := marksAfter(t, shell); !endsWith(marks, want...) {
		t.Errorf("marks around false: %q, want them to end %q", marks, want)
	}
	if got := shell.run(t, `echo "$FROM_PROFILE-$LAST"`); got != "yes-1" {
		t.Errorf("profile and status = %q, want yes-1", got)
	}
}

// Bash before 4.4 has no PS0: a DEBUG trap prints C before the command typed at a prompt,
// once, and not for a line with no command; $? is kept for the user's PROMPT_COMMAND. A new
// bash is made to take that way here, since no old one may be at hand.
func TestOldBashIntegrationMarksCommandsByTheDebugTrap(t *testing.T) {
	home := t.TempDir()
	// A PS0 of the profile's own, as systemd's /etc/profile.d sets one.
	writeFiles(t, home, map[string]string{".bash_profile": "PROMPT_COMMAND='LAST=$?'\nPS0='[ps0]'\n"})
	shell := integrated(t, "bash", home, "ROTARI_BASH_DEBUG_TRAP=1")
	// The host's /etc/profile may set a PS0 of its own, as systemd's does; C is not added to it.
	if got := shell.run(t, `case ${PS0:-} in *'133;C'*) echo C-in-PS0;; *) echo no-C;; esac`); got != "no-C" {
		t.Errorf("PS0 = %q, want no C in it", got)
	}
	if got := shell.run(t, `echo "${ROTARI_BASH_DEBUG_TRAP:-gone}"`); got != "gone" {
		t.Errorf("the test's variable = %q, want it gone", got)
	}
	// An empty line, then a pipeline of two commands that fails, with what came before gone.
	time.Sleep(300 * time.Millisecond)
	shell.mu.Lock()
	shell.output.Reset()
	shell.mu.Unlock()
	if _, err := shell.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := shell.Write([]byte("true | false\n")); err != nil {
		t.Fatal(err)
	}
	ended := func() bool {
		text := shell.text()
		at := strings.Index(text, "133;D;1\a")
		return at >= 0 && strings.Contains(text[at:], "133;B\a")
	}
	for deadline := time.Now().Add(testWait); !ended(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no D;1; the terminal shows:\n%q", shell.text())
		}
	}
	var marks []string
	for _, match := range oscMark.FindAllStringSubmatch(shell.text(), -1) {
		marks = append(marks, match[1])
	}
	// The empty line's prompt has no C, and the pipeline one.
	if !slices.Equal(marks, []string{"D;0", "A", "B", "C", "D;1", "A", "B"}) {
		t.Errorf("marks: %q", marks)
	}
	if got := shell.run(t, `echo "$LAST"`); got != "1" {
		t.Errorf("the user's PROMPT_COMMAND saw $? = %q, want 1", got)
	}
}

// Zsh reads the user's files from where zsh would have: $HOME, a ZDOTDIR it was given, or
// one the user's .zshenv sets; it leaves ZDOTDIR as it was, and marks as bash does.
func TestZshIntegrationReadsTheUsersFilesAndMarks(t *testing.T) {
	theme := "precmd_functions+=(__user_precmd)\n__user_precmd() { LAST=$?; PS1='theme> ' }\n"
	// The host's own /etc/zshrc may run compinit, which asks about insecure directories and
	// takes what the test types as the answer; a user's .zshenv can skip the host's files.
	const noHost = "unsetopt GLOBAL_RCS\n"
	for _, test := range []struct {
		name  string
		files map[string]string
		env   []string
		want  string
	}{
		{"home", map[string]string{".zshenv": noHost + "export FROM=env\n", ".zprofile": "FROM=$FROM-profile\n", ".zshrc": "FROM=$FROM-rc\n" + theme, ".zlogin": "FROM=$FROM-login\n"},
			nil, "env-profile-rc-login-1-unset"},
		{"given", map[string]string{"zd/.zshenv": noHost + "export FROM=env\n", "zd/.zshrc": "FROM=$FROM-rc\n" + theme, ".zshrc": "FROM=wrong\n"},
			[]string{"ZDOTDIR=HOME/zd"}, "env-rc-1-HOME/zd"},
		{"set in .zshenv", map[string]string{".zshenv": noHost + "export FROM=env\nZDOTDIR=$HOME/cfg\n", "cfg/.zshrc": "FROM=$FROM-rc\n" + theme, ".zshrc": "FROM=wrong\n"},
			nil, "env-rc-1-HOME/cfg"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			writeFiles(t, home, test.files)
			var env []string
			for _, entry := range test.env {
				env = append(env, strings.ReplaceAll(entry, "HOME", home))
			}
			shell := integrated(t, "zsh", home, env...)
			if marks := marksAfter(t, shell); !endsWith(marks, "B", "C", "D;1", "A", "B") {
				t.Errorf("marks around false: %q", marks)
			}
			want := strings.ReplaceAll(test.want, "HOME", home)
			if got := shell.run(t, `echo "$FROM-$LAST-${ZDOTDIR-unset}-${ROTARI_USER_ZDOTDIR-}"`); got != want+"-" {
				t.Errorf("files, status, ZDOTDIR = %q, want %q", got, want+"-")
			}
		})
	}
}

// A zsh theme that adds its precmd hook only at the first prompt, after rotari's, and rebuilds
// the prompt in it, as powerlevel10k does, still has its command lines marked.
func TestZshIntegrationMarksAThemeThatComesLater(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		".zshenv": "unsetopt GLOBAL_RCS\n",
		".zshrc":  "__later() { precmd_functions=(${precmd_functions:#__later} __theme) }\n__theme() { PS1='theme> ' }\nprecmd_functions+=(__later)\n",
	})
	shell := integrated(t, "zsh", home)
	if marks := marksAfter(t, shell); !endsWith(marks, "B", "C", "D;1", "A", "B") {
		t.Errorf("marks around false: %q", marks)
	}
	if got := shell.run(t, `print -l $precmd_functions | tail -1`); got != "__theme" {
		t.Errorf("the last precmd hook is %q; the test's theme does not come after rotari's", got)
	}
}

// Only bash and zsh start otherwise, and only with the integration.
func TestShellCommandWithIntegration(t *testing.T) {
	env := []string{"HOME=/h", "ZDOTDIR=/z", "ROTARI_USER_ZDOTDIR=/forged"}
	for _, test := range []struct {
		shell, integration string
		args, env          []string
	}{
		{"/bin/sh", "/i", []string{"-l"}, env},
		{"/usr/bin/fish", "/i", []string{"-l"}, env},
		{"/bin/bash", "", []string{"-l"}, env},
		{"/bin/zsh", "", []string{"-l"}, env},
		{"/bin/bash", "/i", []string{"--init-file", "/i/rotari.bash"}, env},
		{"/usr/local/bin/zsh", "/i", []string{"-l"}, []string{"HOME=/h", "ZDOTDIR=/i/zsh", "ROTARI_USER_ZDOTDIR=/z"}},
	} {
		args, gotEnv := shellCommand(Options{Shell: test.shell, Integration: test.integration, Env: env})
		if !slices.Equal(args, test.args) || !slices.Equal(gotEnv, test.env) {
			t.Errorf("%s with %q: %q %q, want %q %q", test.shell, test.integration, args, gotEnv, test.args, test.env)
		}
	}
	// zsh with no ZDOTDIR is told none of its own.
	if _, got := shellCommand(Options{Shell: "zsh", Integration: "/i", Env: []string{"HOME=/h"}}); !slices.Equal(got, []string{"HOME=/h", "ZDOTDIR=/i/zsh"}) {
		t.Errorf("zsh without ZDOTDIR: %q", got)
	}
}

// The Keeper writes the files when it is made with the integration and removes them on Close.
func TestKeeperWritesAndRemovesTheIntegration(t *testing.T) {
	keeper, err := NewKeeper(KeeperOptions{Shell: Options{Shell: "/bin/sh", Dir: t.TempDir()}, RingSize: 1024, ShellLimit: 1, GroupLimit: 1, ShellIntegration: true})
	if err != nil {
		t.Fatal(err)
	}
	dir := keeper.options.Shell.Integration
	for _, name := range []string{"rotari.bash", "zsh/.zshenv", "zsh/.zprofile", "zsh/.zshrc", "zsh/.zlogin"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Error(err)
		}
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("directory %s: %v, %v", dir, info, err)
	}
	keeper.Close()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("after Close, %s: %v", dir, err)
	}
	plain, err := NewKeeper(KeeperOptions{Shell: Options{Shell: "/bin/sh"}, RingSize: 1024, ShellLimit: 1, GroupLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if plain.options.Shell.Integration != "" {
		t.Error("a Keeper without the integration wrote it")
	}
}
