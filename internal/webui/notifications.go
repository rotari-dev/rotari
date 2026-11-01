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
	// BaseDir is the ID of the basedir whose run recorded the entry.
	BaseDir string `json:"basedir"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	// URL opens the entry's run when the project's browser fields select
	// "link".
	URL string `json:"url,omitempty"`
}

// inboxCursors carries inbox cursors keyed by basedir ID, the form pages use.
type inboxCursors struct {
	BaseDirs map[string]notificationlog.Cursors `json:"basedirs"`
}

type inboxResponse struct {
	Entries []inboxEntry `json:"entries"`
	inboxCursors
	// Problems describes records that could not be read.
	Problems []string `json:"problems,omitempty"`
}

// inboxBasedirs maps the basedir IDs a page names to their paths; an unknown
// ID is an error.
func (s site) inboxBasedirs(ids []string) (map[string]string, error) {
	known := map[string]string{}
	for _, entry := range s.basedirEntries() {
		known[entry.ID] = entry.Path
	}
	paths := make(map[string]string, len(ids))
	for _, id := range ids {
		path, ok := known[id]
		if !ok {
			return nil, fmt.Errorf("unknown basedir %q", id)
		}
		paths[id] = path
	}
	return paths, nil
}

// handleNotifications serves the recorded notifications of the basedirs
// named by the comma-separated basedirs parameter, newest first, each after
// its basedir's cleared cursor and rendered with its project's current
// browser settings, with those basedirs' cursors.
func (s site) handleNotifications(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	if s.NotificationLog == nil {
		http.Error(writer, "this server keeps no notification history", http.StatusNotFound)
		return
	}
	var ids []string
	if value := request.URL.Query().Get("basedirs"); value != "" {
		ids = strings.Split(value, ",")
	}
	selected, err := s.inboxBasedirs(ids)
	if err != nil {
		writeWebError(writer, err)
		return
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
	response := inboxResponse{Entries: []inboxEntry{}, inboxCursors: inboxCursors{BaseDirs: map[string]notificationlog.Cursors{}}, Problems: problems}
	for id, path := range selected {
		response.BaseDirs[id] = inbox.For(path)
	}
	shown := make([]notificationlog.Record, 0, len(records))
	for _, record := range records {
		id := notification.WebBaseDirID(record.BaseDir)
		if _, ok := selected[id]; ok && record.Cursor().After(inbox.For(record.BaseDir).Cleared) {
			shown = append(shown, record)
		}
	}
	for _, rendered := range notificationlog.Entries(shown, inboxLimit) {
		id := notification.WebBaseDirID(rendered.BaseDir)
		entry := inboxEntry{Cursor: rendered.Cursor, BaseDir: id, Title: rendered.Title, Body: rendered.Body}
		if rendered.Link {
			entry.URL = notification.WebBaseDirRoutePrefix + id + "/project/" + url.PathEscape(rendered.Project) + "/run/" + url.PathEscape(rendered.RunID)
		}
		response.Entries = append(response.Entries, entry)
	}
	writeWebJSON(writer, response)
}

// handleNotificationState advances the shared read and cleared cursors of
// the basedirs the request names and answers their cursors.
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
	var advance inboxCursors
	if err := json.NewDecoder(request.Body).Decode(&advance); err != nil {
		writeWebError(writer, err)
		return
	}
	ids := make([]string, 0, len(advance.BaseDirs))
	for id, cursors := range advance.BaseDirs {
		if !cursors.Valid() {
			writeWebError(writer, fmt.Errorf("invalid inbox cursor for basedir %q", id))
			return
		}
		ids = append(ids, id)
	}
	paths, err := s.inboxBasedirs(ids)
	if err != nil {
		writeWebError(writer, err)
		return
	}
	changes := make(map[string]notificationlog.Cursors, len(paths))
	for id, path := range paths {
		changes[path] = advance.BaseDirs[id]
	}
	inbox, err := s.NotificationLog.Advance(changes)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	response := inboxCursors{BaseDirs: map[string]notificationlog.Cursors{}}
	for id, path := range paths {
		response.BaseDirs[id] = inbox.For(path)
	}
	writeWebJSON(writer, response)
}
