package web

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/executor"
)

func awaitInnerMarkedPrompt(page *terminalPage, start int, prompt, mark string) {
	page.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
	defer cancel()
	for {
		text := page.shown.String()[start:]
		if command := strings.Index(text, "\x1b]133;C"); command >= 0 {
			if end := strings.IndexByte(text[command:], '\a'); end >= 0 {
				text = text[command+end+1:]
				at := strings.Index(text, mark)
				promptAt := strings.Index(text, prompt)
				input := strings.Index(text, "\x1b]133;B\a")
				if at >= 0 && promptAt > at && input > promptAt {
					return
				}
			}
		}
		_, output, err := page.connection.Read(ctx)
		if err != nil {
			page.t.Fatalf("waiting for the inner marked prompt: %v\n%s", err, page.shown.String())
		}
		page.shown.Write(output)
	}
}

// A zsh or a bash started by hand inside a panel's bash marks its prompts too, from rotari's
// files kept in the user's cache, with nothing in the user's startup files for it.
func TestTerminalInnerShellsMark(t *testing.T) {
	covers(t, "WEB-53")
	for _, outer := range []string{"bash", "zsh"} {
		for _, test := range []struct{ inner, prompt, a string }{
			{"zsh", "inner-zsh", "\x1b]133;A;rotari_goto=1\a"},
			{"bash", "inner-bash", "\x1b]133;A\a"},
		} {
			t.Run(test.inner+"_inside_"+outer, func(t *testing.T) {
				outerPath, err := exec.LookPath(outer)
				if err != nil {
					t.Skipf("no %s here", outer)
				}
				innerPath, err := exec.LookPath(test.inner)
				if err != nil {
					t.Skipf("no %s here", test.inner)
				}
				e := terminalEnv(t)
				e.Setenv("SHELL", outerPath)
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
				start := page.shown.Len()
				startInner := test.inner
				if test.inner == "bash" {
					startInner = executor.ShellQuote(innerPath)
				}
				page.typeLine(startInner + `; printf '%s%s\n' 'rotari-inner-' 'exited'`)
				t.Cleanup(func() {
					page.typeLine("exit")
					page.await("rotari-inner-exited")
				})
				awaitInnerMarkedPrompt(page, start, test.prompt, test.a)
				page.typeLine(`false; echo "inner-$((2+3))"`)
				page.await("inner-5")
				page.typeLine("false")
				page.typeLine(`echo "after-$((3+4))"`)
				page.await("after-7")
				shown := page.shown.String()
				inner := shown[strings.LastIndex(shown, test.prompt):]
				for _, mark := range []string{"\x1b]133;D;1\a", test.a, "\x1b]133;B\a", "\x1b]133;C;cmdline_url=false\a"} {
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
}
