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
	response := inboxResponse{Entries: []inboxEntry{}, InboxState: inbox, Problems: problems}
	settings := map[string]notification.ChannelSettings{}
	for _, record := range records {
		if len(response.Entries) == inboxLimit || !record.Cursor().After(inbox.Cleared) {
			break
		}
		baseDirID := notification.WebBaseDirID(record.BaseDir)
		if !selected[baseDirID] {
			continue
		}
		key := record.BaseDir + "\x00" + record.Project
		browser, ok := settings[key]
		if !ok {
			loaded, err := notification.Load(record.BaseDir, record.Project)
			if err != nil {
				response.Entries = append(response.Entries, inboxEntry{Cursor: record.Cursor(), Title: "rotari: notification settings unreadable",
					Body: fmt.Sprintf("project: %s\nrun id: %s\nerror: %v", record.Project, record.RunID, err)})
				continue
			}
			browser = loaded.Settings.Browser
			settings[key] = browser
		}
		message, ok := notification.NewBrowserMessage(record.Events, browser)
		if !ok {
			continue
		}
		entry := inboxEntry{Cursor: record.Cursor(), Title: message.Title, Body: message.Body}
		if message.Link {
			entry.URL = notification.WebBaseDirRoutePrefix + baseDirID + "/project/" + url.PathEscape(record.Project) + "/run/" + url.PathEscape(record.RunID)
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
