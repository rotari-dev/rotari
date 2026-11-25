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

//go:embed assets/web_vscode.js
var webVSCodeJS string

// pageFrameJS is the script every page has in its head: Open in VS Code and the page frame.
func pageFrameJS() string {
	return webVSCodeJS + "\n" + webShellJS
}

// vscodeConnection is what a live page's Open in VS Code starts from. Where VS Code runs, and
// the name its ssh knows this host by, belong to the browser's machine, so these are only
// suggestions, and the key under which the browser keeps its own answer.
type vscodeConnection struct {
	// Host is this host's name, or empty when it cannot be read.
	Host string
	// Remote suggests Remote SSH: the server was started in an SSH session.
	Remote bool
}

// currentVSCodeConnection describes this server's host. A name that cannot be read leaves
// Host empty, which the page says.
func currentVSCodeConnection() *vscodeConnection {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	return &vscodeConnection{Host: host, Remote: os.Getenv("SSH_CONNECTION") != ""}
}

// attributes are the element web_vscode.js reads. Its presence offers Open in VS Code.
func (connection vscodeConnection) attributes() string {
	// The first label is what people usually call a host whose name is a full one.
	name, _, _ := strings.Cut(connection.Host, ".")
	return `id="vscode-connection" data-host="` + html.EscapeString(connection.Host) +
		`" data-name="` + html.EscapeString(name) +
		`" data-remote="` + strconv.FormatBool(connection.Remote) + `"`
}

// vscodeControl is the Settings menu's choice whether to show Open in VS Code, and its
// connection.
func (page livePage) vscodeControl() string {
	if page.vscode == nil {
		return ""
	}
	return `<div class="settings-vscode" ` + page.vscode.attributes() +
		`><label><input id="vscode-show" type="checkbox" checked> Show Open in VS Code</label><button id="vscode-connection-button" type="button">VS Code connection...</button></div>`
}

// vscodeTarget is the /api/vscode-target response: the path an artifact has on this host, and
// whether it is a file or a directory.
type vscodeTarget struct {
	Path string              `json:"path"`
	Kind launcher.TargetKind `json:"kind"`
}

// serveVSCodeTarget answers where an artifact is, for the browser to open. It resolves the
// artifact as its content requests do, so it names nothing they would not serve.
func (s site) serveVSCodeTarget(baseDir string, writer http.ResponseWriter, request *http.Request) {
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
	writeWebJSON(writer, vscodeTarget{Path: path, Kind: kind})
}
