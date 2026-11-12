package support

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// FailedDirName is the directory under os.TempDir where a failed test's
// evidence is kept.
const FailedDirName = "rotari-conformance-failed"

// Transcript records the rotari commands a test runs: when each started, in
// which directory, and, for commands run to completion, their exit code and
// full output. It is written out only when the test fails.
type Transcript struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Started records a command that is about to start.
func (transcript *Transcript) Started(dir string, args []string) {
	transcript.printf("%s start in %s: rotari %q\n", stamp(), dir, args)
}

// Finished records a command that ran to completion.
func (transcript *Transcript) Finished(args []string, code int, stdout, stderr string, elapsed time.Duration) {
	transcript.printf("%s exit %d after %s: rotari %q\n--- stdout\n%s--- stderr\n%s---\n",
		stamp(), code, elapsed.Round(time.Millisecond), args, withNewline(stdout), withNewline(stderr))
}

func (transcript *Transcript) printf(format string, args ...any) {
	if transcript == nil {
		return
	}
	transcript.mu.Lock()
	defer transcript.mu.Unlock()
	fmt.Fprintf(&transcript.buf, format, args...)
}

func (transcript *Transcript) bytes() []byte {
	if transcript == nil {
		return nil
	}
	transcript.mu.Lock()
	defer transcript.mu.Unlock()
	return append([]byte(nil), transcript.buf.Bytes()...)
}

func stamp() string {
	return time.Now().UTC().Format("15:04:05.000")
}

func withNewline(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

// KeepOnFailure keeps the evidence of a failed test: when t has failed, its
// state directory root is moved to a new directory under
// os.TempDir()/FailedDirName, with the transcript beside it as commands.log,
// and the place is logged. A passing test keeps nothing. Register it before
// the cleanups that stop the test's servers and processes, so that it runs
// after them.
func KeepOnFailure(t *testing.T, root string, transcript *Transcript) {
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		kept, err := keepEvidence(t.Name(), root, transcript.bytes())
		if err != nil {
			t.Logf("could not keep the failed test's evidence: %v", err)
			return
		}
		t.Logf("kept the state directory and the rotari command transcript in %s", kept)
	})
}

var unsafeNameCharacters = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func keepEvidence(testName, root string, transcript []byte) (string, error) {
	parent := filepath.Join(os.TempDir(), FailedDirName)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	name := unsafeNameCharacters.ReplaceAllString(testName, "_")
	if len(name) > 80 {
		name = name[:80]
	}
	kept, err := os.MkdirTemp(parent, name+"-")
	if err != nil {
		return "", err
	}
	// The state directory is a t.TempDir on the same file system, so it is
	// moved, not copied; its own cleanup then finds nothing to remove.
	if err := os.Rename(root, filepath.Join(kept, "state")); err != nil {
		err = fmt.Errorf("move %s: %w", root, err)
		if writeErr := os.WriteFile(filepath.Join(kept, "commands.log"), transcript, 0o644); writeErr != nil {
			return "", fmt.Errorf("%w; write the transcript: %v", err, writeErr)
		}
		return "", fmt.Errorf("%w; the transcript alone is in %s", err, kept)
	}
	if err := os.WriteFile(filepath.Join(kept, "commands.log"), transcript, 0o644); err != nil {
		return "", fmt.Errorf("write the transcript in %s: %w", kept, err)
	}
	return kept, nil
}
