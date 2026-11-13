package fileops

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func write(t *testing.T, path, content string, perm fs.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestCheckName(t *testing.T) {
	for _, name := range []string{"a", "a b.txt", ".hidden", "日本語", "a..b"} {
		if err := CheckName(name); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "a\nb", "a\x00b", "a\x1bb", "a\u0085b"} {
		if err := CheckName(name); !errors.Is(err, ErrBadName) {
			t.Errorf("%q: %v, want ErrBadName", name, err)
		}
	}
}

func TestMakeAFolderAndAFile(t *testing.T) {
	dir := t.TempDir()
	if err := Mkdir(filepath.Join(dir, "new")); err != nil {
		t.Fatal(err)
	}
	if err := CreateFile(filepath.Join(dir, "new", "empty.txt")); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "new", "empty.txt")); err != nil || info.Size() != 0 || !info.Mode().IsRegular() {
		t.Fatalf("empty file: %v %v", info, err)
	}
	// Nothing is replaced, and a bad name makes nothing.
	write(t, filepath.Join(dir, "taken"), "keep", 0o644)
	for name, err := range map[string]error{
		"a folder over a file": Mkdir(filepath.Join(dir, "taken")),
		"a file over a file":   CreateFile(filepath.Join(dir, "taken")),
		"a file over a folder": CreateFile(filepath.Join(dir, "new")),
	} {
		if !errors.Is(err, fs.ErrExist) {
			t.Errorf("%s: %v, want it to exist", name, err)
		}
	}
	if read(t, filepath.Join(dir, "taken")) != "keep" {
		t.Error("a file was replaced")
	}
	if err := CreateFile(filepath.Join(dir, `a\b`)); !errors.Is(err, ErrBadName) {
		t.Errorf("a bad name: %v", err)
	}
	if err := Mkdir(filepath.Join(dir, "gone", "x")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("in a missing folder: %v", err)
	}
}

