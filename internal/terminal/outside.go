package terminal

import "fmt"

// ShellActivity is what runs in one of a server's shells, seen from outside the server:
// Foreground is the command holding the shell's terminal, empty while the shell is at
// its prompt, and Background the shell's other processes.
type ShellActivity struct {
	PID        int
	Foreground string
	Background []Process
}

// Busy says something runs in the shell.
func (activity ShellActivity) Busy() bool {
	return activity.Foreground != "" || len(activity.Background) != 0
}

// ServerShells describes the shells of the server with the given PID from the process
// table, for a process that cannot ask the server, such as rotari web --shutdown. A
// shell is a child of the server that leads its own session on a terminal, as a Keeper
// starts it; other children, such as launches, are not shells. A failure to read the
// process table is an error, never "no shells".
func ServerShells(serverPID int) ([]ShellActivity, error) {
	all, err := processes()
	if err != nil {
		return nil, fmt.Errorf("cannot list processes: %w", err)
	}
	var shells []ShellActivity
	for _, process := range all {
		if process.PPID != serverPID || !process.sessionLeader || !process.hasTerminal {
			continue
		}
		activity := ShellActivity{PID: process.PID}
		foreground := process.terminalGroup
		below := descendantsOf(all, process.PID)
		if foreground > 0 && foreground != process.group {
			for _, candidate := range append([]Process{process}, below...) {
				if candidate.PID == foreground {
					activity.Foreground = candidate.Command
				}
			}
			if activity.Foreground == "" {
				// The group's leader has exited; any member names it.
				for _, candidate := range below {
					if candidate.group == foreground {
						activity.Foreground = candidate.Command
						break
					}
				}
			}
			if activity.Foreground == "" {
				activity.Foreground = "a command"
			}
		}
		for _, candidate := range below {
			if foreground <= 0 || candidate.group != foreground {
				activity.Background = append(activity.Background, candidate)
			}
		}
		shells = append(shells, activity)
	}
	return shells, nil
}

// descendantsOf are the processes below pid in all, children first, each followed by its
// own.
func descendantsOf(all []Process, pid int) []Process {
	children := map[int][]Process{}
	for _, process := range all {
		children[process.PPID] = append(children[process.PPID], process)
	}
	var below []Process
	var walk func(pid, depth int)
	walk = func(pid, depth int) {
		for _, child := range children[pid] {
			if child.PID == pid {
				continue
			}
			child.Depth = depth
			below = append(below, child)
			walk(child.PID, depth+1)
		}
	}
	walk(pid, 1)
	return below
}
