// Package fileops changes files the way a terminal application's file pane does:
// making a folder or an empty file, moving, copying a file, and removing. Every
// operation refuses to replace what exists, names are checked as one path element,
// and nothing is copied between file systems behind a move. It knows nothing of
// HTTP; paths are absolute and cleaned by the caller's checks or here.
package fileops

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode"
)

// ErrBadName marks a name or path the caller should not have sent: a refused name,
// a move into itself, or the root.
var ErrBadName = errors.New("bad name")

// ErrAlreadyThere marks a move to where the item already is.
var ErrAlreadyThere = errors.New("already there")

// CheckName accepts a new name for one entry of a directory: not empty, not . or ..,
// and without /, \, or a control character. \ is allowed by Linux but refused, as
// everywhere rotari takes one path element, and a control character could not be
// typed into a shell.
func CheckName(name string) error {
	switch name {
	case "":
		return fmt.Errorf("%w: a name is needed", ErrBadName)
	case ".", "..":
		return fmt.Errorf("%w: %q is not a name", ErrBadName, name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: %q holds / or \\", ErrBadName, name)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %q holds a control character", ErrBadName, name)
		}
	}
	return nil
}

// Mkdir makes the folder path, whose parent exists.
func Mkdir(path string) error {
	path = filepath.Clean(path)
	if err := CheckName(filepath.Base(path)); err != nil {
		return err
	}
	return os.Mkdir(path, 0o777)
}

// CreateFile makes the empty file path, whose parent exists.
func CreateFile(path string) error {
	path = filepath.Clean(path)
	if err := CheckName(filepath.Base(path)); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	return file.Close()
}

// Move renames from to to, within one file system, never replacing what is at to.
// A move into itself or below itself is ErrBadName, and to where it is already
// ErrAlreadyThere. A symbolic link moves as the link.
func Move(from, to string) error {
	from, to = filepath.Clean(from), filepath.Clean(to)
	if from == to {
		return fmt.Errorf("%w: %s", ErrAlreadyThere, from)
	}
	if within(to, from) {
		return fmt.Errorf("%w: %s cannot move into itself", ErrBadName, from)
	}
	if err := CheckName(filepath.Base(to)); err != nil {
		return err
	}
	if _, err := os.Lstat(from); err != nil {
		return err
	}
	err := renameNoReplace(from, to)
	if errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("%s and %s are on different file systems, which a move here does not cross; use mv in the shell", from, filepath.Dir(to))
	}
	return err
}

// within says whether path is dir or below it.
func within(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, string(filepath.Separator))+string(filepath.Separator))
}

// renameNoReplaceFallback renames after checking that nothing is at to, for where the
// system cannot refuse to replace by itself. Something made at to in between is
// replaced; that race is the cost.
func renameNoReplaceFallback(from, to string) error {
	if _, err := os.Lstat(to); err == nil {
		// EEXIST, as Linux's no-replace rename reports it, so the message
		// ("file exists") is the same on every system; it still matches
		// fs.ErrExist.
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.EEXIST}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(from, to)
}

// CopyFile copies the file from to to and returns the path it made. A folder is
// refused, a symbolic link is copied as a link, and a file keeps its permission
// bits; what a failed copy wrote is removed. With unique, a name taken at to is not
// an error: the copy is named "name copy.ext", then "name copy 2.ext", and so on.
func CopyFile(from, to string, unique bool) (string, error) {
	from, to = filepath.Clean(from), filepath.Clean(to)
	if err := CheckName(filepath.Base(to)); err != nil {
		return "", err
	}
	info, err := os.Lstat(from)
	if err != nil {
		return "", err
	}
	var target string
	switch mode := info.Mode(); {
	case mode.IsDir():
		return "", fmt.Errorf("%w: %s is a folder, which is not copied here; use cp -r in the shell", ErrBadName, from)
	case mode&fs.ModeSymlink != 0:
		if target, err = os.Readlink(from); err != nil {
			return "", err
		}
	case !mode.IsRegular():
		return "", fmt.Errorf("%w: %s is not a regular file", ErrBadName, from)
	}
	dir, base := filepath.Dir(to), filepath.Base(to)
	for n := 0; ; n++ {
		path := to
		if n > 0 {
			path = filepath.Join(dir, CopyName(base, n))
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			err = os.Symlink(target, path)
		} else {
			err = copyRegular(from, path, info.Mode().Perm())
		}
		if unique && errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return path, nil
	}
}

// CopyName is the name of the nth copy (from 1) of base in its own folder:
// "name copy.ext", "name copy 2.ext". A leading dot is not an extension.
func CopyName(base string, n int) string {
	stem, ext := base, ""
	if dot := strings.LastIndex(base, "."); dot > 0 {
		stem, ext = base[:dot], base[dot:]
	}
	suffix := " copy"
	if n > 1 {
		suffix += " " + strconv.Itoa(n)
	}
	return stem + suffix + ext
}

func copyRegular(from, to string, perm fs.FileMode) (err error) {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := destination.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(to)
		}
	}()
	_, err = io.Copy(destination, source)
	return err
}

// Remove removes path, and everything in it when it is a folder. A symbolic link is
// removed, not what it points to. The root is refused, and a path that is not there
// is an error rather than nothing to do.
func Remove(path string) error {
	path = filepath.Clean(path)
	if path == string(filepath.Separator) || filepath.Dir(path) == path {
		return fmt.Errorf("%w: %s is not removed", ErrBadName, path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}
