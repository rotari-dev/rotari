package webui

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"

	"github.com/rotari-dev/rotari/internal/terminal"
)

// terminalPrefix is where the terminal is served. It sits outside every basedir
// mount: the shell belongs to the server, not to a basedir.
const terminalPrefix = "/_terminal/"

const terminalSocketPath = terminalPrefix + "ws"

//go:embed assets/vendor/xterm.js
var xtermJS string

//go:embed assets/vendor/xterm-addon-fit.js
var xtermFitJS string

//go:embed assets/vendor/xterm.css
var xtermCSS string

//go:embed assets/vendor/xterm-license.txt
var xtermLicense string

// terminalAssets are the files the panel loads when it first opens, so that pages
// stay as light as they are for those who never open it.
var terminalAssets = map[string]struct{ contentType, content string }{
	terminalPrefix + "xterm.js":           {"text/javascript; charset=utf-8", xtermJS},
	terminalPrefix + "xterm-addon-fit.js": {"text/javascript; charset=utf-8", xtermFitJS},
	terminalPrefix + "xterm.css":          {"text/css; charset=utf-8", xtermCSS},
	terminalPrefix + "xterm-license.txt":  {"text/plain; charset=utf-8", xtermLicense},
}

// Close codes the page tells apart. Private codes start at 4000.
const (
	terminalCloseFailed    websocket.StatusCode = 4000 // The shell did not start; the reason says why.
	terminalCloseTakenOver websocket.StatusCode = 4001 // Another connection took the shell.
	terminalCloseExited    websocket.StatusCode = 4002 // The shell ended; the reason says how.
	terminalCloseTooSlow   websocket.StatusCode = 4003 // Attach again to catch up.
	terminalCloseStopping  websocket.StatusCode = 4004 // The server is stopping.
)

const (
	// terminalInputLimit is the largest message accepted, which limits one paste.
	terminalInputLimit = 1 << 20
	terminalWriteLimit = time.Minute
	// terminalPingInterval keeps proxies from dropping an idle connection, and
	// finds one that is gone.
	terminalPingInterval = 30 * time.Second
	// closeReasonLimit is the most a WebSocket close reason can hold.
	closeReasonLimit = 123
)

// terminalControl is a text message from the page; binary messages are input.
type terminalControl struct {
	// Type is "resize", with Rows and Cols, or "end", which ends the shell.
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
	if request.URL.Path != terminalSocketPath {
		http.NotFound(writer, request)
		return
	}
	// A shell is worth more than the rest of the API, which also takes a token over
	// plain HTTP on a trusted network.
	if !allowsBrowserSession(request) {
		http.Error(writer, "the terminal requires HTTPS or a direct connection to localhost", http.StatusForbidden)
		return
	}
	rows, rowsErr := strconv.ParseUint(request.URL.Query().Get("rows"), 10, 16)
	cols, colsErr := strconv.ParseUint(request.URL.Query().Get("cols"), 10, 16)
	if rowsErr != nil || colsErr != nil || rows == 0 || cols == 0 {
		http.Error(writer, "rows and cols must be numbers above zero", http.StatusBadRequest)
		return
	}
	connection, err := websocket.Accept(writer, request, nil)
	if err != nil {
		return // Accept has written the response.
	}
	connection.SetReadLimit(terminalInputLimit)
	attachment, err := s.Terminal.Attach(uint16(rows), uint16(cols))
	if err != nil {
		_ = connection.Close(terminalCloseFailed, closeReason(err.Error()))
		return
	}
	serveTerminalConnection(connection, s.Terminal, attachment)
}

// serveTerminalConnection joins a connection to the shell until either ends. The
// shell goes on after the connection, unless the page asked to end it.
func serveTerminalConnection(connection *websocket.Conn, keeper *terminal.Keeper, attachment *terminal.Attachment) {
	// The request's context ends with the handler's hijack, so this has its own.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer attachment.Detach()
	go func() {
		defer attachment.Detach()
		readTerminalInput(ctx, connection, keeper, attachment)
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

func readTerminalInput(ctx context.Context, connection *websocket.Conn, keeper *terminal.Keeper, attachment *terminal.Attachment) {
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
		case "end":
			keeper.EndShell()
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
