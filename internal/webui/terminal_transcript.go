package webui

import (
	_ "embed"
	"net/http"
	"strings"
)

// terminalTranscriptPath is a page of one pane's commands, which takes them from the
// tab that opened it (WEB-60).
const terminalTranscriptPath = terminalPrefix + "transcript"

//go:embed assets/terminal_transcript.html
var terminalTranscriptHTML string

//go:embed assets/web_transcript.js
var webTranscriptJS string

//go:embed assets/web_transcript.css
var webTranscriptCSS string

// composeTerminalTranscript is the transcript page: its own script, under the page's nonce,
// and nothing of the application; the transcript itself comes from the tab that opened it.
func composeTerminalTranscript(nonce string) string {
	page := strings.NewReplacer("__ROTARI_TRANSCRIPT_SCRIPT__", webTranscriptJS).Replace(terminalTranscriptHTML)
	return strings.NewReplacer(
		scriptNonceSlot, scriptNonceAttribute(nonce),
		"__ROTARI_ICON_DARK__", faviconDataURL(webFaviconDarkSVG),
		"__ROTARI_ICON_LIGHT__", faviconDataURL(webFaviconLightSVG),
	).Replace(page)
}

func serveTerminalTranscript(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	nonce := setPagePolicy(writer)
	writer.Header().Set(headerContentType, "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(composeTerminalTranscript(nonce)))
}
