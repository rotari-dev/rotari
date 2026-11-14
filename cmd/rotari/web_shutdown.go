package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rotari-dev/rotari/internal/terminal"
	"github.com/rotari-dev/rotari/internal/webdetach"
)

// webShutdownWait bounds how long --shutdown waits for the server to stop.
const webShutdownWait = 30 * time.Second

// webShutdownOptions are the options --shutdown takes beside itself.
var webShutdownOptions = []string{"shutdown", "yes", "config"}

// parseWebShutdownFlags checks a --shutdown command line: --yes only with --shutdown,
// and no server option with it, so none is silently ignored.
func parseWebShutdownFlags(fs *flag.FlagSet, shutdown, yes bool) (webCommandFlags, int) {
	if !shutdown {
		printError("--yes only applies with --shutdown")
		return webCommandFlags{}, 1
	}
	var others []string
	fs.Visit(func(flag *flag.Flag) {
		if !slices.Contains(webShutdownOptions, flag.Name) {
			others = append(others, flag.Name)
		}
	})
	if len(others) != 0 {
		printErrorf("--shutdown cannot be combined with --%s; it only stops this host's detached server", strings.Join(others, ", --"))
		return webCommandFlags{}, 1
	}
	if len(fs.Args()) != 0 {
		printError("usage: " + cliUsage("web"))
		return webCommandFlags{}, 1
	}
	return webCommandFlags{shutdown: true, yes: yes}, 0
}

// shutdownDetachedWeb stops this host's detached server, asking first when its shells
// run anything, and waits for it to stop.
func shutdownDetachedWeb(yes bool) int {
	paths, err := webDetachPaths()
	if err != nil {
		printErrorf("cannot stop rotari web: %v", err)
		return 1
	}
	running, found, err := webdetach.Find(paths)
	if err != nil {
		printErrorf("cannot stop rotari web: %v", err)
		return 1
	}
	if !found {
		fmt.Println("No detached rotari web is running on this host.")
		return 0
	}
	printDetachedWebBinaryNote(running)
	if err := confirmWebShutdown(os.Stdin, os.Stderr, stdinIsTerminal(), yes, running, webShellLines(running.PID)); err != nil {
		printError(err)
		return 1
	}
	// The record may have changed while the question waited.
	current, stillFound, err := webdetach.Find(paths)
	if err != nil {
		printErrorf("cannot stop rotari web: %v", err)
		return 1
	}
	if !stillFound || current.PID != running.PID {
		fmt.Printf("rotari web (PID %d) stopped meanwhile.\n", running.PID)
		return 0
	}
	if err := syscall.Kill(running.PID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		printErrorf("cannot stop rotari web (PID %d): %v", running.PID, err)
		return 1
	}
	for deadline := time.Now().Add(webShutdownWait); ; time.Sleep(50 * time.Millisecond) {
		current, stillFound, err = webdetach.Find(paths)
		if err != nil {
			printErrorf("cannot tell whether rotari web (PID %d) stopped: %v", running.PID, err)
			return 1
		}
		if !stillFound || current.PID != running.PID {
			break
		}
		if time.Now().After(deadline) {
			printErrorf("rotari web (PID %d) did not stop within %s; see %s", running.PID, webShutdownWait, paths.Log)
			return 1
		}
	}
	fmt.Printf("Stopped rotari web at %s (PID %d).\n", running.URL, running.PID)
	return 0
}

// webShellLines describes what runs in the server's shells, one line each for those
// running something, or one line saying why that could not be checked.
func webShellLines(serverPID int) []string {
	shells, err := terminal.ServerShells(serverPID)
	if err != nil {
		return []string{"  whether anything runs in its shells could not be checked: " + err.Error()}
	}
	var lines []string
	for _, shell := range shells {
		if !shell.Busy() {
			continue
		}
		line := "  shell (PID " + strconv.Itoa(shell.PID) + "): "
		background := len(shell.Background)
		noun := " background process"
		if background != 1 {
			noun += "es"
		}
		switch {
		case shell.Foreground != "" && background != 0:
			line += shell.Foreground + " (and " + strconv.Itoa(background) + noun + ")"
		case shell.Foreground != "":
			line += shell.Foreground
		default:
			names := []string{}
			for _, process := range shell.Background[:min(3, background)] {
				names = append(names, process.Command)
			}
			more := ""
			if background > 3 {
				more = ", ..."
			}
			line += strconv.Itoa(background) + noun + " (" + strings.Join(names, ", ") + more + ")"
		}
		lines = append(lines, line)
	}
	return lines
}

// confirmWebShutdown asks before stopping a server whose shells run something, unless
// yes is set; without a terminal it then requires yes. With nothing running it does not
// ask.
func confirmWebShutdown(input io.Reader, output io.Writer, interactive, yes bool, running webdetach.Record, lines []string) error {
	if len(lines) == 0 || yes {
		return nil
	}
	shown := lines[:min(5, len(lines))]
	if len(lines) > 5 {
		shown = append(slices.Clone(shown), fmt.Sprintf("  and %d more", len(lines)-5))
	}
	listing := strings.Join(shown, "\n")
	if !interactive {
		return fmt.Errorf("rotari web (PID %d) has shells running something:\n%s\nstopping it ends them; pass --yes to stop it when stdin is not a terminal", running.PID, listing)
	}
	fmt.Fprintf(output, "rotari web at %s (PID %d) has shells running something:\n%s\nStopping it ends every shell, and what runs in it. Stop it anyway? [y/N] ", running.URL, running.PID, listing)
	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && answer == "" {
		return fmt.Errorf("rotari web was not stopped: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return errors.New("rotari web was not stopped")
	}
	return nil
}
