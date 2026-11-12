package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

// sshLoopback returns executor options that reach this host over a real SSH
// session: ssh's ProxyCommand starts sshd in inetd mode as the current user,
// so no daemon or privileges are needed. It skips when ssh or sshd is
// missing.
func sshLoopback(t *testing.T, root string) []string {
	t.Helper()
	sshd := "/usr/sbin/sshd"
	if _, err := os.Stat(sshd); err != nil {
		t.Skipf("sshd not found: %v", err)
	}
	if _, err := os.Stat("/usr/bin/ssh"); err != nil {
		t.Skipf("ssh not found: %v", err)
	}
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "ssh")
	// The remote side keeps its token state under XDG_RUNTIME_DIR, which
	// sshd sets to a directory of the test, not the user's shared one.
	if err := os.MkdirAll(filepath.Join(dir, "runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"host_key", "client_key"} {
		if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", filepath.Join(dir, key)).CombinedOutput(); err != nil {
			t.Skipf("ssh-keygen: %v: %s", err, out)
		}
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(data))
	}
	files := map[string]string{
		"authorized_keys": read("client_key.pub") + "\n",
		"known_hosts":     "rotari-ssh-test " + read("host_key.pub") + "\n",
		"sshd_config": fmt.Sprintf(`HostKey %s
AuthorizedKeysFile %s
UsePAM no
StrictModes no
PasswordAuthentication no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
SetEnv XDG_RUNTIME_DIR=%s
`, filepath.Join(dir, "host_key"), filepath.Join(dir, "authorized_keys"), filepath.Join(dir, "runtime")),
		"config": fmt.Sprintf(`Host rotari-ssh-test
  User %s
  IdentityFile %s
  IdentitiesOnly yes
  UserKnownHostsFile %s
  StrictHostKeyChecking yes
  BatchMode yes
  LogLevel ERROR
  ProxyCommand %s -i -f %s
`, me.Username, filepath.Join(dir, "client_key"), filepath.Join(dir, "known_hosts"), sshd, filepath.Join(dir, "sshd_config")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("/usr/bin/ssh", "-F", filepath.Join(dir, "config"), "rotari-ssh-test", "true").CombinedOutput(); err != nil {
		t.Skipf("loopback SSH session unavailable: %v: %s", err, out)
	}
	return []string{"rotari-ssh-test", "-F " + filepath.Join(dir, "config")}
}

// sshRemoteStateRoot is where the remote side of sshLoopback keeps its
// token state.
func sshRemoteStateRoot(root string) string {
	return filepath.Join(root, "ssh", "runtime", fmt.Sprintf("rotari-%d", os.Getuid()))
}

// seedRetainedRemoteState fills the remote state root with count token
// directories within the retention, as a host that ran many SSH jobs has.
func seedRetainedRemoteState(t *testing.T, root string, count int) {
	t.Helper()
	stateRoot := sshRemoteStateRoot(root)
	for index := 0; index < count; index++ {
		dir := filepath.Join(stateRoot, fmt.Sprintf("%032x", index+1))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "finished"), []byte("0\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(stateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
}

// A job run through the SSH executor in a real run, which gives every job an
// attempt of its own, finishes with its command's result and log.
func TestSSHExecutorRunsAJob(t *testing.T) {
	covers(t, "CORE-7")
	e := support.NewEnv(t)
	options := sshLoopback(t, e.Root)
	args := []string{"add", "-p", "remote", "-e", "ssh"}
	for _, option := range options {
		args = append(args, "--executor-option="+option)
	}
	args = append(args, "--", "sh", "-c", "echo remote-ok")
	jobID := support.AddedJobID(t, e.MustRotari(args...))
	if r := e.RotariRun("run", "-p", "remote", "--quiet"); r.Code != 0 {
		t.Fatalf("run exited %d: %s", r.Code, r)
	}
	summary := readSummary(t, e, "remote")
	attemptID, exit := summaryResult(t, summary, jobID)
	if exit != 0 {
		t.Fatalf("SSH job result = %#v, want exit code 0", summary.Results)
	}
	stdout, err := os.ReadFile(filepath.Join(e.Base, "projects", "remote", "runs", summary.RunID, jobID, "attempts", attemptID, "stdout"))
	if err != nil || strings.TrimSpace(string(stdout)) != "remote-ok" {
		t.Fatalf("SSH job stdout = %q, err = %v", stdout, err)
	}
}

// A job running through the SSH executor reads as running
// in show and the Web API, as a local job does, rather than as unknown, and
// info finds its remote process group active. The host holds thousands of
// retained token directories, which must not delay the job's start: pruning
// runs after the job has started and examines only expired state (SAFE-16).
func TestSSHJobReadsAsRunning(t *testing.T) {
	covers(t, "DUR-1", "SAFE-16")
	e := support.NewEnv(t)
	options := sshLoopback(t, e.Root)
	const retained = 5000
	seedRetainedRemoteState(t, e.Root, retained)
	args := []string{"add", "-p", "remote", "-e", "ssh"}
	for _, option := range options {
		args = append(args, "--executor-option="+option)
	}
	jobID := support.AddedJobID(t, e.MustRotari(append(args, "--", "sleep", "300")...))
	e.MustRotari("run", "-p", "remote", "--quiet", "--async")
	t.Cleanup(func() { _ = e.Rotari("cancel", "-p", "remote", "--wait") })

	var shown struct {
		RunID string `json:"run_id"`
		Jobs  []struct {
			Job struct {
				ID string `json:"id"`
			} `json:"job"`
			Status string `json:"status"`
		} `json:"jobs"`
	}
	// A dispatched attempt reads as unknown until Submit records the running
	// phase once ssh has started (DisplayStatus); wait for that record
	// rather than taking the first status after pending.
	deadline := time.Now().Add(15 * time.Second)
	for {
		shown.RunID, shown.Jobs = "", nil
		_ = json.Unmarshal([]byte(e.Rotari("show", "-p", "remote", "--json").Stdout), &shown)
		if shown.RunID != "" && len(shown.Jobs) == 1 && shown.Jobs[0].Status == "running (recorded)" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("SSH job never read as running (recorded); last show --json: %+v", shown)
		}
		time.Sleep(100 * time.Millisecond)
	}
	var info struct {
		ActiveRuns []struct {
			Assessment struct {
				Executions []struct {
					JobID  string `json:"job_id"`
					State  string `json:"state"`
					Reason string `json:"reason"`
				} `json:"executions"`
			} `json:"execution_assessment"`
		} `json:"active_runs"`
	}
	// The remote side records its process group just after ssh connects;
	// until then the assessment is unknown.
	for deadline := time.Now().Add(15 * time.Second); ; {
		info.ActiveRuns = nil
		if err := json.Unmarshal([]byte(e.MustRotari("info", "-p", "remote", "--json").Stdout), &info); err != nil {
			t.Fatal(err)
		}
		if len(info.ActiveRuns) == 1 && len(info.ActiveRuns[0].Assessment.Executions) == 1 && info.ActiveRuns[0].Assessment.Executions[0].State == "active" {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("info execution assessment = %+v, want the SSH job's remote process group active", info.ActiveRuns)
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if job := shown.Jobs[0]; job.Job.ID != jobID || job.Status != "running (recorded)" {
		t.Errorf("show --json job = %+v, want %s running (recorded)", job, jobID)
	}
	var detail struct {
		Jobs []struct {
			ID              string `json:"id"`
			ExecutionStatus string `json:"execution_status"`
		} `json:"jobs"`
	}
	body := e.HTTPGet(e.StartWeb() + "/api/run?project_name=remote&run_id=" + shown.RunID).Body
	if err := json.Unmarshal([]byte(body), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Jobs) != 1 || detail.Jobs[0].ExecutionStatus != "running (recorded)" {
		t.Errorf("Web API jobs = %+v, want %s running (recorded)", detail.Jobs, jobID)
	}
	// Retained state is kept: the seeded directories and this job's.
	if entries, err := os.ReadDir(sshRemoteStateRoot(e.Root)); err != nil || len(entries) != retained+1 {
		t.Errorf("remote state root holds %d entries (%v), want the %d retained and this job's", len(entries), err, retained)
	}
}
