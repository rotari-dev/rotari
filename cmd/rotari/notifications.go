package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rotari-dev/rotari/internal/joblist"
	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/notificationlog"
	"github.com/rotari-dev/rotari/internal/runlist"
	"github.com/rotari-dev/rotari/internal/state"
)

type notificationJSON struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
	BaseDir   string `json:"basedir"`
	Project   string `json:"project"`
	RunID     string `json:"run_id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Unread    bool   `json:"unread"`
}

// cmdNotifications lists the notifications runs recorded for the Web inbox,
// oldest first, as the inbox renders them, across all known state
// directories; --basedir limits the listing to one directory. It never moves
// the inbox's shared read cursors; --unread lists only what is after its
// basedir's.
func cmdNotifications(args []string) int {
	fs := flag.NewFlagSet("notifications", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	basedir := cliString(fs, "basedir", "")
	projectName := cliString(fs, "project-name", "")
	masterdir := cliString(fs, "masterdir", "")
	since := cliString(fs, "since", joblist.DefaultSinceText)
	unreadOnly := cliBool(fs, "unread", false)
	jsonOutput := cliBool(fs, "json", false)
	if err := cliParse(fs, args); err != nil {
		return 1
	}
	projectFilter, ok := projectListingFilter(fs, "notifications", *projectName)
	if !ok {
		return 1
	}
	window, err := joblist.ParseSince(*since)
	if err != nil {
		printErrorf("invalid --since duration %q", *since)
		return 1
	}
	baseDirs, missingBaseDirs, err := jobsBaseDirs(valueIfSet(fs, "basedir", *basedir), *masterdir)
	if err != nil {
		printError(err)
		return 1
	}
	warnMissingBaseDirs(missingBaseDirs)
	masterDir, err := state.ResolveMasterDir(*masterdir)
	if err != nil {
		printErrorf("failed to resolve master directory: %v", err)
		return 1
	}
	log := notificationlog.Open(masterDir)
	inbox, err := log.State()
	if err != nil {
		printError(err)
		return 1
	}
	records, problems, err := log.Records()
	if err != nil {
		printErrorf("failed to read notifications: %v", err)
		return 1
	}
	for _, problem := range problems {
		printWarningf("WARNING: unreadable notification record: %s", problem)
	}
	inScope := make(map[string]bool, len(baseDirs))
	for _, baseDir := range baseDirs {
		inScope[baseDir] = true
	}
	cutoff := time.Now().Add(-window)
	selected := make([]notificationlog.Record, 0, len(records))
	for _, record := range records {
		created, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
		if err != nil || created.Before(cutoff) {
			continue
		}
		if !inScope[record.BaseDir] || (projectFilter != "" && record.Project != projectFilter) {
			continue
		}
		if *unreadOnly && !record.Cursor().After(inbox.For(record.BaseDir).Read) {
			continue
		}
		selected = append(selected, record)
	}
	entries := notificationlog.Entries(selected, 0)
	// Records come newest first; a listing reads oldest first.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	if *jsonOutput {
		rows := make([]notificationJSON, 0, len(entries))
		for _, entry := range entries {
			rows = append(rows, notificationJSON{CreatedAt: entry.CreatedAt, ID: entry.ID, BaseDir: entry.BaseDir, Project: entry.Project, RunID: entry.RunID,
				Title: entry.Title, Body: entry.Body, Unread: entry.Cursor.After(inbox.For(entry.BaseDir).Read)})
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(rows); err != nil {
			printError(err)
			return 1
		}
		return 0
	}
	if len(entries) == 0 {
		kind := "notifications"
		if *unreadOnly {
			kind = "unread notifications"
		}
		fmt.Printf("No %s recorded within %s in %s.\n", kind, *since, runlist.DescribeScope(baseDirs, projectFilter))
		return 0
	}
	for _, entry := range entries {
		header := model.FormatDisplayTimestamp(entry.CreatedAt) + "  " + entry.Title
		if entry.Cursor.After(inbox.For(entry.BaseDir).Read) {
			header += "  " + yellow("(unread)")
		}
		fmt.Println(header)
		location := "  project=" + entry.Project + " run_id=" + entry.RunID
		if len(baseDirs) > 1 {
			location += " basedir=" + entry.BaseDir
		}
		fmt.Println(location)
		for _, line := range strings.Split(entry.Body, "\n") {
			if line == "" {
				fmt.Println()
				continue
			}
			fmt.Println("    " + line)
		}
		fmt.Println()
	}
	fmt.Println("To inspect a run:")
	fmt.Println("  rotari show -r RUN_ID")
	return 0
}
