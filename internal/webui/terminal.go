package webui

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coder/websocket"

	"github.com/rotari-dev/rotari/internal/resolve"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/terminal"
)

// terminalPrefix is where the terminal is served. It sits outside every basedir
// mount: the shell belongs to the server, not to a basedir.
const terminalPrefix = "/_terminal/"

const (
	terminalSocketPath = terminalPrefix + "ws"
	terminalShellsPath = terminalPrefix + "shells"
	// terminalPreviewName follows a shell's path for what the overview shows of it.
	terminalPreviewName = "preview"
	terminalHerePath    = terminalPrefix + "here"
	terminalAlivePath   = terminalPrefix + "alive"
	terminalOrderPath   = terminalPrefix + "order"
	// terminalWindowPath is the panel in a window of its own. What follows it is the path
	// of the page whose project it shows, such as /project/p.
	terminalWindowPath = terminalPrefix + "window"
)

//go:embed assets/vendor/xterm.js
var xtermJS string

//go:embed assets/vendor/xterm-addon-fit.js
var xtermFitJS string

//go:embed assets/vendor/xterm-addon-web-links.js
var xtermWebLinksJS string

//go:embed assets/vendor/xterm.css
var xtermCSS string

//go:embed assets/vendor/xterm-license.txt
var xtermLicense string

// terminalAssets are the files the panel loads when it first opens, so that pages
// stay as light as they are for those who never open it.
var terminalAssets = map[string]struct{ contentType, content string }{
	terminalPrefix + "xterm.js":                 {"text/javascript; charset=utf-8", xtermJS},
	terminalPrefix + "xterm-addon-fit.js":       {"text/javascript; charset=utf-8", xtermFitJS},
	terminalPrefix + "xterm-addon-web-links.js": {"text/javascript; charset=utf-8", xtermWebLinksJS},
	terminalPrefix + "xterm.css":                {"text/css; charset=utf-8", xtermCSS},
	terminalPrefix + "xterm-license.txt":        {"text/plain; charset=utf-8", xtermLicense},
}

// Close codes the page tells apart. Private codes start at 4000.
const (
	terminalCloseFailed    websocket.StatusCode = 4000 // The shell did not start; the reason says why.
	terminalCloseTakenOver websocket.StatusCode = 4001 // Another connection took the shell.
	terminalCloseExited    websocket.StatusCode = 4002 // The shell ended; the reason says how.
	terminalCloseTooSlow   websocket.StatusCode = 4003 // Attach again to catch up.
	terminalCloseStopping  websocket.StatusCode = 4004 // The server is stopping.
	terminalCloseNoShell   websocket.StatusCode = 4005 // The shell asked for is not running.
)

const (
	// terminalInputLimit is the largest message accepted, which limits one paste.
	terminalInputLimit = 1 << 20
	terminalWriteLimit = time.Minute
	// terminalPingInterval keeps proxies from dropping an idle connection, and
	// finds one that is gone.
	terminalPingInterval = 30 * time.Second
	// terminalNameLimit is the longest name a tab can have, and terminalNameBodyLimit
	// holds it in any encoding, as well as every shell's ID.
	terminalNameLimit     = 64
	terminalNameBodyLimit = 4096
	// closeReasonLimit is the most a WebSocket close reason can hold.
	closeReasonLimit = 123
)

