package fileops

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// failingReader gives some bytes, then fails, as a broken upload does.
type failingReader struct{ given bool }

func (reader *failingReader) Read(buffer []byte) (int, error) {
	if reader.given {
		return 0, errors.New("connection reset")
	}
	reader.given = true
	return copy(buffer, "partial"), nil
}

func leftIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestWriteNew(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteNew(dir, "up load.csv", strings.NewReader("a,b\n"))
	if err != nil || path != filepath.Join(dir, "up load.csv") || read(t, path) != "a,b\n" {
		t.Fatalf("write: %q %v", path, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o666&^currentUmask() {
		t.Errorf("mode: %v %v", info, err)
	}
	if _, err := WriteNew(dir, "up load.csv", strings.NewReader("other")); !errors.Is(err, fs.ErrExist) {
		t.Errorf("over a file: %v", err)
	}
	if read(t, path) != "a,b\n" {
		t.Error("a file was replaced")
	}
	if _, err := WriteNew(dir, "a/b", strings.NewReader("x")); !errors.Is(err, ErrBadName) {
		t.Errorf("a bad name: %v", err)
	}
	if _, err := WriteNew(dir, "broken.bin", &failingReader{}); err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("a broken upload: %v", err)
	}
	if names := leftIn(t, dir); strings.Join(names, ",") != "up load.csv" {
		t.Errorf("left %v", names)
	}
	if _, err := WriteNew(filepath.Join(dir, "gone"), "x", strings.NewReader("x")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("into a missing folder: %v", err)
	}
}

func zipEntries(t *testing.T, archive []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]string{}
	for _, file := range reader.File {
		opened, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(opened)
		opened.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name] = string(content)
	}
	return entries
}

func TestWriteZip(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "results")
	for _, sub := range []string{"empty", "deep/er"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(dir, "a.csv"), "a\n", 0o644)
	write(t, filepath.Join(dir, "deep", "er", "b.txt"), "b\n", 0o644)
	write(t, filepath.Join(dir, "raw\xffname"), "r", 0o644)
	write(t, filepath.Join(root, "outside"), "secret", 0o644)
	if err := os.Symlink(filepath.Join(root, "outside"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := WriteZip(context.Background(), &archive, dir); err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, archive.Bytes())
	var names []string
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	want := []string{"results/", "results/a.csv", "results/deep/", "results/deep/er/", "results/deep/er/b.txt", "results/empty/", "results/raw\xffname"}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Errorf("entries %q, want %q", names, want)
	}
	if entries["results/deep/er/b.txt"] != "b\n" || entries["results/raw\xffname"] != "r" {
		t.Errorf("contents %q", entries)
	}
	if ZipName(dir) != "results.zip" || ZipName("/") != "root.zip" {
		t.Errorf("names %q %q", ZipName(dir), ZipName("/"))
	}
}

func TestWriteZipFailsRatherThanSkipping(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "locked"), "x", 0)
	if err := WriteZip(context.Background(), io.Discard, dir); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("an unreadable file: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WriteZip(ctx, io.Discard, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled download: %v", err)
	}
}
