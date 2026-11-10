package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/terminal"
)

// Each project with shells says what its last run is doing: a fixed failure is not
// counted once a later run succeeds, a new failure is, a project without runs says so,
// one whose runs cannot be read says why, and shells without a project have no entry.
func TestTerminalProjectsSayWhatTheLastRunIsDoing(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	writeTestJobsRun(t, baseDir, "fixed", "run-1", "build", day, day.Add(time.Minute), 1)
	writeTestJobsRun(t, baseDir, "fixed", "run-2", "build", day.Add(time.Hour), day.Add(time.Hour+time.Minute), 0)
	writeTestJobsRun(t, baseDir, "broke", "run-1", "build", day, day.Add(2*time.Minute), 1)
	// A failed run's summary says so with its exit code, which the fixture leaves out.
	brokeSummary := filepath.Join(baseDir, "projects", "broke", "runs", "run-1", "summary.json")
	summary, err := stateinternal.LoadRunSummary(brokeSummary)
	if err != nil {
		t.Fatal(err)
	}
	summary.ExitCode = 1
	if err := stateinternal.WriteJSON(brokeSummary, summary); err != nil {
		t.Fatal(err)
	}
	// A run records itself as its project's last as it starts.
	for project, runID := range map[string]string{"fixed": "run-2", "broke": "run-1"} {
		paths, err := stateinternal.ResolveProjectPaths(baseDir, project)
		if err != nil {
			t.Fatal(err)
		}
		if err := stateinternal.WriteJSON(paths.MetaFile, model.Meta{LastRunID: runID}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "projects", "fresh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(filepath.Join(baseDir, "projects", "unreadable", "runs"), []byte("not a directory")); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	keeper, err := terminal.NewKeeper(terminal.KeeperOptions{
		Shell:     terminal.Options{Shell: "/bin/sh", Dir: home, Env: []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "PS1=$ "}},
		IdleLimit: time.Hour, RingSize: 1 << 16, ShellLimit: 8, GroupLimit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	for _, group := range []string{"fixed", "broke", "fixed", "fresh", "unreadable", ""} {
		if group != "" {
			group = baseDir + "\x00" + group
		}
		if _, err := keeper.Start(terminal.StartOptions{Rows: 24, Cols: 80, Group: group, Dir: home}); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(WithAuthToken(Handler(Options{BaseDir: baseDir, Terminal: keeper}), terminalTestToken, AuthOptions{}))
	t.Cleanup(server.Close)

	status, body := terminalRequest(t, server, http.MethodGet, terminalProjectsPath)
	var listed struct {
		Projects []terminalProjectResponse `json:"projects"`
		Now      time.Time                 `json:"now"`
	}
	if err := json.Unmarshal([]byte(body), &listed); status != http.StatusOK || err != nil || listed.Now.IsZero() {
		t.Fatalf("projects: status %d, %v: %s", status, err, body)
	}
	byName := map[string]terminalProjectResponse{}
	for _, entry := range listed.Projects {
		if _, twice := byName[entry.Project]; twice || entry.BaseDir != baseDir {
			t.Errorf("project %q is listed twice or in the wrong basedir: %s", entry.Project, body)
		}
		byName[entry.Project] = entry
	}
	if len(byName) != 4 {
		t.Errorf("listed %d projects, want fixed, broke, fresh, and unreadable: %s", len(byName), body)
	}
	at := func(value time.Time) *time.Time { return &value }
	for name, want := range map[string]terminalProjectResponse{
		"fixed": {BaseDir: baseDir, Project: "fixed", Runs: 2, LastRun: &terminalLastRunSummary{RunID: "run-2", Lifecycle: "finished", Tone: "ok", StartedAt: at(day.Add(time.Hour)), FinishedAt: at(day.Add(time.Hour + time.Minute)), Jobs: map[string]int{"ok": 1}}},
		"broke": {BaseDir: baseDir, Project: "broke", Runs: 1, LastRun: &terminalLastRunSummary{RunID: "run-1", Lifecycle: "failed", Tone: "bad", StartedAt: at(day), FinishedAt: at(day.Add(2 * time.Minute)), Jobs: map[string]int{"bad": 1}}},
		"fresh": {BaseDir: baseDir, Project: "fresh"},
	} {
		got := byName[name]
		if got.LastRun != nil && want.LastRun != nil {
			for _, times := range [][2]*time.Time{{got.LastRun.StartedAt, want.LastRun.StartedAt}, {got.LastRun.FinishedAt, want.LastRun.FinishedAt}} {
				if times[0] == nil || !times[0].Equal(*times[1]) {
					t.Errorf("%s: last run times %v, want %v", name, times[0], *times[1])
				}
			}
			got.LastRun.StartedAt, got.LastRun.FinishedAt = want.LastRun.StartedAt, want.LastRun.FinishedAt
		}
		if !reflect.DeepEqual(got, want) {
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)
			t.Errorf("%s:\n got %s\nwant %s", name, gotJSON, wantJSON)
		}
	}
	if unreadable := byName["unreadable"]; unreadable.StatusError == "" || unreadable.LastRun != nil || unreadable.Runs != 0 {
		t.Errorf("a project whose runs cannot be read does not say why: %+v", unreadable)
	}
	if status, body := terminalRequest(t, server, http.MethodPost, terminalProjectsPath); status != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d: %s", status, body)
	}
	if !strings.Contains(body, `"jobs":{"ok":1}`) {
		t.Errorf("the counts are not keyed by the Jobs card's groups: %s", body)
	}
}
