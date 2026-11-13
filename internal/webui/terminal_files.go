package webui

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rotari-dev/rotari/internal/terminal"
)

// The terminal panel's Explorer lists directories through terminalFilesPath: the one a
// running shell is in now (?shell=ID), or any other by its absolute path (?path=/ABS).
// The page may name any path, since whoever can use the terminal can read it there; the
// listing is served only where the terminal is, behind the same checks.

const terminalFilesPath = terminalPrefix + "files"

// terminalFilesLimit is the most entries one listing holds. Total says how many there are.
const terminalFilesLimit = 2000

// The kinds of a listed entry, and of what a symbolic link reaches.
const (
	terminalFileDir     = "dir"
	terminalFileFile    = "file"
	terminalFileSymlink = "symlink"
	terminalFileOther   = "other"
	// terminalFileMissing is a symbolic link's target that does not exist, or cannot be reached.
	terminalFileMissing = "missing"
)

// terminalFileEntry is one entry of a directory. A name that is not valid UTF-8 cannot go
// into JSON as it is: Name then holds it with U+FFFD in place of the bad bytes, and NotUTF8
// says that the page cannot name the file, as with a link's Target and TargetNotUTF8.
type terminalFileEntry struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	NotUTF8 bool   `json:"not_utf8,omitempty"`
	// Target is what a symbolic link holds, and TargetKind what it reaches.
	Target        string `json:"target,omitempty"`
	TargetNotUTF8 bool   `json:"target_not_utf8,omitempty"`
	TargetKind    string `json:"target_kind,omitempty"`
}

// terminalFilesResponse is a directory and its entries, folders first, at most
// terminalFilesLimit of them; Total is how many it has.
type terminalFilesResponse struct {
	Dir     string              `json:"dir"`
	Entries []terminalFileEntry `json:"entries"`
	Total   int                 `json:"total"`
}

func (s site) serveTerminalFiles(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := request.URL.Query()
	shell, path := query.Get("shell"), query.Get("path")
	var dir string
	switch {
	case query.Has("shell") == query.Has("path"):
		http.Error(writer, "name either a running shell (shell) or an absolute directory (path)", http.StatusBadRequest)
		return
	case query.Has("shell"):
		found, err := s.Terminal.Dir(shell)
		if errors.Is(err, terminal.ErrNoShell) {
			http.Error(writer, err.Error(), http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(writer, "where the shell is now is not known: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
		if !utf8.ValidString(found) {
			http.Error(writer, "the shell is in a directory whose name is not valid UTF-8, which the Explorer cannot name", http.StatusUnprocessableEntity)
			return
		}
		dir = found
	default:
		if err := checkTerminalDir(path); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		dir = filepath.Clean(path)
	}
	listing, err := listTerminalDir(dir)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeTerminalJSON(writer, http.StatusOK, listing)
}

// checkTerminalDir accepts a path a page names for the terminal's Explorer: absolute,
// and nothing JSON or a file system call would change.
func checkTerminalDir(path string) error {
	switch {
	case path == "":
		return errors.New("path is empty")
	case !utf8.ValidString(path):
		return errors.New("path is not valid UTF-8")
	case strings.ContainsRune(path, 0):
		return errors.New("path holds a NUL character")
	case !filepath.IsAbs(path):
		return fmt.Errorf("path %q is not absolute", path)
	}
	return nil
}

// listTerminalDir lists dir's immediate entries.
func listTerminalDir(dir string) (terminalFilesResponse, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return terminalFilesResponse{}, err
	}
	if !info.IsDir() {
		return terminalFilesResponse{}, fmt.Errorf("%s is not a directory", dir)
	}
	listed, err := os.ReadDir(dir)
	if err != nil {
		return terminalFilesResponse{}, err
	}
	entries := make([]terminalFileEntry, 0, len(listed))
	for _, item := range listed {
		entries = append(entries, terminalFileOf(dir, item))
	}
	folder := func(entry terminalFileEntry) bool {
		return entry.Kind == terminalFileDir || entry.Kind == terminalFileSymlink && entry.TargetKind == terminalFileDir
	}
	slices.SortStableFunc(entries, func(a, b terminalFileEntry) int {
		if folder(a) != folder(b) {
			if folder(a) {
				return -1
			}
			return 1
		}
		if order := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return strings.Compare(a.Name, b.Name)
	})
	total := len(entries)
	if total > terminalFilesLimit {
		entries = entries[:terminalFilesLimit]
	}
	return terminalFilesResponse{Dir: dir, Entries: entries, Total: total}, nil
}

func terminalFileOf(dir string, item fs.DirEntry) terminalFileEntry {
	name := item.Name()
	entry := terminalFileEntry{Name: strings.ToValidUTF8(name, "�"), NotUTF8: !utf8.ValidString(name), Kind: terminalFileKind(item.Type())}
	if entry.Kind != terminalFileSymlink {
		return entry
	}
	path := filepath.Join(dir, name)
	if target, err := os.Readlink(path); err == nil {
		entry.Target = strings.ToValidUTF8(target, "�")
		entry.TargetNotUTF8 = !utf8.ValidString(target)
	}
	entry.TargetKind = terminalFileMissing
	if info, err := os.Stat(path); err == nil {
		entry.TargetKind = terminalFileKind(info.Mode().Type())
	}
	return entry
}

func terminalFileKind(mode fs.FileMode) string {
	switch {
	case mode.IsDir():
		return terminalFileDir
	case mode&fs.ModeSymlink != 0:
		return terminalFileSymlink
	case mode.IsRegular():
		return terminalFileFile
	default:
		return terminalFileOther
	}
}
