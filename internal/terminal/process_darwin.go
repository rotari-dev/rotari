package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// processWait is how long lsof or ps may take to describe a process.
const processWait = 5 * time.Second

// ask runs a program that comes with macOS and returns what it printed.
func ask(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), processWait)
	defer cancel()
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, name, args...)
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, message)
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return string(output), nil
}

// processDir is the working directory of the process with the given ID, as lsof
// reports it.
func processDir(pid int) (string, error) {
	output, err := ask("/usr/sbin/lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn")
	if err != nil {
		return "", err
	}
	// -F prints one field a line, each led by its letter; n is the name.
	for line := range strings.SplitSeq(output, "\n") {
		if name, ok := strings.CutPrefix(line, "n"); ok && name != "" {
			return name, nil
		}
	}
	return "", errors.New("lsof named no directory")
}

// processCommand is the command line of the process with the given ID, as ps
// reports it.
func processCommand(pid int) (string, error) {
	output, err := ask("/bin/ps", "-o", "command=", "-p", strconv.Itoa(pid))
	if err != nil {
		return "", err
	}
	command := strings.TrimSpace(output)
	if command == "" {
		return "", errors.New("ps named no command")
	}
	return command, nil
}

// processes lists this host's processes, each with its parent and command line, as ps
// reports them.
func processes() ([]Process, error) {
	output, err := ask("/bin/ps", "-A", "-o", "pid=,ppid=,command=")
	if err != nil {
		return nil, err
	}
	var listed []Process
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		command := strings.TrimSpace(line)
		command = strings.TrimSpace(strings.TrimPrefix(command, fields[0]))
		command = strings.TrimSpace(strings.TrimPrefix(command, fields[1]))
		listed = append(listed, Process{PID: pid, PPID: ppid, Command: command})
	}
	return listed, nil
}
