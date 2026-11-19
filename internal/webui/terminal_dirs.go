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
	"time"
	"unicode/utf8"

	stateinternal "github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/terminal"
)

// The panel's Recent directories dropdown asks terminalDirsName under a shell's path for
// the directories of the shell's group: where its shells have been, which the Keeper
// remembers, and where a new shell of the group would start, which is where the
// project's recent runs started, or a basedir's workspace.

// terminalDirsName follows a shell's path for the directories of its group.
const terminalDirsName = "dirs"

const (
	// terminalDirRunsRead is how many of a project's newest runs are read for where they
	// started, so that a project with thousands of runs costs as many context reads.
	terminalDirRunsRead = 20
	// terminalDirRunsShown is how many distinct start directories are offered.
	terminalDirRunsShown = 5
)

// terminalDirFacts is a directory as the page needs it. A name that is not valid UTF-8
// cannot go into JSON as it is: Dir then holds it with U+FFFD in place of the bad bytes,
// and NotUTF8 says that the page cannot name it. Missing says it is not a directory on
// this host now; StatError says why it could not be told.
type terminalDirFacts struct {
	Dir       string `json:"dir"`
	NotUTF8   bool   `json:"not_utf8,omitempty"`
	Missing   bool   `json:"missing,omitempty"`
	StatError string `json:"stat_error,omitempty"`
}

func newTerminalDirFacts(dir string) terminalDirFacts {
	facts := terminalDirFacts{Dir: strings.ToValidUTF8(dir, "�"), NotUTF8: !utf8.ValidString(dir)}
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		facts.Missing = true
	case err != nil:
		facts.StatError = err.Error()
	case !info.IsDir():
		facts.Missing = true
	}
	return facts
}

// terminalHistoryEntry is a directory the group's shells have been in.
type terminalHistoryEntry struct {
	terminalDirFacts
	EnteredAt time.Time `json:"entered_at"`
}

