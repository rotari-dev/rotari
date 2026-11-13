package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestWebShutdownStopsTheDetachedServer checks that rotari web --shutdown stops this
// host's detached server and waits for it, asks first when a shell runs something (and,
// without a terminal, needs --yes), says when none runs, and takes no server option.
func TestWebShutdownStopsTheDetachedServer(t *testing.T) {
	covers(t, "WEB-40")
	e := terminalEnv(t)
	if none := e.MustRotari("web", "--shutdown"); !strings.Contains(none.Stdout, "No detached rotari web is running") {
		t.Errorf("--shutdown with no server: %s", none)
	}
	for _, args := range [][]string{{"--shutdown", "--port", "1"}, {"--shutdown", "--detach"}, {"--yes"}} {
		if result := e.Rotari(append([]string{"web"}, args...)...); result.Code == 0 || result.Stdout != "" {
			t.Errorf("rotari web %v did not fail: %s", args, result)
		}
	}

	server, _ := startDetached(t, e, "--allow-terminal")
	record, _ := detachedFiles(t, e)
	page := openTerminalPage(t, server.url)
	// The foreground command itself prints the mark, so the shell is busy once it shows;
	// its command line names sleep 300 before and after the exec.
	page.typeLine(`sh -c 'echo ready-$((40+2)); exec sleep 300'`)
	page.await("ready-42")
	refused := e.Rotari("web", "--shutdown")
	if refused.Code == 0 || !strings.Contains(refused.Stderr, "sleep 300") || !strings.Contains(refused.Stderr, "--yes") {
		t.Errorf("--shutdown without a terminal or --yes, with a shell running sleep: %s", refused)
	}
	if status := e.HTTPDo(http.MethodGet, server.url+"/", map[string]string{"X-Rotari-Token": launchToken}, "").Status; status != http.StatusOK {
		t.Fatalf("a refused --shutdown stopped the server: %d", status)
	}
	stopped := e.MustRotari("web", "--shutdown", "--yes")
	if !strings.Contains(stopped.Stdout, "Stopped rotari web at "+server.url+" (PID "+strconv.Itoa(server.pid)+")") {
		t.Errorf("--shutdown --yes did not say it stopped the server: %s", stopped)
	}
	// --shutdown returns once the server is gone, so nothing is left to wait for.
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(server.pid))); err == nil {
		t.Errorf("the server (PID %d) is still running after --shutdown returned", server.pid)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Errorf("the record outlived --shutdown: %v", err)
	}

	idle, _ := startDetached(t, e)
	if result := e.MustRotari("web", "--shutdown"); !strings.Contains(result.Stdout, "PID "+strconv.Itoa(idle.pid)) {
		t.Errorf("--shutdown of a server with no shells asked or failed: %s", result)
	}
}

// TestServerShutdownRoute checks that POST /_server/shutdown stops a server once it has
// answered, needs job control and a JSON request, and that a detached server stopped
// this way removes its record.
func TestServerShutdownRoute(t *testing.T) {
	covers(t, "WEB-41")
	e := terminalEnv(t)
	token := map[string]string{"X-Rotari-Token": launchToken}
	readOnly := e.StartWeb("--allow-control=false")
	if status := e.HTTPDo(http.MethodPost, readOnly+"/_server/shutdown", map[string]string{"X-Rotari-Token": launchToken, "Content-Type": "application/json"}, "{}").Status; status != http.StatusForbidden {
		t.Errorf("a read-only server answered Stop server with %d, want 403", status)
	}

	server, _ := startDetached(t, e, "--allow-terminal")
	record, _ := detachedFiles(t, e)
	if status := e.HTTPDo(http.MethodPost, server.url+"/_server/shutdown", map[string]string{"Content-Type": "application/json"}, "{}").Status; status != http.StatusUnauthorized && status != http.StatusForbidden {
		t.Errorf("Stop server without the token answered %d", status)
	}
	form := map[string]string{"X-Rotari-Token": launchToken, "Content-Type": "application/x-www-form-urlencoded"}
	if status := e.HTTPDo(http.MethodPost, server.url+"/_server/shutdown", form, "a=b").Status; status != http.StatusUnsupportedMediaType {
		t.Errorf("Stop server without a JSON body answered %d, want 415", status)
	}
	if status := e.HTTPDo(http.MethodGet, server.url+"/_server/shutdown", token, "").Status; status != http.StatusMethodNotAllowed {
		t.Errorf("GET of Stop server answered %d, want 405", status)
	}
	withJSON := map[string]string{"X-Rotari-Token": launchToken, "Content-Type": "application/json"}
	if result := e.HTTPDo(http.MethodPost, server.url+"/_server/shutdown", withJSON, "{}"); result.Status != http.StatusAccepted {
		t.Fatalf("Stop server answered %d: %s", result.Status, result.Body)
	}
	awaitProcessGone(t, server.pid, "POST /_server/shutdown")
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Errorf("the record outlived a stop from the Web UI: %v", err)
	}
	if result := e.MustRotari("web", "--shutdown"); !strings.Contains(result.Stdout, "No detached rotari web") {
		t.Errorf("after the Web UI stopped it, --shutdown still found a server: %s", result)
	}
}
