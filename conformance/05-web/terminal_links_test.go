package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

type terminalLinkAnswer struct {
	Text    string `json:"text"`
	Kind    string `json:"kind"`
	Link    string `json:"link,omitempty"`
	Path    string `json:"path,omitempty"`
	Project string `json:"project,omitempty"`
}

// Which words of a shell's output are links is what exists: a run, a job of the shell's
// project in its runs or queue, or a path, absolute or where the shell is; only where the
// terminal is served.
func TestTerminalOutputLinks(t *testing.T) {
	covers(t, "WEB-46")
	e := terminalEnv(t)
	run := e.CreateFinishedRun()
	queued := support.AddedJobID(t, e.MustRotari("add", "-p", run.Project, "--", "true"))
	base := e.StartWeb("--allow-terminal")
	headers := map[string]string{"X-Rotari-Token": launchToken}

	status, body := terminalShellsRequest(t, http.MethodPost, base+"/_terminal/shells?rows=24&cols=80&project="+run.Project,
		http.Header{"X-Rotari-Token": {launchToken}})
	var shell struct {
		ID  string `json:"id"`
		Dir string `json:"dir"`
	}
	if err := json.Unmarshal(body, &shell); status != http.StatusCreated || err != nil {
		t.Fatalf("starting a shell of %s: status %d, %v: %s", run.Project, status, err, body)
	}
	dir, err := filepath.EvalSymlinks(shell.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "out dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "out dir", "x.csv"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := base + "/_terminal/shells/" + shell.ID + "/links"
	ask := func(candidates ...string) []terminalLinkAnswer {
		t.Helper()
		request, err := json.Marshal(map[string][]string{"candidates": candidates})
		if err != nil {
			t.Fatal(err)
		}
		got := e.HTTPDo(http.MethodPost, links, headers, string(request))
		var answer struct {
			Links    []terminalLinkAnswer `json:"links"`
			DirError string               `json:"dir_error"`
			Problems []string             `json:"problems"`
		}
		if err := json.Unmarshal([]byte(got.Body), &answer); got.Status != http.StatusOK || err != nil {
			t.Fatalf("links: status %d, %v: %s", got.Status, err, got.Body)
		}
		if answer.DirError != "" || len(answer.Problems) != 0 {
			t.Errorf("links: dir_error %q, problems %q", answer.DirError, answer.Problems)
		}
		return answer.Links
	}

	runPage := "/project/" + run.Project + "/run/" + run.RunID
	got := ask(run.RunID, "20200101-000000-deadbeef", run.OKJob, queued, "012345678",
		"out dir/x.csv:3:7", filepath.Join(dir, "out dir"), "nowhere/x", "plain")
	want := []terminalLinkAnswer{
		{Text: run.RunID, Kind: "run", Project: run.Project, Link: runPage},
		{Text: run.OKJob, Kind: "job", Project: run.Project, Link: runPage + "?job_id=" + run.OKJob},
		{Text: queued, Kind: "job", Project: run.Project, Link: "/project/" + run.Project},
		{Text: "out dir/x.csv:3:7", Kind: "file", Path: filepath.Join(dir, "out dir", "x.csv")},
		{Text: filepath.Join(dir, "out dir"), Kind: "dir", Path: filepath.Join(dir, "out dir")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("links:\n got %+v\nwant %+v", got, want)
	}

	for name, test := range map[string]struct {
		headers map[string]string
		body    string
		want    int
	}{
		"too many": {headers, `{"candidates":[` + strings.Repeat(`"a",`, 64) + `"a"]}`, http.StatusBadRequest},
		"too long": {headers, `{"candidates":["` + strings.Repeat("a", 4097) + `"]}`, http.StatusBadRequest},
		"not JSON": {map[string]string{"X-Rotari-Token": launchToken, "Content-Type": "text/plain"}, `{"candidates":[]}`, http.StatusUnsupportedMediaType},
		"no token": {nil, `{"candidates":[]}`, http.StatusForbidden},
	} {
		if got := e.HTTPDo(http.MethodPost, links, test.headers, test.body); got.Status != test.want {
			t.Errorf("%s: (%d, %s), want %d", name, got.Status, got.Body, test.want)
		}
	}
	if got := e.HTTPDo(http.MethodPost, base+"/_terminal/shells/77/links", headers, `{"candidates":[]}`); got.Status != http.StatusNotFound {
		t.Errorf("a shell not running: (%d, %s), want 404", got.Status, got.Body)
	}
	if got := e.HTTPDo(http.MethodGet, links, headers, ""); got.Status != http.StatusMethodNotAllowed {
		t.Errorf("GET: (%d, %s), want 405", got.Status, got.Body)
	}
	plain := support.NewEnv(t).StartWeb()
	if got := e.HTTPDo(http.MethodPost, plain+"/_terminal/shells/1/links", nil, `{"candidates":[]}`); got.Status != http.StatusNotFound {
		t.Errorf("links without the terminal: (%d, %s), want 404", got.Status, got.Body)
	}
}
