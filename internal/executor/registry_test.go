package executor

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/state"
)

func TestRegistryNamesAndLookup(t *testing.T) {
	registry := NewRegistry(state.NewStore(0o700, 0o600), nil)
	if got := registry.Names(); !reflect.DeepEqual(got, []string{"local", "lsf", "pbs", "sge", "slurm", "ssh"}) {
		t.Fatalf("Names() = %#v", got)
	}
	if !registry.Known("sge") || !registry.Known("slurm") || registry.Known("kubernetes") {
		t.Fatal("Known() mismatch")
	}
	if executor, ok := registry.Lookup("pbs"); !ok || executor.Name() != "pbs" {
		t.Fatalf("Lookup(pbs) = %v, %v", executor, ok)
	}
}

func TestRegistryOwner(t *testing.T) {
	store := state.NewStore(0o700, 0o600)
	registry := NewRegistry(store, nil)
	jobDir := t.TempDir()
	if _, err := registry.Owner(store, jobDir); !errors.Is(err, ErrJobNotRunning) {
		t.Fatalf("Owner() of an unsubmitted job = %v, want ErrJobNotRunning", err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "pid"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if owner, err := registry.Owner(store, jobDir); err != nil || owner.Name() != "local" {
		t.Fatalf("Owner() with pid = %v, %v, want local", owner, err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "job.json"), []byte(`{"executor":"lsf"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if owner, err := registry.Owner(store, jobDir); err != nil || owner.Name() != "lsf" {
		t.Fatalf("Owner() with job.json = %v, %v, want lsf", owner, err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "job.json"), []byte(`{"executor":"sge"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if owner, err := registry.Owner(store, jobDir); err != nil || owner.Name() != "sge" {
		t.Fatalf("Owner() with SGE job.json = %v, %v, want sge", owner, err)
	}
}

func TestLocalElsewhere(t *testing.T) {
	store := state.NewStore(0o700, 0o600)
	registry := NewRegistry(store, nil)
	runDir := t.TempDir()
	jobDir := filepath.Join(runDir, "job")
	if err := os.MkdirAll(jobDir, 0o700); err != nil {
		t.Fatal(err)
	}
	local, _ := registry.Lookup("local")
	slurm, _ := registry.Lookup("slurm")
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := func(executor JobExecutor) *state.ElsewhereError {
		t.Helper()
		var refused *state.ElsewhereError
		if err := LocalElsewhere(store, executor, runDir, jobDir); err != nil && !errors.As(err, &refused) {
			t.Fatalf("unexpected error %v", err)
		}
		return refused
	}
	// An attempt from an earlier rotari records only a pid; the run's host
	// places it.
	if err := os.WriteFile(filepath.Join(runDir, "context.json"), []byte(`{"hostname":"elsewhere.invalid"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "pid"), []byte("12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if refused := elsewhere(local); refused == nil || refused.Observation.Verdict != state.ProcessOtherHost || refused.Identity.Host != "elsewhere.invalid" {
		t.Fatalf("local job on another host = %+v", refused)
	}
	if elsewhere(slurm) != nil {
		t.Fatal("scheduler jobs are controllable from any host")
	}
	// With the run on this host, the job's own record decides: this host,
	// another PID namespace.
	if err := store.WriteJSON(filepath.Join(runDir, "context.json"), map[string]string{"hostname": host}); err != nil {
		t.Fatal(err)
	}
	identity := localIdentity{Executor: "local", Host: host, PID: 12345, PIDNamespace: "pid:[1]"}
	if err := store.WriteJSON(filepath.Join(jobDir, "job.json"), identity); err != nil {
		t.Fatal(err)
	}
	refused := elsewhere(local)
	if state.PIDNamespace() == "" {
		if refused != nil {
			t.Fatalf("without a namespace here nothing can differ: %+v", refused)
		}
		return
	}
	if refused == nil || refused.Observation.Verdict != state.ProcessOtherNamespace || !strings.Contains(refused.Error(), "pid:[1]") {
		t.Fatalf("local job in another namespace = %+v", refused)
	}
	identity.PIDNamespace = state.PIDNamespace()
	if err := store.WriteJSON(filepath.Join(jobDir, "job.json"), identity); err != nil {
		t.Fatal(err)
	}
	if refused := elsewhere(local); refused != nil {
		t.Fatalf("a job recorded here is not elsewhere: %+v", refused)
	}
}