// terminalControl is a text message from the page; binary messages are input.
type terminalControl struct {
	// Type is "resize", with Rows and Cols, or "clear", which forgets the output kept
	// to replay.
	Type string `json:"type"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

func (s site) serveTerminal(writer http.ResponseWriter, request *http.Request) {
	if s.Terminal == nil {
		http.NotFound(writer, request)
		return
	}
	if asset, ok := terminalAssets[request.URL.Path]; ok {
		writer.Header().Set(headerContentType, asset.contentType)
		_, _ = writer.Write([]byte(asset.content))
		return
	}
	// A shell is worth more than the rest of the API, which also takes a token over
	// plain HTTP on a trusted network.
	if !allowsBrowserSession(request) {
		http.Error(writer, "the terminal requires HTTPS or a direct connection to localhost", http.StatusForbidden)
		return
	}
	switch path := request.URL.Path; {
	case path == terminalSocketPath:
		s.serveTerminalSocket(writer, request)
	case path == terminalShellsPath:
		s.serveTerminalShells(writer, request)
	case path == terminalHerePath:
		s.serveTerminalHere(writer, request)
	case path == terminalAlivePath:
		s.serveTerminalAlive(writer, request)
	case path == terminalOrderPath:
		s.serveTerminalOrder(writer, request)
	case path == terminalWindowPath || strings.HasPrefix(path, terminalWindowPath+"/"):
		serveTerminalWindow(writer, request)
	case strings.HasPrefix(path, terminalShellsPath+"/"):
		s.serveTerminalShell(writer, request, strings.TrimPrefix(path, terminalShellsPath+"/"))
	default:
		http.NotFound(writer, request)
	}
}

// terminalSize reads a request's rows and cols, answering the request if they are bad.
func terminalSize(writer http.ResponseWriter, request *http.Request) (rows, cols uint16, ok bool) {
	parsedRows, rowsErr := strconv.ParseUint(request.URL.Query().Get("rows"), 10, 16)
	parsedCols, colsErr := strconv.ParseUint(request.URL.Query().Get("cols"), 10, 16)
	if rowsErr != nil || colsErr != nil || parsedRows == 0 || parsedCols == 0 {
		http.Error(writer, "rows and cols must be numbers above zero", http.StatusBadRequest)
		return 0, 0, false
	}
	return uint16(parsedRows), uint16(parsedCols), true
}

// terminalPlace is where a page's new shell belongs and starts.
type terminalPlace struct {
	// BaseDir and Project are empty for a page of no project.
	BaseDir string `json:"basedir"`
	Project string `json:"project"`
	Dir     string `json:"dir"`
	// note says why the shell could not start where the page would have it.
	note string
}

// terminalPlaceFor reads a page's place from its basedir ID, project, and run, looking
// the directory up here rather than taking one from the page. An empty run is the
// project's latest. It answers the request if the place is bad.
func (s site) terminalPlaceFor(writer http.ResponseWriter, request *http.Request) (terminalPlace, bool) {
	query := request.URL.Query()
	id, project, run := query.Get("basedir"), query.Get("project"), query.Get("run")
	place := terminalPlace{Dir: s.Terminal.DefaultDir()}
	if project == "" {
		if run != "" {
			http.Error(writer, "a run needs its project", http.StatusBadRequest)
			return place, false
		}
		return place, true
	}
	if !stateinternal.IsValidPathElement(project) || (run != "" && !stateinternal.IsValidPathElement(run)) {
		http.Error(writer, "invalid project or run", http.StatusBadRequest)
		return place, false
	}
	place.BaseDir = s.BaseDir
	if id != "" {
		baseDir, ok := s.basedirPath(id)
		if !ok {
			http.Error(writer, "no such basedir", http.StatusNotFound)
			return place, false
		}
		place.BaseDir = baseDir
	}
	if absolute, err := filepath.Abs(place.BaseDir); err == nil {
		place.BaseDir = filepath.Clean(absolute)
	}
	place.Project = project
	dir, err := s.terminalRunDir(place.BaseDir, project, run)
	if err != nil {
		place.note = err.Error()
		return place, true
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		place.note = fmt.Sprintf("%q does not exist on this host", dir)
		return place, true
	}
	place.Dir = dir
	return place, true
}

// terminalRunDir is where a run was started.
func (s site) terminalRunDir(baseDir, project, run string) (string, error) {
	paths, err := stateinternal.ResolveProjectPaths(baseDir, project)
	if err != nil {
		return "", fmt.Errorf("project %q was not found", project)
	}
	resolved, err := resolve.RunID(paths, run)
	if err != nil {
		if run == "" {
			return "", fmt.Errorf("project %q has no run to start from", project)
		}
		return "", fmt.Errorf("run %q was not found", run)
	}
	context, err := stateinternal.LoadContext(s.Store, filepath.Join(paths.RunsDir, resolved))
	if err != nil || context.CWD == "" {
		return "", fmt.Errorf("run %q does not say where it started", resolved)
	}
	return context.CWD, nil
}

// terminalGroup keys the shells of one project of one basedir. A path holds no NUL.
func terminalGroup(place terminalPlace) string {
	if place.Project == "" {
		return ""
	}
	return place.BaseDir + "\x00" + place.Project
}

// terminalShellResponse is a shell as a page sees it.
type terminalShellResponse struct {
	ID      string `json:"id"`
	Number  int    `json:"number"`
	Name    string `json:"name"`
	BaseDir string `json:"basedir"`
	Project string `json:"project"`
	Dir     string `json:"dir"`
	// UnseenOutput is what a tab marks.
	UnseenOutput bool `json:"unseen_output"`
	// CommandFinished is a command ended out of sight, which a tab and a card mark with a bell.
	CommandFinished bool `json:"command_finished"`
	// Link is the page that shows this shell's tabs.
	Link string `json:"link"`
}

func (s site) terminalShellResponse(shell terminal.ShellInfo) terminalShellResponse {
	response := terminalShellResponse{ID: shell.ID, Number: shell.Number, Name: shell.Name, Dir: shell.Dir, UnseenOutput: shell.UnseenOutput, CommandFinished: shell.CommandFinished, Link: "/"}
	baseDir, project, ok := strings.Cut(shell.Group, "\x00")
	if !ok {
		return response
	}
	response.BaseDir, response.Project = baseDir, project
	for _, entry := range s.basedirEntries() {
		if entry.Path == baseDir {
			prefix := ""
			if !entry.Current {
				prefix = basedirRoutePrefix + entry.ID
			}
			response.Link = prefix + "/project/" + url.PathEscape(project)
		}
	}
	return response
}

// serveTerminalOrder arranges one project's tabs on PUT.
func (s site) serveTerminalOrder(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		writer.Header().Set("Allow", http.MethodPut)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		IDs []string `json:"ids"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, terminalNameBodyLimit)
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		http.Error(writer, "the body must be JSON with ids", http.StatusBadRequest)
		return
	}
	s.Terminal.Reorder(body.IDs)
	writer.WriteHeader(http.StatusNoContent)
}

