package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestShortDuration(t *testing.T) {
	for given, want := range map[time.Duration]string{
		3 * time.Hour: "3h", 90 * time.Minute: "1h30m", 2 * time.Minute: "2m", 90 * time.Second: "1m30s",
		24 * time.Hour: "24h", 45 * time.Second: "45s", time.Hour + time.Second: "1h0m1s",
	} {
		if got := shortDuration(given); got != want {
			t.Errorf("shortDuration(%v) = %q, want %q", given, got, want)
		}
	}
}

func TestWatchWebClientInput(t *testing.T) {
	for name, c := range map[string]struct {
		input  string
		detach bool
	}{"Ctrl-D as end of input": {"typed\n", true}, "detach byte": {"ab\x04cd", true}} {
		detach, gone := make(chan struct{}, 1), make(chan struct{}, 1)
		watchWebClientInput(strings.NewReader(c.input), detach, gone)
		if (len(detach) == 1) != c.detach || len(gone) != 0 {
			t.Errorf("%s: detach %d, gone %d", name, len(detach), len(gone))
		}
	}
	detach, gone := make(chan struct{}, 1), make(chan struct{}, 1)
	watchWebClientInput(failingReader{}, detach, gone)
	if len(detach) != 0 || len(gone) != 1 {
		t.Errorf("a failed read: detach %d, gone %d", len(detach), len(gone))
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errTerminalGone }

// errTerminalGone is what reading a terminal that went away gives.
var errTerminalGone = errors.New("input/output error")
