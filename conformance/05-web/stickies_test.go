package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// Sticky notes saved through the Web API belong to their page, outlive the
// server, and are listed by `rotari stickies`; a read-only server shows them
// but refuses to change them, and invalid notes are refused.
func TestWebStickiesAreSharedWithTheCLI(t *testing.T) {
	covers(t, "WEB-62")
	e := support.NewEnv(t)
	base := e.StartWeb()
	note := map[string]any{"id": "n1", "text": "re-run seed 2", "color": "pink", "x": 40, "y": 60}
	if response := e.HTTPPostJSON(base+"/api/stickies", map[string]any{"page": "projects/demo", "notes": []any{note}}); response.Status != 200 {
		t.Fatalf("POST /api/stickies = %d: %s", response.Status, response.Body)
	}
	if response := e.HTTPGet(base + "/api/stickies?page=projects/other"); response.Status != 200 || strings.Contains(response.Body, "seed 2") {
		t.Fatalf("another page = %d: %s", response.Status, response.Body)
	}
	all := e.HTTPGet(base + "/api/stickies")
	if all.Status != 200 || !strings.Contains(all.Body, "projects/demo") || !strings.Contains(all.Body, `"created_at":"20`) {
		t.Fatalf("GET /api/stickies without a page = %d: %s", all.Status, all.Body)
	}
	bad := map[string]any{"id": "n2", "text": "", "color": "red", "x": 0, "y": 0}
	if response := e.HTTPPostJSON(base+"/api/stickies", map[string]any{"page": "p", "notes": []any{bad}}); response.Status != 400 {
		t.Fatalf("invalid note = %d: %s", response.Status, response.Body)
	}

	readOnly := e.StartWeb("--allow-control=false")
	if response := e.HTTPPostJSON(readOnly+"/api/stickies", map[string]any{"page": "p", "notes": []any{}}); response.Status != 403 {
		t.Fatalf("read-only POST = %d: %s", response.Status, response.Body)
	}
	if response := e.HTTPGet(readOnly + "/api/stickies?page=projects/demo"); response.Status != 200 || !strings.Contains(response.Body, "re-run seed 2") {
		t.Fatalf("read-only GET = %d: %s", response.Status, response.Body)
	}

	listed := e.Rotari("stickies")
	if listed.Code != 0 || !strings.Contains(listed.Stdout, "projects/demo") || !strings.Contains(listed.Stdout, "re-run seed 2") {
		t.Fatalf("rotari stickies = %d: %s%s", listed.Code, listed.Stdout, listed.Stderr)
	}
	jsonList := e.Rotari("stickies", "--json", "--page", "projects/demo")
	var decoded struct {
		Pages []struct {
			Page  string `json:"page"`
			Notes []struct {
				Text string `json:"text"`
			} `json:"notes"`
		} `json:"pages"`
	}
	if err := json.Unmarshal([]byte(jsonList.Stdout), &decoded); err != nil || len(decoded.Pages) != 1 || decoded.Pages[0].Notes[0].Text != "re-run seed 2" {
		t.Fatalf("rotari stickies --json = %v: %s", err, jsonList.Stdout)
	}
	if none := e.Rotari("stickies", "--page", "projects/none"); none.Code != 0 || !strings.Contains(none.Stdout, "No sticky notes found.") {
		t.Fatalf("rotari stickies --page of an empty page = %d: %s", none.Code, none.Stdout)
	}
}

// Searching sticky notes finds the notes whose text holds every word, ignoring
// case, across pages or within one page, the same way through the Web API and
// `rotari stickies --search`; an empty search is refused rather than listing
// everything.
func TestWebStickiesAreSearchable(t *testing.T) {
	covers(t, "WEB-62")
	e := support.NewEnv(t)
	base := e.StartWeb()
	save := func(page string, notes ...map[string]any) {
		t.Helper()
		items := []any{}
		for _, note := range notes {
			items = append(items, note)
		}
		if response := e.HTTPPostJSON(base+"/api/stickies", map[string]any{"page": page, "notes": items}); response.Status != 200 {
			t.Fatalf("POST /api/stickies = %d: %s", response.Status, response.Body)
		}
	}
	save("projects/demo",
		map[string]any{"id": "a", "text": "Re-run SEED 2", "color": "pink", "x": 0, "y": 0},
		map[string]any{"id": "b", "text": "seed only", "color": "blue", "x": 0, "y": 0})
	save("projects/other", map[string]any{"id": "c", "text": "re-run later", "color": "green", "x": 0, "y": 0})

	if response := e.HTTPGet(base + "/api/stickies?q=SEED+re-run"); response.Status != 200 ||
		!strings.Contains(response.Body, "Re-run SEED 2") || strings.Contains(response.Body, "seed only") || strings.Contains(response.Body, "re-run later") {
		t.Fatalf("search across pages = %d: %s", response.Status, response.Body)
	}
	if response := e.HTTPGet(base + "/api/stickies?page=projects/demo&q=seed"); response.Status != 200 ||
		!strings.Contains(response.Body, "seed only") || !strings.Contains(response.Body, "Re-run SEED 2") {
		t.Fatalf("search within a page = %d: %s", response.Status, response.Body)
	}
	if response := e.HTTPGet(base + "/api/stickies?page=projects/other&q=seed"); response.Status != 200 || strings.Contains(response.Body, "later") || !strings.Contains(response.Body, `"notes":[]`) {
		t.Fatalf("search without a match = %d: %s", response.Status, response.Body)
	}
	if response := e.HTTPGet(base + "/api/stickies?q="); response.Status != 400 {
		t.Fatalf("empty search = %d: %s", response.Status, response.Body)
	}

	found := e.Rotari("stickies", "--search", "later")
	if found.Code != 0 || !strings.Contains(found.Stdout, "projects/other") || strings.Contains(found.Stdout, "seed") {
		t.Fatalf("rotari stickies --search = %d: %s%s", found.Code, found.Stdout, found.Stderr)
	}
	if none := e.Rotari("stickies", "--search", "later", "--page", "projects/demo"); none.Code != 0 || !strings.Contains(none.Stdout, "No sticky notes found.") {
		t.Fatalf("rotari stickies --search with --page = %d: %s", none.Code, none.Stdout)
	}
	if empty := e.Rotari("stickies", "--search", " "); empty.Code == 0 || !strings.Contains(empty.Stderr, "--search") {
		t.Fatalf("empty --search = %d: %s%s", empty.Code, empty.Stdout, empty.Stderr)
	}
}
