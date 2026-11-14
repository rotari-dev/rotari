package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// With the terminal on, bash and zsh start with rotari's own startup files, which read
// the user's and mark prompts and commands with OSC 133, zsh also taking a clicked index; with
// --terminal-shell-integration=false they start as plain login shells, which read the
// user's files and mark nothing. The user's files are left as they were.
func TestTerminalShellIntegration(t *testing.T) {
	covers(t, "WEB-49")
	for _, test := range []struct {
		shell string
		files map[string]string
		// check is typed after false, and prints the line wanted.
		check, want string
	}{
		{"bash", map[string]string{".bash_profile": "export FROM_FILES=profile\n"}, `echo "files-$FROM_FILES"`, "files-profile"},
		// unsetopt GLOBAL_RCS skips the host's /etc/zshrc, which may run compinit and ask
		// about insecure directories, taking what the test types as the answer.
		{"zsh", map[string]string{".zshenv": "unsetopt GLOBAL_RCS\nexport FROM_FILES=env\n", ".zshrc": "FROM_FILES=$FROM_FILES-rc\n"}, `echo "files-$FROM_FILES-${ZDOTDIR-unset}"`, "files-env-rc-unset"},
	} {
		path, err := exec.LookPath(test.shell)
		if err != nil {
			t.Logf("no %s here", test.shell)
			continue
		}
		for _, integrated := range []bool{true, false} {
			name := test.shell + map[bool]string{true: " integrated", false: " plain"}[integrated]
			t.Run(name, func(t *testing.T) {
				e := terminalEnv(t)
				e.Setenv("SHELL", path)
				for file, content := range test.files {
					if err := os.WriteFile(filepath.Join(e.Root, file), []byte(content), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"--allow-terminal"}
				if !integrated {
					args = append(args, "--terminal-shell-integration=false")
				}
				page := openTerminalPage(t, e.StartWeb(args...))
				page.typeLine("false")
				page.typeLine(test.check)
				page.await(test.want)
				shown := page.shown.String()
				if integrated {
					// The prompt after false says how it ended, starts a prompt (zsh's saying it
					// takes a clicked index), and marks the command line's start.
					a := "\x1b]133;A\a"
					if test.shell == "zsh" {
						a = "\x1b]133;A;rotari_goto=1\a"
					}
					for _, mark := range []string{"\x1b]133;D;1\a", a, "\x1b]133;B\a"} {
						if !strings.Contains(shown, mark) {
							t.Errorf("no %q in what %s showed:\n%q", mark, test.shell, shown)
						}
					}
					if test.shell == "zsh" {
						// The index after zsh's key sequence puts the cursor there: Z after
						// "echo ", and Q past the end, which stops at the end.
						page.typeLine("echo abc\x1b[9998~5;Z\x1b[9998~99;Q")
						page.await("ZabcQ")
					}
				} else if strings.Contains(shown, "\x1b]133;") {
					t.Errorf("%s marked its prompts with the integration off:\n%q", test.shell, shown)
				}
				for file, content := range test.files {
					if got, err := os.ReadFile(filepath.Join(e.Root, file)); err != nil || string(got) != content {
						t.Errorf("%s became %q, %v", file, got, err)
					}
				}
			})
		}
	}
}
