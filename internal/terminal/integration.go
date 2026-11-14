package terminal

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// Shell integration: bash and zsh started so that they mark their prompts and commands
// with OSC 133, for the Web terminal's blocks, without the user's startup files being
// changed. Bash runs rotari's script in place of a login shell's files, and the script
// reads those files; zsh reads rotari's files from ZDOTDIR, and each reads the user's.
// Other shells start as they would without it.

//go:embed integration/rotari.bash integration/marks.bash integration/marks.zsh integration/zsh
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

// integrationLayout is each file of the integration: where it is embedded, and where it
// goes in the integration's directory. zsh reads dot files, which go:embed would leave out
// under their own names.
var integrationLayout = [][2]string{
	{"integration/rotari.bash", integrationBash},
	{"integration/marks.bash", "marks.bash"},
	{"integration/marks.zsh", "marks.zsh"},
	{"integration/zsh/zshenv", "zsh/.zshenv"},
	{"integration/zsh/zprofile", "zsh/.zprofile"},
	{"integration/zsh/zshrc", "zsh/.zshrc"},
}

// integrationVariable names the integration's directory in every shell's environment, for
// rotari's files and for shells started inside, which inherit it.
const integrationVariable = "ROTARI_SHELL_INTEGRATION"

// integrationHash names the integration's files by their content, so that a rotari with
// other files writes them beside these rather than over them.
func integrationHash() (string, error) {
	sum := sha256.New()
	for _, file := range integrationLayout {
		content, err := integrationFiles.ReadFile(file[0])
		if err != nil {
			return "", err
		}
		fmt.Fprintf(sum, "%s\x00%d\x00", file[1], len(content))
		sum.Write(content)
	}
	return hex.EncodeToString(sum.Sum(nil))[:16], nil
}

// WriteIntegration writes the shell integration's files where the shells can read them for
// as long as any of them, or anything started in them, lives: a directory under the user's
// cache named by the files' content, written once and kept, so that a tmux or an editor
// started in a shell and outliving the server still finds them. When that cannot be, it
// writes a private temporary directory instead, for the caller to remove, and says why in
// note.
func WriteIntegration() (dir string, lasting bool, note string, err error) {
	base, err := os.UserCacheDir()
	if err == nil {
		if dir, err = writeLastingIntegration(base); err == nil {
			return dir, true, "", nil
		}
	}
	note = err.Error()
	dir, err = writeTemporaryIntegration()
	return dir, false, note, err
}

// writeLastingIntegration writes the integration under base, the user's cache directory:
// rotari/shell-integration/HASH, each directory of it this user's and not writable by
// others, and each file as the server has it, rewritten when it differs.
func writeLastingIntegration(base string) (string, error) {
	hash, err := integrationHash()
	if err != nil {
		return "", err
	}
	parent := filepath.Join(base, "rotari", "shell-integration")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", fmt.Errorf("make %s: %w", parent, err)
	}
	for _, path := range []string{filepath.Dir(parent), parent} {
		if err := ownedPrivately(path, true); err != nil {
			return "", err
		}
	}
	dir := filepath.Join(parent, hash)
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		// Made whole beside it and renamed into place, so that a shell never reads half.
		staged, err := os.MkdirTemp(parent, ".staging-")
		if err != nil {
			return "", fmt.Errorf("make the shell integration's files: %w", err)
		}
		err = fillIntegration(staged)
		if err == nil {
			err = os.Rename(staged, dir)
		}
		if err != nil {
			_ = os.RemoveAll(staged)
			// Another server may have put the same files there first.
			if _, statErr := os.Lstat(dir); statErr != nil {
				return "", fmt.Errorf("write the shell integration's files: %w", err)
			}
		}
	}
	if err := ownedPrivately(dir, true); err != nil {
		return "", err
	}
	if err := ownedPrivately(filepath.Join(dir, integrationZsh), true); err != nil {
		return "", err
	}
	for _, file := range integrationLayout {
		content, err := integrationFiles.ReadFile(file[0])
		if err != nil {
			return "", err
		}
		path := filepath.Join(dir, file[1])
		if ownedPrivately(path, false) == nil {
			if kept, err := os.ReadFile(path); err == nil && bytes.Equal(kept, content) {
				continue
			}
		}
		if err := replaceFile(path, content); err != nil {
			return "", fmt.Errorf("write %s: %w", path, err)
		}
	}
	return dir, nil
}

// writeTemporaryIntegration writes the integration into a new private temporary directory.
func writeTemporaryIntegration() (string, error) {
	dir, err := os.MkdirTemp("", "rotari-shell-integration-")
	if err != nil {
		return "", fmt.Errorf("make the shell integration's directory: %w", err)
	}
	if err := fillIntegration(dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("write the shell integration's files: %w", err)
	}
	return dir, nil
}

// fillIntegration writes the integration's files into an empty directory.
func fillIntegration(dir string) error {
	if err := os.Mkdir(filepath.Join(dir, integrationZsh), 0o700); err != nil {
		return err
	}
	for _, file := range integrationLayout {
		content, err := integrationFiles.ReadFile(file[0])
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, file[1]), content, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// replaceFile puts content at path by renaming a new file over it, so that a shell reads
// the old file or the new one, never a mix.
func replaceFile(path string, content []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".replace-")
	if err != nil {
		return err
	}
	_, err = file.Write(content)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	if err != nil {
		_ = os.Remove(file.Name())
	}
	return err
}

// ownedPrivately refuses a path that is not a directory (or, without dir, a regular file),
// a symbolic link among them, of this user that others cannot write: the shells read what
// is there.
func ownedPrivately(path string, dir bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	switch {
	case dir && !info.IsDir():
		return fmt.Errorf("%s is not a directory", path)
	case !dir && !info.Mode().IsRegular():
		return fmt.Errorf("%s is not a regular file", path)
	case info.Mode().Perm()&0o022 != 0:
		return fmt.Errorf("%s can be written by others", path)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("%s belongs to another user", path)
	}
	return nil
}

// shellCommand is how a shell starts: its arguments and environment. With integration,
// bash and zsh, by their program's name, start with rotari's files; any shell else, and
// every shell without, starts as a login shell.
func shellCommand(options Options) ([]string, []string) {
	env := append([]string{}, options.Env...)
	if options.Integration == "" {
		return []string{"-l"}, env
	}
	// A server started inside a panel's shell has inherited that shell's integration: its
	// ZDOTDIR is rotari's, and the user's is in ROTARI_USER_ZDOTDIR.
	inherited := map[string]string{}
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok {
			inherited[name] = value
		}
	}
	rotariZdotdir := inherited[integrationVariable] != "" && inherited["ZDOTDIR"] == filepath.Join(inherited[integrationVariable], integrationZsh)
	env = slices.DeleteFunc(env, func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return name == integrationVariable
	})
	env = append(env, integrationVariable+"="+options.Integration)
	switch filepath.Base(options.Shell) {
	case "bash":
		// --init-file needs a shell that is not a login shell, so the script reads the
		// login shell's files itself.
		return []string{"--init-file", filepath.Join(options.Integration, integrationBash)}, env
	case "zsh":
		user, had := inherited["ZDOTDIR"]
		if rotariZdotdir {
			user, had = inherited[integrationUserZdotdir]
		}
		env = slices.DeleteFunc(env, func(entry string) bool {
			name, _, _ := strings.Cut(entry, "=")
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
