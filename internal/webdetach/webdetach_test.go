package webdetach

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	paths, err := PathsFor(t.TempDir(), "node1")
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestPathsForRejectsSeparators(t *testing.T) {
	for _, host := range []string{"", ".", "..", "a/b", `a\b`, "a\x00b"} {
		if _, err := PathsFor("/m", host); err == nil {
			t.Errorf("PathsFor(%q) succeeded", host)
		}
	}
	paths, err := PathsFor("/m", "node1.example")
	if err != nil || paths.Record != "/m/web/node1.example.json" || paths.Log != "/m/web/node1.example.log" {
		t.Fatalf("PathsFor = %+v, %v", paths, err)
	}
}

func TestFindHoldPublishClose(t *testing.T) {
	paths := testPaths(t)
	if _, running, err := Find(paths); err != nil || running {
		t.Fatalf("Find with nothing = %v, %v", running, err)
	}
	holder, err := Hold(paths, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Find(paths); !errors.Is(err, ErrStarting) {
		t.Fatalf("Find before Publish = %v; want ErrStarting", err)
	}
	if _, err := Hold(paths, 0o700); !errors.Is(err, ErrRunning) {
		t.Fatalf("second Hold = %v; want ErrRunning", err)
	}
	want := Record{Host: "node1", PID: 42, URL: "http://127.0.0.1:1", StartedAt: time.Unix(1, 0).UTC(), Settings: Settings{BaseDir: "/b", AllowTerminal: true}}
	if err := holder.Publish(want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(paths.Record)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("record mode = %v, %v; want 0600", info, err)
	}
	got, running, err := Find(paths)
	if err != nil || !running || !reflect.DeepEqual(got, want) {
		t.Fatalf("Find = %+v, %v, %v; want %+v", got, running, err, want)
	}
	if err := holder.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.Record); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("record after Close: %v", err)
	}
	if _, running, err := Find(paths); err != nil || running {
		t.Fatalf("Find after Close = %v, %v", running, err)
	}
}

func TestFindRemovesTheRecordOfAServerThatIsGone(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Record, []byte(`{"pid": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Lease, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, running, err := Find(paths); err != nil || running {
		t.Fatalf("Find = %v, %v; want not running", running, err)
	}
	if _, err := os.Stat(paths.Record); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale record kept: %v", err)
	}
}

func TestFindReportsAnUnreadableLease(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	paths := testPaths(t)
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Lease, nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, running, err := Find(paths); err == nil || running {
		t.Fatalf("Find = %v, %v; want an error", running, err)
	}
}

func TestDifferences(t *testing.T) {
	running := Settings{BaseDir: "/b", Host: "127.0.0.1", Port: 8080, AllowControl: true, AllowTerminal: true, AllowedHosts: []string{"a", "b"}, TokenSHA256: TokenDigest("t")}
	same := running
	same.AllowedHosts = []string{"b", "a"}
	same.Port = 9000
	same.TokenSHA256 = ""
	if names := running.Differences(same, false); len(names) != 0 {
		t.Fatalf("Differences = %v; want none (port not explicit, no token named)", names)
	}
	if names := running.Differences(same, true); !reflect.DeepEqual(names, []string{"port"}) {
		t.Fatalf("Differences with an explicit port = %v", names)
	}
	terminal := running
	terminal.TerminalIdleTimeout = "3h0m0s"
	if names := terminal.Differences(Settings{BaseDir: "/b", Host: "127.0.0.1", AllowControl: true, AllowTerminal: false, AllowedHosts: []string{"a", "b"}}, false); !reflect.DeepEqual(names, []string{"allow-terminal"}) {
		t.Fatalf("Differences without the terminal = %v; want only allow-terminal", names)
	}
	longer := terminal
	longer.TerminalIdleTimeout = "24h0m0s"
	if names := terminal.Differences(longer, false); !reflect.DeepEqual(names, []string{"terminal-idle-timeout"}) {
		t.Fatalf("Differences of the idle timeout = %v", names)
	}
	plain := terminal
	plain.TerminalNoShellIntegration = true
	if names := terminal.Differences(plain, false); !reflect.DeepEqual(names, []string{"terminal-shell-integration"}) {
		t.Fatalf("Differences of shell integration = %v", names)
	}
	other := running
	other.AllowTerminal = false
	other.BaseDir = "/c"
	other.TokenSHA256 = TokenDigest("u")
	if names := running.Differences(other, false); !reflect.DeepEqual(names, []string{"basedir", "allow-terminal", "auth-token"}) {
		t.Fatalf("Differences = %v", names)
	}
}

func TestLogRotation(t *testing.T) {
	paths := testPaths(t)
	file, offset, err := OpenLog(paths, 0o700)
	if err != nil || offset != 0 {
		t.Fatalf("OpenLog = %v, %v", offset, err)
	}
	if _, err := file.WriteString("first\n"); err != nil {
		t.Fatal(err)
	}
	if err := KeepLogBounded(paths, file); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.RotatedLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a small log was rotated: %v", err)
	}
	big := strings.Repeat("x", MaxLogBytes)
	if _, err := file.WriteString(big); err != nil {
		t.Fatal(err)
	}
	if err := KeepLogBounded(paths, file); err != nil {
		t.Fatal(err)
	}
	rotated, err := os.ReadFile(paths.RotatedLog)
	if err != nil || string(rotated) != "first\n"+big {
		t.Fatalf("rotated log has %d bytes, %v", len(rotated), err)
	}
	// Appends land at the new end after the truncation.
	if _, err := file.WriteString("after\n"); err != nil {
		t.Fatal(err)
	}
	if current, err := os.ReadFile(paths.Log); err != nil || string(current) != "after\n" {
		t.Fatalf("log after truncation = %q, %v", current, err)
	}
	_ = file.Close()

	// A new server rotates an oversized log before it starts.
	if err := os.WriteFile(paths.Log, []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	file, offset, err = OpenLog(paths, 0o700)
	if err != nil || offset != 0 {
		t.Fatalf("OpenLog of an oversized log = %v, %v", offset, err)
	}
	_ = file.Close()
	if rotated, err := os.ReadFile(paths.RotatedLog); err != nil || len(rotated) != MaxLogBytes {
		t.Fatalf("rotated log has %d bytes, %v", len(rotated), err)
	}
	if entries, _ := filepath.Glob(filepath.Join(paths.Dir, "*.log.1.*")); len(entries) != 0 {
		t.Fatalf("temporary files left: %v", entries)
	}
}

func TestReadLogFrom(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Log, []byte("old\nnew line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if text, err := ReadLogFrom(paths, 4, 100); err != nil || text != "new line\n" {
		t.Fatalf("ReadLogFrom = %q, %v", text, err)
	}
	if text, err := ReadLogFrom(paths, 0, 5); err != nil || text != "line\n" {
		t.Fatalf("ReadLogFrom with a limit = %q, %v", text, err)
	}
}
