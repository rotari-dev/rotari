package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestPagesKeepPlaceholderTextInValues names a project and a job's command
// after the pages' own template placeholders. A static export's script once
// failed to parse, because a log went into the command, and the Recent jobs
// table went into the sidebar.
func TestPagesKeepPlaceholderTextInValues(t *testing.T) {
	covers(t, "WEB-26")
	const project = "__ROTARI_CONTENT__"
	const planted = "__ROTARI_STATIC_LOGS_DATA__"
	e := support.NewEnv(t)
	e.MustRotari("add", "-p", project, "--", "echo", planted)
	e.MustRotari("run", "-p", project, "--quiet")

	export := filepath.Join(e.Root, "export")
	e.MustRotari("web", "--static-dir", export)
	index, err := os.ReadFile(filepath.Join(export, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"window.__ROTARI_STATIC_LOGS__ = {", `"echo","` + planted + `"`} {
		if !strings.Contains(string(index), want) {
			t.Errorf("static export lacks %q", want)
		}
	}

	got := e.HTTPGet(e.StartWeb() + "/jobs/")
	if got.Status != 200 {
		t.Fatalf("GET /jobs/: status %d", got.Status)
	}
	sidebar, _, found := strings.Cut(got.Body, `<span class="crumb-here">`)
	if !found || strings.Contains(sidebar, `class="jobs-section"`) || !strings.Contains(sidebar, "project/"+project+`"`) {
		t.Errorf("the Recent jobs table is not outside a sidebar that names %s:\n%s", project, got.Body)
	}
	if !strings.Contains(got.Body, `class="jobs-section"`) {
		t.Errorf("the Recent jobs page lists no jobs:\n%s", got.Body)
	}
}
