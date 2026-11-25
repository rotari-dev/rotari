package webui

import (
	_ "embed"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rotari-dev/rotari/internal/launcher"
)

//go:embed assets/web_editor.js
var webEditorJS string

// pageFrameJS is the script every page has in its head: Open in EDITOR and the page frame.
func pageFrameJS() string {
	return webEditorJS + "\n" + webShellJS
}

// editorConnection is what a live page's Open in EDITOR starts from. Where the editor runs, and
// the name its ssh knows this host by, belong to the browser's machine, so these are only
// suggestions, and the key under which the browser keeps its own answer.
type editorConnection struct {
	// Host is this host's name, or empty when it cannot be read.
	Host string
	// Remote suggests SSH: the server was started in an SSH session.
	Remote bool
}

// currentEditorConnection describes this server's host. A name that cannot be read leaves
// Host empty, which the page says.
func currentEditorConnection() *editorConnection {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	return &editorConnection{Host: host, Remote: os.Getenv("SSH_CONNECTION") != ""}
}

// attributes are the element web_editor.js reads. Its presence offers Open in EDITOR.
func (connection editorConnection) attributes() string {
	// The first label is what people usually call a host whose name is a full one.
	name, _, _ := strings.Cut(connection.Host, ".")
	return `id="editor-connection" data-host="` + html.EscapeString(connection.Host) +
		`" data-name="` + html.EscapeString(name) +
		`" data-remote="` + strconv.FormatBool(connection.Remote) + `"`
}

// editorControl is the Settings menu's choice whether to show Open in EDITOR, of the editor,
// whose list web_editor.js fills, and of how it connects.
func (page livePage) editorControl() string {
	if page.editor == nil {
		return ""
	}
	return `<div class="settings-editor" ` + page.editor.attributes() +
		`><label><input id="editor-show" type="checkbox" checked> Show Open in editor</label><label>Editor <select id="editor-choice"></select></label><button id="editor-connection-button" type="button">Editor connection...</button></div>`
}

// editorTarget is the /api/editor-target response: the path an artifact has on this host, and
// whether it is a file or a directory.
type editorTarget struct {
	Path string              `json:"path"`
	Kind launcher.TargetKind `json:"kind"`
}

// serveEditorTarget answers where an artifact is, for the browser to open. It resolves the
// artifact as its content requests do, so it names nothing they would not serve.
func (s site) serveEditorTarget(baseDir string, writer http.ResponseWriter, request *http.Request) {
	served, err := s.resolveArtifact(baseDir, request.URL.Query())
	if err != nil {
		writeArtifactError(writer, err)
		return
	}
	kind, err := artifactKind(served)
	if err != nil {
		writeArtifactError(writer, err)
		return
	}
	path := filepath.Join(served.root, served.relative)
	if !utf8.ValidString(path) {
		writeWebError(writer, fmt.Errorf("the path is not valid UTF-8, so a page cannot name it"))
		return
	}
	noStore(writer)
	writeWebJSON(writer, editorTarget{Path: path, Kind: kind})
}
