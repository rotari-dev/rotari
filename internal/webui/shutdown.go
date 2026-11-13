package webui

import (
	"mime"
	"net/http"
)

// serverShutdownPath stops the server, as the Settings menu's Stop server does. It
// belongs to the server, not to a basedir, so it is not under a basedir's mount.
const serverShutdownPath = "/_server/shutdown"

// serveShutdown stops the server on a POST with a JSON body: it answers 202, then asks
// for the same shutdown SIGTERM starts. A JSON content type cannot come from a
// cross-site form without a preflight, which this server never grants.
func (s site) serveShutdown(writer http.ResponseWriter, request *http.Request) {
	if s.Shutdown == nil {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		methodNotAllowed(writer)
		return
	}
	if !s.AllowControl {
		forbiddenReadOnly(writer)
		return
	}
	if kind, _, err := mime.ParseMediaType(request.Header.Get("Content-Type")); err != nil || kind != "application/json" {
		http.Error(writer, "stopping the server needs a JSON request (Content-Type: application/json)", http.StatusUnsupportedMediaType)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	_, _ = writer.Write([]byte(`{"message":"rotari web is stopping"}` + "\n"))
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
	// After this handler returns, so the answer is sent before connections close.
	go s.Shutdown()
}
