package terminal

import (
	"bytes"
	"errors"
	"os"
	"strconv"
	"strings"
)

// processDir is the working directory of the process with the given ID.
func processDir(pid int) (string, error) {
	return os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
}

// processCommand is the command line of the process with the given ID, its arguments
// joined by spaces.
func processCommand(pid int) (string, error) {
	line, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return "", err
	}
	command := strings.Join(strings.Split(strings.TrimRight(string(line), "\x00"), "\x00"), " ")
	if command == "" {
		return "", errors.New("the program has no command line")
	}
	return command, nil
}

// processes lists this host's processes, each with its parent, command line, and
// terminal facts, from /proc. A process that ends while it is read is left out.
func processes() ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var listed []Process
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if err != nil {
			continue
		}
		// The command's name is in parentheses and may hold anything, so the fields
		// after it start after the last ')'.
		end := bytes.LastIndexByte(stat, ')')
		start := bytes.IndexByte(stat, '(')
		if end < 0 || start < 0 || end+2 > len(stat) {
			continue
		}
		// After the name: state, ppid, pgrp, session, tty_nr, tpgid.
		fields := strings.Fields(string(stat[end+2:]))
		if len(fields) < 6 {
			continue
		}
		var numbers [5]int
		valid := true
		for index := range numbers {
			if numbers[index], err = strconv.Atoi(fields[index+1]); err != nil {
				valid = false
			}
		}
		if !valid {
			continue
		}
		command, err := processCommand(pid)
		if err != nil {
			command = "[" + string(stat[start+1:end]) + "]"
		}
		listed = append(listed, Process{PID: pid, PPID: numbers[0], Command: command,
			group: numbers[1], sessionLeader: numbers[2] == pid, hasTerminal: numbers[3] != 0, terminalGroup: numbers[4]})
	}
	return listed, nil
}
