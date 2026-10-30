// Package tableview reads what the Table viewer shows: delimited text and
// JSON Lines files or a directory's files as rows of cells, a page at a
// time, and the image, audio, video, and PDF files its cells name. It knows
// nothing of jobs, runs, or HTTP: callers supply a Target, the file or
// directory and the roots whose files may be read.
package tableview

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Target is what a viewer shows: a table or document file, or a directory,
// by absolute path, under one of Roots. Every file the viewer reads, the
// target and the media its cells name, is opened through os.Root under a
// root that lexically contains it, so a symlink cannot leave the roots.
type Target struct {
	Path  string
	Roots []string
	// Bases are directories a relative media path in a file's cells may be
	// resolved from, the first being the default; the file's own directory
	// (FileBase) is added after them. A directory's paths are relative to
	// the directory itself (DirectoryBase).
	Bases []Base
}

// Base is a directory media paths may be relative to.
type Base struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"-"`
}

// The bases a Target adds for itself.
const (
	FileBase      = "file"
	DirectoryBase = "directory"
)

// ErrOutsideRoots marks a path that no allowed root contains.
var ErrOutsideRoots = errors.New("outside the allowed roots")

// RootOf returns the first root that lexically contains path, and path
// relative to it. Opening through os.Root then refuses a symlink that
// leaves the root.
func RootOf(path string, roots []string) (string, string, bool) {
	for _, root := range roots {
		relative, err := filepath.Rel(root, path)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return root, relative, true
		}
	}
	return "", "", false
}

// open opens path through the root that contains it.
func (target Target) open(path string) (*os.File, error) {
	rootPath, relative, ok := RootOf(path, target.Roots)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrOutsideRoots, path)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(relative)
}

// Stat describes the target, opened through its root.
func (target Target) Stat() (os.FileInfo, error) {
	file, err := target.open(target.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

// AllBases lists the bases media paths of the target may be relative to:
// for a file, the caller's and then the file's directory; for a directory,
// only the directory.
func (target Target) AllBases(directory bool) []Base {
	if directory {
		return []Base{{ID: DirectoryBase, Label: "this directory", Path: target.Path}}
	}
	bases := append([]Base(nil), target.Bases...)
	return append(bases, Base{ID: FileBase, Label: "this file's directory", Path: filepath.Dir(target.Path)})
}

// ResolveMedia returns the absolute path of a supported preview file a cell
// names: an absolute path as it is, or a relative one joined to the base
// with that ID. The file must be under a root; whether it exists is found
// when it is opened.
func (target Target) ResolveMedia(directory bool, baseID, ref string) (string, error) {
	if ref == "" || strings.ContainsRune(ref, 0) {
		return "", fmt.Errorf("a media path is required")
	}
	if MediaKind(ref) == "" {
		return "", fmt.Errorf("%s is not a supported preview file", ref)
	}
	return target.ResolvePath(directory, baseID, ref)
}

// ResolvePath returns the absolute path of any file a cell names, as
// ResolveMedia does without asking for a media file: one the viewer reads
// as text.
func (target Target) ResolvePath(directory bool, baseID, ref string) (string, error) {
	if ref == "" || strings.ContainsRune(ref, 0) {
		return "", fmt.Errorf("a path is required")
	}
	path := ref
	if !filepath.IsAbs(path) {
		base, err := target.base(directory, baseID)
		if err != nil {
			return "", err
		}
		path = filepath.Join(base.Path, ref)
	}
	path = filepath.Clean(path)
	if _, _, ok := RootOf(path, target.Roots); !ok {
		return "", fmt.Errorf("%w: %s", ErrOutsideRoots, path)
	}
	return path, nil
}

// Open opens a file ResolveMedia or ResolvePath returned.
func (target Target) Open(path string) (*os.File, error) {
	return target.open(path)
}

func (target Target) base(directory bool, id string) (Base, error) {
	bases := target.AllBases(directory)
	if id == "" {
		return bases[0], nil
	}
	for _, base := range bases {
		if base.ID == id {
			return base, nil
		}
	}
	ids := make([]string, 0, len(bases))
	for _, base := range bases {
		ids = append(ids, base.ID)
	}
	return Base{}, fmt.Errorf("base must be one of %s", strings.Join(ids, ", "))
}

// MediaTypes are the files shown inline, by lower-case extension: images,
// audio, video, and PDFs.
var MediaTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".svg": "image/svg+xml",
	".wav": "audio/wav", ".mp3": "audio/mpeg", ".flac": "audio/flac",
	".ogg": "audio/ogg", ".oga": "audio/ogg", ".opus": "audio/ogg", ".m4a": "audio/mp4",
	".mp4": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime",
	".pdf": "application/pdf",
}

// MediaKind is "image", "audio", "video", or "pdf" for a name
// MediaTypes lists, and "" for any other.
func MediaKind(name string) string {
	contentType, ok := MediaTypes[strings.ToLower(filepath.Ext(name))]
	if !ok {
		return ""
	}
	kind, _, _ := strings.Cut(contentType, "/")
	if kind == "application" {
		return "pdf"
	}
	return kind
}

// DocumentPreview reports a document the viewer opens directly: it is
// "markdown" for Markdown files, "text" for supported source/config files,
// and empty for other names. Media types such as PDF are classified separately.
func DocumentPreview(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown":
		return "markdown"
	case ".py", ".sh", ".bash", ".zsh", ".js", ".mjs", ".cjs", ".ts", ".tsx",
		".go", ".rs", ".java", ".c", ".h", ".cc", ".cpp", ".cxx", ".hpp",
		".cs", ".rb", ".php", ".swift", ".kt", ".kts", ".lua", ".pl", ".r",
		".sql", ".json", ".yaml", ".yml", ".xml", ".html", ".htm", ".css",
		".ini", ".cfg", ".conf", ".toml", ".diff", ".patch":
		return "text"
	default:
		base := filepath.Base(name)
		if strings.EqualFold(base, "Dockerfile") || strings.EqualFold(base, "Makefile") {
			return "text"
		}
		return ""
	}
}
