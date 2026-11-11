package attachment

import (
	"os"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/state"
)

// CurrentLaunchOrigin is the process that ran this rotari command: its
// parent, such as the shell or script, on this host. Like a shell's wait,
// wait without a selector follows the runs that the same process started.
func CurrentLaunchOrigin() model.LaunchOrigin {
	host, _ := os.Hostname()
	parent := os.Getppid()
	return model.LaunchOrigin{Host: host, PIDNamespace: state.PIDNamespace(), PID: parent, ProcessStart: state.ProcessStart(parent)}
}

// SameLaunchOrigin reports whether a run's recorded origin is current: the
// same PID in the same host and PID namespace (state.ProcessIdentity.Locate).
// A start time known on both sides must match, so a reused PID never
// matches. An origin from an earlier rotari, without a namespace, matches
// on host and PID as before.
func SameLaunchOrigin(recorded *model.LaunchOrigin, current model.LaunchOrigin) bool {
	if recorded == nil || recorded.Host != current.Host || recorded.PID != current.PID {
		return false
	}
	identity := state.ProcessIdentity{Host: recorded.Host, PIDNamespace: recorded.PIDNamespace, PID: recorded.PID}
	if verdict, _ := identity.Locate(current.Host, current.PIDNamespace); verdict != state.ProcessHere {
		return false
	}
	return recorded.ProcessStart == "" || current.ProcessStart == "" || recorded.ProcessStart == current.ProcessStart
}
