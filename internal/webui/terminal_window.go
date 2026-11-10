package webui

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed assets/terminal_window.html
var terminalWindowHTML string

// composeTerminalWindow is the page of the panel in a window of its own: the page
// frame's script, which builds the panel, and nothing of the application.
func composeTerminalWindow(nonce string) string {
	page := strings.NewReplacer("__ROTARI_SHELL_SCRIPT__", webShellJS).Replace(terminalWindowHTML)
	return strings.NewReplacer(
		scriptNonceSlot, scriptNonceAttribute(nonce),
		"__ROTARI_ICON_DARK__", faviconDataURL(webFaviconDarkSVG),
		"__ROTARI_ICON_LIGHT__", faviconDataURL(webFaviconLightSVG),
	).Replace(page)
}

func serveTerminalWindow(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nonce := setPagePolicy(writer)
	writer.Header().Set(headerContentType, "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(composeTerminalWindow(nonce)))
}
