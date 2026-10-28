package coordination

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestRunWaitsForAFinishingSupervisor starts runs while the project's
// supervisor lease is still held, as it is for a moment after a run has
// finished and its client has returned while its supervisor exits. With no
// active run, run and retry wait for the lease instead of reporting that a
// run is starting or running; a script that runs them back to back is not
// refused on a loaded host.
func TestRunWaitsForAFinishingSupervisor(t *testing.T) {
	covers(t, "COORD-7")
	e := support.NewEnv(t)
	e.MustRotari("add", "-p", "back", "--", "sh", "-c", "exit 3")
	e.RotariRun("run", "-p", "back", "--quiet")
	for _, command := range []string{"run", "retry"} {
		if command == "run" {
			e.MustRotari("add", "-p", "back", "--", "true")
		}
		lease, err := os.OpenFile(filepath.Join(e.Base, "projects", "back", "server.lock"), os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX); err != nil {
			t.Fatal(err)
		}
		released := make(chan struct{})
		go func() {
			defer close(released)
			time.Sleep(time.Second)
			_ = lease.Close()
		}()
		e.RotariRun(command, "-p", "back", "--quiet")
		<-released
	}
}
