package web

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// Live pages offer Open in VS Code, read-only or not, and the server says where an
// artifact is under the rules of its previews; a static export offers nothing.
func TestWebOpensArtifactsInVSCode(t *testing.T) {
	covers(t, "WEB-64")
	e := support.NewEnv(t)
	jobID := support.AddedJobID(t, e.MustRotari("add", "-p", "art", "--", "sh", "-c", "echo x > out.txt"))
	e.RotariRun("run", "-p", "art", "--quiet")
	var shown struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(e.MustRotari("show", "-p", "art", "--json").Stdout), &shown); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"--allow-control=false"}} {
		base := e.StartWeb(args...)
		for _, path := range []string{"/", "/jobs/"} {
			if page := e.HTTPGet(base + path); page.Status != 200 || strings.Count(page.Body, `id="vscode-connection" data-host="`) != 1 {
				t.Errorf("web %v: %s = %d does not offer VS Code", args, path, page.Status)
			}
		}
		var listing struct {
			WorkingDirectory string `json:"working_directory"`
		}
		listed := e.HTTPGet(base + "/api/artifacts?" + url.Values{"project_name": {"art"}, "run_id": {shown.RunID}, "job_id": {jobID}}.Encode())
		if err := json.Unmarshal([]byte(listed.Body), &listing); err != nil || listing.WorkingDirectory == "" {
			t.Fatalf("/api/artifacts = %d %s", listed.Status, listed.Body)
		}
		work := listing.WorkingDirectory
		ask := func(entry, child string) support.HTTPResult {
			return e.HTTPGet(base + "/api/vscode-target?" + url.Values{"project_name": {"art"}, "run_id": {shown.RunID}, "job_id": {jobID}, "entry": {entry}, "child": {child}}.Encode())
		}
		for _, test := range []struct{ entry, child, path, kind string }{
			{"0", "", filepath.Join(work, "out.txt"), "file"},
			{"wd", "", work, "directory"},
			{"wd", "out.txt", filepath.Join(work, "out.txt"), "file"},
		} {
			var got struct{ Path, Kind string }
			response := ask(test.entry, test.child)
			if err := json.Unmarshal([]byte(response.Body), &got); err != nil || response.Status != 200 || got.Path != test.path || got.Kind != test.kind {
				t.Errorf("web %v: target %s/%s = %d %s, want %s %s", args, test.entry, test.child, response.Status, response.Body, test.kind, test.path)
			}
		}
		for _, test := range [][2]string{{"wd", "../x"}, {"wd", "nothing"}, {"9", ""}, {"", ""}, {"/etc/passwd", ""}} {
			if response := ask(test[0], test[1]); response.Status < 400 {
				t.Errorf("web %v: target %q/%q = %d %s, want a refusal", args, test[0], test[1], response.Status, response.Body)
			}
		}
	}
	static := filepath.Join(e.Root, "static")
	e.MustRotari("web", "--static-dir", static)
	for _, name := range []string{"index.html", "jobs/index.html"} {
		page, err := os.ReadFile(filepath.Join(static, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(page), `id="vscode-connection"`) {
			t.Errorf("the static %s offers VS Code", name)
		}
	}
}
