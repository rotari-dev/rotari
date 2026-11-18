// Package stickies keeps the sticky notes users put on Web UI pages.
//
// A basedir holds one file, <basedir>/stickies.json, mapping a page key (the
// page's route inside the Web UI, an arbitrary string rather than a path) to
// the notes on that page. Notes are free-form reminders: no job, run, or
// result depends on them. Knows nothing of HTTP.
package stickies

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/rotari-dev/rotari/internal/state"
)

const (
	fileName     = "stickies.json"
	lockFileName = "stickies.lock"

	MaxNotesPerPage = 100
	MaxPages        = 1000
	MaxTextRunes    = 4000
	MaxIDLength     = 64
	MaxPageKeyBytes = 1024
	MaxCoordinate   = 1e6

	// Sizes are in pixels; a note of size 0 has the default size.
	MinWidth  = 120
	MinHeight = 80
	MaxSize   = 2000
)

// Colors are the colours a note may have; the first is the default.
var Colors = []string{"yellow", "pink", "green", "blue", "orange"}

// Note is one sticky note. X and Y are pixels from the page's top-left
// corner.
type Note struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	Color string  `json:"color"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	// Width and Height are the note's size in pixels, 0 for the default.
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	// CreatedAt is when the note was made, RFC 3339; Save fills it in for a
	// note that has none.
	CreatedAt string `json:"created_at"`
}

type file struct {
	Pages map[string][]Note `json:"pages"`
}

// Validate reports why notes cannot be saved for a page.
func Validate(page string, notes []Note) error {
	if page == "" {
		return errors.New("page is required")
	}
	if len(page) > MaxPageKeyBytes || !utf8.ValidString(page) {
		return fmt.Errorf("page must be valid UTF-8 of at most %d bytes", MaxPageKeyBytes)
	}
	if len(notes) > MaxNotesPerPage {
		return fmt.Errorf("a page holds at most %d sticky notes", MaxNotesPerPage)
	}
	seen := map[string]bool{}
	for _, note := range notes {
		if note.ID == "" || len(note.ID) > MaxIDLength {
			return fmt.Errorf("sticky note id must be 1 to %d bytes", MaxIDLength)
		}
		if seen[note.ID] {
			return fmt.Errorf("duplicate sticky note id %q", note.ID)
		}
		seen[note.ID] = true
		if utf8.RuneCountInString(note.Text) > MaxTextRunes || !utf8.ValidString(note.Text) {
			return fmt.Errorf("sticky note text must be valid UTF-8 of at most %d characters", MaxTextRunes)
		}
		if note.CreatedAt != "" {
			if _, err := time.Parse(time.RFC3339, note.CreatedAt); err != nil {
				return fmt.Errorf("sticky note created_at must be RFC 3339: %w", err)
			}
		}
		known := false
		for _, color := range Colors {
			known = known || color == note.Color
		}
		if !known {
			return fmt.Errorf("unknown sticky note color %q", note.Color)
		}
		for _, size := range []struct{ value, min float64 }{{note.Width, MinWidth}, {note.Height, MinHeight}} {
			if math.IsNaN(size.value) || size.value != 0 && (size.value < size.min || size.value > MaxSize) {
				return fmt.Errorf("sticky note size must be 0 or between %g and %d pixels", size.min, MaxSize)
			}
		}
		for _, value := range []float64{note.X, note.Y} {
			if math.IsNaN(value) || value < 0 || value > MaxCoordinate {
				return fmt.Errorf("sticky note position must be between 0 and %g", MaxCoordinate)
			}
		}
	}
	return nil
}

func path(baseDir string) string { return filepath.Join(baseDir, fileName) }

func read(baseDir string) (file, error) {
	var stored file
	err := state.NewStore(state.DirectoryMode(), state.FileMode()).ReadJSON(path(baseDir), &stored)
	if errors.Is(err, os.ErrNotExist) {
		return file{}, nil
	}
	if err != nil {
		return file{}, fmt.Errorf("failed to read sticky notes: %w", err)
	}
	return stored, nil
}

// Load returns the notes on a page; a page without notes has none.
func Load(baseDir, page string) ([]Note, error) {
	if page == "" {
		return nil, errors.New("page is required")
	}
	stored, err := read(baseDir)
	if err != nil {
		return nil, err
	}
	notes := stored.Pages[page]
	if notes == nil {
		notes = []Note{}
	}
	return notes, nil
}

// Page is a page key with its notes.
type Page struct {
	Page  string `json:"page"`
	Notes []Note `json:"notes"`
}

// Pages returns every page that has notes, ordered by page key.
func Pages(baseDir string) ([]Page, error) {
	stored, err := read(baseDir)
	if err != nil {
		return nil, err
	}
	pages := make([]Page, 0, len(stored.Pages))
	for page, notes := range stored.Pages {
		if len(notes) > 0 {
			pages = append(pages, Page{Page: page, Notes: notes})
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Page < pages[j].Page })
	return pages, nil
}

// Save replaces the notes on a page; no notes removes the page.
func Save(baseDir, page string, notes []Note) error {
	if err := Validate(page, notes); err != nil {
		return err
	}
	notes = append([]Note(nil), notes...)
	for index := range notes {
		if notes[index].CreatedAt == "" {
			notes[index].CreatedAt = time.Now().Format(time.RFC3339)
		}
	}
	if err := os.MkdirAll(baseDir, state.DirectoryMode()); err != nil {
		return err
	}
	release, err := state.AcquireStateLock(filepath.Join(baseDir, lockFileName))
	if err != nil {
		return fmt.Errorf("lock sticky notes: %w", err)
	}
	defer release()
	stored, err := read(baseDir)
	if err != nil {
		return err
	}
	if stored.Pages == nil {
		stored.Pages = map[string][]Note{}
	}
	if len(notes) == 0 {
		delete(stored.Pages, page)
	} else {
		if _, exists := stored.Pages[page]; !exists && len(stored.Pages) >= MaxPages {
			return fmt.Errorf("sticky notes are kept for at most %d pages", MaxPages)
		}
		stored.Pages[page] = notes
	}
	return state.NewStore(state.DirectoryMode(), state.FileMode()).WriteJSON(path(baseDir), stored)
}
