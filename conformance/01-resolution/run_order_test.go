package resolution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestOneRunOrder checks that the commands and the Web UI agree on which run
// is newest (RES-29): meta.json's last run, then the start second in the run
// ID, then, for runs begun in the same second, the order their coordinators
// started in. Name order and directory times disagree with it here.
func TestOneRunOrder(t *testing.T) {
	covers(t, "RES-29", "RES-15")
	e := support.NewEnv(t)
	const project = "order"
	runsDir := filepath.Join(e.Base, "projects", project, "runs")
	// Runs 1 and 2 share a second; by name, run 2 sorts first.
	names := []string{"20260101-000000-ffffffff", "20260101-000000-00000000", "20260101-000001-aaaaaaaa"}
	for _, name := range names {
		runID, _ := e.FinishedJobRun(project)
		renameRun(t, runsDir, runID, name)
	}
	// Run 1's directory has the newest modification time.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(runsDir, names[0]), future, future); err != nil {
		t.Fatal(err)
	}
	newestFirst := []string{names[2], names[1], names[0]}

	writeLastRunID(t, e, project, "")
	if got := showRunID(t, e.MustRotari("show", "-p", project, "--json").Stdout); got != names[2] {
		t.Errorf("show without a run chose %q, want %q", got, names[2])
	}
	if got := showRunID(t, e.MustRotari("show", "-p", project, "-r", "latest", "--json").Stdout); got != names[2] {
		t.Errorf("show -r latest chose %q, want %q", got, names[2])
	}

	web := e.StartWeb()
	var index struct {
		Projects []struct {
			Name     string   `json:"project_name"`
			RunOrder []string `json:"run_order"`
			Runs     []struct {
				RunID string `json:"run_id"`
			} `json:"runs"`
		} `json:"projects"`
	}
	got := e.HTTPGet(web + "/api/state")
	if err := json.Unmarshal([]byte(got.Body), &index); got.Status != 200 || err != nil || len(index.Projects) != 1 {
		t.Fatalf("GET /api/state: status %d: %v: %s", got.Status, err, got.Body)
	}
	shown := []string{}
	for _, run := range index.Projects[0].Runs {
		shown = append(shown, run.RunID)
	}
	if !reflect.DeepEqual(shown, newestFirst[:2]) || !reflect.DeepEqual(index.Projects[0].RunOrder, newestFirst) {
		t.Errorf("Web index runs %v and run_order %v; want the two newest of %v", shown, index.Projects[0].RunOrder, newestFirst)
	}
	var full struct {
		RunOrder []string `json:"run_order"`
	}
	got = e.HTTPGet(web + "/api/project?project_name=" + project)
	if err := json.Unmarshal([]byte(got.Body), &full); got.Status != 200 || err != nil || !reflect.DeepEqual(full.RunOrder, newestFirst) {
		t.Errorf("GET /api/project run_order %v (status %d, %v); want %v", full.RunOrder, got.Status, err, newestFirst)
	}

	// Deleting the last run makes the next newest the last run.
	writeLastRunID(t, e, project, names[2])
	e.MustRotari("delete", "-p", project, "-r", names[2])
	data, err := os.ReadFile(filepath.Join(e.Base, "projects", project, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		LastRunID string `json:"last_run_id"`
	}
	if err := json.Unmarshal(data, &meta); err != nil || meta.LastRunID != names[1] {
		t.Errorf("last run after delete = %q (%v), want %q", meta.LastRunID, err, names[1])
	}
}

// renameRun gives a finished run another ID, rewriting the ID in its files.
func renameRun(t *testing.T, runsDir, from, to string) {
	t.Helper()
	target := filepath.Join(runsDir, to)
	if err := os.Rename(filepath.Join(runsDir, from), target); err != nil {
		t.Fatal(err)
	}
	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte(strings.ReplaceAll(string(data), from, to)), info.Mode())
	})
	if err != nil {
		t.Fatal(err)
	}
}
