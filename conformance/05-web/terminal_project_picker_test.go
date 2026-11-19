package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"
)

// A shell reads the directories and commands of another project of its basedir, which
// the server heard from that project's shells, and says which projects there are.
func TestTerminalReadsAnotherProjectsHistory(t *testing.T) {
	covers(t, "WEB-63")
	e := terminalEnv(t)
	own := e.CreateFinishedRun()
	otherRun, _ := e.FinishedJobRun("p2")
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	start := func(project string) string {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80&project="+project, headers)
		var shell struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &shell); status != http.StatusCreated || err != nil {
			t.Fatalf("starting a shell of %s: status %d, %v: %s", project, status, err, body)
		}
		return shell.ID
	}
	get := func(id, name, query string, into any) (int, []byte) {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells/"+id+"/"+name+query, headers)
		if status == http.StatusOK {
			if err := json.Unmarshal(body, into); err != nil {
				t.Fatalf("%s: %v: %s", name, err, body)
			}
		}
		return status, body
	}

	// The first shell is the one the page attaches to; printf prints the marks of a
	// command that failed, as a shell with integration would.
	start("p2")
	page := openTerminalPage(t, base)
	page.typeLine(`printf '\033]133;C;cmdline_url=make%%20test\007\033]133;D;2\007done-%s\n' p2`)
	page.await("done-p2")
	mine := start(own.Project)

	var commands struct {
		Commands []struct {
			Command string `json:"command"`
			Exit    *int   `json:"exit"`
		} `json:"commands"`
		Project  string   `json:"project"`
		Projects []string `json:"projects"`
	}
	for deadline := time.Now().Add(terminalWait); len(commands.Commands) == 0; time.Sleep(50 * time.Millisecond) {
		if status, body := get(mine, "commands", "?project=p2", &commands); status != http.StatusOK {
			t.Fatalf("commands of p2: status %d: %s", status, body)
		}
		if time.Now().After(deadline) {
			t.Fatal("no command of p2 was listed")
		}
	}
	if got := commands.Commands[0]; got.Command != "make test" || got.Exit == nil || *got.Exit != 2 || commands.Project != "p2" || !slices.Equal(commands.Projects, []string{own.Project, "p2"}) {
		t.Errorf("commands of p2 = %+v", commands)
	}
	var ownCommands struct {
		Commands []struct{} `json:"commands"`
		Project  string     `json:"project"`
	}
	get(mine, "commands", "", &ownCommands)
	if ownCommands.Project != own.Project || len(ownCommands.Commands) != 0 {
		t.Errorf("the shell's own commands = %+v", ownCommands)
	}

	var dirs terminalDirsAnswer
	if status, body := get(mine, "dirs", "?project=p2", &dirs); status != http.StatusOK || len(dirs.Runs) != 1 || dirs.Runs[0].Run != otherRun {
		t.Errorf("dirs of p2: status %d, runs %+v: %s", status, dirs.Runs, body)
	}

	for _, name := range []string{"dirs", "commands"} {
		for _, project := range []string{"nope", "..", `a\b`, "a/b"} {
			var ignored map[string]any
			if status, body := get(mine, name, "?project="+project, &ignored); status != http.StatusNotFound {
				t.Errorf("%s of project %q: status %d: %s, want 404", name, project, status, body)
			}
		}
	}
}
