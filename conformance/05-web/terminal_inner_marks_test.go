package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A zsh or a bash started by hand inside a panel's bash marks its prompts too, from rotari's
// files kept in the user's cache, with nothing in the user's startup files for it.
func TestTerminalInnerShellsMark(t *testing.T) {
	covers(t, "WEB-53")
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	for _, test := range []struct{ inner, prompt, a string }{
		{"zsh", "inner-zsh", "\x1b]133;A;rotari_goto=1\a"},
		{"bash", "inner-bash", "\x1b]133;A\a"},
	} {
		t.Run(test.inner, func(t *testing.T) {
			if _, err := exec.LookPath(test.inner); err != nil {
				t.Skipf("no %s here", test.inner)
			}
			e := terminalEnv(t)
			e.Setenv("SHELL", bash)
			for file, content := range map[string]string{
				".zshenv": "unsetopt GLOBAL_RCS\n",
				".zshrc":  "PS1='inner-zsh%% '\n",
				".bashrc": "PS1='inner-bash$ '\n",
			} {
				if err := os.WriteFile(filepath.Join(e.Root, file), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			page := openTerminalPage(t, e.StartWeb("--allow-terminal"))
			page.typeLine(`echo "outer-$((1+1))"`)
			page.await("outer-2")
			// The inner shell's prompt, and the marks of a command in it.
			page.typeLine(test.inner)
			page.typeLine(`false; echo "inner-$((2+3))"`)
			page.await("inner-5")
			page.typeLine("false")
			page.typeLine(`echo "after-$((3+4))"`)
			page.await("after-7")
			shown := page.shown.String()
			inner := shown[strings.LastIndex(shown, test.prompt):]
			for _, mark := range []string{"\x1b]133;D;1\a", test.a, "\x1b]133;B\a", "\x1b]133;C"} {
				if !strings.Contains(shown[strings.Index(shown, "inner-5"):], mark) {
					t.Errorf("no %q from the inner %s:\n%q", mark, test.inner, inner)
				}
			}
			// The files are in the user's cache, where they outlast the server.
			kept, _ := filepath.Glob(filepath.Join(e.Root, "cache", "rotari", "shell-integration", "*", "marks.bash"))
			if len(kept) != 1 {
				t.Errorf("the integration's files in the cache: %q", kept)
			}
		})
	}
}
