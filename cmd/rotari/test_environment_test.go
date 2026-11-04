package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/basedirregistry"
	"github.com/rotari-dev/rotari/internal/notification"
	"github.com/rotari-dev/rotari/internal/state"
)

// Isolate discovery records and user settings even for tests that only set
// --basedir. The
// subprocesses they launch inherit this masterdir, and individual tests can
// still override it with t.Setenv. Clean up before os.Exit, also on failure.
func TestMain(m *testing.M) {
	masterDir, err := os.MkdirTemp("", "rotari-test-master-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	configDir, err := os.MkdirTemp("", "rotari-test-config-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := 1
	if err := isolateUserSettings(masterDir, configDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		code = m.Run()
	}
	for _, dir := range []string{masterDir, configDir} {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, "remove test directory:", err)
			code = 1
		}
	}
	os.Exit(code)
}

// isolateUserSettings keeps tests away from the user's registry and settings.
// In particular, in-process runs notify through runNotifications, which would
// otherwise post the tests' jobs to the webhook in the user's configuration.
// Its batch timer never fires, because a batch that a test left pending would
// flush in its own goroutine and read os.Stderr while another test swaps it;
// tests of the batch window use their own manager.
func isolateUserSettings(masterDir, configDir string) error {
	if err := os.Setenv("ROTARI_MASTERDIR", masterDir); err != nil {
		return err
	}
	if err := os.Setenv("XDG_CONFIG_HOME", configDir); err != nil {
		return err
	}
	if err := os.Unsetenv(envWebhookURL); err != nil {
		return err
	}
	runNotifications.after = func(time.Duration, func()) *time.Timer { return time.NewTimer(time.Hour) }
	return nil
}

const notificationProbeEnv = "ROTARI_TEST_NOTIFICATION_PROBE"
const registryProbeEnv = "ROTARI_TEST_REGISTRY_PROBE"
const registryProbeMarker = "test masterdir: "

// TestTestEnvironmentSendsNoUserNotifications runs the test binary as a user
// with a webhook configured both ways. In-process runs use the shared manager,
// so tests that read these would post their jobs to the user's real webhook.
func TestTestEnvironmentSendsNoUserNotifications(t *testing.T) {
	if os.Getenv(notificationProbeEnv) != "" {
		probeTestNotifications(t)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	userConfig := filepath.Join(t.TempDir(), "rotari")
	if err := os.MkdirAll(userConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	settings := fmt.Sprintf("[webhook]\nurl = %q\n", server.URL)
	if err := os.WriteFile(filepath.Join(userConfig, notification.FileName), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestTestEnvironmentSendsNoUserNotifications$", "-test.v")
	command.Env = append(os.Environ(), notificationProbeEnv+"=1", "XDG_CONFIG_HOME="+filepath.Dir(userConfig), envWebhookURL+"="+server.URL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("child test failed: %v\n%s", err, output)
	}
	if count := requests.Load(); count != 0 {
		t.Fatalf("test binary sent %d requests to the user's webhook, want none", count)
	}
}

func probeTestNotifications(t *testing.T) {
	t.Helper()
	paths, err := state.ResolveProjectPaths(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := loadRunWebhookSettings(paths, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if settings.URL != "" {
		t.Errorf("test binary reads webhook URL %q, want none", settings.URL)
	}
	// A batch left pending by a test must not flush later in its own goroutine,
	// where it would read os.Stderr while another test swaps it.
	fired := make(chan struct{})
	timer := runNotifications.after(time.Millisecond, func() { close(fired) })
	defer timer.Stop()
	select {
	case <-fired:
		t.Error("the shared notification manager's batch timer fires in tests")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTestEnvironmentCleansRegistry(t *testing.T) {
	if probe := os.Getenv(registryProbeEnv); probe != "" {
		probeTestRegistry(t, probe)
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range []string{"success", "failure"} {
		t.Run(outcome, func(t *testing.T) {
			checkTestRegistryCleanup(t, executable, outcome)
		})
	}
}

func probeTestRegistry(t *testing.T, outcome string) {
	t.Helper()
	masterDir, err := state.ResolveMasterDir("")
	if err != nil {
		t.Fatal(err)
	}
	baseDir := t.TempDir()
	if code := cmdAdd([]string{"--basedir", baseDir, "--project-name", "demo", "echo", "hello"}); code != 0 {
		t.Fatalf("cmdAdd exit code = %d, want 0", code)
	}
	baseDirs, err := basedirregistry.Open(masterDir).BaseDirs()
	if err != nil || len(baseDirs) != 1 || baseDirs[0] != baseDir {
		t.Fatalf("registered basedirs = %v, %v; want [%s]", baseDirs, err, baseDir)
	}
	fmt.Println(registryProbeMarker + masterDir)
	if outcome == "failure" {
		t.Fatal("intentional failure to check cleanup")
	}
}

func checkTestRegistryCleanup(t *testing.T, executable, outcome string) {
	t.Helper()
	userMaster := t.TempDir()
	command := exec.Command(executable, "-test.run=^TestTestEnvironmentCleansRegistry$", "-test.v")
	command.Env = append(os.Environ(), "ROTARI_MASTERDIR="+userMaster, registryProbeEnv+"="+outcome)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil {
		t.Fatalf("could not start child test: %v", err)
	}
	if outcome == "success" && err != nil {
		t.Fatalf("child test failed: %v\n%s", err, output)
	}
	if outcome == "failure" && command.ProcessState.ExitCode() != 1 {
		t.Fatalf("child exit = %d, want 1: %v\n%s", command.ProcessState.ExitCode(), err, output)
	}
	masterDir := reportedTestMasterDir(t, output)
	if masterDir == userMaster {
		t.Fatal("test used the user's masterdir instead of an isolated temporary directory")
	}
	if _, err := os.Stat(masterDir); !os.IsNotExist(err) {
		t.Fatalf("test masterdir remains after child exit: %s (%v)", masterDir, err)
	}
	entries, err := os.ReadDir(userMaster)
	if err != nil || len(entries) != 0 {
		t.Fatalf("user masterdir was modified: %v, %v", entries, err)
	}
}

func reportedTestMasterDir(t *testing.T, output []byte) string {
	t.Helper()
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, registryProbeMarker) {
			return strings.TrimPrefix(line, registryProbeMarker)
		}
	}
	t.Fatalf("child did not report its masterdir:\n%s", output)
	return ""
}