// terminalRunDirEntry is where a run started. StartedAt is absent for a run whose ID
// records no start. Error says why the run's directory could not be read, and Dir is
// then empty.
type terminalRunDirEntry struct {
	terminalDirFacts
	Run       string     `json:"run"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// terminalWorkspaceEntry is the workspace a basedir belongs to, where its own shells
// start. Error says why that could not be found, and Dir is then empty.
type terminalWorkspaceEntry struct {
	terminalDirFacts
	Error string `json:"error,omitempty"`
}

// terminalDirsResponse is what the dropdown offers. Runs is for a project's shells, and
// RunsError says why its runs could not be listed; Workspace is for a basedir's own
// shells that belong to a workspace. Shells of no project get neither.
type terminalDirsResponse struct {
	History   []terminalHistoryEntry  `json:"history"`
	Runs      []terminalRunDirEntry   `json:"runs"`
	RunsError string                  `json:"runs_error,omitempty"`
	Workspace *terminalWorkspaceEntry `json:"workspace,omitempty"`
	// Basedir is the ID of the state directory this answer is for, and Basedirs are those
	// the server knows, to choose another from; Project is the project of it, empty for the
	// basedir's own shells, and Projects are those it has. All are empty for shells of no
	// project.
	Basedir  string       `json:"basedir"`
	Basedirs []webBaseDir `json:"basedirs"`
	Project  string       `json:"project"`
	Projects []string     `json:"projects"`
}

// terminalGroupChoice is the group a request for a shell's dirs or commands reads, and
// what the page needs to offer the others.
type terminalGroupChoice struct {
	Group    string
	Basedir  string
	Basedirs []webBaseDir
	Project  string
	Projects []string
}

// terminalChosenGroup resolves the group a request asks for: the shell's own, or with a
// basedir parameter (an ID the server knows) that basedir's own shells, or with a project
// parameter, empty for a basedir's own shells, that project of the chosen basedir. A
// basedir that is not the shell's own starts at its own shells when no project is named.
// It writes the error itself and reports false when the shell is gone, the shell is of no
// project and so has no basedir to choose among, or the basedir or project is not known.
func (s site) terminalChosenGroup(writer http.ResponseWriter, request *http.Request, id string) (terminalGroupChoice, bool) {
	own, ok := s.terminalShellGroup(id)
	if !ok {
		http.Error(writer, terminal.ErrNoShell.Error(), http.StatusNotFound)
		return terminalGroupChoice{}, false
	}
	query := request.URL.Query()
	ownBaseDir, ownProject := terminalGroupPlace(own)
	choice := terminalGroupChoice{Group: own, Basedirs: []webBaseDir{}, Projects: []string{}}
	if ownBaseDir == "" {
		if query.Has("basedir") || query.Has("project") {
			http.Error(writer, "a shell of no project has no other basedirs or projects", http.StatusBadRequest)
			return terminalGroupChoice{}, false
		}
		return choice, true
	}
	baseDir, project := ownBaseDir, ownProject
	if query.Has("basedir") {
		path, ok := s.basedirPath(query.Get("basedir"))
		if !ok {
			http.Error(writer, "no such basedir", http.StatusNotFound)
			return terminalGroupChoice{}, false
		}
		if path = filepath.Clean(path); path != ownBaseDir {
			baseDir, project = path, ""
		}
	}
	if query.Has("project") {
		project = query.Get("project")
	}
	names, err := stateinternal.ProjectNamesOrNone(baseDir)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return terminalGroupChoice{}, false
	}
	if project != "" && (!stateinternal.IsValidPathElement(project) || !slices.Contains(names, project)) {
		http.Error(writer, fmt.Sprintf("project %q was not found", project), http.StatusNotFound)
		return terminalGroupChoice{}, false
	}
	choice.Group = baseDir + "\x00" + project
	choice.Basedir = basedirID(baseDir)
	choice.Project = project
	choice.Projects = append(choice.Projects, names...)
	choice.Basedirs = s.basedirEntries()
	if !slices.ContainsFunc(choice.Basedirs, func(entry webBaseDir) bool { return entry.ID == choice.Basedir }) {
		choice.Basedirs = append(choice.Basedirs, newWebBaseDir(choice.Basedir, baseDir, false))
	}
	return choice, true
}

// serveTerminalDirs answers GET with the directories of a shell's group. It attaches to
// nothing, so it takes the shell from no page.
func (s site) serveTerminalDirs(writer http.ResponseWriter, request *http.Request, id string) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	choice, ok := s.terminalChosenGroup(writer, request, id)
	if !ok {
		return
	}
	group := choice.Group
	response := terminalDirsResponse{History: []terminalHistoryEntry{}, Runs: []terminalRunDirEntry{}, Basedir: choice.Basedir, Basedirs: choice.Basedirs, Project: choice.Project, Projects: choice.Projects}
	for _, entry := range s.Terminal.History(group) {
		response.History = append(response.History, terminalHistoryEntry{terminalDirFacts: newTerminalDirFacts(entry.Dir), EnteredAt: entry.EnteredAt})
	}
	baseDir, project := terminalGroupPlace(group)
	switch {
	case project != "":
		runs, err := s.terminalRecentRunDirs(baseDir, project)
		if err != nil {
			response.RunsError = err.Error()
		} else {
			response.Runs = runs
		}
	case baseDir != "":
		dir, err := terminalBaseDirStart(baseDir)
		switch {
		case err != nil:
			response.Workspace = &terminalWorkspaceEntry{Error: err.Error()}
		case dir != "":
			response.Workspace = &terminalWorkspaceEntry{terminalDirFacts: newTerminalDirFacts(dir)}
		}
	}
	writeTerminalJSON(writer, http.StatusOK, response)
}

// terminalShellGroup is the group of a running shell.
func (s site) terminalShellGroup(id string) (string, bool) {
	for _, shell := range s.Terminal.Shells() {
		if shell.ID == id {
			return shell.Group, true
		}
	}
	return "", false
}

// terminalRecentRunDirs is where the project's newest runs started, each directory once,
// newest first. A run whose directory cannot be read is listed with the reason.
func (s site) terminalRecentRunDirs(baseDir, project string) ([]terminalRunDirEntry, error) {
	paths, err := stateinternal.ResolveProjectPaths(baseDir, project)
	if err != nil {
		return nil, fmt.Errorf("project %q was not found", project)
	}
	runIDs, err := stateinternal.RunsNewestFirst(paths)
	if err != nil {
		return nil, err
	}
	entries := []terminalRunDirEntry{}
	seen := make(map[string]bool)
	for _, runID := range runIDs[:min(len(runIDs), terminalDirRunsRead)] {
		if len(entries) == terminalDirRunsShown {
			break
		}
		entry := terminalRunDirEntry{Run: runID}
		if start, ok := stateinternal.RunIDStart(runID); ok {
			entry.StartedAt = &start
		}
		context, err := stateinternal.LoadContext(s.Store, filepath.Join(paths.RunsDir, runID))
		switch {
		case err != nil:
			entry.Error = err.Error()
		case context.CWD == "":
			entry.Error = fmt.Sprintf("run %q does not say where it started", runID)
		case seen[context.CWD]:
			continue
		default:
			seen[context.CWD] = true
			entry.terminalDirFacts = newTerminalDirFacts(context.CWD)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
