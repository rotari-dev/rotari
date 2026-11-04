package webui

import (
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/joblist"
)

// A value is inserted as written even when it contains the text of a
// placeholder: the placeholder keeps its own value, and the value keeps its
// text. Replacing one placeholder at a time would search values inserted
// earlier, and a value with quotes in it would then end the string around it.
func TestComposeKeepsPlaceholderTextInValues(t *testing.T) {
	t.Run("static bootstrap", func(t *testing.T) {
		const planted = `"__ROTARI_STATIC_LOGS_DATA__ __ROTARI_STATIC_ACTIVITY_DATA__"`
		bootstrap := composeStaticBootstrap(planted, `{"log":1}`, "{}", "{}", "{}", "{}", "{}", "{}", `[{"activity":1}]`)
		for _, want := range []string{
			"window.__ROTARI_STATIC_STATE__ = " + planted + ";",
			`window.__ROTARI_STATIC_LOGS__ = {"log":1};`,
			`window.__ROTARI_STATIC_ACTIVITY__ = [{"activity":1}];`,
		} {
			if !strings.Contains(bootstrap, want) {
				t.Errorf("bootstrap lacks %q:\n%s", want, bootstrap[:min(len(bootstrap), 500)])
			}
		}
	})

	t.Run("main page", func(t *testing.T) {
		const planted = "/state/__ROTARI_STATIC_BOOTSTRAP__/__ROTARI_NOTIFICATION_SESSION__/__ROTARI_STATUS_TONES__"
		page := composeWebHTMLWithSession(nil, false, "<script>bootstrap</script>", "session-key",
			[]webBaseDir{newWebBaseDir("id", planted, true)})
		for _, want := range []string{planted, "<script>bootstrap</script>", "session-key", statusTonesJSON()} {
			if !strings.Contains(page, want) {
				t.Errorf("page lacks %q", want)
			}
		}
		if strings.Contains(page, `/<script>bootstrap`) {
			t.Error("the bootstrap went into the basedir path")
		}
	})

	t.Run("recent jobs page", func(t *testing.T) {
		const project = "__ROTARI_CONTENT__"
		const scope = "__ROTARI_JOBS_CRUMBS__"
		rows := []joblist.Row{{State: "success", Project: project, RunID: "r", JobName: scope, Command: "c", FullCommand: "__ROTARI_JOBS_SCOPE__"}}
		page := jobsHTML("/", []string{project, scope}, rows, jobsSelection{}, true, false)
		for _, want := range []string{`/project/` + project + `"`, `/project/` + scope + `"`, `<span class="crumb-here">Recent jobs</span>`, `<section class="jobs-section">`} {
			if !strings.Contains(page, want) {
				t.Errorf("page lacks %q", want)
			}
		}
		if got := strings.Count(page, `<section class="jobs-section">`); got != 1 {
			t.Errorf("jobs tables = %d, want 1", got)
		}
		sidebar, _, _ := strings.Cut(page, `<span class="crumb-here">`)
		if strings.Contains(sidebar, `<section class="jobs-section">`) {
			t.Error("the jobs table went into the sidebar")
		}
	})
}
