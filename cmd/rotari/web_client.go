package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rotari-dev/rotari/internal/webdetach"
)

// webClientDrain bounds how long a detaching client waits for the server's last output.
const webClientDrain = time.Second

// webRunsAsClient says whether rotari web starts its server in the background and
// stays attached as its client: only when the server could be detached, by Ctrl-D at a
// terminal or by --disconnect-action detach. Otherwise, as from a script or a service,
// it serves in its own process as it always has.
func webRunsAsClient(flags webCommandFlags) bool {
	return stdinIsTerminal() || flags.disconnectAction == webDisconnectDetach
}

// runWebClient starts the server of flags in a session of its own and stays attached:
// it shows the server's output, stops it on Ctrl-C, detaches it on Ctrl-D, and on
// SIGHUP, SIGTERM, or a lost terminal lets it follow --disconnect-action. A server the
// client loses without a word, such as to SIGKILL, follows that action too.
func runWebClient(flags webCommandFlags) int {
	action := flags.disconnectAction
	if action == webDisconnectDetach {
		paths, err := webDetachPaths()
		if err == nil {
			var running webdetach.Record
			var found bool
			if running, found, err = webdetach.Find(paths); err == nil && found {
				printErrorf("--disconnect-action detach: a detached rotari web is already running on this host at %s (PID %d), so this one could not detach; stop it with rotari web --shutdown, or use --disconnect-action stop", running.URL, running.PID)
				return 1
			}
		}
		if err != nil {
			printErrorf("--disconnect-action detach: %v", err)
			return 1
		}
	}
	executable, err := os.Executable()
	if err != nil {
		printErrorf("cannot start rotari web: %v", err)
		return 1
	}
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		printErrorf("cannot start rotari web: %v", err)
		return 1
	}
	defer devNull.Close()
	var pipes [4][2]*os.File // stdout, stderr, readiness, lifeline: read, write
	for index := range pipes {
		if pipes[index][0], pipes[index][1], err = os.Pipe(); err != nil {
			printErrorf("cannot start rotari web: %v", err)
			return 1
		}
	}
	child := exec.Command(executable, os.Args[1:]...)
	child.Env = append(os.Environ(), webDetachedChildEnv+"="+webChildAttachedPrefix+action)
	child.Stdin, child.Stdout, child.Stderr = devNull, pipes[0][1], pipes[1][1]
	child.ExtraFiles = []*os.File{pipes[2][1], pipes[3][0]} // webDetachedReadyFD, webAttachedLifelineFD
	// A session of its own: Ctrl-C reaches the client, which decides.
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err = child.Start()
	// The child has its own copies of these.
	for _, end := range []*os.File{pipes[0][1], pipes[1][1], pipes[2][1], pipes[3][0]} {
		_ = end.Close()
	}
	if err != nil {
		printErrorf("cannot start rotari web: %v", err)
		return 1
	}
	lifeline := pipes[3][1]
	tell := func(message string) { _, _ = fmt.Fprintln(lifeline, message) }

	var relays sync.WaitGroup
	for _, relay := range []struct {
		from *os.File
		to   io.Writer
	}{{pipes[0][0], os.Stdout}, {pipes[1][0], os.Stderr}} {
		relays.Add(1)
		go func() {
			defer relays.Done()
			_, _ = io.Copy(relay.to, relay.from)
		}()
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	events := make(chan webDetachedReady)
	go func() {
		defer close(events)
		lines := bufio.NewScanner(pipes[2][0])
		for lines.Scan() {
			var event webDetachedReady
			if json.Unmarshal(lines.Bytes(), &event) == nil {
				events <- event
			}
		}
	}()
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	detachKey := make(chan struct{}, 1)
	terminalGone := make(chan struct{}, 1)
	watchKeys := func() {
		if stdinIsTerminal() {
			go watchWebClientInput(os.Stdin, detachKey, terminalGone)
		}
	}

	for {
		select {
		case event, open := <-events:
			if !open {
				events = nil
				continue
			}
			switch event.Event {
			case webEventReady:
				printWebClientHints(flags)
				watchKeys()
			case webEventDetached:
				drainRelays(&relays)
				fmt.Printf("rotari web keeps running in the background at %s (PID %d)\n", event.URL, event.PID)
				if paths, err := webDetachPaths(); err == nil {
					printDetachedWebHelp(paths)
				}
				return 0
			case webEventRefused:
				printErrorf("rotari web cannot detach: %s", event.Reason)
				watchKeys()
			}
		case received := <-signals:
			if received == os.Interrupt {
				tell(webLifelineStop)
			} else {
				tell(webLifelineDisconnect)
			}
		case <-detachKey:
			tell(webLifelineDetach)
		case <-terminalGone:
			tell(webLifelineDisconnect)
		case err := <-exited:
			relays.Wait()
			var exitErr *exec.ExitError
			switch {
			case err == nil:
				return 0
			case errors.As(err, &exitErr) && exitErr.ExitCode() >= 0:
				return exitErr.ExitCode()
			default:
				printErrorf("rotari web ended: %v", err)
				return 1
			}
		}
	}
}

// drainRelays gives the relays a moment to pass on what the server wrote before it
// moved its output to the log.
func drainRelays(relays *sync.WaitGroup) {
	done := make(chan struct{})
	go func() { relays.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(webClientDrain):
	}
}

// watchWebClientInput reports Ctrl-D, which a canonical-mode terminal gives as the end
// of input at the start of a line, or the detach byte, and a terminal that went away.
// Other input is discarded.
func watchWebClientInput(input io.Reader, detach, gone chan<- struct{}) {
	var buffer [256]byte
	for {
		n, err := input.Read(buffer[:])
		if bytes.IndexByte(buffer[:n], terminalDetachByte) >= 0 || errors.Is(err, io.EOF) {
			detach <- struct{}{}
			return
		}
		if err != nil {
			gone <- struct{}{}
			return
		}
	}
}

// printWebClientHints says, once the server listens, how to detach and how long shells
// last.
func printWebClientHints(flags webCommandFlags) {
	if stdinIsTerminal() {
		fmt.Fprintln(os.Stderr, "Press Ctrl-D to detach and keep the server running; Ctrl-C to stop it.")
	}
	if flags.allowTerminal {
		fmt.Fprintln(os.Stderr, webIdleTimeoutLine(flags.terminal.idleLimit))
	}
}

// webIdleTimeoutLine says how long shells last: with no limit, until the server stops
// or the user signs out; otherwise the limit, written without zero units.
func webIdleTimeoutLine(limit time.Duration) string {
	if limit == 0 {
		return "Shells end when the server stops or you sign out."
	}
	return fmt.Sprintf("Shells end %s after the last page closes (--terminal-idle-timeout), and when the server stops.", shortDuration(limit))
}

// shortDuration writes d as Go does, without the zero minutes and seconds Go adds after
// hours or minutes: 3h, 1h30m, 90s.
func shortDuration(d time.Duration) string {
	text := d.String()
	if strings.HasSuffix(text, "m0s") {
		text = strings.TrimSuffix(text, "0s")
	}
	if strings.HasSuffix(text, "h0m") {
		text = strings.TrimSuffix(text, "0m")
	}
	return text
}
