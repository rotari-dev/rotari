package terminal

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCommandMarks(t *testing.T) {
	now := time.Unix(1000, 0)
	read := func(marks *commandMarks, chunks ...string) []LastCommand {
		var ended []LastCommand
		for i, chunk := range chunks {
			ended = append(ended, marks.read([]byte(chunk), now.Add(time.Duration(i)*time.Second))...)
		}
		return ended
	}
	status := func(command LastCommand) string {
		if command.Exit == nil {
			return command.Command + ":none"
		}
		return command.Command + ":" + string(rune('0'+*command.Exit))
	}
	var marks commandMarks
	// A command, its marks split across chunks, its line said percent-encoded, ended by ST.
	ended := read(&marks, "\x1b]133;A\a$ \x1b]133;B\a", "make all\r\n\x1b]13", "3;C;cmdline_url=make%20all\a", "built\r\n\x1b]133;D;2\x1b", "\\")
	if len(ended) != 1 || status(ended[0]) != "make all:2" || ended[0].Sequence != 1 || ended[0].EndedAt.Sub(ended[0].StartedAt) != 2*time.Second {
		t.Fatalf("first command: %+v", ended)
	}
	// An exit status that is no number is none, and an empty line (D with no C) is nothing.
	ended = read(&marks, "\x1b]133;D;0\a\x1b]133;A\a", "\x1b]133;C\afoo\x1b]133;D;x\a")
	if len(ended) != 1 || status(ended[0]) != ":none" || ended[0].Sequence != 2 {
		t.Fatalf("a command with no line and a bad status: %+v", ended)
	}
	// A prompt with no D before it ends the command without a status.
	ended = read(&marks, "\x1b]133;C;cmdline_url=sleep%209\a^C\r\n\x1b]133;A\a")
	if len(ended) != 1 || status(ended[0]) != "sleep 9:none" {
		t.Fatalf("a command ended by a prompt: %+v", ended)
	}
	// Marks in the alternate screen are a full-screen program's, not the shell's.
	ended = read(&marks, "\x1b]133;C;cmdline_url=vim\a\x1b[?1049h", "\x1b]133;D;5\a\x1b]133;C\a\x1b[?1049l", "\x1b]133;D;0\a")
	if len(ended) != 1 || status(ended[0]) != "vim:0" {
		t.Fatalf("around the alternate screen: %+v", ended)
	}
	// An escape sequence longer than the limit is dropped, and what follows still read.
	ended = read(&marks, "\x1b]133;C;cmdline_url="+strings.Repeat("a", markLimit)+"\a", "\x1b]133;C;cmdline_url=ok\a\x1b]133;D;0\a")
	if len(ended) != 1 || status(ended[0]) != "ok:0" {
		t.Fatalf("after a sequence past the limit: %+v", ended)
	}
	// A line that does not decode is none.
	ended = read(&marks, "\x1b]133;C;cmdline_url=%zz\a\x1b]133;D;1\a")
	if len(ended) != 1 || status(ended[0]) != ":1" {
		t.Fatalf("an undecodable line: %+v", ended)
	}
}

// The Keeper keeps how a shell's last command ended, read from the marks rotari's bash
// prints, and counts the commands.
func TestKeeperKeepsTheLastCommand(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	useCache(t)
	home := t.TempDir()
	keeper, err := NewKeeper(KeeperOptions{Shell: Options{Shell: bash, Dir: home, Env: []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PS1=$ "}}, RingSize: 1 << 16, ShellLimit: 1, GroupLimit: 1, ShellIntegration: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	info, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	attachment, err := keeper.Attach(info.ID, 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			if _, ok := attachment.Next(); !ok {
				return
			}
		}
	}()
	last := func(what string, want func(*LastCommand) bool) {
		t.Helper()
		var got *LastCommand
		for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			got = keeper.Shells()[0].LastCommand
			if want(got) {
				return
			}
		}
		t.Fatalf("%s: %+v", what, got)
	}
	if _, err := attachment.Write([]byte("sleep 0.2; false\n")); err != nil {
		t.Fatal(err)
	}
	last("false", func(got *LastCommand) bool {
		return got != nil && got.Command == "sleep 0.2; false" && got.Exit != nil && *got.Exit == 1 && got.Sequence == 1 && got.EndedAt.Sub(got.StartedAt) >= 200*time.Millisecond
	})
	if _, err := attachment.Write([]byte("true\n")); err != nil {
		t.Fatal(err)
	}
	last("true", func(got *LastCommand) bool {
		return got != nil && got.Command == "true" && got.Exit != nil && *got.Exit == 0 && got.Sequence == 2
	})
}
