package web

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The shell list says how a shell's last marked command ended, counts the commands, and
// says whether its group's notification config ([terminal]) asks for it to be notified.
func TestTerminalShellListSaysTheLastCommand(t *testing.T) {
	covers(t, "WEB-55")
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	e := terminalEnv(t)
	e.Setenv("SHELL", bash)
	// A shell of no project reads the global notification config: failures only, at once.
	global := filepath.Join(e.Root, "config", "rotari", "notifications.toml")
	if err := os.MkdirAll(filepath.Dir(global), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte("[terminal]\ncommand_success = false\nmin_seconds = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := e.StartWeb("--allow-terminal")
	headers := http.Header{"X-Rotari-Token": {launchToken}}
	page := openTerminalPage(t, base)
	type lastCommand struct {
		Command   string    `json:"command"`
		Exit      *int      `json:"exit"`
		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"ended_at"`
		Sequence  int       `json:"sequence"`
		Notify    bool      `json:"notify"`
	}
	await := func(what string, want func(*lastCommand) bool) *lastCommand {
		t.Helper()
		var got *lastCommand
		for deadline := time.Now().Add(terminalWait); ; time.Sleep(50 * time.Millisecond) {
			_, body := terminalShellsRequest(t, http.MethodGet, base+"/_terminal/shells", headers)
			var list struct {
				Shells []struct {
					LastCommand *lastCommand `json:"last_command"`
				} `json:"shells"`
			}
			if err := json.Unmarshal(body, &list); err != nil || len(list.Shells) != 1 {
				t.Fatalf("list: %v: %s", err, body)
			}
			if got = list.Shells[0].LastCommand; want(got) {
				return got
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: last_command %+v", what, got)
			}
		}
	}
	page.typeLine(`echo "ready-$((1+1))"`)
	page.await("ready-2")
	await("before any command ends", func(got *lastCommand) bool { return got == nil || got.Command == `echo "ready-$((1+1))"` })
	page.typeLine("sleep 0.3; false")
	failed := await("false", func(got *lastCommand) bool {
		return got != nil && got.Command == "sleep 0.3; false" && got.Exit != nil && *got.Exit == 1
	})
	if !failed.Notify || failed.EndedAt.Sub(failed.StartedAt) < 300*time.Millisecond {
		t.Errorf("false: %+v, want notify and at least 0.3 s", failed)
	}
	page.typeLine("true")
	succeeded := await("true", func(got *lastCommand) bool { return got != nil && got.Command == "true" })
	if succeeded.Exit == nil || *succeeded.Exit != 0 || succeeded.Notify || succeeded.Sequence != failed.Sequence+1 {
		t.Errorf("true: %+v after %+v, want exit 0, no notify, the next sequence", succeeded, failed)
	}
}
