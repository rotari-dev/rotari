package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseWebTerminalSize(t *testing.T) {
	for value, want := range map[string]int{
		"1024": 1 << 10, "1KiB": 1 << 10, "256KiB": 256 << 10, "1MiB": 1 << 20, "64MiB": 64 << 20,
	} {
		if got, err := parseWebTerminalSize(value); err != nil || got != want {
			t.Errorf("%s: %d, %v, want %d", value, got, err, want)
		}
	}
	for _, value := range []string{"", "1023", "65MiB", "64MB", "1kib", "-1KiB", "1.5MiB", "256 KiB", "999999999999MiB"} {
		if got, err := parseWebTerminalSize(value); err == nil {
			t.Errorf("%q: %d, want an error", value, got)
		}
	}
}

func TestNewWebTerminalSettings(t *testing.T) {
	settings, err := newWebTerminalSettings(5*time.Minute, 4, 2, "1MiB", "/bin/sh", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := webTerminalSettings{idleLimit: 5 * time.Minute, shellLimit: 4, groupLimit: 2, ringSize: 1 << 20, shell: "/bin/sh"}
	if settings != want {
		t.Errorf("settings = %+v, want %+v", settings, want)
	}
	for _, test := range []struct {
		idle          time.Duration
		shells, group int
		size, shell   string
		says          string
	}{
		{time.Minute, 1, 1, "1KiB", "", "must be 0 (no limit) or at least 2m0s"},
		{-time.Hour, 1, 1, "1KiB", "", "--terminal-idle-timeout"},
		{time.Hour, 0, 1, "1KiB", "", "--terminal-max-shells"},
		{time.Hour, 1, 0, "1KiB", "", "--terminal-max-shells-per-project"},
		{time.Hour, 1, 1, "big", "", "--terminal-replay-size"},
		{time.Hour, 1, 1, "1KiB", "/no/such/shell", "--terminal-shell"},
	} {
		if _, err := newWebTerminalSettings(test.idle, test.shells, test.group, test.size, test.shell, nil); err == nil || !strings.Contains(err.Error(), test.says) {
			t.Errorf("%+v: %v", test, err)
		}
	}
}

// An idle limit of 0, the default, means none.
func TestWebTerminalIdleLimitZeroIsNone(t *testing.T) {
	settings, err := newWebTerminalSettings(0, 1, 1, "1KiB", "", nil)
	if err != nil || settings.idleLimit != 0 {
		t.Fatalf("an idle limit of 0: %+v, %v", settings, err)
	}
	if webTerminalIdleLimit != 0 {
		t.Errorf("the default idle limit is %v, want 0 (none)", webTerminalIdleLimit)
	}
}

// The defaults stand on their own, so a bad one cannot wait for someone to open the panel.
func TestWebTerminalDefaultsAreValid(t *testing.T) {
	if _, err := newWebTerminalSettings(webTerminalIdleLimit, webTerminalShellLimit, webTerminalGroupLimit, webTerminalRingSize, "", nil); err != nil {
		t.Error(err)
	}
}
