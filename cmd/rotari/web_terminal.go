package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rotari-dev/rotari/internal/terminal"
)

// Provisional limits of the Web UI's shell, until they become configuration.
const (
	webTerminalIdleLimit = 30 * time.Minute
	webTerminalRingSize  = 256 << 10
	// webTerminalDefaultShell runs when $SHELL is unset.
	webTerminalDefaultShell = "/bin/sh"
)

// newWebTerminal returns the keeper of the Web UI's shell. The shell starts in the
// home directory when the browser first asks for it.
func newWebTerminal() (*terminal.Keeper, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find the shell's home directory: %w", err)
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = webTerminalDefaultShell
	}
	return terminal.NewKeeper(terminal.KeeperOptions{
		Shell:     terminal.Options{Shell: shell, Dir: home, Env: webChildEnvironment(os.Environ())},
		IdleLimit: webTerminalIdleLimit,
		RingSize:  webTerminalRingSize,
		Log:       func(message string) { fmt.Fprintln(os.Stderr, message) },
	})
}
