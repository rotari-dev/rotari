package resolution

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// readOnlyCommands only read state, so they say when the basedir given to
// them does not exist (RES-28). wait is silent without a selector (RES-16).
var readOnlyCommands = []string{"check", "show", "export", "jobs", "runs", "notifications", "projects", "lineage"}

// TestMissingGivenBasedirIsSaid checks that each command that only reads
// state warns, naming the basedir and its source, when the basedir given on
// the command line, in ROTARI_BASEDIR, or in config does not exist, and
// otherwise behaves as for an existing basedir without state, with the same
// exit status. An existing basedir without state, the implicit default before
// its first run, and wait say nothing, and a command that writes state still
// creates its basedir.
func TestMissingGivenBasedirIsSaid(t *testing.T) {
	covers(t, "RES-28")
	e := support.NewEnv(t)
	missing := filepath.Join(e.Root, "mistyped-state")
	fresh := filepath.Join(e.Root, "fresh-state")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	globalConfig := filepath.Join(e.Root, "config", "rotari", "config.toml")
	warning := "WARNING: basedir " + missing + " does not exist (from "
	for _, command := range readOnlyCommands {
		t.Run(command, func(t *testing.T) {
			empty := e.Rotari(command, "-b", fresh)
			if strings.Contains(empty.Stderr, "does not exist (from") {
				t.Errorf("%s on an existing basedir without state: %s", command, empty)
			}
			got := e.Rotari(command, "-b", missing)
			if got.Code != empty.Code || !strings.Contains(got.Stderr, warning+"--basedir)") {
				t.Errorf("%s -b MISSING: %s; want exit %d and the warning", command, got, empty.Code)
			}
			if help := e.Rotari(command, "-b", missing, "--help"); help.Code != 0 || strings.Contains(help.Stderr, "does not exist") {
				t.Errorf("%s -b MISSING --help: %s", command, help)
			}
		})
	}
	if got := e.WithVar("ROTARI_BASEDIR", missing).Rotari("check"); !strings.Contains(got.Stderr, warning+"ROTARI_BASEDIR)") {
		t.Errorf("check with a missing ROTARI_BASEDIR: %s", got)
	}
	if err := os.MkdirAll(filepath.Dir(globalConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalConfig, []byte("basedir = \""+missing+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := e.Without("ROTARI_BASEDIR").Rotari("check"); !strings.Contains(got.Stderr, warning+"global config "+globalConfig+")") {
		t.Errorf("check with a missing config basedir: %s", got)
	}
	if err := os.Remove(globalConfig); err != nil {
		t.Fatal(err)
	}
	if got := e.Rotari("wait", "-b", missing); got.Code != 0 || strings.TrimSpace(got.Stderr) != "" {
		t.Errorf("wait without a selector must stay silent: %s", got)
	}

	// The implicit default before its first run is an empty basedir.
	implicit := e.Without("ROTARI_BASEDIR").In(fresh)
	for _, command := range []string{"check", "jobs", "projects", "runs"} {
		if got := implicit.Rotari(command); strings.Contains(got.Stderr, "does not exist") {
			t.Errorf("%s with the implicit default before its first run: %s", command, got)
		}
	}

	// A command that writes state creates the basedir it is given.
	created := filepath.Join(e.Root, "new-state")
	e.MustRotari("add", "-b", created, "--", "true")
	if _, err := os.Stat(created); err != nil {
		t.Errorf("add did not create its basedir: %v", err)
	}
}

// TestRemovedRegisteredBasedirIsReported checks that commands reading every
// registered basedir warn of one whose directory is gone and still list the
// others, and that the Web UI marks it missing and answers its project list
// with an error naming it, while the server's own basedir before its first
// run keeps its default project (RES-28).
func TestRemovedRegisteredBasedirIsReported(t *testing.T) {
	covers(t, "RES-28")
	e := support.NewEnv(t)
	kept := filepath.Join(e.Root, "kept-state")
	gone := filepath.Join(e.Root, "gone-state")
	for _, baseDir := range []string{kept, gone} {
		e.MustRotari("add", "-b", baseDir, "-p", filepath.Base(baseDir), "--", "true")
		e.MustRotari("run", "-b", baseDir, "-p", filepath.Base(baseDir), "--quiet")
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	aggregate := e.Without("ROTARI_BASEDIR")
	for _, command := range []string{"jobs", "projects", "runs", "notifications"} {
		got := aggregate.Rotari(command)
		if got.Code != 0 || !strings.Contains(got.Stderr, "registered basedir "+gone+" does not exist") || !strings.Contains(got.Stderr, "rotari gc") {
			t.Errorf("%s with a removed registered basedir: %s", command, got)
		}
		if command != "notifications" && !strings.Contains(got.Stdout, "kept-state") {
			t.Errorf("%s must still list the other basedirs: %s", command, got)
		}
	}

	server := e.WithVar("ROTARI_BASEDIR", filepath.Join(e.Root, "server-state"))
	base := server.StartWeb()
	response := server.HTTPGet(base + "/api/basedirs")
	var listed struct {
		Basedirs []struct {
			ID      string `json:"id"`
			Path    string `json:"path"`
			Current bool   `json:"current"`
			Missing bool   `json:"missing"`
		} `json:"basedirs"`
	}
	if err := json.Unmarshal([]byte(response.Body), &listed); err != nil {
		t.Fatalf("/api/basedirs: %v: %s", err, response.Body)
	}
	ids := map[string]string{}
	for _, entry := range listed.Basedirs {
		ids[entry.Path] = entry.ID
		if want := entry.Path == gone; entry.Missing != want {
			t.Errorf("basedir %s missing = %t, want %t", entry.Path, entry.Missing, want)
		}
		if entry.Current {
			if got := server.HTTPGet(base + "/api/projects?basedir_id=" + url.QueryEscape(entry.ID)); got.Status != 200 || !strings.Contains(got.Body, `"default"`) {
				t.Errorf("projects of the server's basedir before its first run: %d %s", got.Status, got.Body)
			}
		}
	}
	if ids[gone] == "" || ids[kept] == "" {
		t.Fatalf("/api/basedirs = %s, want the kept and the removed basedir", response.Body)
	}
	if got := server.HTTPGet(base + "/api/projects?basedir_id=" + url.QueryEscape(ids[gone])); got.Status != 404 || !strings.Contains(got.Body, "basedir "+gone+" does not exist") {
		t.Errorf("projects of the removed basedir: %d %s", got.Status, got.Body)
	}
	if got := server.HTTPGet(base + "/api/projects?basedir_id=" + url.QueryEscape(ids[kept])); got.Status != 200 || !strings.Contains(got.Body, "kept-state") {
		t.Errorf("projects of the kept basedir: %d %s", got.Status, got.Body)
	}
	if page := server.HTTPGet(base + "/jobs/"); !strings.Contains(page.Body, `class="basedir-missing"`) {
		t.Errorf("the jobs page sidebar must mark the removed basedir missing")
	}
}
