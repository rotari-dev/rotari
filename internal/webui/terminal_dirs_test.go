package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/terminal"
)

// writeTerminalDirRun makes a run of project whose ID starts at the given minute, with
// context.json saying it started in cwd; a nil cwd writes no context.
func writeTerminalDirRun(t *testing.T, baseDir, project string, minute int, cwd *string) string {
	t.Helper()
	paths, err := stateinternal.ResolveProjectPaths(baseDir, project)
	if err != nil {
		t.Fatal(err)
	}
	runID := stateinternal.NewRunID(time.Date(2026, 11, 14, 0, minute, 0, 0, time.UTC))
	runDir := filepath.Join(paths.RunsDir, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if cwd != nil {
		if err := stateinternal.WriteJSON(filepath.Join(runDir, "context.json"), model.RunContext{CWD: *cwd}); err != nil {
			t.Fatal(err)
		}
	}
	return runID
}

// The dirs of a shell's group: the Keeper's history, with where the project's newest runs
// started, each directory once and at most five, or the workspace for a basedir's own
// shells; nothing more for shells of no project.
func TestTerminalDirsOfAShellsGroup(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(root, terminalWorkspaceBaseDir)
	dirA, dirB, dirC := filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "c")
	for _, dir := range []string{dirA, dirB, dirC} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	missing, empty := filepath.Join(root, "gone"), ""
	// Oldest first; the newest is dirA, where the project's shells start.
	writeTerminalDirRun(t, baseDir, "p", 1, &dirC)
	runB := writeTerminalDirRun(t, baseDir, "p", 2, &dirB)
	runEmpty := writeTerminalDirRun(t, baseDir, "p", 3, &empty)
	runNone := writeTerminalDirRun(t, baseDir, "p", 4, nil)
	runMissing := writeTerminalDirRun(t, baseDir, "p", 5, &missing)
	writeTerminalDirRun(t, baseDir, "p", 6, &dirA)
	runA := writeTerminalDirRun(t, baseDir, "p", 7, &dirA)
	server, _ := terminalServerAt(t, "/bin/sh", baseDir)

	dirs := func(id string) terminalDirsResponse {
		t.Helper()
		status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath+"/"+id+"/"+terminalDirsName)
		var response terminalDirsResponse
		if err := json.Unmarshal([]byte(body), &response); status != http.StatusOK || err != nil {
			t.Fatalf("dirs: status %d, %v: %s", status, err, body)
		}
		return response
	}
	start := func(query string) terminal.ShellInfo {
		t.Helper()
		status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80"+query)
		var shell terminal.ShellInfo
		if err := json.Unmarshal([]byte(body), &shell); status != http.StatusCreated || err != nil {
			t.Fatalf("starting a shell: status %d, %v: %s", status, err, body)
		}
		return shell
	}
	historyDirs := func(response terminalDirsResponse) []string {
		var dirs []string
		for _, entry := range response.History {
			dirs = append(dirs, entry.Dir)
		}
		return dirs
	}
	awaitHistory := func(id string, want ...string) terminalDirsResponse {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(20 * time.Millisecond) {
			response := dirs(id)
			if slices.Equal(historyDirs(response), want) {
				return response
			}
			if time.Now().After(deadline) {
				t.Fatalf("history = %q, want %q", historyDirs(response), want)
			}
		}
	}

	// A project's shell: its history, and its runs newest first, dirA once, a missing
	// directory marked, unreadable runs with the reason, and dirC, the sixth, left out.
	project := start("&project=p")
	client := dialTerminalShell(t, server, project.ID)
	client.typeLine(`cd '` + dirB + `' && d=$(printf 'x\377y') && mkdir "$d" && cd "$d" && echo "odd-$((1+1))"`)
	client.await("odd-2")
	response := awaitHistory(project.ID, filepath.Join(dirB, "x�y"), dirA)
	if !response.History[0].NotUTF8 || response.History[1].NotUTF8 || response.History[0].Missing || response.History[0].EnteredAt.IsZero() {
		t.Errorf("history entries = %+v", response.History)
	}
	var runs []string
	for _, entry := range response.Runs {
		runs = append(runs, entry.Run+" "+entry.Dir)
	}
	if want := []string{runA + " " + dirA, runMissing + " " + missing, runNone + " ", runEmpty + " ", runB + " " + dirB}; !slices.Equal(runs, want) {
		t.Errorf("runs = %q, want %q", runs, want)
	}
	if len(response.Runs) == 5 {
		if got := response.Runs; got[0].Missing || !got[1].Missing || got[2].Error == "" || !strings.Contains(got[3].Error, "does not say where it started") || got[4].Error != "" {
			t.Errorf("runs = %+v", got)
		}
		if got := response.Runs[0].StartedAt; got == nil || !got.Equal(time.Date(2026, 11, 14, 0, 7, 0, 0, time.UTC)) {
			t.Errorf("the newest run's start = %v", got)
		}
	}
	if response.Workspace != nil || response.RunsError != "" {
		t.Errorf("a project's shell got workspace %+v, runs error %q", response.Workspace, response.RunsError)
	}

	// A basedir's own shell: the workspace, and no runs.
	own := start("&basedir=")
	response = awaitHistory(own.ID, root)
	if response.Workspace == nil || response.Workspace.Dir != root || response.Workspace.Error != "" || len(response.Runs) != 0 {
		t.Errorf("a basedir's shell: workspace %+v, runs %+v", response.Workspace, response.Runs)
	}

	// A shell of no project: its history alone.
	none := start("")
	for deadline := time.Now().Add(15 * time.Second); len(response.History) != 1 || response.History[0].Dir == root; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the history of no project = %+v", response.History)
		}
		response = dirs(none.ID)
	}
	if response.Workspace != nil || len(response.Runs) != 0 || response.RunsError != "" {
		t.Errorf("a shell of no project: %+v", response)
	}

	if status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath+"/nope/"+terminalDirsName); status != http.StatusNotFound {
		t.Errorf("an unknown shell: status %d: %s", status, body)
	}
	if status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"/"+project.ID+"/"+terminalDirsName); status != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d: %s", status, body)
	}
}

// Only the newest 20 runs are read, so a directory used only by older runs is not offered.
func TestTerminalDirsReadTheNewestTwentyRuns(t *testing.T) {
	baseDir := t.TempDir()
	old, recent := t.TempDir(), t.TempDir()
	writeTerminalDirRun(t, baseDir, "p", 0, &old)
	for minute := 1; minute <= terminalDirRunsRead; minute++ {
		writeTerminalDirRun(t, baseDir, "p", minute, &recent)
	}
	server, _ := terminalServerAt(t, "/bin/sh", baseDir)
	status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80&project=p")
	var shell terminal.ShellInfo
	if err := json.Unmarshal([]byte(body), &shell); status != http.StatusCreated || err != nil {
		t.Fatalf("starting a shell: status %d, %v: %s", status, err, body)
	}
	status, body = terminalRequest(t, server, http.MethodGet, terminalShellsPath+"/"+shell.ID+"/"+terminalDirsName)
	var response terminalDirsResponse
	if err := json.Unmarshal([]byte(body), &response); status != http.StatusOK || err != nil {
		t.Fatalf("dirs: status %d, %v: %s", status, err, body)
	}
	if len(response.Runs) != 1 || response.Runs[0].Dir != recent {
		t.Errorf("runs = %s, want only %q", fmt.Sprint(response.Runs), recent)
	}
}
