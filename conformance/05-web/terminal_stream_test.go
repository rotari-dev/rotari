package web

import (
	"context"
	"strings"
	"testing"
)

func TestTerminalStreamsOutputBurst(t *testing.T) {
	covers(t, "WEB-34")
	e := terminalEnv(t)
	page := openTerminalPage(t, e.StartWeb("--allow-terminal"))
	const lines = 1000000
	const line = "line\r\n"
	const done = "rotari-burst-2\r\n"
	page.typeLine(`yes line | head -n 1000000; echo "rotari-burst-$((1+1))"`)

	ctx, cancel := context.WithTimeout(context.Background(), terminalWait)
	defer cancel()
	var lineTail, doneTail string
	var count, size int
	for {
		_, data, err := page.connection.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for the burst: %v; received %d bytes and %d lines", err, size, count)
		}
		size += len(data)
		window := lineTail + string(data)
		count += strings.Count(window, line)
		lineTail = streamTail(window, len(line)-1)
		window = doneTail + string(data)
		if strings.Contains(window, done) {
			break
		}
		doneTail = streamTail(window, len(done)-1)
	}
	if count != lines {
		t.Errorf("the terminal delivered %d lines, want %d", count, lines)
	}
}

func streamTail(text string, length int) string {
	if len(text) > length {
		return text[len(text)-length:]
	}
	return text
}
