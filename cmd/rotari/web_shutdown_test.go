package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/webdetach"
)

func TestConfirmWebShutdown(t *testing.T) {
	running := webdetach.Record{PID: 42, URL: "http://127.0.0.1:1"}
	lines := []string{"  shell (PID 7): sleep 30"}
	if err := confirmWebShutdown(strings.NewReader(""), io.Discard, false, false, running, nil); err != nil {
		t.Fatalf("nothing running must not ask: %v", err)
	}
	if err := confirmWebShutdown(strings.NewReader(""), io.Discard, false, true, running, lines); err != nil {
		t.Fatalf("--yes must not ask: %v", err)
	}
	if err := confirmWebShutdown(strings.NewReader(""), io.Discard, false, false, running, lines); err == nil || !strings.Contains(err.Error(), "--yes") || !strings.Contains(err.Error(), "sleep 30") {
		t.Fatalf("without a terminal: %v", err)
	}
	var output bytes.Buffer
	if err := confirmWebShutdown(strings.NewReader("y\n"), &output, true, false, running, lines); err != nil {
		t.Fatalf("confirmed: %v", err)
	}
	if !strings.Contains(output.String(), "PID 42") || !strings.Contains(output.String(), "sleep 30") || !strings.HasSuffix(output.String(), "[y/N] ") {
		t.Fatalf("prompt = %q", output.String())
	}
	if err := confirmWebShutdown(strings.NewReader("\n"), io.Discard, true, false, running, lines); err == nil {
		t.Fatal("an empty answer stopped the server")
	}
	many := []string{"a", "b", "c", "d", "e", "f", "g"}
	if err := confirmWebShutdown(strings.NewReader(""), io.Discard, false, false, running, many); err == nil || !strings.Contains(err.Error(), "and 2 more") || strings.Contains(err.Error(), "\nf") {
		t.Fatalf("a long list is not cut to five: %v", err)
	}
}
