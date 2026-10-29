package webui

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/rotari-dev/rotari/internal/joblist"
	"github.com/rotari-dev/rotari/internal/runlist"
)

// jobsWindows are the preset windows before now the Recent jobs page
// offers; the first is the default.
var jobsWindows = []struct {
	Text   string
	Window time.Duration
}{
	{"24h", joblist.DefaultSince},
	{"6h", 6 * time.Hour},
	{"1h", time.Hour},
}

// jobsViews are the widths the page's range bar can show. A range is
// dragged on the bar, so none is longer than the widest.
var jobsViews = map[string]time.Duration{
	"1d":  24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

const jobsMaxRange = 30 * 24 * time.Hour

// jobsSelection is what the Recent jobs page lists finished jobs for:
// a preset Window before now, or the range From up to To dragged on the
// bar. Spans holds the one span either means. Default marks a query that
// chose nothing, which resolveJobsDefault settles; Latest marks a range it
// set to the 24 hours up to the latest run's end.
type jobsSelection struct {
	Window  time.Duration
	From    time.Time
	To      time.Time
	Spans   []joblist.Interval
	Default bool
	Latest  bool
}

// parseJobsSelection reads the Recent jobs query: window (a preset), or
// from and to (RFC 3339, from before now, at most jobsMaxRange apart).
// view and end, the bar's width and right edge, are the page script's and
// only checked here. The since parameter the page used to take is
// rejected rather than ignored.
func parseJobsSelection(query url.Values, now time.Time) (jobsSelection, error) {
	if query.Has("since") {
		return jobsSelection{}, errors.New("since is no longer accepted on Recent jobs; choose a window or drag a range on its bar")
	}
	for _, name := range []string{"window", "from", "to", "view", "end"} {
		if len(query[name]) > 1 {
			return jobsSelection{}, fmt.Errorf("%s was given more than once", name)
		}
	}
	windowText, fromText, toText := query.Get("window"), query.Get("from"), query.Get("to")
	if view := query.Get("view"); view != "" {
		if _, ok := jobsViews[view]; !ok {
			return jobsSelection{}, fmt.Errorf("invalid view %q: want 1d, 7d, or 30d", view)
		}
	}
	if endText := query.Get("end"); endText != "" {
		end, err := time.Parse(time.RFC3339, endText)
		if err != nil {
			return jobsSelection{}, fmt.Errorf("invalid end %q: want an RFC 3339 time", endText)
		}
		if end.After(now) {
			return jobsSelection{}, fmt.Errorf("end %s is after now", endText)
		}
	}
	if windowText != "" && (fromText != "" || toText != "") {
		return jobsSelection{}, errors.New("choose either a window or from and to, not both")
	}
	if fromText == "" && toText == "" {
		if windowText == "" {
			return jobsSelection{Default: true}, nil
		}
		for _, preset := range jobsWindows {
			if preset.Text == windowText {
				return jobsSelection{Window: preset.Window, Spans: []joblist.Interval{{Start: now.Add(-preset.Window)}}}, nil
			}
		}
		return jobsSelection{}, fmt.Errorf("invalid window %q: want 24h, 6h, or 1h", windowText)
	}
	if fromText == "" || toText == "" {
		return jobsSelection{}, errors.New("from and to must be given together")
	}
	from, err := time.Parse(time.RFC3339, fromText)
	if err != nil {
		return jobsSelection{}, fmt.Errorf("invalid from %q: want an RFC 3339 time", fromText)
	}
	to, err := time.Parse(time.RFC3339, toText)
	if err != nil {
		return jobsSelection{}, fmt.Errorf("invalid to %q: want an RFC 3339 time", toText)
	}
	if !to.After(from) {
		return jobsSelection{}, fmt.Errorf("to %s is not after from %s", toText, fromText)
	}
	if !from.Before(now) {
		return jobsSelection{}, fmt.Errorf("from %s is not before now", fromText)
	}
	if to.Sub(from) > jobsMaxRange {
		return jobsSelection{}, fmt.Errorf("from %s to %s is longer than 30 days", fromText, toText)
	}
	return jobsSelection{From: from, To: to, Spans: []joblist.Interval{{Start: from, End: to}}}, nil
}

// resolveJobsDefault settles a selection that chose nothing: the 24 hours
// before now when baseDir's latest run ended within them, is running, or
// there is no run, and otherwise the 24 hours up to that run's end, rounded
// up to the minute so the jobs it finished last are inside. Other
// selections are returned as they are.
func resolveJobsDefault(selection jobsSelection, baseDir string, now time.Time) (jobsSelection, error) {
	if !selection.Default {
		return selection, nil
	}
	recent := jobsSelection{Window: joblist.DefaultSince, Spans: []joblist.Interval{{Start: now.Add(-joblist.DefaultSince)}}}
	latestStart, ok := runlist.LatestStart([]string{baseDir}, now)
	if !ok {
		return recent, nil
	}
	// A run that started earlier may still end later, so every run
	// meeting the time since the latest start is read.
	runs, err := runlist.CollectBetween([]string{baseDir}, "", latestStart, now)
	if err != nil {
		return jobsSelection{}, err
	}
	var latestEnd time.Time
	for _, run := range runlist.Overlapping(runs, latestStart, now, now) {
		if _, end, ok := run.Span(now); ok && end.After(latestEnd) {
			latestEnd = end
		}
	}
	if latestEnd.IsZero() || !latestEnd.Before(now.Add(-joblist.DefaultSince)) {
		return recent, nil
	}
	to := latestEnd.Truncate(time.Minute).Add(time.Minute)
	from := to.Add(-joblist.DefaultSince)
	return jobsSelection{From: from, To: to, Spans: []joblist.Interval{{Start: from, End: to}}, Latest: true}, nil
}

// jobsScopeText describes what the page lists: the dragged range, or the
// window before now.
func jobsScopeText(selection jobsSelection) string {
	if selection.Latest {
		return "Unfinished jobs and jobs finished in the 24 hours up to " + joblist.FormatTimestamp(selection.To) + ", around the latest run"
	}
	if !selection.From.IsZero() {
		return "Unfinished jobs and jobs finished from " + joblist.FormatTimestamp(selection.From) + " to " + joblist.FormatTimestamp(selection.To)
	}
	window := selection.Window
	if window == 0 {
		window = joblist.DefaultSince
	}
	if window == time.Hour {
		return "Unfinished jobs and jobs finished in the last hour"
	}
	return fmt.Sprintf("Unfinished jobs and jobs finished in the last %.0f hours", window.Hours())
}
