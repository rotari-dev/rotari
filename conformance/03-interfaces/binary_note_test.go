package interfaces

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

// anotherBuild copies the rotari under test to a path of its own, which counts as
// another build of the same version.
func anotherBuild(t *testing.T, e *support.Env) string {
	t.Helper()
	source, err := os.Open(e.Command("version").Path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	path := filepath.Join(e.Root, "other", "rotari")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	target, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// runWith runs rotari at binary with e's environment.
func runWith(t *testing.T, e *support.Env, binary string, args ...string) (string, string, int) {
	t.Helper()
	command := e.Command(args...)
	command.Path, command.Args[0] = binary, binary
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), code
}

// TestNotesAnotherRotariBinary checks that wait, attaching to a run whose coordinator
// runs another rotari binary, and the web commands reporting or stopping a detached
// server another binary runs, say so in one note on stderr, naming both binaries, and
// that the same binary says nothing.
func TestNotesAnotherRotariBinary(t *testing.T) {
	covers(t, "CLI-31")
	e := support.NewEnv(t)
	other := anotherBuild(t, e)
	this := e.Command("version").Path
	differentBuild := regexp.QuoteMeta("a different build of ")

	e.MustRotari("add", "-p", "p", "--", "sleep", "300")
	if _, stderr, code := runWith(t, e, other, "run", "-p", "p", "--quiet", "--async"); code != 0 {
		t.Fatalf("run --async with another build: %s", stderr)
	}
	t.Cleanup(func() { _ = e.Command("cancel", "-p", "p", "--wait").Run() })
	var shown struct {
		RunID string `json:"run_id"`
	}
	for deadline := time.Now().Add(15 * time.Second); shown.RunID == "" || !strings.Contains(e.Rotari("jobs", "p", "--format", "%s").Stdout, "running"); time.Sleep(100 * time.Millisecond) {
		_ = json.Unmarshal([]byte(e.Rotari("show", "-p", "p", "--json").Stdout), &shown)
		if time.Now().After(deadline) {
			t.Fatal("the run did not start")
		}
	}
	waited := e.Rotari("wait", "-p", "p", shown.RunID, "--timeout", "1s")
	want := regexp.MustCompile(`Note: run ` + shown.RunID + ` is managed by ` + differentBuild + `rotari \S+.* \(` + regexp.QuoteMeta(other) + `\), not this one \(` + regexp.QuoteMeta(this) + `\)\.`)
	if !want.MatchString(waited.Stderr) {
		t.Errorf("wait did not note the run's other binary: %s", waited)
	}
	if strings.Contains(waited.Stdout, "Note:") {
		t.Errorf("the note went to stdout: %s", waited)
	}
	if _, stderr, _ := runWith(t, e, other, "wait", "-p", "p", shown.RunID, "--timeout", "1s"); strings.Contains(stderr, "is managed by") {
		t.Errorf("wait with the run's own binary noted it: %s", stderr)
	}

	stdout, stderr, code := runWith(t, e, other, "web", "--detach", "--port", "0")
	if code != 0 {
		t.Fatalf("web --detach with another build: %s%s", stdout, stderr)
	}
	t.Cleanup(func() { _ = e.Command("web", "--shutdown", "--yes").Run() })
	webNote := regexp.MustCompile(`Note: the detached rotari web \(PID \d+\) runs ` + differentBuild + `rotari \S+.* \(` + regexp.QuoteMeta(other) + `\)`)
	if reported := e.MustRotari("web", "--detach", "--port", "0"); !webNote.MatchString(reported.Stderr) {
		t.Errorf("web --detach did not note the server's other binary: %s", reported)
	}
	if _, stderr, _ := runWith(t, e, other, "web", "--detach", "--port", "0"); strings.Contains(stderr, "Note: the detached") {
		t.Errorf("web --detach with the server's own binary noted it: %s", stderr)
	}
	if stopped := e.MustRotari("web", "--shutdown", "--yes"); !webNote.MatchString(stopped.Stderr) {
		t.Errorf("web --shutdown did not note the server's other binary: %s", stopped)
	}
}
