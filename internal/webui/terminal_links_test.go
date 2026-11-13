package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/model"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/terminal"
)

// askTerminalLinks posts candidates as the panel does and returns the status and body.
func askTerminalLinks(t *testing.T, server *httptest.Server, id, contentType, body string) (int, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, server.URL+terminalShellsPath+"/"+id+"/"+terminalLinksName, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Rotari-Token", terminalTestToken)
	request.Header.Set("Content-Type", contentType)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	text, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(text)
}

func terminalLinksOf(t *testing.T, server *httptest.Server, id string, candidates ...string) terminalLinksResponse {
	t.Helper()
	body, err := json.Marshal(terminalLinksRequest{Candidates: candidates})
	if err != nil {
		t.Fatal(err)
	}
	status, text := askTerminalLinks(t, server, id, "application/json", string(body))
	var response terminalLinksResponse
	if err := json.Unmarshal([]byte(text), &response); status != http.StatusOK || err != nil {
		t.Fatalf("links: status %d, %v: %s", status, err, text)
	}
	return response
}

// writeTerminalLinkJobs writes the jobs of a run's snapshot, or of the project's queue
// with an empty runID.
func writeTerminalLinkJobs(t *testing.T, baseDir, project, runID string, commands ...model.QueuedCommand) {
	t.Helper()
	paths, err := stateinternal.ResolveProjectPaths(baseDir, project)
	if err != nil {
		t.Fatal(err)
	}
	file := paths.QueueFile
	if runID != "" {
		file = filepath.Join(paths.RunsDir, runID, "commands.json")
	}
	if err := stateinternal.WriteJSON(file, model.Queue{Commands: commands}); err != nil {
		t.Fatal(err)
	}
}

// Which words of a shell's output are links: run IDs of any project of the shell's basedir
// (of any basedir for a shell of no project), job IDs of the shell's project in its newest
// runs or queue, and paths that exist, absolute or where the shell is.
func TestTerminalLinksAreWhatExists(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	baseDir, otherBase := filepath.Join(root, "base"), filepath.Join(root, "other")
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(filepath.Join(work, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "sub", "x.csv"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	older := writeTerminalDirRun(t, baseDir, "p", 1, &work)
	newer := writeTerminalDirRun(t, baseDir, "p", 2, &work)
	inQ := writeTerminalDirRun(t, baseDir, "q", 3, &work)
	inOther := writeTerminalDirRun(t, otherBase, "r", 4, &work)
	writeTerminalLinkJobs(t, baseDir, "p", newer, model.QueuedCommand{ID: "aaaaaaaa1", Command: []string{"true"}})
	writeTerminalLinkJobs(t, baseDir, "p", older, model.QueuedCommand{ID: "bbbbbbbb2", Command: []string{"true"}, Array: &model.ArraySpec{First: 1, Last: 3}})
	writeTerminalLinkJobs(t, baseDir, "p", "", model.QueuedCommand{ID: "ccccccccc", Command: []string{"true"}})
	writeTerminalLinkJobs(t, baseDir, "q", inQ, model.QueuedCommand{ID: "eeeeeeeee", Command: []string{"true"}})
	server, _ := terminalServerWith(t, "/bin/sh", Options{BaseDir: baseDir, BaseDirs: []string{otherBase}})
	start := func(query string) string {
		t.Helper()
		status, body := terminalRequest(t, server, http.MethodPost, terminalShellsPath+"?rows=24&cols=80"+query)
		var shell terminal.ShellInfo
		if err := json.Unmarshal([]byte(body), &shell); status != http.StatusCreated || err != nil {
			t.Fatalf("starting a shell: status %d, %v: %s", status, err, body)
		}
		return shell.ID
	}

	project := start("&project=p")
	got := terminalLinksOf(t, server, project,
		newer, inQ, "20260101-000000-deadbeef",
		"aaaaaaaa1", "bbbbbbbb2-3", "ccccccccc", "ddddddddd", "eeeeeeeee",
		"sub/x.csv", "sub/x.csv:12:3", "./sub", work, "/nonexistent/zz", "hello", newer)
	want := []terminalLink{
		{Text: newer, Kind: "run", Project: "p", Link: "/project/p/run/" + newer},
		{Text: inQ, Kind: "run", Project: "q", Link: "/project/q/run/" + inQ},
		{Text: "aaaaaaaa1", Kind: "job", Project: "p", Link: "/project/p/run/" + newer + "?job_id=aaaaaaaa1"},
		{Text: "bbbbbbbb2-3", Kind: "job", Project: "p", Link: "/project/p/run/" + older + "?job_id=bbbbbbbb2-3"},
		{Text: "ccccccccc", Kind: "job", Project: "p", Link: "/project/p"},
		{Text: "sub/x.csv", Kind: "file", Path: filepath.Join(work, "sub", "x.csv")},
		{Text: "sub/x.csv:12:3", Kind: "file", Path: filepath.Join(work, "sub", "x.csv")},
		{Text: "./sub", Kind: "dir", Path: filepath.Join(work, "sub")},
		{Text: work, Kind: "dir", Path: work},
	}
	if !reflect.DeepEqual(got.Links, want) || got.DirError != "" || len(got.Problems) != 0 {
		t.Errorf("links of a project's shell:\n got %+v\nwant %+v", got, want)
	}

	// A shell of no project finds runs in every basedir, and no jobs.
	none := start("")
	got = terminalLinksOf(t, server, none, inOther, newer, "aaaaaaaa1")
	want = []terminalLink{
		{Text: inOther, Kind: "run", Project: "r", Link: basedirRoutePrefix + basedirID(otherBase) + "/project/r/run/" + inOther},
		{Text: newer, Kind: "run", Project: "p", Link: "/project/p/run/" + newer},
	}
	if !reflect.DeepEqual(got.Links, want) {
		t.Errorf("links of a shell of no project:\n got %+v\nwant %+v", got.Links, want)
	}

	// Only the newest 20 runs are searched for a job.
	for minute := 10; minute < 10+terminalLinkJobRunsRead; minute++ {
		writeTerminalDirRun(t, baseDir, "p", minute, &work)
	}
	if got := terminalLinksOf(t, server, project, "aaaaaaaa1").Links; len(got) != 0 {
		t.Errorf("a job beyond the newest 20 runs linked: %+v", got)
	}

	for name, test := range map[string]struct {
		id, contentType, body string
		want                  int
	}{
		"too many":       {project, "application/json", `{"candidates":[` + strings.Repeat(`"a",`, terminalLinksLimit) + `"a"]}`, http.StatusBadRequest},
		"too long":       {project, "application/json", `{"candidates":["` + strings.Repeat("a", terminalLinkLimit+1) + `"]}`, http.StatusBadRequest},
		"unknown fields": {project, "application/json", `{"candidates":[],"more":1}`, http.StatusBadRequest},
		"not JSON":       {project, "text/plain", `{"candidates":[]}`, http.StatusUnsupportedMediaType},
		"unknown shell":  {"nope", "application/json", `{"candidates":[]}`, http.StatusNotFound},
	} {
		if status, body := askTerminalLinks(t, server, test.id, test.contentType, test.body); status != test.want {
			t.Errorf("%s: status %d: %s, want %d", name, status, body, test.want)
		}
	}
	if status, body := terminalRequest(t, server, http.MethodGet, terminalShellsPath+"/"+project+"/"+terminalLinksName); status != http.StatusMethodNotAllowed {
		t.Errorf("GET: status %d: %s", status, body)
	}
}
