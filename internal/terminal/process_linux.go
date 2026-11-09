package terminal

import (
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
