package webui

import (
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/joblist"
)

// A Recent jobs row shows the command under the job name, as the run page
// does, and the attempt ID by its end with the whole ID kept in its text,
// title, and copy button.
func TestJobsHTMLShowsCommandUnderJobAndAttemptByItsEnd(t *testing.T) {
	attempt := "att_20261010-205222-3185eff0-c18f6264e-1-0"
	page := jobsHTML("/", nil, []joblist.Row{{State: "success", Project: "p", RunID: "r", JobName: "decode[1]", Command: "sh -c echo…", FullCommand: "sh -c echo hi", AttemptID: attempt}}, joblist.DefaultSinceText, false, false)
	for _, want := range []string{
		`decode[1]</a><div class="jobs-command"><code title="sh -c echo hi">sh -c echo…</code>`,
		`data-copy-value="sh -c echo hi"`,
		`<td class="jobs-attempt" data-sort-value="` + attempt + `"><span class="id-tail" title="` + attempt + `"><bdi>` + attempt + `</bdi></span>`,
		`data-copy-value="` + attempt + `"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("jobs page lacks %q", want)
		}
	}
	if strings.Contains(page, `data-sort="command"`) {
		t.Error("jobs page still has a separate Command column")
	}
}

// The Recent jobs subtitle names the window the page lists, not a fixed
// default.
func TestJobsHTMLDescribesSinceWindow(t *testing.T) {
	for _, test := range []struct {
		since string
		want  string
	}{
		{"", "Unfinished jobs and jobs finished in the last 1d"},
		{joblist.DefaultSinceText, "Unfinished jobs and jobs finished in the last 1d"},
		{"7d", "Unfinished jobs and jobs finished in the last 7d"},
		{"90m", "Unfinished jobs and jobs finished in the last 90m"},
		{"0", "Unfinished jobs<"},
		{"0s", "Unfinished jobs<"},
	} {
		page := jobsHTML("/", nil, nil, test.since, true, false)
		if !strings.Contains(page, `<div class="facts">`+test.want) {
			t.Errorf("since %q: jobs page lacks subtitle %q", test.since, test.want)
		}
		if strings.Contains(page, "24 hours") {
			t.Errorf("since %q: jobs page still names a fixed 24-hour window", test.since)
		}
	}
}
