package webui

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/rotari-dev/rotari/internal/tableview"
	"github.com/rotari-dev/rotari/internal/terminal"
)

// A shell's Table viewer shows the directory a terminal shell was in when the
// viewer was asked for. The page never names a path: POST
// /api/tableview-sources names a running shell, the server looks up its
// directory as a launch on a shell does, and keeps it under a random source
// id that /tableview/?source=ID names from then on. The directory is pinned
// there, so the shell changing directory or ending does not move the view.
// Sources last in this server's memory only, the most recently used
// maxShellTableViews of them.

const maxShellTableViews = 64

// errShellTableViewGone is a source id this server does not hold: never
// issued, dropped as least recently used, or issued before a restart.
var errShellTableViewGone = errors.New("this Table viewer source is no longer held by the server (it restarted, or the source was dropped as unused); open the Table viewer again from the terminal's Launch here")

// shellTableViews holds the sources issued for shells' directories.
type shellTableViews struct {
	mu      sync.Mutex
	sources map[string]tableViewSource
	// order lists ids from least to most recently used.
	order []string
}

func newShellTableViews() *shellTableViews {
	return &shellTableViews{sources: map[string]tableViewSource{}}
}

func (views *shellTableViews) add(source tableViewSource) string {
	id := rand.Text()
	views.mu.Lock()
	defer views.mu.Unlock()
	views.sources[id] = source
	views.order = append(views.order, id)
	for len(views.order) > maxShellTableViews {
		delete(views.sources, views.order[0])
		views.order = views.order[1:]
	}
	return id
}

func (views *shellTableViews) get(id string) (tableViewSource, error) {
	views.mu.Lock()
	defer views.mu.Unlock()
	source, ok := views.sources[id]
	if !ok {
		return tableViewSource{}, errShellTableViewGone
	}
	for i, held := range views.order {
		if held == id {
			views.order = append(append(views.order[:i:i], views.order[i+1:]...), id)
			break
		}
	}
	return source, nil
}

// shellDirectory is the directory a running shell is in now, looked up on the
// server, with the shell. It needs the terminal on.
func (s site) shellDirectory(id string) (string, terminal.ShellInfo, error) {
	if s.Terminal == nil {
		return "", terminal.ShellInfo{}, errors.New("a shell's directory needs the terminal, which this server does not offer; restart with `rotari web --allow-terminal`")
	}
	var shell *terminal.ShellInfo
	for _, item := range s.Terminal.Shells() {
		if item.ID == id {
			shell = &item
			break
		}
	}
	if shell == nil {
		return "", terminal.ShellInfo{}, fmt.Errorf("shell %s is not running", id)
	}
	dir, err := s.Terminal.Dir(id)
	if err != nil {
		return "", *shell, fmt.Errorf("where shell %d is now is not known: %w", shell.Number, err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", *shell, fmt.Errorf("shell %d is in %s, which is not a directory on this host", shell.Number, dir)
	}
	return dir, *shell, nil
}

// shellTableViewRequest names a running shell, or for the terminal's Explorer an
// absolute path, and nothing else.
type shellTableViewRequest struct {
	Shell string `json:"shell"`
	Path  string `json:"path"`
}

// shellTableViewSource is what POST /api/tableview-sources answers: the
// source id and the directory pinned under it, and for a file the Explorer
// named, the file's name in it to open as child.
type shellTableViewSource struct {
	Source string `json:"source"`
	Dir    string `json:"dir"`
	Child  string `json:"child,omitempty"`
}

func (s site) registerShellTableViewRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/tableview-sources", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			methodNotAllowed(writer)
			return
		}
		var body shellTableViewRequest
		decoder := json.NewDecoder(http.MaxBytesReader(nil, request.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeLaunchError(writer, http.StatusBadRequest, webLaunchError{Error: "invalid request: it names only a running shell: " + err.Error()})
			return
		}
		if body.Path != "" {
			s.serveExplorerTableView(writer, request, body)
			return
		}
		if body.Shell == "" {
			writeLaunchError(writer, http.StatusBadRequest, webLaunchError{Error: "shell is required"})
			return
		}
		dir, shell, err := s.shellDirectory(body.Shell)
		if err != nil {
			status := http.StatusBadRequest
			if s.Terminal == nil {
				status = http.StatusForbidden
			}
			writeLaunchError(writer, status, webLaunchError{Error: err.Error()})
			return
		}
		source := tableViewSource{target: tableview.Target{Path: dir, Roots: []string{dir}}, directory: true, name: dir}
		if _, project := terminalGroupPlace(shell.Group); project != "" {
			source.trail = append(source.trail, project)
		}
		source.trail = append(source.trail, "shell "+strconv.Itoa(shell.Number))
		writeWebJSON(writer, shellTableViewSource{Source: s.shellTableViews.add(source), Dir: dir})
	})
}

