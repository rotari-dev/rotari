package support

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// Process is one entry of the process table, as tests look at it from outside.
type Process struct {
	PID, PPID, PGID int
	// Args are the command-line arguments. On Linux they are exact; elsewhere
	// they come from ps and are split at spaces, so an argument holding a space
	// reads as several. Tests match paths without spaces and whole words.
	Args []string
}

// CommandLine joins the arguments with spaces, for substring matches.
func (p Process) CommandLine() string { return strings.Join(p.Args, " ") }

// HasArg reports whether one argument is exactly arg.
func (p Process) HasArg(arg string) bool { return slices.Contains(p.Args, arg) }

// Processes lists the processes this user can see: from /proc on Linux, and
// from ps elsewhere. A process that has exited but is not yet reaped has no
// arguments.
func Processes() ([]Process, error) {
	if runtime.GOOS == "linux" {
		return linuxProcesses()
	}
	out, err := exec.Command("ps", "-A", "-ww", "-o", "pid=,ppid=,pgid=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	var processes []Process
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		numbers := make([]int, 3)
		for i := range numbers {
			if numbers[i], err = strconv.Atoi(fields[i]); err != nil {
				return nil, fmt.Errorf("ps line %q: %w", line, err)
			}
		}
		processes = append(processes, Process{PID: numbers[0], PPID: numbers[1], PGID: numbers[2], Args: fields[3:]})
	}
	return processes, nil
}

func linuxProcesses() ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var processes []Process
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		fields := strings.Fields(string(stat[bytes.LastIndexByte(stat, ')')+1:]))
		if len(fields) < 3 {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		pgid, _ := strconv.Atoi(fields[2])
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		var args []string
		if trimmed := strings.TrimSuffix(string(cmdline), "\x00"); trimmed != "" {
			args = strings.Split(trimmed, "\x00")
		}
		processes = append(processes, Process{PID: pid, PPID: ppid, PGID: pgid, Args: args})
	}
	return processes, nil
}

// MustProcesses is Processes, failing the test on an error rather than letting
// a caller find nothing and pass.
func MustProcesses(t interface {
	Helper()
	Fatalf(string, ...any)
}) []Process {
	t.Helper()
	processes, err := Processes()
	if err != nil {
		t.Fatalf("cannot list processes: %v", err)
	}
	return processes
}

// ProcessArgs returns pid's arguments, or false when it is not running.
func ProcessArgs(pid int) ([]string, bool, error) {
	processes, err := Processes()
	if err != nil {
		return nil, false, err
	}
	for _, process := range processes {
		if process.PID == pid {
			return process.Args, len(process.Args) != 0, nil
		}
	}
	return nil, false, nil
}

// ProcessEnvironment returns pid's environment as NAME=VALUE entries: from
// /proc on Linux and from ps -E elsewhere, where entries follow the arguments
// split at spaces. It fails when pid is gone or not this user's.
func ProcessEnvironment(pid int) ([]string, error) {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "environ"))
		if err != nil {
			return nil, err
		}
		return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00"), nil
	}
	out, err := exec.Command("ps", "-E", "-ww", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil, fmt.Errorf("ps -E -p %d: %w", pid, err)
	}
	return strings.Fields(string(out)), nil
}
