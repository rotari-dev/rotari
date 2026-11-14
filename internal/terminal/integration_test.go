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
	dir, err := writeLastingIntegration(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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

// marksAfter types false, and returns the OSC 133 marks the shell printed by the prompt after it,
// but zsh's rotari_cursor.
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
				// zsh's word of where its cursor is, before each redraw, is checked on its own.
				if !strings.HasPrefix(match[1], "rotari_cursor=") {
					marks = append(marks, match[1])
				}
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

// Bash reads a login shell's files, keeps $? for the user's PROMPT_COMMAND, marks a prompt
// that a theme rebuilds each time, and says the command line in C when history took it.
func TestBashIntegrationReadsTheLoginFilesAndMarks(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		".bash_profile": "export FROM_PROFILE=yes\nPROMPT_COMMAND='LAST=$?; PS1=\"theme> \";'\nHISTCONTROL=ignorespace\nHISTTIMEFORMAT='%F '\n",
		// Only the first of these a login shell would read.
		".profile": "export FROM_PROFILE=no\n",
	})
	shell := integrated(t, "bash", home)
	// The marks are checked from false's output on; the last B is the one put back in the
	// theme's prompt.
	want := []string{"C;cmdline_url=false", "D;1", "A", "B"}
	if marks := marksAfter(t, shell); !endsWith(marks, want...) {
		t.Errorf("marks around false: %q, want them to end %q", marks, want)
	}
	if got := shell.run(t, `echo "$FROM_PROFILE-$LAST"`); got != "yes-1" {
		t.Errorf("profile and status = %q, want yes-1", got)
	}
	// A line history leaves out is not said, rather than said as the one before it.
	time.Sleep(300 * time.Millisecond)
	shell.mu.Lock()
	shell.output.Reset()
	shell.mu.Unlock()
	if _, err := shell.Write([]byte(" true\n")); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(testWait); !strings.Contains(shell.text(), "133;D;0\a"); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no D;0 after a line history left out; the terminal shows:\n%q", shell.text())
		}
	}
	var marks []string
	for _, match := range oscMark.FindAllStringSubmatch(shell.text(), -1) {
		marks = append(marks, match[1])
	}
	if !slices.Contains(marks, "C") || slices.ContainsFunc(marks, func(mark string) bool { return strings.HasPrefix(mark, "C;") }) {
		t.Errorf("marks for a line history left out: %q, want a C that says no line", marks)
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
	if !slices.Equal(marks, []string{"D;0", "A", "B", "C;cmdline_url=true%20%7C%20false", "D;1", "A", "B"}) {
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
			if marks := marksAfter(t, shell); !endsWith(marks, "B", "C;cmdline_url=false", "D;1", "A;rotari_goto=1", "B") {
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
	if marks := marksAfter(t, shell); !endsWith(marks, "B", "C;cmdline_url=false", "D;1", "A;rotari_goto=1", "B") {
		t.Errorf("marks around false: %q", marks)
	}
	// zsh says the command line in C, percent-encoded byte by byte, as kitty does. (Bytes
	// above 127 need a UTF-8 locale, which zsh's line editor reads them by and the test's
	// environment may not have.)
	if _, err := shell.Write([]byte("echo \"a b;100%\" >/dev/null\n")); err != nil {
		t.Fatal(err)
	}
	const said = "133;C;cmdline_url=echo%20%22a%20b%3B100%25%22%20%3E%2Fdev%2Fnull\a"
	for deadline := time.Now().Add(testWait); !strings.Contains(shell.text(), said); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no %q; the terminal shows:\n%q", said, shell.text())
		}
	}
	if got := shell.run(t, `print -l $precmd_functions | tail -1`); got != "__theme" {
		t.Errorf("the last precmd hook is %q; the test's theme does not come after rotari's", got)
	}
}

