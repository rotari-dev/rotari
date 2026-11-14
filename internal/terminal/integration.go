package terminal

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Shell integration: bash and zsh started so that they mark their prompts and commands
// with OSC 133, for the Web terminal's blocks, without the user's startup files being
// changed. Bash runs rotari's script in place of a login shell's files, and the script
// reads those files; zsh reads rotari's files from ZDOTDIR, and each reads the user's.
// Other shells start as they would without it.

//go:embed integration/rotari.bash integration/zsh
var integrationFiles embed.FS

// integrationBash is the script bash runs, and integrationZsh the directory zsh is
// given as ZDOTDIR, in the directory WriteIntegration fills.
const (
	integrationBash = "rotari.bash"
	integrationZsh  = "zsh"
)

// integrationUserZdotdir carries the ZDOTDIR the shell was to have, which rotari's zsh
// files read and unset.
const integrationUserZdotdir = "ROTARI_USER_ZDOTDIR"

// WriteIntegration writes the shell integration's files into a new private directory
// and returns it; the caller removes it when no shell will start with it again.
func WriteIntegration() (string, error) {
	dir, err := os.MkdirTemp("", "rotari-shell-integration-")
	if err != nil {
		return "", fmt.Errorf("make the shell integration's directory: %w", err)
	}
	write := func(from, to string) error {
		content, err := integrationFiles.ReadFile(from)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, to), content, 0o600)
	}
	err = write("integration/rotari.bash", integrationBash)
	if err == nil {
		err = os.Mkdir(filepath.Join(dir, integrationZsh), 0o700)
	}
	// zsh reads dot files, which go:embed would leave out under their own names.
	for _, name := range []string{"zshenv", "zprofile", "zshrc", "zlogin"} {
		if err == nil {
			err = write("integration/zsh/"+name, filepath.Join(integrationZsh, "."+name))
		}
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("write the shell integration's files: %w", err)
	}
	return dir, nil
}

// shellCommand is how a shell starts: its arguments and environment. With integration,
// bash and zsh, by their program's name, start with rotari's files; any shell else, and
// every shell without, starts as a login shell.
func shellCommand(options Options) ([]string, []string) {
	env := append([]string{}, options.Env...)
	if options.Integration == "" {
		return []string{"-l"}, env
	}
	switch filepath.Base(options.Shell) {
	case "bash":
		// --init-file needs a shell that is not a login shell, so the script reads the
		// login shell's files itself.
		return []string{"--init-file", filepath.Join(options.Integration, integrationBash)}, env
	case "zsh":
		user, had := "", false
		env = slices.DeleteFunc(env, func(entry string) bool {
			name, value, _ := strings.Cut(entry, "=")
			if name == "ZDOTDIR" {
				user, had = value, true
			}
			return name == "ZDOTDIR" || name == integrationUserZdotdir
		})
		env = append(env, "ZDOTDIR="+filepath.Join(options.Integration, integrationZsh))
		if had {
			env = append(env, integrationUserZdotdir+"="+user)
		}
		return []string{"-l"}, env
	}
	return []string{"-l"}, env
}
