package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rotari-dev/rotari/internal/fileops"
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
	info, err := os.Stat(dir) // NOSONAR: any path is allowed by design; only someone who passes the terminal's checks gets here, and they already have a shell.
	if err != nil {
		return terminalFilesResponse{}, err
	}
	if !info.IsDir() {
		return terminalFilesResponse{}, fmt.Errorf("%s is not a directory", dir)
	}
	listed, err := os.ReadDir(dir) // NOSONAR: any path is allowed by design; only someone who passes the terminal's checks gets here, and they already have a shell.
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
	if target, err := os.Readlink(path); err == nil { // NOSONAR: any path is allowed by design; only someone who passes the terminal's checks gets here, and they already have a shell.
		entry.Target = strings.ToValidUTF8(target, "�")
		entry.TargetNotUTF8 = !utf8.ValidString(target)
	}
	entry.TargetKind = terminalFileMissing
	if info, err := os.Stat(path); err == nil { // NOSONAR: any path is allowed by design; only someone who passes the terminal's checks gets here, and they already have a shell.
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

// The Explorer's changes, each a POST of JSON under terminalFilesPath, served only where
// the terminal is: mkdir and create take path; move takes from and to; copy takes from,
// to, and unique (a name of its own when to is taken, for a copy pasted beside its
// file); delete takes path. Each answers the path it made or changed, or the reason it
// did not. internal/fileops holds the rules.
type terminalFileChange struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	To     string `json:"to"`
	Unique bool   `json:"unique"`
}

func (s site) serveTerminalFileChange(writer http.ResponseWriter, request *http.Request, op string) {
	writer.Header().Set("Cache-Control", "no-store")
	switch op {
	case "download":
		serveTerminalDownload(writer, request)
		return
	case "upload":
		serveTerminalUpload(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// A form from another site cannot send JSON without a preflight, which is never granted.
	if mediaType, _, _ := mime.ParseMediaType(request.Header.Get(headerContentType)); mediaType != "application/json" {
		http.Error(writer, "a change to files needs a JSON body", http.StatusUnsupportedMediaType)
		return
	}
	var change terminalFileChange
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&change); err != nil {
		http.Error(writer, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	paths := map[string][]string{"mkdir": {change.Path}, "create": {change.Path}, "delete": {change.Path}, "move": {change.From, change.To}, "copy": {change.From, change.To}}[op]
	if paths == nil {
		http.NotFound(writer, request)
		return
	}
	for _, path := range paths {
		if err := checkTerminalDir(path); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	}
	done := filepath.Clean(paths[len(paths)-1])
	var err error
	switch op {
	case "mkdir":
		err = fileops.Mkdir(done)
	case "create":
		err = fileops.CreateFile(done)
	case "delete":
		err = fileops.Remove(done)
	case "move":
		err = fileops.Move(change.From, done)
	case "copy":
		done, err = fileops.CopyFile(change.From, done, change.Unique)
	}
	writeFileChange(writer, done, err)
}

// writeFileChange answers a change with the path it made, or its refusal: 400 for what
// should not have been asked, 409 for a name taken, 422 for any other failure.
func writeFileChange(writer http.ResponseWriter, done string, err error) {
	switch {
	case errors.Is(err, fileops.ErrBadName):
		http.Error(writer, err.Error(), http.StatusBadRequest)
	case errors.Is(err, fs.ErrExist), errors.Is(err, fileops.ErrAlreadyThere):
		http.Error(writer, err.Error(), http.StatusConflict)
	case err != nil:
		http.Error(writer, err.Error(), http.StatusUnprocessableEntity)
	default:
		writeTerminalJSON(writer, http.StatusOK, map[string]string{"path": done})
	}
}

// serveTerminalDownload sends ?path= as an attachment: a file as it is, with ranges, and
// a folder as a zip written while it is read. A zip that fails partway, as on a file it
// cannot read, aborts the connection, so the browser reports a failed download rather
// than keeping a short archive.
func serveTerminalDownload(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := request.URL.Query().Get("path")
	if err := checkTerminalDir(path); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path) // NOSONAR: any path is allowed by design; only someone who passes the terminal's checks gets here, and they already have a shell.
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	switch {
	case info.IsDir():
		writer.Header().Set(headerContentType, "application/zip")
		writer.Header().Set("Content-Disposition", attachmentDisposition(fileops.ZipName(path)))
		if request.Method == http.MethodHead {
			return
		}
		if err := fileops.WriteZip(request.Context(), writer, path); err != nil {
			if request.Context().Err() == nil {
				log.Printf("terminal download of %s: %v", path, err)
			}
			panic(http.ErrAbortHandler)
		}
	case info.Mode().IsRegular():
		file, err := os.Open(path) // NOSONAR: any path is allowed by design; only someone who passes the terminal's checks gets here, and they already have a shell.
		if err != nil {
			http.Error(writer, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		defer file.Close()
		writer.Header().Set(headerContentType, "application/octet-stream")
		writer.Header().Set("Content-Disposition", attachmentDisposition(filepath.Base(path)))
		http.ServeContent(writer, request, "", info.ModTime(), file)
	default:
		http.Error(writer, path+" is neither a file nor a folder", http.StatusBadRequest)
	}
}

// serveTerminalUpload writes the body as the new file ?name= in ?dir=, never replacing
// anything. It needs the file as application/octet-stream, which a form from another
// site cannot send.
func serveTerminalUpload(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if mediaType, _, _ := mime.ParseMediaType(request.Header.Get(headerContentType)); mediaType != "application/octet-stream" {
		http.Error(writer, "an upload sends the file as application/octet-stream", http.StatusUnsupportedMediaType)
		return
	}
	query := request.URL.Query()
	dir := query.Get("dir")
	if err := checkTerminalDir(dir); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	path, err := fileops.WriteNew(dir, query.Get("name"), request.Body)
	writeFileChange(writer, path, err)
}
