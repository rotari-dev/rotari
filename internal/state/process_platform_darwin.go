package state

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// processStart is the kernel's p_starttime for pid, from sysctl
// kern.proc.pid, as seconds.microseconds. A PID with no process makes the
// sysctl return no record, which x/sys reports as an error.
func processStart(pid int) string {
	if pid <= 0 {
		return ""
	}
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || int(info.Proc.P_pid) != pid {
		return ""
	}
	start := info.Proc.P_starttime
	if start.Sec == 0 && start.Usec == 0 {
		return ""
	}
	return fmt.Sprintf("%d.%06d", start.Sec, start.Usec)
}

func pidNamespace() string { return NoPIDNamespaces }
