package terminal

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// programNameLimit is the longest program name given, in bytes.
const programNameLimit = 64

// programName is what a process is called by its arguments: the last path element of its
// first, without a login shell's leading "-". A program chooses its own first argument, so
// what is not UTF-8, a control, or a bidirectional formatting character becomes U+FFFD, and
// a long one is cut. It is "" for a process without one.
func programName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	first := strings.TrimPrefix(args[0], "-")
	if first == "" {
		return ""
	}
	name := strings.Map(func(char rune) rune {
		if unicode.IsControl(char) || unicode.Is(unicode.Bidi_Control, char) {
			return utf8.RuneError
		}
		return char
	}, strings.ToValidUTF8(filepath.Base(first), string(utf8.RuneError)))
	if len(name) <= programNameLimit {
		return name
	}
	end := programNameLimit
	for !utf8.RuneStart(name[end]) {
		end--
	}
	return name[:end]
}
