package lease

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestHoldAndProbe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	if held, err := Probe(path, nil); err != nil || held {
		t.Fatalf("Probe of a missing lease = %v, %v; want free", held, err)
	}
	file, err := Hold(path, 0o600, 0)
	if err != nil {
		t.Fatal(err)
	}
	// flock locks belong to open file descriptions, so a second open in the
	// same process contends as another process would.
	if held, err := Probe(path, func() error { t.Error("whileFree ran on a held lease"); return nil }); err != nil || !held {
		t.Fatalf("Probe of a held lease = %v, %v; want held", held, err)
	}
	if _, err := Hold(path, 0o600, 30*time.Millisecond); !errors.Is(err, ErrHeld) {
		t.Fatalf("second Hold = %v; want ErrHeld", err)
	}
	_ = file.Close()
	ran := false
	if held, err := Probe(path, func() error { ran = true; return nil }); err != nil || held || !ran {
		t.Fatalf("Probe of a released lease = %v, %v, ran %v; want free and run", held, err, ran)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the lease file must stay: %v", err)
	}
}

func TestProbeReturnsWhileFreeError(t *testing.T) {
	want := errors.New("boom")
	if _, err := Probe(filepath.Join(t.TempDir(), "x.lock"), func() error { return want }); !errors.Is(err, want) {
		t.Fatalf("Probe = %v; want %v", err, want)
	}
}

func TestProbeReportsOpenFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	path := filepath.Join(t.TempDir(), "x.lock")
	if err := os.WriteFile(path, nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Probe(path, nil); err == nil {
		t.Fatal("Probe of an unreadable lease must fail, not read as free")
	}
}

func TestHoldEndsWithTheProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("flock(1) not installed")
	}
	command := exec.Command("flock", "-x", "-o", path, "sleep", "30") // -o: the lock stays with flock, not sleep
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	group := command.Process.Pid
	// Only the test's own group: flock and its sleep.
	t.Cleanup(func() { _ = syscall.Kill(-group, syscall.SIGKILL) })
	deadline := time.Now().Add(5 * time.Second)
	for {
		held, err := Probe(path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if held {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("flock(1) never took the lease")
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	// flock -o closes its descriptor in the forked command only after the
	// fork, so the command may share the lock for an instant.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		held, err := Probe(path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !held {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the lease outlived its holder")
		}
	}
}
