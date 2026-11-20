package stickies

import (
	"strings"
	"testing"
	"time"
)

func TestSaveLoadRoundTripAndRemoval(t *testing.T) {
	dir := t.TempDir()
	notes := []Note{{ID: "a", Text: "check this", Color: "pink", X: 10, Y: 20.5, Width: 300, Height: 200, CreatedAt: "2026-11-18T14:00:00+09:00"}}
	if err := Save(dir, "projects/demo", notes); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir, "projects/demo")
	if err != nil || len(got) != 1 || got[0] != notes[0] {
		t.Fatalf("Load() = %#v, %v", got, err)
	}
	other, err := Load(dir, "projects/other")
	if err != nil || other == nil || len(other) != 0 {
		t.Fatalf("Load(other) = %#v, %v", other, err)
	}
	if err := Save(dir, "projects/demo", nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(dir, "projects/demo"); len(got) != 0 {
		t.Fatalf("notes after removal = %#v", got)
	}
}

func TestSaveRejectsInvalidNotes(t *testing.T) {
	ok := Note{ID: "a", Color: "yellow"}
	for name, test := range map[string]struct {
		page  string
		notes []Note
	}{
		"empty page":     {"", []Note{ok}},
		"empty id":       {"p", []Note{{Color: "yellow"}}},
		"duplicate id":   {"p", []Note{ok, ok}},
		"unknown color":  {"p", []Note{{ID: "a", Color: "red"}}},
		"narrow note":    {"p", []Note{{ID: "a", Color: "yellow", Width: 50}}},
		"huge note":      {"p", []Note{{ID: "a", Color: "yellow", Height: MaxSize + 1}}},
		"negative x":     {"p", []Note{{ID: "a", Color: "yellow", X: -1}}},
		"long text":      {"p", []Note{{ID: "a", Color: "yellow", Text: strings.Repeat("x", MaxTextRunes+1)}}},
		"too many notes": {"p", make([]Note, MaxNotesPerPage+1)},
		"long page":      {strings.Repeat("p", MaxPageKeyBytes+1), []Note{ok}},
	} {
		if err := Save(t.TempDir(), test.page, test.notes); err == nil {
			t.Errorf("%s: Save() succeeded", name)
		}
	}
}

func TestPagesListsPagesWithNotesInOrder(t *testing.T) {
	dir := t.TempDir()
	if pages, err := Pages(dir); err != nil || len(pages) != 0 {
		t.Fatalf("Pages() of an empty basedir = %#v, %v", pages, err)
	}
	for _, page := range []string{"projects/b", "/", "projects/a"} {
		if err := Save(dir, page, []Note{{ID: "n", Color: "yellow", Text: page}}); err != nil {
			t.Fatal(err)
		}
	}
	pages, err := Pages(dir)
	if err != nil || len(pages) != 3 || pages[0].Page != "/" || pages[1].Page != "projects/a" || pages[2].Page != "projects/b" {
		t.Fatalf("Pages() = %#v, %v", pages, err)
	}
}

func TestSaveFillsAndChecksCreationTime(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "p", []Note{{ID: "a", Color: "yellow"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := Load(dir, "p")
	if _, err := time.Parse(time.RFC3339, got[0].CreatedAt); err != nil {
		t.Fatalf("created_at = %q", got[0].CreatedAt)
	}
	if err := Save(dir, "p", []Note{{ID: "a", Color: "yellow", CreatedAt: "yesterday"}}); err == nil {
		t.Fatal("Save() accepted a created_at that is not RFC 3339")
	}
}

func TestSearchMatchesEveryWordIgnoringCase(t *testing.T) {
	pages := []Page{
		{Page: "/", Notes: []Note{{ID: "a", Text: "Re-run SEED 2"}, {ID: "b", Text: "seed only"}}},
		{Page: "projects/x", Notes: []Note{{ID: "c", Text: "other"}}},
	}
	got, err := Search(pages, "  seed RE-run ")
	if err != nil || len(got) != 1 || got[0].Page != "/" || len(got[0].Notes) != 1 || got[0].Notes[0].ID != "a" {
		t.Fatalf("Search() = %#v, %v", got, err)
	}
	if got, err := Search(pages, "absent"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("Search(absent) = %#v, %v", got, err)
	}
	if _, err := Search(pages, " \t"); err == nil {
		t.Fatal("a query without words was accepted")
	}
}