// zsh puts its cursor at a character index sent after its private key sequence, stops at
// the buffer's end, and moves by no widget that a suggestion is taken by.
func TestZshIntegrationPutsTheCursorWhereClicked(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		".zshenv": "unsetopt GLOBAL_RCS\n",
		// zsh-autosuggestions takes a suggestion on forward-char, which this notes.
		".zshrc": "__spy() { SPIED=yes; zle .forward-char }\nzle -N forward-char __spy\n",
	})
	shell := integrated(t, "zsh", home)
	marksAfter(t, shell)
	// "echo abc", then Z after "echo ", then Q past the end, which stops at the end.
	if _, err := shell.Write([]byte("echo abc\x1b[9998~5;Z\x1b[9998~99;Q\n")); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(testWait); !strings.Contains(oscSequence.ReplaceAllString(shell.text(), ""), "\nZabcQ\r\n"); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no ZabcQ; the terminal shows:\n%q", shell.text())
		}
	}
	if got := shell.run(t, "echo ${SPIED:-no}"); got != "no" {
		t.Errorf("forward-char ran, which takes a suggestion: %q", got)
	}
	// Before each redraw, zsh says where its cursor is in the line, Ctrl+L's too, so that the
	// panel finds the line's start from it when the prompt is drawn again elsewhere.
	if _, err := shell.Write([]byte("echo ab")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	shell.mu.Lock()
	shell.output.Reset()
	shell.mu.Unlock()
	if _, err := shell.Write([]byte("\x0c")); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(testWait); !strings.Contains(shell.text(), "\x1b]133;rotari_cursor=7\a"); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no cursor said after Ctrl+L; the terminal shows:\n%q", shell.text())
		}
	}
}

// Only bash and zsh start otherwise, and only with the integration.
func TestShellCommandWithIntegration(t *testing.T) {
	env := []string{"HOME=/h", "ZDOTDIR=/z", "ROTARI_USER_ZDOTDIR=/forged"}
	with := append(slices.Clone(env), "ROTARI_SHELL_INTEGRATION=/i")
	for _, test := range []struct {
		shell, integration string
		args, env          []string
	}{
		{"/bin/sh", "/i", []string{"-l"}, with},
		{"/usr/bin/fish", "/i", []string{"-l"}, with},
		{"/bin/bash", "", []string{"-l"}, env},
		{"/bin/zsh", "", []string{"-l"}, env},
		{"/bin/bash", "/i", []string{"--init-file", "/i/rotari.bash"}, with},
		{"/usr/local/bin/zsh", "/i", []string{"-l"}, []string{"HOME=/h", "ROTARI_SHELL_INTEGRATION=/i", "ZDOTDIR=/i/zsh", "ROTARI_USER_ZDOTDIR=/z"}},
	} {
		args, gotEnv := shellCommand(Options{Shell: test.shell, Integration: test.integration, Env: env})
		if !slices.Equal(args, test.args) || !slices.Equal(gotEnv, test.env) {
			t.Errorf("%s with %q: %q %q, want %q %q", test.shell, test.integration, args, gotEnv, test.args, test.env)
		}
	}
	// zsh with no ZDOTDIR is told none of its own.
	if _, got := shellCommand(Options{Shell: "zsh", Integration: "/i", Env: []string{"HOME=/h"}}); !slices.Equal(got, []string{"HOME=/h", "ROTARI_SHELL_INTEGRATION=/i", "ZDOTDIR=/i/zsh"}) {
		t.Errorf("zsh without ZDOTDIR: %q", got)
	}
	// A server started inside a panel's shell: its ZDOTDIR is that shell's rotari's, and
	// the user's own is in ROTARI_USER_ZDOTDIR, or there is none.
	inside := []string{"HOME=/h", "ROTARI_SHELL_INTEGRATION=/old", "ZDOTDIR=/old/zsh", "ROTARI_USER_ZDOTDIR=/z"}
	if _, got := shellCommand(Options{Shell: "zsh", Integration: "/i", Env: inside}); !slices.Equal(got, []string{"HOME=/h", "ROTARI_SHELL_INTEGRATION=/i", "ZDOTDIR=/i/zsh", "ROTARI_USER_ZDOTDIR=/z"}) {
		t.Errorf("zsh from inside a panel's shell: %q", got)
	}
	if _, got := shellCommand(Options{Shell: "zsh", Integration: "/i", Env: inside[:3]}); !slices.Equal(got, []string{"HOME=/h", "ROTARI_SHELL_INTEGRATION=/i", "ZDOTDIR=/i/zsh"}) {
		t.Errorf("zsh from inside a panel's shell, with no ZDOTDIR of the user's: %q", got)
	}
}

