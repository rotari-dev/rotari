package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/stickies"
)

// cmdStickies lists the sticky notes users put on Web UI pages, read from the
// basedir the Web UI keeps them in. It never changes them.
func cmdStickies(args []string) int {
	fs := flag.NewFlagSet("stickies", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	basedir := cliString(fs, "basedir", "")
	page := cliString(fs, "page", "")
	search := cliString(fs, "search", "")
	jsonOutput := cliBool(fs, "json", false)
	if err := cliParse(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		printError("usage: " + cliUsage("stickies"))
		return 1
	}
	baseDir, _, err := state.ResolveBaseDir(*basedir)
	if err != nil {
		printError(err)
		return 1
	}
	pages, err := stickies.Pages(baseDir)
	if err != nil {
		printError(err)
		return 1
	}
	if cliOptionSet(fs, "page") {
		if *page == "" {
			printError("--page must not be empty; the home page is /")
			return 1
		}
		filtered := pages[:0]
		for _, entry := range pages {
			if entry.Page == *page {
				filtered = append(filtered, entry)
			}
		}
		pages = filtered
	}
	if cliOptionSet(fs, "search") {
		pages, err = stickies.Search(pages, *search)
		if err != nil {
			printError("--search: " + err.Error())
			return 1
		}
	}
	if *jsonOutput {
		encoded, err := json.MarshalIndent(map[string]any{"basedir": baseDir, "pages": pages}, "", "  ")
		if err != nil {
			printError(err)
			return 1
		}
		fmt.Println(string(encoded))
		return 0
	}
	if len(pages) == 0 {
		fmt.Println("No sticky notes found.")
		return 0
	}
	for index, entry := range pages {
		if index > 0 {
			fmt.Println()
		}
		fmt.Printf("%s\n", cyan("=== "+entry.Page+" ==="))
		for _, note := range entry.Notes {
			text := strings.TrimSpace(note.Text)
			if text == "" {
				text = "(empty)"
			}
			fmt.Printf("[%s] %s  %s\n", note.Color, formatStickyTime(note.CreatedAt), strings.ReplaceAll(text, "\n", "\n      "))
		}
	}
	return 0
}

func formatStickyTime(value string) string {
	created, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "-"
	}
	return created.Local().Format("2006-01-02 15:04")
}