// serveTerminalAlive hears that a page is still open, which keeps the shells.
func (s site) serveTerminalAlive(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.Terminal.KeepAlive()
	writer.WriteHeader(http.StatusNoContent)
}

func (s site) serveTerminalHere(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if place, ok := s.terminalPlaceFor(writer, request); ok {
		writeTerminalJSON(writer, http.StatusOK, place)
	}
}

// serveTerminalShells lists the shells on GET, and starts one on POST for a page's place.
func (s site) serveTerminalShells(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	shellLimit, groupLimit := s.Terminal.Limits()
	switch request.Method {
	case http.MethodGet:
		shells := []terminalShellResponse{}
		for _, shell := range s.Terminal.Shells() {
			shells = append(shells, s.terminalShellResponse(shell))
		}
		writeTerminalJSON(writer, http.StatusOK, map[string]any{"shells": shells, "limit": shellLimit, "group_limit": groupLimit})
	case http.MethodPost:
		rows, cols, ok := terminalSize(writer, request)
		if !ok {
			return
		}
		place, ok := s.terminalPlaceFor(writer, request)
		if !ok {
			return
		}
		start := terminal.StartOptions{Rows: rows, Cols: cols, Group: terminalGroup(place), Dir: place.Dir}
		if place.Project != "" {
			start.Env = []string{stateinternal.BaseDirEnv + "=" + place.BaseDir, stateinternal.ProjectNameEnv + "=" + place.Project}
		}
		if place.note != "" {
			start.Banner = "rotari: " + place.note + ", so this shell opened in your home directory.\r\n"
		}
		shell, err := s.Terminal.Start(start)
		switch {
		case errors.Is(err, terminal.ErrTooManyShells):
			http.Error(writer, "at most "+strconv.Itoa(shellLimit)+" shells can run at once", http.StatusConflict)
		case errors.Is(err, terminal.ErrTooManyInGroup):
			http.Error(writer, "at most "+strconv.Itoa(groupLimit)+" shells can run for one project", http.StatusConflict)
		case errors.Is(err, terminal.ErrKeeperClosed):
			http.Error(writer, "the server is stopping", http.StatusServiceUnavailable)
		case err != nil:
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		default:
			writeTerminalJSON(writer, http.StatusCreated, s.terminalShellResponse(shell))
		}
	default:
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeTerminalJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set(headerContentType, "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

// serveTerminalShell ends a shell on DELETE, whose connection closes as for an exit, and
// names it on PATCH.
func (s site) serveTerminalShell(writer http.ResponseWriter, request *http.Request, id string) {
	if id, ok := strings.CutSuffix(id, "/"+terminalPreviewName); ok {
		s.serveTerminalPreview(writer, request, id)
		return
	}
	switch request.Method {
	case http.MethodDelete:
		if err := s.Terminal.End(id); errors.Is(err, terminal.ErrNoShell) {
			http.Error(writer, err.Error(), http.StatusNotFound)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	case http.MethodPatch:
		s.renameTerminalShell(writer, request, id)
	default:
		writer.Header().Set("Allow", "DELETE, PATCH")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// terminalPreviewResponse is what the overview shows of a shell beyond the list: where
// it is now, and what it is running. Cwd is empty when it could not be found, and
// CwdError says why. Command is empty while the shell is at its prompt, and when it could
// not be found, which CommandError then says.
type terminalPreviewResponse struct {
	Cwd          string `json:"cwd,omitempty"`
	CwdError     string `json:"cwd_error,omitempty"`
	Command      string `json:"command,omitempty"`
	CommandError string `json:"command_error,omitempty"`
}

// serveTerminalPreview describes a shell on GET without attaching to it, so it takes the
// shell from no page.
func (s site) serveTerminalPreview(writer http.ResponseWriter, request *http.Request, id string) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var preview terminalPreviewResponse
	cwd, err := s.Terminal.Dir(id)
	if errors.Is(err, terminal.ErrNoShell) {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		preview.CwdError = err.Error()
	}
	preview.Cwd = cwd
	command, err := s.Terminal.Foreground(id)
	if errors.Is(err, terminal.ErrNoShell) {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		preview.CommandError = err.Error()
	}
	preview.Command = command
	writeTerminalJSON(writer, http.StatusOK, preview)
}

func (s site) renameTerminalShell(writer http.ResponseWriter, request *http.Request, id string) {
	var body struct {
		Name string `json:"name"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, terminalNameBodyLimit)
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		http.Error(writer, "the body must be JSON with a name", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if utf8.RuneCountInString(name) > terminalNameLimit || strings.ContainsFunc(name, unicode.IsControl) {
		http.Error(writer, "a name is up to "+strconv.Itoa(terminalNameLimit)+" characters on one line", http.StatusBadRequest)
		return
	}
	shell, err := s.Terminal.Rename(id, name)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	writeTerminalJSON(writer, http.StatusOK, s.terminalShellResponse(shell))
}

func (s site) serveTerminalSocket(writer http.ResponseWriter, request *http.Request) {
	id := request.URL.Query().Get("shell")
	if id == "" {
		http.Error(writer, "shell must name a shell", http.StatusBadRequest)
		return
	}
	rows, cols, ok := terminalSize(writer, request)
	if !ok {
		return
	}
	connection, err := websocket.Accept(writer, request, nil)
	if err != nil {
		return // Accept has written the response.
	}
	connection.SetReadLimit(terminalInputLimit)
	// A page reconnecting by itself must not take the shell from another tab.
	attach := s.Terminal.Attach
	if request.URL.Query().Get("takeover") == "0" {
		attach = s.Terminal.AttachIfFree
	}
	attachment, err := attach(id, rows, cols)
	switch {
	case errors.Is(err, terminal.ErrAttachedElsewhere):
		_ = connection.Close(terminalCloseTakenOver, "another connection has the terminal")
	case errors.Is(err, terminal.ErrNoShell):
		_ = connection.Close(terminalCloseNoShell, "the shell is not running")
	case err != nil:
		_ = connection.Close(terminalCloseFailed, closeReason(err.Error()))
	default:
		serveTerminalConnection(connection, attachment)
	}
}

// serveTerminalConnection joins a connection to the shell until either ends. The
// shell goes on after the connection.
func serveTerminalConnection(connection *websocket.Conn, attachment *terminal.Attachment) {
	// The request's context ends with the handler's hijack, so this has its own.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer attachment.Detach()
	go func() {
		defer attachment.Detach()
		readTerminalInput(ctx, connection, attachment)
	}()
	go func() {
		ticker := time.NewTicker(terminalPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, stop := context.WithTimeout(ctx, terminalPingInterval)
				err := connection.Ping(pingCtx)
				stop()
				if err != nil {
					attachment.Detach()
					return
				}
			}
		}
	}()
	for {
		output, ok := attachment.Next()
		if !ok {
			break
		}
		writeCtx, stop := context.WithTimeout(ctx, terminalWriteLimit)
		err := connection.Write(writeCtx, websocket.MessageBinary, output)
		stop()
		if err != nil {
			attachment.Detach()
		}
	}
	code, reason := terminalCloseStatus(attachment)
	if code == websocket.StatusNormalClosure {
		_ = connection.CloseNow()
		return
	}
	_ = connection.Close(code, reason)
}

func readTerminalInput(ctx context.Context, connection *websocket.Conn, attachment *terminal.Attachment) {
	for {
		kind, message, err := connection.Read(ctx)
		if err != nil {
			return
		}
		if kind == websocket.MessageBinary {
			if _, err := attachment.Write(message); err != nil {
				return
			}
			continue
		}
		var control terminalControl
		if err := json.Unmarshal(message, &control); err != nil {
			_ = connection.Close(websocket.StatusInvalidFramePayloadData, "a control message must be JSON")
			return
		}
		switch control.Type {
		case "resize":
			// A bad size is a bug in the page, not a reason to drop the shell.
			if err := attachment.Resize(control.Rows, control.Cols); err != nil && errors.Is(err, terminal.ErrDetached) {
				return
			}
		case "clear":
			if err := attachment.ClearOutput(); err != nil {
				return
			}
		default:
			_ = connection.Close(websocket.StatusPolicyViolation, closeReason("unknown control message "+control.Type))
			return
		}
	}
}

func terminalCloseStatus(attachment *terminal.Attachment) (websocket.StatusCode, string) {
	switch attachment.Reason() {
	case terminal.TakenOver:
		return terminalCloseTakenOver, "another connection took over the terminal"
	case terminal.ShellExited:
		return terminalCloseExited, attachment.Result().String()
	case terminal.TooSlow:
		return terminalCloseTooSlow, "the connection fell behind"
	case terminal.KeeperClosed:
		return terminalCloseStopping, "the server is stopping"
	default:
		return websocket.StatusNormalClosure, ""
	}
}

// closeReason cuts text to what a close frame holds.
func closeReason(text string) string {
	if len(text) > closeReasonLimit {
		return text[:closeReasonLimit]
	}
	return text
}
