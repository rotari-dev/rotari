package notificationlog

import (
	"fmt"

	"github.com/rotari-dev/rotari/internal/notification"
)

// Entry is one record as the Web inbox and `rotari notifications` show it.
type Entry struct {
	Cursor
	BaseDir string
	Project string
	RunID   string
	notification.BrowserMessage
}

// Entries renders records, in their order, with each project's current
// browser settings, up to limit entries (no limit when limit is 0). A record
// whose events the settings all leave out gives no entry; a project whose
// settings cannot be read gives an entry that says so.
func Entries(records []Record, limit int) []Entry {
	entries := make([]Entry, 0, len(records))
	settings := map[string]notification.ChannelSettings{}
	for _, record := range records {
		if limit > 0 && len(entries) == limit {
			break
		}
		entry := Entry{Cursor: record.Cursor(), BaseDir: record.BaseDir, Project: record.Project, RunID: record.RunID}
		key := record.BaseDir + "\x00" + record.Project
		browser, ok := settings[key]
		if !ok {
			loaded, err := notification.Load(record.BaseDir, record.Project)
			if err != nil {
				entry.Title = "rotari: notification settings unreadable"
				entry.Body = fmt.Sprintf("project: %s\nrun id: %s\nerror: %v", record.Project, record.RunID, err)
				entries = append(entries, entry)
				continue
			}
			browser = loaded.Settings.Browser
			settings[key] = browser
		}
		message, ok := notification.NewBrowserMessage(record.Events, browser)
		if !ok {
			continue
		}
		entry.BrowserMessage = message
		entries = append(entries, entry)
	}
	return entries
}