func TestMove(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.txt"), "a", 0o644)
	write(t, filepath.Join(dir, "b.txt"), "b", 0o644)
	if err := os.MkdirAll(filepath.Join(dir, "folder", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	// A rename, a move into a folder, and a link moved as the link.
	if err := Move(filepath.Join(dir, "a.txt"), filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	if err := Move(filepath.Join(dir, "renamed.txt"), filepath.Join(dir, "folder", "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "folder", "renamed.txt")) != "a" {
		t.Error("the moved file changed")
	}
	if err := Move(filepath.Join(dir, "link"), filepath.Join(dir, "folder", "link")); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(dir, "folder", "link")); err != nil || target != "a.txt" {
		t.Errorf("the link: %q %v", target, err)
	}
	for name, test := range map[string]struct {
		from, to string
		want     error
	}{
		"over a file":         {"b.txt", "folder/renamed.txt", fs.ErrExist},
		"over a folder":       {"b.txt", "folder", fs.ErrExist},
		"into itself":         {"folder", "folder/inner/folder", ErrBadName},
		"where it is":         {"b.txt", "b.txt", ErrAlreadyThere},
		"a bad name":          {"b.txt", "b\nc", ErrBadName},
		"something not there": {"gone", "here", fs.ErrNotExist},
	} {
		if err := Move(filepath.Join(dir, test.from), filepath.Join(dir, test.to)); !errors.Is(err, test.want) {
			t.Errorf("%s: %v, want %v", name, err, test.want)
		}
	}
	if read(t, filepath.Join(dir, "b.txt")) != "b" || read(t, filepath.Join(dir, "folder", "renamed.txt")) != "a" {
		t.Error("a refused move changed something")
	}
	// The fallback refuses as the one call does.
	if err := renameNoReplaceFallback(filepath.Join(dir, "b.txt"), filepath.Join(dir, "folder", "renamed.txt")); !errors.Is(err, fs.ErrExist) {
		t.Errorf("fallback over a file: %v", err)
	}
	if err := renameNoReplaceFallback(filepath.Join(dir, "b.txt"), filepath.Join(dir, "c.txt")); err != nil {
		t.Errorf("fallback: %v", err)
	}
}

// A move between file systems is refused, not copied. /dev/shm is a tmpfs on most Linux
// hosts, apart from the test's temporary directory.
func TestMoveDoesNotCrossFileSystems(t *testing.T) {
	other, err := os.MkdirTemp("/dev/shm", "fileops-")
	if err != nil {
		t.Skip("no /dev/shm here:", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(other) })
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a"), "a", 0o644)
	err = Move(filepath.Join(dir, "a"), filepath.Join(other, "a"))
	if err == nil {
		t.Skip("the temporary directory and /dev/shm are one file system here")
	}
	if !strings.Contains(err.Error(), "different file systems") {
		t.Errorf("%v, want it to say why", err)
	}
	if read(t, filepath.Join(dir, "a")) != "a" {
		t.Error("the refused move changed the file")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "run.sh"), "#!/bin/sh\n", 0o751)
	write(t, filepath.Join(dir, ".hidden"), "h", 0o600)
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("run.sh", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	made, err := CopyFile(filepath.Join(dir, "run.sh"), filepath.Join(dir, "folder", "run.sh"), false)
	if err != nil || made != filepath.Join(dir, "folder", "run.sh") {
		t.Fatalf("copy: %q %v", made, err)
	}
	if info, err := os.Stat(made); err != nil || info.Mode().Perm() != 0o751&^currentUmask() || read(t, made) != "#!/bin/sh\n" {
		t.Errorf("the copy: %v %v", info, err)
	}
	// Elsewhere, a name taken is refused; in its own folder, the copy gets a name of its own.
	if _, err := CopyFile(filepath.Join(dir, "run.sh"), filepath.Join(dir, "folder", "run.sh"), false); !errors.Is(err, fs.ErrExist) {
		t.Errorf("over a file: %v", err)
	}
	for _, want := range []string{"run copy.sh", "run copy 2.sh", "run copy 3.sh"} {
		if made, err := CopyFile(filepath.Join(dir, "run.sh"), filepath.Join(dir, "run.sh"), true); err != nil || made != filepath.Join(dir, want) {
			t.Errorf("a copy beside it: %q %v, want %s", made, err, want)
		}
	}
	if made, err := CopyFile(filepath.Join(dir, ".hidden"), filepath.Join(dir, ".hidden"), true); err != nil || filepath.Base(made) != ".hidden copy" {
		t.Errorf("a dot file: %q %v", made, err)
	}
	if made, err := CopyFile(filepath.Join(dir, "link"), filepath.Join(dir, "folder", "link"), false); err != nil {
		t.Errorf("a link: %v", err)
	} else if target, err := os.Readlink(made); err != nil || target != "run.sh" {
		t.Errorf("a link copied as %q %v, want a link to run.sh", target, err)
	}
	if _, err := CopyFile(filepath.Join(dir, "folder"), filepath.Join(dir, "folder2"), false); !errors.Is(err, ErrBadName) || !strings.Contains(err.Error(), "cp -r") {
		t.Errorf("a folder: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "folder2")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("a refused folder copy made something")
	}
	if _, err := CopyFile(filepath.Join(dir, "gone"), filepath.Join(dir, "here"), false); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("something not there: %v", err)
	}
}

// A copy that fails partway leaves nothing behind. Reading a directory as a file fails
// after the destination is made.
func TestCopyFileLeavesNothingWhenItFails(t *testing.T) {
	dir := t.TempDir()
	err := copyRegular(dir, filepath.Join(dir, "partial"), 0o644)
	if err == nil {
		t.Fatal("copying a directory's bytes did not fail")
	}
	if _, err := os.Lstat(filepath.Join(dir, "partial")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a failed copy left %v", err)
	}
}

func TestCopyName(t *testing.T) {
	for base, want := range map[string][2]string{
		"a.txt":          {"a copy.txt", "a copy 2.txt"},
		"archive.tar.gz": {"archive.tar copy.gz", "archive.tar copy 2.gz"},
		".bashrc":        {".bashrc copy", ".bashrc copy 2"},
		"Makefile":       {"Makefile copy", "Makefile copy 2"},
	} {
		if got := [2]string{CopyName(base, 1), CopyName(base, 2)}; got != want {
			t.Errorf("%s: %q, want %q", base, got, want)
		}
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tree", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "tree", "deep", "f"), "f", 0o644)
	write(t, filepath.Join(dir, "target"), "t", 0o644)
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "tree"), filepath.Join(dir, "dirlink")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"link", "dirlink", "tree"} {
		if err := Remove(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s is still there", name)
		}
	}
	if read(t, filepath.Join(dir, "target")) != "t" {
		t.Error("removing a link removed its target")
	}
	if err := Remove(filepath.Join(dir, "gone")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("something not there: %v", err)
	}
	if err := Remove("/"); !errors.Is(err, ErrBadName) {
		t.Errorf("the root: %v", err)
	}
}

// currentUmask is the process's file mode creation mask, which a new file's mode passes
// through. Reading it means setting it, so it is put back at once.
func currentUmask() fs.FileMode {
	mask := syscall.Umask(0)
	syscall.Umask(mask)
	return fs.FileMode(mask)
}