// innerMarks types a command that starts a shell inside, waits for that shell's prompt, and
// returns the marks for false typed in it, from its prompt on.
func innerMarks(t *testing.T, shell *startedShell, start, ready string) []string {
	t.Helper()
	if _, err := shell.Write([]byte(start + "\n")); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(testWait); !strings.Contains(shell.text(), ready); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%q did not start; the terminal shows:\n%q", start, shell.text())
		}
	}
	time.Sleep(300 * time.Millisecond)
	shell.mu.Lock()
	shell.output.Reset()
	shell.mu.Unlock()
	return marksAfter(t, shell)
}

// lines counts the lines of a file a test's startup files append to, one for each time
// they are read.
func lines(t *testing.T, path string) int {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(content), "\n")
}

// A shell started by hand inside one of rotari's marks its prompts too, reading the user's
// files once: zsh through rotari's ZDOTDIR, bash through the loader in the PROMPT_COMMAND it
// inherits; and a zsh's own command lines still see the user's ZDOTDIR.
func TestInnerShellsMark(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		if _, err := exec.LookPath(shell); err != nil {
			t.Skipf("no %s here", shell)
		}
	}
	files := func(home string) map[string]string {
		return map[string]string{
			".bash_profile": "[ -r ~/.bashrc ] && . ~/.bashrc\n",
			// A .bashrc that adds to PROMPT_COMMAND, as many do.
			".bashrc":  "echo bashrc >> " + home + "/read.log\nPS1='bash$ '\nPROMPT_COMMAND=\"BASHRC_RAN=1;$PROMPT_COMMAND\"\n",
			".zshenv":  "unsetopt GLOBAL_RCS\n",
			".zshrc":   "echo zshrc >> " + home + "/read.log\nPS1='zsh%# '\n",
			"read.log": "",
		}
	}
	for _, test := range []struct{ outer, inner, ready, a string }{
		{"bash", "zsh", "zsh%", "A;rotari_goto=1"},
		{"zsh", "zsh", "zsh%", "A;rotari_goto=1"},
		{"zsh", "bash", "bash$", "A"},
		{"bash", "bash", "bash$", "A"},
	} {
		t.Run(test.inner+" inside "+test.outer, func(t *testing.T) {
			home := t.TempDir()
			writeFiles(t, home, files(home))
			outer := integrated(t, test.outer, home)
			marksAfter(t, outer)
			before := lines(t, filepath.Join(home, "read.log"))
			// The inner shell's prompt, once it has marked it, is where false is typed.
			if marks := innerMarks(t, outer, test.inner, test.ready); !endsWith(marks, "D;1", test.a, "B") || !slices.ContainsFunc(marks, func(mark string) bool { return strings.HasPrefix(mark, "C") }) {
				t.Errorf("marks in %s inside %s: %q", test.inner, test.outer, marks)
			}
			if read := lines(t, filepath.Join(home, "read.log")) - before; read != 1 {
				t.Errorf("the inner %s read the user's startup file %d times", test.inner, read)
			}
			if test.inner == "bash" {
				// Each prompt marks once, though .bashrc added to an inherited PROMPT_COMMAND.
				if got := outer.run(t, `printf '%s' "$PROMPT_COMMAND" | grep -c __rotari_prompt`); got != "1" {
					t.Errorf("the inner bash runs __rotari_prompt %s times", got)
				}
			}
			if test.inner == "zsh" {
				if got := outer.run(t, `echo "${ZDOTDIR-unset}-${ROTARI_USER_ZDOTDIR-unset}"`); got != "unset-unset" {
					t.Errorf("the inner zsh's own ZDOTDIR: %q", got)
				}
			}
		})
	}
}

// The Keeper writes the files when it is made with the integration and removes them on Close.
// useCache makes the user's cache directory a new one for the test, as os.UserCacheDir
// finds it on Linux and macOS.
func useCache(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("HOME", home)
	base, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return base
}

