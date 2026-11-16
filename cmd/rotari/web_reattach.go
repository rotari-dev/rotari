package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rotari-dev/rotari/internal/webdetach"
)

// webReattachPoll is how often a reattached client shows the log and checks the server.
const webReattachPoll = 200 * time.Millisecond

// detachedWebToReattach returns this host's detached server when rotari web, about to
// start a server as its client, should attach to it instead: when one runs with the
// options given. One with other options is returned with their names. A server that
// cannot be looked for is an error, never "none".
func detachedWebToReattach(flags webCommandFlags, baseDir string) (running webdetach.Record, paths webdetach.Paths, found bool, differences []string, err error) {
	if paths, err = webDetachPaths(); err != nil {
		return running, paths, false, nil, err
	}
	if running, found, err = webdetach.Find(paths); err != nil || !found {
		return running, paths, false, nil, err
	}
	want, err := webDetachSettings(flags, baseDir, flags.authToken, flags.port)
	if err != nil {
		return running, paths, false, nil, err
	}
	return running, paths, true, running.Settings.Differences(want, flags.portExplicit), nil
}

// reattachWebClient attaches to the detached server running, which outlives this
// client: it shows what the server writes to its log from now on, stops the server on
// Ctrl-C, and leaves it running on Ctrl-D, SIGHUP, SIGTERM, or a lost terminal. It
// exits once the server stops, however it was stopped.
func reattachWebClient(running webdetach.Record, paths webdetach.Paths, flags webCommandFlags) int {
	tail, err := webdetach.TailLog(paths)
	if err != nil {
		printErrorf("cannot attach to rotari web (PID %d): %v", running.PID, err)
		return 1
	}
	fmt.Printf("rotari web listening at %s in the background (PID %d, running since %s); showing its log\n", running.URL, running.PID, running.StartedAt.Local().Format(time.DateTime))
	printDetachedWebBinaryNote(running)
	printDetachedWebTokenHint(running, flags)
	printWebClientHints(flags)

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	detachKey := make(chan struct{}, 1)
	terminalGone := make(chan struct{}, 1)
	if stdinIsTerminal() {
		go watchWebClientInput(os.Stdin, detachKey, terminalGone)
	}
	ticker := time.NewTicker(webReattachPoll)
	defer ticker.Stop()
	showLog := func() {
		if err := tail.CopyTo(os.Stdout); err != nil {
			printWarningf("WARNING: cannot read %s: %v", paths.Log, err)
		}
	}
	leave := func() int {
		showLog()
		fmt.Printf("rotari web keeps running in the background at %s (PID %d)\n", running.URL, running.PID)
		printDetachedWebHelp(paths)
		return 0
	}
	var stopAsked time.Time
	for {
		select {
		case received := <-signals:
			if received != os.Interrupt {
				return leave()
			}
			if stopAsked.IsZero() {
				if code, done := stopReattachedWeb(running, paths); done {
					showLog()
					return code
				}
				stopAsked = time.Now()
			}
		case <-detachKey:
			return leave()
		case <-terminalGone:
			return 0
		case <-ticker.C:
			showLog()
			ended, clean, err := webdetach.Ended(paths, running.PID)
			if err != nil {
				printErrorf("cannot tell whether rotari web (PID %d) is running: %v", running.PID, err)
				return 1
			}
			if ended {
				showLog()
				if !clean {
					printErrorf("rotari web (PID %d) ended without removing its record, as when killed; see %s", running.PID, paths.Log)
					return 1
				}
				fmt.Printf("Stopped rotari web at %s (PID %d).\n", running.URL, running.PID)
				return 0
			}
			if !stopAsked.IsZero() && time.Since(stopAsked) > webShutdownWait {
				printErrorf("rotari web (PID %d) did not stop within %s; see %s", running.PID, webShutdownWait, paths.Log)
				return 1
			}
		}
	}
}

// stopReattachedWeb sends SIGTERM to the server, once its record shows it is still
// the one attached to, so that a reused PID is never hit. It is done, with the exit
// code, only when the server could not be signalled or had already stopped.
func stopReattachedWeb(running webdetach.Record, paths webdetach.Paths) (int, bool) {
	ended, clean, err := webdetach.Ended(paths, running.PID)
	switch {
	case err != nil:
		printErrorf("cannot stop rotari web (PID %d): %v", running.PID, err)
		return 1, true
	case ended && clean:
		fmt.Printf("Stopped rotari web at %s (PID %d).\n", running.URL, running.PID)
		return 0, true
	case ended:
		printErrorf("rotari web (PID %d) ended without removing its record, as when killed; see %s", running.PID, paths.Log)
		return 1, true
	}
	if err := syscall.Kill(running.PID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		printErrorf("cannot stop rotari web (PID %d): %v", running.PID, err)
		return 1, true
	}
	return 0, false
}

// printDetachedWebTokenHint says how to sign in to a detached server that requires a
// token this command was not given, since only a digest of it was recorded.
func printDetachedWebTokenHint(running webdetach.Record, flags webCommandFlags) {
	if running.Settings.TokenSHA256 != "" && flags.authToken == "" {
		fmt.Fprintf(os.Stderr, "Sign in with user name %q and the token printed when it started; set %s to keep one token across restarts.\n", "rotari", envWebAuthToken)
	}
}

// differentWebOptions names, as flags, options that differ from a detached server's.
func differentWebOptions(names []string) string {
	return "--" + strings.Join(names, ", --")
}
