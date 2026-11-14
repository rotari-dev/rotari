package web

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rotari-dev/rotari/internal/executor"
)

// A shell started by hand inside a panel's shell is the current shell: the preview gives its
// directory and no command while it waits at its prompt, a command it runs is named, Recent
// directories records where it goes, and the shell list does not count it as running.
func TestTerminalFollowsAShellStartedInside(t *testing.T) {
	covers(t, "WEB-52")
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	page := openTerminalPage(t, base)
	id := listTerminalShells(t, base, headers)[0].ID
	inner, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	type preview struct {
		Cwd     string `json:"cwd"`
		Command string `json:"command"`
	}
	// await reads the preview and the shell list until they say what is wanted.
	await := func(what, cwd, command string, running bool) {
		t.Helper()
		var got preview
		var listed bool
		for deadline := time.Now().Add(terminalWait); ; time.Sleep(50 * time.Millisecond) {
			status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells/"+id+"/preview", headers)
			if err := json.Unmarshal(body, &got); status != http.StatusOK || err != nil {
				t.Fatalf("preview: status %d, %v: %s", status, err, body)
			}
			got.Cwd, _ = filepath.EvalSymlinks(got.Cwd)
			_, list := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells", headers)
			var shells struct {
				Shells []struct {
					ID      string `json:"id"`
					Running bool   `json:"running"`
				} `json:"shells"`
			}
			_ = json.Unmarshal(list, &shells)
			for _, shell := range shells.Shells {
				if shell.ID == id {
					listed = shell.Running
				}
			}
			if got.Cwd == cwd && got.Command == command && listed == running {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: preview %+v, running %v; want cwd %q, command %q, running %v", what, got, listed, cwd, command, running)
			}
		}
	}
	page.typeLine(`PS1='inner$ ' sh -i`)
	page.typeLine(`cd ` + inner + ` && echo "moved-$((2+3))"`)
	page.await("moved-5")
	await("at the inner shell's prompt", inner, "", false)
	page.typeLine("sleep 30")
	await("a command in the inner shell", inner, "sleep 30", true)
	page.send(websocket.MessageBinary, "\x03")
	await("the inner shell's prompt again", inner, "", false)
	// Recent directories has the inner shell's directory.
	var dirs struct {
		History []struct {
			Dir string `json:"dir"`
		} `json:"history"`
	}
	for deadline := time.Now().Add(terminalWait); ; time.Sleep(50 * time.Millisecond) {
		status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells/"+id+"/dirs", headers)
		if err := json.Unmarshal(body, &dirs); status != http.StatusOK || err != nil {
			t.Fatalf("dirs: status %d, %v: %s", status, err, body)
		}
		if slices.ContainsFunc(dirs.History, func(entry struct {
			Dir string `json:"dir"`
		}) bool {
			dir, _ := filepath.EvalSymlinks(entry.Dir)
			return dir == inner
		}) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Recent directories lacks the inner shell's %s: %+v", inner, dirs.History)
		}
	}
	// Leaving it, the session's shell is current again.
	page.typeLine("exit")
	home, err := filepath.EvalSymlinks(e.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatal(err)
	}
	await("the session's shell after exit", home, "", false)
}

func TestTerminalFollowsShellWithSpacedOption(t *testing.T) {
	covers(t, "WEB-52")
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	e := terminalEnv(t)
	base := e.StartWeb("--allow-terminal")
	page := openTerminalPage(t, base)
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	id := listTerminalShells(t, base, headers)[0].ID
	dir := filepath.Join(e.Root, "shell config")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	initFile := filepath.Join(dir, "init file")
	if err := os.WriteFile(initFile, []byte("PS1='inner$ '\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page.typeLine(executor.ShellQuote(bash) + " --init-file " + executor.ShellQuote(initFile) +
		` -i; printf '%s%s\n' 'rotari-outer-' 'ready'`)
	t.Cleanup(func() {
		page.typeLine("exit")
		page.await("rotari-outer-ready")
	})
	page.typeLine(`echo "nested-$((1+1))"`)
	page.await("nested-2")

	home, err := filepath.EvalSymlinks(e.Root)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(terminalWait); ; time.Sleep(50 * time.Millisecond) {
		status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells/"+id+"/preview", headers)
		var got struct {
			Cwd          string `json:"cwd"`
			Command      string `json:"command"`
			CommandError string `json:"command_error"`
		}
		if err := json.Unmarshal(body, &got); status != http.StatusOK || err != nil {
			t.Fatalf("preview: status %d, %v: %s", status, err, body)
		}
		got.Cwd, _ = filepath.EvalSymlinks(got.Cwd)
		if got.Cwd == home && got.Command == "" && got.CommandError == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("nested bash with a spaced option was not at its prompt: %+v", got)
		}
	}
}
