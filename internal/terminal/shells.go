package terminal

import (
	"path/filepath"
	"strings"
)

// shellPrograms are the programs that count as shells a user may start inside another.
var shellPrograms = map[string]bool{
	"bash": true, "zsh": true, "fish": true, "sh": true, "dash": true,
	"ksh": true, "mksh": true, "tcsh": true, "csh": true,
}

// shellOptionsWithValue are the shells' options whose value is the next argument.
var shellOptionsWithValue = map[string]bool{
	"--init-file": true, "--rcfile": true, "-o": true, "+o": true, "-O": true, "+O": true,
	"--init-command": true, "--command": true,
}

// interactiveShell says whether a command line is a shell waiting for the user, rather
// than one running a script or a command: one of shellPrograms (a login shell's leading
// "-" taken off) with options only, no operand and no -c, or with -i.
func interactiveShell(args []string) bool {
	if len(args) == 0 || !shellPrograms[filepath.Base(strings.TrimPrefix(args[0], "-"))] {
		return false
	}
	interactive, operand := false, false
	for i := 1; i < len(args) && !operand; i++ {
		arg := args[i]
		switch {
		case arg == "--":
			operand = i+1 < len(args)
		case arg == "--command" || arg == "-c":
			return false
		case arg == "--interactive":
			interactive = true
		case shellOptionsWithValue[arg]:
			i++
		case strings.HasPrefix(arg, "--"):
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			if strings.ContainsRune(arg[1:], 'c') {
				return false
			}
			interactive = interactive || strings.ContainsRune(arg[1:], 'i')
		case strings.HasPrefix(arg, "+") && len(arg) > 1:
		default:
			operand = true
		}
	}
	return interactive || !operand
}
