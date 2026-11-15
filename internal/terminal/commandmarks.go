package terminal

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Command results: the server reads the OSC 133 marks a shell prints (C before a command,
// with its command line as cmdline_url=, and D;STATUS after it) as its output passes, and
// keeps how the last command ended. Marks printed while the alternate screen is on, by a
// full-screen program, are not the shell's.

// LastCommand is how a shell's last marked command ended.
type LastCommand struct {
	// Command is its command line, as the shell said it in C, or "" when it said none.
	Command string
	// Exit is its exit status, or nil when the shell gave none (no D, or one without an
	// integer).
	Exit *int
	// StartedAt is when its C passed, and EndedAt when its D, or the next prompt, did.
	StartedAt, EndedAt time.Time
	// Sequence counts the shell's ended commands, this one included, so that a reader can
	// tell a new one.
	Sequence int
}

// markLimit bounds an escape sequence held while it is read, so that output that never
// ends one cannot grow it.
const markLimit = 64 << 10

// commandMarks reads one shell's output for its commands' marks, across the chunks the
// output comes in.
type commandMarks struct {
	state     markState
	sequence  []byte
	overflow  bool
	alternate bool
	running   bool
	command   string
	startedAt time.Time
	ended     int
}

type markState int

const (
	markText markState = iota
	markEscape
	markOSC
	markOSCEscape
	markCSI
)

// read takes the next chunk of output and returns the commands that ended in it.
func (marks *commandMarks) read(output []byte, now time.Time) []LastCommand {
	var ended []LastCommand
	for _, b := range output {
		switch marks.state {
		case markText:
			if b == 0x1b {
				marks.state = markEscape
			}
		case markEscape:
			marks.sequence, marks.overflow = marks.sequence[:0], false
			switch b {
			case ']':
				marks.state = markOSC
			case '[':
				marks.state = markCSI
			case 0x1b:
			default:
				marks.state = markText
			}
		case markOSC:
			switch b {
			case 0x07:
				ended = marks.osc(now, ended)
				marks.state = markText
			case 0x1b:
				marks.state = markOSCEscape
			default:
				marks.hold(b)
			}
		case markOSCEscape:
			if b == '\\' {
				ended = marks.osc(now, ended)
				marks.state = markText
			} else {
				// An escape that does not end the OSC begins something else.
				marks.state = markEscape
				if b != 0x1b {
					marks.state = markText
				}
			}
		case markCSI:
			if b >= 0x40 && b <= 0x7e {
				marks.csi(b)
				marks.state = markText
			} else {
				marks.hold(b)
			}
		}
	}
	return ended
}

func (marks *commandMarks) hold(b byte) {
	if len(marks.sequence) < markLimit {
		marks.sequence = append(marks.sequence, b)
	} else {
		marks.overflow = true
	}
}

// csi follows the alternate screen: modes 1049, 1047, and 47, set and reset.
func (marks *commandMarks) csi(final byte) {
	if marks.overflow || (final != 'h' && final != 'l') || len(marks.sequence) == 0 || marks.sequence[0] != '?' {
		return
	}
	for _, mode := range strings.Split(string(marks.sequence[1:]), ";") {
		if mode == "1049" || mode == "1047" || mode == "47" {
			marks.alternate = final == 'h'
		}
	}
}

// osc reads one OSC 133 mark.
func (marks *commandMarks) osc(now time.Time, ended []LastCommand) []LastCommand {
	data, ok := strings.CutPrefix(string(marks.sequence), "133;")
	if marks.overflow || !ok || marks.alternate {
		return ended
	}
	parts := strings.Split(data, ";")
	switch parts[0] {
	case "C":
		marks.running, marks.startedAt, marks.command = true, now, ""
		for _, part := range parts[1:] {
			if line, ok := strings.CutPrefix(part, "cmdline_url="); ok {
				if decoded, err := url.PathUnescape(line); err == nil {
					marks.command = strings.TrimSpace(decoded)
				}
			}
		}
	case "D", "A":
		// A prompt with no D before it ends the command without an exit status.
		if !marks.running {
			return ended
		}
		var exit *int
		if parts[0] == "D" && len(parts) > 1 {
			if status, err := strconv.Atoi(parts[1]); err == nil {
				exit = &status
			}
		}
		marks.running = false
		marks.ended++
		ended = append(ended, LastCommand{Command: marks.command, Exit: exit, StartedAt: marks.startedAt, EndedAt: now, Sequence: marks.ended})
	}
	return ended
}