// The Keeper writes the files under the user's cache and keeps them after Close, for what
// outlives the server; a cache it cannot trust makes it use a temporary directory, which it
// removes, and says why.
func TestKeeperKeepsTheIntegrationInTheCache(t *testing.T) {
	base := useCache(t)
	keeper, err := NewKeeper(KeeperOptions{Shell: Options{Shell: "/bin/sh", Dir: t.TempDir()}, RingSize: 1024, ShellLimit: 1, GroupLimit: 1, ShellIntegration: true})
	if err != nil {
		t.Fatal(err)
	}
	dir := keeper.options.Shell.Integration
	if filepath.Dir(dir) != filepath.Join(base, "rotari", "shell-integration") {
		t.Errorf("the files are in %s, not under the cache %s", dir, base)
	}
	keeper.Close()
	if _, err := os.Stat(filepath.Join(dir, "rotari.bash")); err != nil {
		t.Errorf("after Close, the files in the cache: %v", err)
	}
	// Others can write the cache's rotari directory: a temporary directory instead.
	if err := os.Chmod(filepath.Join(base, "rotari"), 0o777); err != nil {
		t.Fatal(err)
	}
	var logged []string
	fallback, err := NewKeeper(KeeperOptions{Shell: Options{Shell: "/bin/sh", Dir: t.TempDir()}, RingSize: 1024, ShellLimit: 1, GroupLimit: 1, ShellIntegration: true,
		Log: func(message string) { logged = append(logged, message) }})
	if err != nil {
		t.Fatal(err)
	}
	temporary := fallback.options.Shell.Integration
	if strings.HasPrefix(temporary, base) || len(logged) != 1 || !strings.Contains(logged[0], "can be written by others") {
		t.Errorf("with a cache others can write: files in %s, logged %q", temporary, logged)
	}
	fallback.Close()
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Errorf("after Close, the temporary %s: %v", temporary, err)
	}
	plain, err := NewKeeper(KeeperOptions{Shell: Options{Shell: "/bin/sh"}, RingSize: 1024, ShellLimit: 1, GroupLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if plain.options.Shell.Integration != "" {
		t.Error("a Keeper without the integration wrote it")
	}
}

// The files in the cache are named by their content, written once and reused, put back
// when changed, and refused where they could have been put by someone else.
func TestLastingIntegration(t *testing.T) {
	base := t.TempDir()
	dir, err := writeLastingIntegration(base)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := integrationHash()
	if dir != filepath.Join(base, "rotari", "shell-integration", hash) {
		t.Errorf("dir %s", dir)
	}
	for _, file := range integrationLayout {
		info, err := os.Stat(filepath.Join(dir, file[1]))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v, %v", file[1], info, err)
		}
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("directory %s: %v, %v", dir, info, err)
	}
	// Changed, it is put back; the second write gives the same directory.
	zshrc := filepath.Join(dir, "zsh", ".zshrc")
	if err := os.WriteFile(zshrc, []byte("echo changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if again, err := writeLastingIntegration(base); err != nil || again != dir {
		t.Fatalf("again: %s, %v", again, err)
	}
	want, _ := integrationFiles.ReadFile("integration/zsh/zshrc")
	if got, _ := os.ReadFile(zshrc); string(got) != string(want) {
		t.Error("a changed file was not put back")
	}
	// A file others can write is replaced by one they cannot.
	if err := os.Chmod(zshrc, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := writeLastingIntegration(base); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(zshrc); info.Mode().Perm() != 0o600 {
		t.Errorf("a file others could write kept %v", info.Mode())
	}
	// A directory that is a link, or that others can write, is refused.
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := writeLastingIntegration(base); err == nil || !strings.Contains(err.Error(), "can be written by others") {
		t.Errorf("a directory others can write: %v", err)
	}
	linked := t.TempDir()
	if err := os.MkdirAll(filepath.Join(linked, "rotari"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(linked, "rotari", "shell-integration")); err != nil {
		t.Fatal(err)
	}
	if _, err := writeLastingIntegration(linked); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("a linked directory: %v", err)
	}
}
