package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKeepEvidenceMovesStateAndWritesTranscript(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	root := filepath.Join(temp, "root")
	if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "base", "meta.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var transcript Transcript
	transcript.Started(root, []string{"run", "-p", "demo"})
	transcript.Finished([]string{"run", "-p", "demo"}, 1, "out", "refused: busy\n", 1500*time.Millisecond)

	kept, err := keepEvidence("TestDemo/sub case", root, transcript.bytes())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(kept) != filepath.Join(temp, FailedDirName) || !strings.HasPrefix(filepath.Base(kept), "TestDemo_sub_case-") {
		t.Errorf("kept in %s", kept)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("the state directory was not moved: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(kept, "state", "base", "meta.json")); err != nil || string(data) != "{}" {
		t.Errorf("kept state: %q, %v", data, err)
	}
	log, err := os.ReadFile(filepath.Join(kept, "commands.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`start in ` + root + `: rotari ["run" "-p" "demo"]`, `exit 1 after 1.5s`, "--- stdout\nout\n", "--- stderr\nrefused: busy\n"} {
		if !strings.Contains(string(log), want) {
			t.Errorf("commands.log lacks %q:\n%s", want, log)
		}
	}
}
