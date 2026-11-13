package fileops

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// WriteNew writes content as the new file name in dir and returns its path. It never
// replaces anything: content goes into a temporary file in dir first, which is renamed
// to name without replacing, so a failure partway leaves no file under name, and the
// temporary file is removed.
func WriteNew(dir, name string, content io.Reader) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Clean(dir), name)
	if _, err := os.Lstat(path); err == nil {
		return "", &fs.PathError{Op: "write", Path: path, Err: fs.ErrExist}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	temporary, file, err := createTemporary(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(file, content)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = renameNoReplace(temporary, path)
	}
	if err != nil {
		_ = os.Remove(temporary)
		return "", err
	}
	return path, nil
}

// createTemporary makes a new hidden file in dir with the mode a new file gets, which
// os.CreateTemp would make private.
func createTemporary(dir string) (string, *os.File, error) {
	for range 16 {
		path := filepath.Join(dir, ".rotari-upload-"+rand.Text())
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return path, file, err
	}
	return "", nil, fmt.Errorf("no free temporary name in %s", dir)
}

// WriteZip writes dir as a zip archive to writer while reading it, with every entry
// under dir's own name and folders as entries of their own, so that an empty one
// survives. Symbolic links are neither followed nor stored, and other special files are
// left out. Any failure, such as a file that cannot be read, ends it with the error, and
// so does ctx ending; nothing is skipped silently.
func WriteZip(ctx context.Context, writer io.Writer, dir string) error {
	dir = filepath.Clean(dir)
	archive := zip.NewWriter(writer)
	top := filepath.Base(dir)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		name := top
		if relative != "." {
			name += "/" + filepath.ToSlash(relative)
		}
		switch mode := entry.Type(); {
		case mode&fs.ModeSymlink != 0:
			return nil
		case mode.IsDir():
			return addZipEntry(archive, entry, name+"/", "")
		case mode.IsRegular():
			return addZipEntry(archive, entry, name, path)
		default:
			return nil
		}
	})
	if err != nil {
		return err
	}
	return archive.Close()
}

func addZipEntry(archive *zip.Writer, entry fs.DirEntry, name, path string) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.NonUTF8 = !utf8.ValidString(name)
	if path == "" {
		header.Method = zip.Store
		_, err = archive.CreateHeader(header)
		return err
	}
	header.Method = zip.Deflate
	out, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(out, file)
	return err
}

// ZipName is the name a folder downloads as; the root, which has none, is "root.zip".
func ZipName(dir string) string {
	base := filepath.Base(filepath.Clean(dir))
	if base == string(filepath.Separator) {
		base = "root"
	}
	return base + ".zip"
}
