package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rotari-dev/rotari/internal/notification"
	"github.com/rotari-dev/rotari/internal/notificationlog"
)

// inboxLimit is how many entries the inbox shows.
const inboxLimit = 50

// inboxEntry is one recorded notification batch rendered for the inbox.
type inboxEntry struct {
	notificationlog.Cursor
	Title string `json:"title"`
	Body  string `json:"body"`
	// URL opens the entry's run when the project's browser fields select
	// "link".
	URL string `json:"url,omitempty"`
}

type inboxResponse struct {
	Entries []inboxEntry `json:"entries"`
	notificationlog.InboxState
	// Problems describes records that could not be read.
	Problems []string `json:"problems,omitempty"`
}

// handleNotifications serves the recorded notifications of the basedirs
// named by the comma-separated basedirs parameter, newest first, after the
// cleared cursor and rendered with each project's current browser settings.
func (s site) handleNotifications(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	if s.NotificationLog == nil {
		http.Error(writer, "this server keeps no notification history", http.StatusNotFound)
		return
	}
	selected := map[string]bool{}
	if value := request.URL.Query().Get("basedirs"); value != "" {
		known := map[string]bool{}
		for _, entry := range s.basedirEntries() {
			known[entry.ID] = true
		}
		for _, id := range strings.Split(value, ",") {
			if !known[id] {
				writeWebError(writer, fmt.Errorf("unknown basedir %q", id))
				return
			}
			selected[id] = true
		}
	}
	inbox, err := s.NotificationLog.State()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	records, problems, err := s.NotificationLog.Records()
	if err != nil {
		http.Error(writer, "read notification history: "+err.Error(), http.StatusInternalServerError)
		return
	}
	shown := make([]notificationlog.Record, 0, len(records))
	for _, record := range records {
		if !record.Cursor().After(inbox.Cleared) {
			break
		}
		if selected[notification.WebBaseDirID(record.BaseDir)] {
			shown = append(shown, record)
		}
	}
	response := inboxResponse{Entries: []inboxEntry{}, InboxState: inbox, Problems: problems}
	for _, rendered := range notificationlog.Entries(shown, inboxLimit) {
		entry := inboxEntry{Cursor: rendered.Cursor, Title: rendered.Title, Body: rendered.Body}
		if rendered.Link {
			entry.URL = notification.WebBaseDirRoutePrefix + notification.WebBaseDirID(rendered.BaseDir) + "/project/" + url.PathEscape(rendered.Project) + "/run/" + url.PathEscape(rendered.RunID)
		}
		response.Entries = append(response.Entries, entry)
	}
	writeWebJSON(writer, response)
}

// handleNotificationState advances the shared read and cleared cursors.
func (s site) handleNotificationState(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	if !s.AllowControl {
		forbiddenReadOnly(writer)
		return
	}
	if s.NotificationLog == nil {
		http.Error(writer, "this server keeps no notification history", http.StatusNotFound)
		return
	}
	var advance notificationlog.InboxState
	if err := json.NewDecoder(request.Body).Decode(&advance); err != nil {
		writeWebError(writer, err)
		return
	}
	if !advance.Read.Valid() || !advance.Cleared.Valid() {
		writeWebError(writer, fmt.Errorf("invalid inbox cursor"))
		return
	}
	inbox, err := s.NotificationLog.Advance(advance.Read, advance.Cleared)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeWebJSON(writer, inbox)
}
