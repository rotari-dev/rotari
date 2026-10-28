package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

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
	if err := os.MkdirAll(dir, 0o700); err != nil {
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
`, filepath.Join(dir, "host_key"), filepath.Join(dir, "authorized_keys")),
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
