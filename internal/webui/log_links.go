package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rotari-dev/rotari/internal/jobstatus"
	"github.com/rotari-dev/rotari/internal/tableview"
)

// Paths in printed text: which words of a shell's output (terminal_links.go) or of a job's
// log name a file or directory. A page finds candidates by their form, and the server says
// which of them exist.

const (
	// linkCandidatesLimit is the most candidates one request asks about: a few lines' worth.
	linkCandidatesLimit = 64
	// linkCandidateLimit is the longest candidate, in bytes.
	linkCandidateLimit = 4096
)

// linePosition is the line, and column, that compilers and tracebacks print after a path.
var linePosition = regexp.MustCompile(`(:[0-9]+){1,2}$`)

// checkLinkCandidates refuses more, or longer, candidates than a request may have.
func checkLinkCandidates(candidates []string) error {
	if len(candidates) > linkCandidatesLimit {
		return fmt.Errorf("at most %d candidates at once", linkCandidatesLimit)
	}
	for _, candidate := range candidates {
		if len(candidate) > linkCandidateLimit {
			return fmt.Errorf("a candidate is longer than %d bytes", linkCandidateLimit)
		}
	}
	return nil
}

// printedPath is what a path in printed text names.
type printedPath struct {
	// kind is dir or file, and path absolute.
	kind, path string
	// line and column are a file's, 0 when it was printed with none.
	line, column int
}

// findPrintedPath is what a candidate names: absolute or relative to dir, asked only then
// and "" when not known, with a line and column left off its path. One without a slash needs
// a line, and names no directory, as a time would. stat says what a path is, or refuses it.
func findPrintedPath(candidate string, dir func() string, stat func(string) (os.FileInfo, error)) (printedPath, bool) {
	slash := strings.Contains(candidate, "/")
	if !slash && !linePosition.MatchString(candidate) {
		return printedPath{}, false
	}
	path := linePosition.ReplaceAllString(candidate, "")
	if !filepath.IsAbs(path) {
		base := dir()
		if base == "" {
			return printedPath{}, false
		}
		path = filepath.Join(base, path)
	}
	path = filepath.Clean(path)
	info, err := stat(path)
	switch {
	case err != nil:
		return printedPath{}, false
	case info.IsDir():
		return printedPath{kind: "dir", path: path}, slash
	case info.Mode().IsRegular():
		line, column := printedPosition(candidate)
		return printedPath{kind: "file", path: path, line: line, column: column}, true
	}
	return printedPath{}, false
}

// printedPosition is the line and column after a path, each 0 when there is none. One that
// is not positive, or too large for an editor's, is none; a column needs a line.
func printedPosition(candidate string) (line, column int) {
	number := func(text string) int {
		value, err := strconv.ParseInt(text, 10, 32)
		if err != nil || value < 1 {
			return 0
		}
		return int(value)
	}
	parts := strings.Split(strings.TrimPrefix(linePosition.FindString(candidate), ":"), ":")
	if line = number(parts[0]); line > 0 && len(parts) > 1 {
		column = number(parts[1])
	}
	return line, column
}

// logLinksRequest asks about candidates in the log of a job's attempt, its latest without
// one, as /api/log names it.
type logLinksRequest struct {
	ProjectName string   `json:"project_name"`
	RunID       string   `json:"run_id"`
	JobID       string   `json:"job_id"`
	AttemptID   string   `json:"attempt_id"`
	Candidates  []string `json:"candidates"`
}

// logLink is a candidate that names a file, by its absolute path.
type logLink struct {
	Text   string `json:"text"`
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// logLinksResponse lists the candidates that are links. DirError says why relative paths
// could not be resolved.
type logLinksResponse struct {
	Links    []logLink `json:"links"`
	DirError string    `json:"dir_error,omitempty"`
}

// serveLogLinks answers POST with which candidates in a job's log name a file on this host.
// Whoever may use the terminal may ask about any path, as they may there; others only about
// paths under the roots that the attempt's artifacts are served from.
func (s site) serveLogLinks(baseDir string, writer http.ResponseWriter, request *http.Request) {
	if mediaType, _, _ := mime.ParseMediaType(request.Header.Get(headerContentType)); mediaType != "application/json" {
		http.Error(writer, "links are asked with a JSON body", http.StatusUnsupportedMediaType)
		return
	}
	var asked logLinksRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, (linkCandidateLimit+16)*(linkCandidatesLimit+4)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&asked); err != nil {
		writeWebError(writer, fmt.Errorf("invalid request: %w", err))
		return
	}
	if err := checkLinkCandidates(asked.Candidates); err != nil {
		writeWebError(writer, err)
		return
	}
	runsDir, origin, err := webArtifactOrigin(baseDir, asked.ProjectName, asked.RunID, asked.JobID, asked.AttemptID)
	if err != nil {
		writeWebError(writer, err)
		return
	}
	// The record alone: a listing would look at each of the attempt's candidates too.
	record, _ := jobstatus.Artifacts(s.Store, runsDir, origin)
	response := logLinksResponse{Links: []logLink{}}
	workingDirectory := func() string {
		if !filepath.IsAbs(record.WorkingDirectory) {
			response.DirError = "the attempt recorded no working directory"
			return ""
		}
		return record.WorkingDirectory
	}
	stat := os.Stat
	if s.Terminal == nil || !allowsBrowserSession(request) {
		roots, _ := s.artifactRoots(jobstatus.ArtifactListing{WorkingDirectory: record.WorkingDirectory}, baseDir, asked.ProjectName, asked.RunID)
		stat = func(path string) (os.FileInfo, error) { return statUnder(path, roots) }
	}
	seen := make(map[string]bool)
	for _, candidate := range asked.Candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if found, ok := findPrintedPath(candidate, workingDirectory, stat); ok && found.kind == "file" {
			response.Links = append(response.Links, logLink{Text: candidate, Path: found.path, Line: found.line, Column: found.column})
		}
	}
	noStore(writer)
	writeWebJSON(writer, response)
}

// statUnder describes a path under one of roots, through os.Root, which refuses one that
// leaves its root, through a symbolic link too.
func statUnder(path string, roots []string) (os.FileInfo, error) {
	rootPath, relative, inside := tableview.RootOf(path, roots)
	if !inside {
		return nil, errors.New("outside the allowed roots")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Stat(relative)
}
