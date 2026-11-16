package web

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

// With the terminal on, a shell of a project moves to another project, or to the basedir's
// own shells, through POST /_terminal/shells/ID/place, named as a new shell's place is; the
// list shows it there, and the shell itself, rotari's bash, takes that place's
// ROTARI_PROJECT_NAME (unset for the basedir's own) for the next command. A shell of no
// project, and a project that does not exist, are refused.
func TestTerminalMovesAShell(t *testing.T) {
	covers(t, "WEB-57")
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	e := terminalEnv(t)
	e.Setenv("SHELL", bash)
	e.MustRotari("add", "-p", "p1", "--", "true")
	e.MustRotari("add", "-p", "p2", "--", "true")
	base := e.StartWeb("--allow-terminal")
	token := http.Header{"X-Rotari-Token": {launchToken}}
	status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80&project=p1", token)
	var shell terminalShell
	if err := json.Unmarshal(body, &shell); status != http.StatusCreated || err != nil {
		t.Fatalf("starting p1's shell: status %d: %s", status, body)
	}
	if !strings.Contains(string(body), `"movable":true`) {
		t.Errorf("p1's bash is not movable: %s", body)
	}
	page := openTerminalPage(t, base)
	page.typeLine(`printf 'before=[%s]\n' "$ROTARI_PROJECT_NAME"`)
	page.await("before=[p1]")
	move := func(query string) (int, string) {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells/"+shell.ID+"/place"+query, token)
		return status, string(body)
	}
	status, moved := move("?project=p2")
	var answer terminalShell
	if err := json.Unmarshal([]byte(moved), &answer); status != http.StatusOK || err != nil || answer.Project != "p2" || answer.Link != "/project/p2" {
		t.Fatalf("moving to p2: status %d: %s", status, moved)
	}
	if listed := listTerminalShells(t, base, token); len(listed) != 1 || listed[0].Project != "p2" {
		t.Errorf("listed after the move: %+v", listed)
	}
	page.typeLine(`printf 'moved=[%s]\n' "$ROTARI_PROJECT_NAME"`)
	page.await("moved=[p2]")
	if status, body := move("?basedir="); status != http.StatusOK {
		t.Fatalf("moving to the basedir's own: status %d: %s", status, body)
	}
	page.typeLine(`printf 'own=[%s]\n' "${ROTARI_PROJECT_NAME-unset}"`)
	page.await("own=[unset]")
	if status, body := move("?project=nope"); status != http.StatusNotFound {
		t.Errorf("moving to a project that does not exist: status %d: %s", status, body)
	}
	plain := startTerminalShell(t, base, token)
	status, body = terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells/"+plain.ID+"/place?project=p1", token)
	if status != http.StatusConflict {
		t.Errorf("moving a shell of no project: status %d: %s", status, body)
	}
}
