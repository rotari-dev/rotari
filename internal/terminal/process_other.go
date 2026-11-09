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