// serveExplorerTableView pins what the terminal's Explorer names: a directory, or a
// file's directory with the file as its child. Whoever can use the terminal can read
// anything there, so it needs the terminal on and the connection the terminal itself
// needs. Symbolic links are followed first, as a shell would, since the viewer opens
// files through os.Root, which refuses a link to an absolute path; the viewer then reads
// under /, so that the media a file names are found wherever they are.
func (s site) serveExplorerTableView(writer http.ResponseWriter, request *http.Request, body shellTableViewRequest) {
	if body.Shell != "" {
		writeLaunchError(writer, http.StatusBadRequest, webLaunchError{Error: "name either a running shell or a path, not both"})
		return
	}
	if s.Terminal == nil {
		writeLaunchError(writer, http.StatusForbidden, webLaunchError{Error: "a path from the Explorer needs the terminal, which this server does not offer; restart with `rotari web --allow-terminal`"})
		return
	}
	if !allowsBrowserSession(request) {
		writeLaunchError(writer, http.StatusForbidden, webLaunchError{Error: "a path from the Explorer requires HTTPS or a direct connection to localhost, as the terminal does"})
		return
	}
	if err := checkTerminalDir(body.Path); err != nil {
		writeLaunchError(writer, http.StatusBadRequest, webLaunchError{Error: err.Error()})
		return
	}
	path, err := filepath.EvalSymlinks(filepath.Clean(body.Path))
	if err != nil {
		writeLaunchError(writer, http.StatusBadRequest, webLaunchError{Error: err.Error()})
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeLaunchError(writer, http.StatusBadRequest, webLaunchError{Error: err.Error()})
		return
	}
	dir, child := path, ""
	if !info.IsDir() {
		dir, child = filepath.Dir(path), filepath.Base(path)
	}
	source := tableViewSource{target: tableview.Target{Path: dir, Roots: []string{string(filepath.Separator)}}, directory: true, name: dir, trail: []string{"Explorer"}}
	writeWebJSON(writer, shellTableViewSource{Source: s.shellTableViews.add(source), Dir: dir, Child: child})
}

// tableViewSourceOf resolves a /tableview/ query: a shell's pinned source by
// source, with only child beside it, or else an artifact.
func (s site) tableViewSourceOf(baseDir string, query url.Values) (tableViewSource, error) {
	id := query.Get("source")
	if id == "" {
		if query.Has("source") {
			return tableViewSource{}, errors.New("source is empty")
		}
		return s.artifactTableView(baseDir, query)
	}
	for _, key := range []string{"project_name", "run_id", "job_id", "attempt_id", "entry"} {
		if query.Has(key) {
			return tableViewSource{}, fmt.Errorf("a shell's Table viewer source names its directory, so it takes no %s", key)
		}
	}
	if s.shellTableViews == nil {
		return tableViewSource{}, errShellTableViewGone
	}
	source, err := s.shellTableViews.get(id)
	if err != nil {
		return tableViewSource{}, err
	}
	child, err := source.child(query.Get("child"))
	if err != nil {
		return tableViewSource{}, err
	}
	child.trail = source.trail
	return child, nil
}
