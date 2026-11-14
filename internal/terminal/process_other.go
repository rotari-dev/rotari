//go:build !linux && !darwin

package terminal

import (
	"errors"
	"runtime"
)

var errProcessesUnknown = errors.New("describing a process is not supported on " + runtime.GOOS)

// processDir is not known on this system.
func processDir(int) (string, error) {
	return "", errProcessesUnknown
}

// processCommand is not known on this system.
func processCommand(int) (string, error) {
	return "", errProcessesUnknown
}

// processArgs is not known on this system.
func processArgs(int) ([]string, error) {
	return nil, errProcessesUnknown
}

// processes are not known on this system.
func processes() ([]Process, error) {
	return nil, errProcessesUnknown
}
