package webui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/terminal"
)

// A shell's dirs and commands name the projects of its basedir, and with a project
// parameter answer for that project's group instead of the shell's own.
func TestTerminalDirsAndCommandsOfAnotherProject(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, terminalWorkspaceBaseDir)
	dirQ := filepath.Join(root, "q")
	if err := os.MkdirAll(dirQ, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTerminalDirRun(t, baseDir, "p", 1, &root)
	runQ := writeTerminalDirRun(t, baseDir, "q", 1, &dirQ)
	otherBase := filepath.Join(root, "other-state")
	dirZ := filepath.Join(root, "z")
	if err := os.MkdirAll(dirZ, 0o755); err != nil {
		t.Fatal(err)
	}
	runZ := writeTerminalDirRun(t, otherBase, "z", 1, &dirZ)
	server, _ := terminalServerWith(t, "/bin/sh", Options{BaseDir: baseDir, BaseDirs: []string{otherBase}})

	start := func(query string) terminal.ShellInfo {
		t.Helper()
		status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80"+query)
		var shell terminal.ShellInfo
		if err := json.Unmarshal([]byte(body), &shell); status != http.StatusCreated || err != nil {
			t.Fatalf("starting a shell: status %d, %v: %s", status, err, body)
		}
		return shell
	}
	get := func(id, name, query string, into any) (int, string) {
		t.Helper()
		status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath+"/"+id+"/"+name+query)
		if status == http.StatusOK {
			if err := json.Unmarshal([]byte(body), into); err != nil {
				t.Fatalf("%s: %v: %s", name, err, body)
			}
		}
		return status, body
	}

	shellP := start("&project=p")
	shellQ := start("&project=q")
	client := dialTerminalShell(t, server, shellQ.ID)
	// printf prints the marks a shell with integration would, so the Keeper hears them.
	client.typeLine(`printf '\033]133;C;cmdline_url=make%%20test\007\033]133;D;2\007done-%s\n' q`)
	client.await("done-q")

	var commands terminalCommandsResponse
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if status, body := get(shellP.ID, terminalCommandsName, "?project=q", &commands); status != http.StatusOK {
			t.Fatalf("commands of q: status %d: %s", status, body)
		}
		if len(commands.Commands) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no command of q was recorded")
		}
	}
	if got := commands.Commands[0]; got.Command != "make test" || got.Exit == nil || *got.Exit != 2 {
		t.Errorf("q's command = %+v", got)
	}
	if commands.Project != "q" || !slices.Equal(commands.Projects, []string{"p", "q"}) {
		t.Errorf("project %q of %q", commands.Project, commands.Projects)
	}
	var own terminalCommandsResponse
	get(shellP.ID, terminalCommandsName, "", &own)
	if own.Project != "p" || len(own.Commands) != 0 {
		t.Errorf("p's own commands = %+v", own)
	}

	var dirs terminalDirsResponse
	if status, body := get(shellP.ID, terminalDirsName, "?project=q", &dirs); status != http.StatusOK {
		t.Fatalf("dirs of q: status %d: %s", status, body)
	}
	if dirs.Project != "q" || len(dirs.Runs) != 1 || dirs.Runs[0].Run != runQ || dirs.Runs[0].Dir != dirQ {
		t.Errorf("dirs of q = %+v", dirs)
	}
	for _, name := range []string{terminalDirsName, terminalCommandsName} {
		for _, project := range []string{"nope", "..", `a\b`, "a/b"} {
			var ignored map[string]any
			if status, body := get(shellP.ID, name, "?project="+project, &ignored); status != http.StatusNotFound {
				t.Errorf("%s of project %q: status %d: %s", name, project, status, body)
			}
		}
	}

	// Another basedir, by its ID: its own shells first, then its projects.
	otherID := basedirID(otherBase)
	var otherDirs terminalDirsResponse
	if status, body := get(shellP.ID, terminalDirsName, "?basedir="+otherID, &otherDirs); status != http.StatusOK {
		t.Fatalf("dirs of the other basedir: status %d: %s", status, body)
	}
	if otherDirs.Basedir != otherID || otherDirs.Project != "" || !slices.Equal(otherDirs.Projects, []string{"z"}) || len(otherDirs.Basedirs) != 2 {
		t.Errorf("dirs of the other basedir = %+v", otherDirs)
	}
	if status, body := get(shellP.ID, terminalDirsName, "?basedir="+otherID+"&project=z", &otherDirs); status != http.StatusOK ||
		otherDirs.Project != "z" || len(otherDirs.Runs) != 1 || otherDirs.Runs[0].Run != runZ {
		t.Errorf("dirs of z: status %d, %+v: %s", status, otherDirs, body)
	}
	// A project of the other basedir is not one of the shell's own basedir's.
	var ignored map[string]any
	if status, body := get(shellP.ID, terminalDirsName, "?project=z", &ignored); status != http.StatusNotFound {
		t.Errorf("z in the shell's own basedir: status %d: %s", status, body)
	}
	if status, body := get(shellP.ID, terminalCommandsName, "?basedir=nope", &ignored); status != http.StatusNotFound {
		t.Errorf("an unknown basedir: status %d: %s", status, body)
	}
	// The own basedir's own shells, named by an empty project.
	var basedirOwn terminalCommandsResponse
	if status, body := get(shellP.ID, terminalCommandsName, "?basedir="+basedirID(baseDir)+"&project=", &basedirOwn); status != http.StatusOK || basedirOwn.Project != "" || basedirOwn.Basedir != basedirID(baseDir) {
		t.Errorf("the basedir's own shells: status %d, %+v: %s", status, basedirOwn, body)
	}

	// A shell of no project has no other projects to read.
	none := start("")
	if status, body := get(none.ID, terminalCommandsName, "?project=p", &ignored); status != http.StatusBadRequest {
		t.Errorf("project of a shell of no project: status %d: %s", status, body)
	}
}
