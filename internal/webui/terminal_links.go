package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rotari-dev/rotari/internal/resolve"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/terminal"
)

// The panel asks terminalLinksName under a shell's path which words of its output are
// links: a run ID, a job ID, or a path that exists. The page finds candidates by their
// form; only the server knows which of them name something, so a nine-hex word that is
// no job, such as a git hash, stays text.

// terminalLinksName follows a shell's path for the links in its output.
const terminalLinksName = "links"

const (
	// terminalLinksLimit is the most candidates one request asks about: a few lines' worth.
	terminalLinksLimit = 64
	// terminalLinkLimit is the longest candidate, in bytes.
	terminalLinkLimit = 4096
	// terminalLinkJobRunsRead is how many of a project's newest runs are searched for a
	// job, so that a hover costs as many snapshot reads at most.
	terminalLinkJobRunsRead = 20
)

var (
	// terminalRunIDForm is a run ID as rotari makes one: its UTC start and a random tail.
	terminalRunIDForm = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[0-9a-f]{8}$`)
	// terminalJobIDForm is a job ID as rotari makes one, or an array task's.
	terminalJobIDForm = regexp.MustCompile(`^[0-9a-f]{9}(-[0-9]+)?$`)
	// terminalLinePosition is the line, and column, that compilers and tracebacks print
	// after a path.
	terminalLinePosition = regexp.MustCompile(`(:[0-9]+){1,2}$`)
)

// terminalLinksRequest asks about candidates. Dir, when given, is the directory the
// command that printed them ran in, as the shell said it (OSC 7): relative paths are
// resolved there rather than where the shell is now. Host is the host that directory is
// on, as the shell said it; another host's paths are no links here.
type terminalLinksRequest struct {
	Candidates []string `json:"candidates"`
	Dir        string   `json:"dir,omitempty"`
	Host       string   `json:"host,omitempty"`
}

// terminalLink is a candidate that names something: Kind is run, job, dir, or file. A
// run or job has the page's Link; a path has its absolute Path, without a line number. A
// file has the Line, and Column, given after it, when they are positive. Project is the
// run's or job's project.
type terminalLink struct {
	Text    string `json:"text"`
	Kind    string `json:"kind"`
	Link    string `json:"link,omitempty"`
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Project string `json:"project,omitempty"`
}

// terminalLinksResponse lists the candidates that are links. DirError says why relative
// paths could not be resolved, and Problems what could not be read while looking, such
// as a run's snapshot; neither is a reason to answer nothing.
type terminalLinksResponse struct {
	Links    []terminalLink `json:"links"`
	DirError string         `json:"dir_error,omitempty"`
	Problems []string       `json:"problems,omitempty"`
}

// serveTerminalLinks answers POST with which candidates are links. It attaches to
// nothing, so it takes the shell from no page.
func (s site) serveTerminalLinks(writer http.ResponseWriter, request *http.Request, id string) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// A form from another site cannot send JSON without a preflight, which is never granted.
	if mediaType, _, _ := mime.ParseMediaType(request.Header.Get(headerContentType)); mediaType != "application/json" {
		http.Error(writer, "links are asked with a JSON body", http.StatusUnsupportedMediaType)
		return
	}
	var asked terminalLinksRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, (terminalLinkLimit+16)*(terminalLinksLimit+2)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&asked); err != nil {
		http.Error(writer, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(asked.Candidates) > terminalLinksLimit {
		http.Error(writer, fmt.Sprintf("at most %d candidates at once", terminalLinksLimit), http.StatusBadRequest)
		return
	}
	for _, candidate := range asked.Candidates {
		if len(candidate) > terminalLinkLimit {
			http.Error(writer, fmt.Sprintf("a candidate is longer than %d bytes", terminalLinkLimit), http.StatusBadRequest)
			return
		}
	}
	if asked.Dir != "" && (!filepath.IsAbs(asked.Dir) || len(asked.Dir) > terminalLinkLimit) {
		http.Error(writer, fmt.Sprintf("dir must be an absolute path of at most %d bytes", terminalLinkLimit), http.StatusBadRequest)
		return
	}
	if len(asked.Host) > terminalLinkLimit {
		http.Error(writer, fmt.Sprintf("host is longer than %d bytes", terminalLinkLimit), http.StatusBadRequest)
		return
	}
	group, ok := s.terminalShellGroup(id)
	if !ok {
		http.Error(writer, terminal.ErrNoShell.Error(), http.StatusNotFound)
		return
	}
	finder := terminalLinkFinder{site: s, shell: id, group: group, projects: make(map[string][]string)}
	if asked.Dir != "" {
		finder.dir, finder.dirRead = asked.Dir, true
	}
	if asked.Host != "" {
		local, err := terminalLocalHost(asked.Host)
		if err != nil {
			finder.problem(err)
		}
		finder.remote = !local
	}
	response := terminalLinksResponse{Links: []terminalLink{}}
	seen := make(map[string]bool)
	for _, candidate := range asked.Candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if link, ok := finder.find(candidate); ok {
			response.Links = append(response.Links, link)
		}
	}
	response.DirError, response.Problems = finder.dirError, finder.problems
	writeTerminalJSON(writer, http.StatusOK, response)
}

// terminalLinkFinder resolves one request's candidates for a shell, reading each
// basedir's projects and the shell's directory once.
type terminalLinkFinder struct {
	site     site
	shell    string
	group    string
	projects map[string][]string
	dir      string
	dirRead  bool
	dirError string
	problems []string
	// remote is set when the candidates were printed on another host, whose paths are
	// no links here.
	remote bool
}

func (finder *terminalLinkFinder) find(candidate string) (terminalLink, bool) {
	link := terminalLink{Text: candidate}
	switch {
	case terminalRunIDForm.MatchString(candidate):
		link.Kind = "run"
		link.Project, link.Link = finder.run(candidate)
		return link, link.Link != ""
	case terminalJobIDForm.MatchString(candidate):
		link.Kind = "job"
		link.Project, link.Link = finder.job(candidate)
		return link, link.Link != ""
	case finder.remote:
		return link, false
	case strings.Contains(candidate, "/") || terminalLinePosition.MatchString(candidate):
		link.Kind, link.Path = finder.path(candidate)
		if link.Kind == "dir" && !strings.Contains(candidate, "/") {
			// A word and a number, such as a time, names no directory.
			return link, false
		}
		if link.Kind == "file" {
			link.Line, link.Column = terminalPosition(candidate)
		}
		return link, link.Path != ""
	}
	return link, false
}

// run is the page of a run: of the shell's project, another project of its basedir, or,
// for a shell of no project, a project of any basedir.
func (finder *terminalLinkFinder) run(runID string) (project, link string) {
	baseDir, own := terminalGroupPlace(finder.group)
	baseDirs := []string{baseDir}
	if baseDir == "" {
		baseDirs = nil
		for _, entry := range finder.site.basedirEntries() {
			if !entry.Missing {
				baseDirs = append(baseDirs, entry.Path)
			}
		}
	}
	for _, baseDir := range baseDirs {
		projects := finder.projectsOf(baseDir)
		if own != "" {
			projects = append([]string{own}, slices.DeleteFunc(slices.Clone(projects), func(name string) bool { return name == own })...)
		}
		for _, name := range projects {
			paths, err := stateinternal.ResolveProjectPaths(baseDir, name)
			if err != nil {
				continue
			}
			if info, err := os.Stat(filepath.Join(paths.RunsDir, runID)); err == nil && info.IsDir() {
				return name, finder.site.terminalPageLink(baseDir, "/project/"+url.PathEscape(name)+"/run/"+url.PathEscape(runID))
			}
		}
	}
	return "", ""
}

// job is a job of the shell's project on the newest of its latest runs that has it, or,
// only queued, the project's page. Shells of no project, or of a basedir's own pages,
// have no project to look in.
func (finder *terminalLinkFinder) job(jobID string) (project, link string) {
	baseDir, project := terminalGroupPlace(finder.group)
	if project == "" {
		return "", ""
	}
	paths, err := stateinternal.ResolveProjectPaths(baseDir, project)
	if err != nil {
		finder.problem(err)
		return "", ""
	}
	runIDs, err := stateinternal.RunsNewestFirst(paths)
	if err != nil {
		finder.problem(err)
	}
	projectPage := "/project/" + url.PathEscape(project)
	for _, runID := range runIDs[:min(len(runIDs), terminalLinkJobRunsRead)] {
		job, found, err := resolve.JobInRun(paths, runID, jobID, false)
		if err != nil {
			finder.problem(fmt.Errorf("run %s: %w", runID, err))
			continue
		}
		if found {
			return project, finder.site.terminalPageLink(baseDir, projectPage+"/run/"+url.PathEscape(runID)+"?job_id="+url.QueryEscape(job.JobID))
		}
	}
	queue, err := stateinternal.LoadQueue(paths.QueueFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			finder.problem(fmt.Errorf("the queue: %w", err))
		}
		return "", ""
	}
	if _, found := resolve.JobInQueue(queue, jobID, false); found {
		return project, finder.site.terminalPageLink(baseDir, projectPage)
	}
	return "", ""
}

// path is what a path names, absolute or relative to the directory asked about or, with
// none, where the shell is now, with a line and column left off: a directory or a file.
// Anything else is no link.
func (finder *terminalLinkFinder) path(candidate string) (kind, path string) {
	path = terminalLinePosition.ReplaceAllString(candidate, "")
	if !filepath.IsAbs(path) {
		dir := finder.shellDir()
		if dir == "" {
			return "", ""
		}
		path = filepath.Join(dir, path)
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	switch {
	case err != nil:
		return "", ""
	case info.IsDir():
		return "dir", path
	case info.Mode().IsRegular():
		return "file", path
	}
	return "", ""
}

// terminalPosition is the line and column after a path, each 0 when there is none. One that
// is not positive, or too large for an editor's, is none; a column needs a line.
func terminalPosition(candidate string) (line, column int) {
	number := func(text string) int {
		value, err := strconv.ParseInt(text, 10, 32)
		if err != nil || value < 1 {
			return 0
		}
		return int(value)
	}
	parts := strings.Split(strings.TrimPrefix(terminalLinePosition.FindString(candidate), ":"), ":")
	if line = number(parts[0]); line > 0 && len(parts) > 1 {
		column = number(parts[1])
	}
	return line, column
}

// terminalLocalHost says whether a host a shell named in OSC 7 is this one: the same name
// ignoring case, the same up to the first dot (a short name and a full one), empty, or
// localhost. A host name that cannot be read makes every named host another's.
func terminalLocalHost(host string) (bool, error) {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true, nil
	}
	own, err := os.Hostname()
	if err != nil {
		return false, fmt.Errorf("the server's host name could not be read, so paths from %s are no links: %w", host, err)
	}
	short := func(name string) string {
		name, _, _ = strings.Cut(name, ".")
		return name
	}
	return strings.EqualFold(host, own) || strings.EqualFold(short(host), short(own)), nil
}

func (finder *terminalLinkFinder) shellDir() string {
	if !finder.dirRead {
		finder.dirRead = true
		dir, err := finder.site.Terminal.Dir(finder.shell)
		if err != nil {
			finder.dirError = err.Error()
		}
		finder.dir = dir
	}
	return finder.dir
}

func (finder *terminalLinkFinder) projectsOf(baseDir string) []string {
	projects, ok := finder.projects[baseDir]
	if !ok {
		var err error
		projects, err = stateinternal.ProjectNamesOrNone(baseDir)
		if err != nil {
			finder.problem(fmt.Errorf("the projects of %s: %w", baseDir, err))
		}
		finder.projects[baseDir] = projects
	}
	return projects
}

func (finder *terminalLinkFinder) problem(err error) {
	if message := err.Error(); !slices.Contains(finder.problems, message) {
		finder.problems = append(finder.problems, message)
	}
}

// terminalPageLink is a page of a basedir: as it is for the server's own, and under
// /_basedir/ID for another.
func (s site) terminalPageLink(baseDir, page string) string {
	prefix, _ := s.basedirLinkPrefix(baseDir)
	return prefix + page
}

// basedirLinkPrefix is what a basedir's pages start with: nothing for the server's own,
// and /_basedir/ID for another. ok is false for a basedir the server does not show.
func (s site) basedirLinkPrefix(baseDir string) (prefix string, ok bool) {
	for _, entry := range s.basedirEntries() {
		if entry.Path == baseDir {
			if entry.Current {
				return "", true
			}
			return basedirRoutePrefix + entry.ID, true
		}
	}
	return "", false
}
