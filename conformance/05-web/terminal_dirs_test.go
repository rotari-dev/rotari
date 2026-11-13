package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

type terminalDirsAnswer struct {
	History []struct {
		Dir       string    `json:"dir"`
		EnteredAt time.Time `json:"entered_at"`
	} `json:"history"`
	Runs []struct {
		Dir       string     `json:"dir"`
		Run       string     `json:"run"`
		StartedAt *time.Time `json:"started_at"`
		Missing   bool       `json:"missing"`
		Error     string     `json:"error"`
	} `json:"runs"`
	RunsError string `json:"runs_error"`
}

// A project's shells share where they have been, seen at the prompt after Enter, and
// are offered where the project's runs started, only where the terminal is served.
func TestTerminalRecentDirectories(t *testing.T) {
	covers(t, "WEB-45")
	e := terminalEnv(t)
	run := e.CreateFinishedRun()
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	start := func() (id, dir string) {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80&project="+run.Project, headers)
		var shell struct {
			ID  string `json:"id"`
			Dir string `json:"dir"`
		}
		if err := json.Unmarshal(body, &shell); status != http.StatusCreated || err != nil {
			t.Fatalf("starting a shell of %s: status %d, %v: %s", run.Project, status, err, body)
		}
		return shell.ID, shell.Dir
	}
	dirs := func(id string) terminalDirsAnswer {
		t.Helper()
		status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells/"+id+"/dirs", headers)
		var answer terminalDirsAnswer
		if err := json.Unmarshal(body, &answer); status != http.StatusOK || err != nil {
			t.Fatalf("dirs: status %d, %v: %s", status, err, body)
		}
		return answer
	}
	awaitHistory := func(id string, want ...string) terminalDirsAnswer {
		t.Helper()
		for deadline := time.Now().Add(terminalWait); ; time.Sleep(50 * time.Millisecond) {
			answer := dirs(id)
			var got []string
			for _, entry := range answer.History {
				got = append(got, entry.Dir)
			}
			if slices.Equal(got, want) {
				return answer
			}
			if time.Now().After(deadline) {
				t.Fatalf("history = %q, want %q", got, want)
			}
		}
	}

	first, runDir := start()
	realRunDir, err := filepath.EvalSymlinks(runDir)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(e.Root, "elsewhere dir")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	realElsewhere, err := filepath.EvalSymlinks(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	page := openTerminalPage(t, base)
	page.typeLine(`cd '` + elsewhere + `' && echo "moved-$((1+1))"`)
	page.await("moved-2")
	answer := awaitHistory(first, realElsewhere, realRunDir)
	if len(answer.Runs) != 1 || answer.Runs[0].Run != run.RunID || answer.Runs[0].Dir != runDir || answer.Runs[0].StartedAt == nil ||
		answer.Runs[0].Missing || answer.Runs[0].Error != "" || answer.RunsError != "" {
		t.Errorf("runs = %+v, runs_error %q; want %s started in %s", answer.Runs, answer.RunsError, run.RunID, runDir)
	}

	// Another shell of the project shares the history, where it starts moving to the front.
	second, _ := start()
	awaitHistory(second, realRunDir, realElsewhere)

	if status, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells/77/dirs", headers); status != http.StatusNotFound {
		t.Errorf("a shell not running: status %d: %s, want 404", status, body)
	}
	if status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells/"+first+"/dirs", headers); status != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d: %s, want 405", status, body)
	}
	plain := support.NewEnv(t).StartWeb()
	if got := e.HTTPDo(http.MethodGet, plain+"/_terminal/shells/1/dirs", nil, ""); got.Status != http.StatusNotFound {
		t.Errorf("dirs without the terminal: (%d, %s), want 404", got.Status, got.Body)
	}
}
