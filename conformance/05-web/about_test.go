package web

import (
	"html"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestSettingsSayWhichRotariServes checks that the Settings menu of both page kinds
// names the version rotari version prints and the binary serving the page.
func TestSettingsSayWhichRotariServes(t *testing.T) {
	covers(t, "WEB-37")
	e := support.NewEnv(t)
	version := strings.TrimSpace(e.MustRotari("version").Stdout)
	binary := e.Command("version").Path
	web := e.StartWeb()
	for _, path := range []string{"/", "/jobs/"} {
		got := e.HTTPGet(web + path)
		if got.Status != 200 {
			t.Fatalf("%s: status %d", path, got.Status)
		}
		for _, want := range []string{
			`<div class="settings-about-version">` + html.EscapeString(version) + `</div>`,
			`data-copy-value="` + html.EscapeString(binary) + `"`,
		} {
			if !strings.Contains(got.Body, want) {
				t.Errorf("%s: the Settings menu has no %s", path, want)
			}
		}
	}
}
