package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/rotari-dev/rotari/internal/terminal"
)

// Defaults of the Web UI's shells.
const (
	webTerminalIdleLimit  = 30 * time.Minute
	webTerminalRingSize   = "256KiB"
	webTerminalShellLimit = 32
	webTerminalGroupLimit = 8
	// webTerminalDefaultShell runs when neither --terminal-shell nor $SHELL says.
	webTerminalDefaultShell = "/bin/sh"
	// webTerminalMinIdleLimit stays well above how often an open page says so, which a
	// browser can slow to about a minute in the background. Under it, an open page's
	// shells could end.
	webTerminalMinIdleLimit = 2 * time.Minute
	webTerminalMinRingSize  = 1 << 10
	webTerminalMaxRingSize  = 64 << 20
)

// webTerminalOptions are the flags that say how the terminal runs.
var webTerminalOptions = []string{"terminal-idle-timeout", "terminal-max-shells", "terminal-max-shells-per-project", "terminal-replay-size", "terminal-shell"}

// webTerminalSettings say how the Web UI's shells run.
type webTerminalSettings struct {
	idleLimit  time.Duration
	shellLimit int
	groupLimit int
	ringSize   int
	shell      string
}

var webTerminalSize = regexp.MustCompile(`^([0-9]{1,12})(KiB|MiB)?$`)

// parseWebTerminalSize reads a number of bytes, KiB, or MiB.
func parseWebTerminalSize(value string) (int, error) {
	match := webTerminalSize.FindStringSubmatch(value)
	if match == nil {
		return 0, fmt.Errorf("--terminal-replay-size %q must be a number of bytes, or a number with KiB or MiB", value)
	}
	size, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("--terminal-replay-size %q: %w", value, err)
	}
	switch match[2] {
	case "KiB":
		size <<= 10
	case "MiB":
		size <<= 20
	}
	if size < webTerminalMinRingSize || size > webTerminalMaxRingSize {
		return 0, fmt.Errorf("--terminal-replay-size %q must be from 1KiB to 64MiB", value)
	}
	return size, nil
}

// newWebTerminalSettings checks what the options say. A shell that cannot run is reported
// now, rather than when someone first opens the panel. from names where an option's value
// came from when it was not the command line, such as an environment variable or a config
// file, for the error to say.
func newWebTerminalSettings(idleLimit time.Duration, shellLimit, groupLimit int, replaySize, shell string, from map[string]string) (webTerminalSettings, error) {
	settings := webTerminalSettings{idleLimit: idleLimit, shellLimit: shellLimit, groupLimit: groupLimit, shell: shell}
	fail := func(name string, err error) (webTerminalSettings, error) {
		if source := from[name]; source != "" {
			err = fmt.Errorf("%w (from %s)", err, source)
		}
		return settings, err
	}
	if idleLimit < webTerminalMinIdleLimit {
		return fail("terminal-idle-timeout", fmt.Errorf("--terminal-idle-timeout %s must be at least %s", idleLimit, webTerminalMinIdleLimit))
	}
	if shellLimit < 1 {
		return fail("terminal-max-shells", fmt.Errorf("--terminal-max-shells %d must be at least 1", shellLimit))
	}
	if groupLimit < 1 {
		return fail("terminal-max-shells-per-project", fmt.Errorf("--terminal-max-shells-per-project %d must be at least 1", groupLimit))
	}
	var err error
	if settings.ringSize, err = parseWebTerminalSize(replaySize); err != nil {
		return fail("terminal-replay-size", err)
	}
	if shell != "" {
		if _, err := exec.LookPath(shell); err != nil {
			return fail("terminal-shell", fmt.Errorf("--terminal-shell: %w", err))
		}
	}
	return settings, nil
}

// newWebTerminal returns the keeper of the Web UI's shells. Each starts in the home
// directory unless its page says otherwise.
func newWebTerminal(settings webTerminalSettings) (*terminal.Keeper, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find the shell's home directory: %w", err)
	}
	shell := settings.shell
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = webTerminalDefaultShell
	}
	return terminal.NewKeeper(terminal.KeeperOptions{
		Shell:      terminal.Options{Shell: shell, Dir: home, Env: webChildEnvironment(os.Environ())},
		IdleLimit:  settings.idleLimit,
		RingSize:   settings.ringSize,
		ShellLimit: settings.shellLimit,
		GroupLimit: settings.groupLimit,
		Log:        func(message string) { fmt.Fprintln(os.Stderr, message) },
	})
}
