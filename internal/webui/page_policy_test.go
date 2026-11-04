package webui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var inlineHandler = regexp.MustCompile(`[\s"'` + "`" + `]on[a-z]+=\\?["']`)

// A Content-Security-Policy that allows only the page's scripts stops inline
// handlers, so no asset may use one.
func TestAssetsHaveNoInlineHandlers(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("assets", "*"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, "assets.go")
	for _, path := range paths {
		if !strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".html") && !strings.HasSuffix(path, ".go") {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for number, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
				continue
			}
			if match := inlineHandler.FindString(line); match != "" {
				t.Errorf("%s:%d has an inline handler: %s", path, number+1, trimmed)
			}
		}
	}
}

// Every action a control names must be one web_shell.js can run.
func TestPageActionsAreRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, match := range regexp.MustCompile(`(?m)^    (\w+): `).FindAllStringSubmatch(webShellJS, -1) {
		registered[match[1]] = true
	}
	named := regexp.MustCompile(`data-(?:change-|toggle-)?action=\\?"(\w+)|actionAttributes\(\s*"(\w+)"`)
	bells := regexp.MustCompile(`BellHTML\((?:entry, "", )?"(\w+)"`)
	sources := map[string]string{"web_template.html": webTemplateHTML, "jobs_template.html": jobsTemplateHTML}
	paths, _ := filepath.Glob(filepath.Join("assets", "web_app_*.js"))
	paths = append(paths, "assets.go")
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = string(content)
	}
	count := 0
	for source, content := range sources {
		for _, pattern := range []*regexp.Regexp{named, bells} {
			for _, match := range pattern.FindAllStringSubmatch(content, -1) {
				name := match[1]
				if name == "" {
					name = match[2]
				}
				count++
				if !registered[name] {
					t.Errorf("%s names action %q, which web_shell.js does not register", source, name)
				}
			}
		}
	}
	if count < 50 {
		t.Fatalf("found %d named actions, so the patterns no longer match the assets", count)
	}
}

// Every script tag of a page takes the nonce, and a value holding the
// placeholder's text cannot obtain it.
func TestPagesApplyScriptNonceInOnePass(t *testing.T) {
	const nonce = "test-nonce"
	const want = ` nonce="` + nonce + `"`
	basedirs := []webBaseDir{newWebBaseDir("id", "/state/"+scriptNoncePlaceholder, true)}
	pages := map[string]string{
		"main":         composeWebPage(nonce, nil, false, "", "", basedirs),
		"recent jobs":  jobsPage(nonce, "/", []string{scriptNoncePlaceholder}, nil, jobsSelection{}, true, false, ""),
		"Table viewer": tableViewPage(nil, "", nonce),
	}
	for name, page := range pages {
		tags := regexp.MustCompile(`(?m)^\s*<script\b[^>]*>`).FindAllString(page, -1)
		if len(tags) == 0 {
			t.Fatalf("%s has no script tags", name)
		}
		for _, tag := range tags {
			if !strings.Contains(tag, want) {
				t.Errorf("%s: %s has no nonce", name, tag)
			}
		}
	}
	if !strings.Contains(pages["main"], scriptNoncePlaceholder) || !strings.Contains(pages["recent jobs"], "project/"+scriptNoncePlaceholder) {
		t.Error("a value holding the placeholder's text did not stay as written")
	}
	for _, page := range []string{composeWebHTML(nil, false, ""), tableViewPage(nil, "", "")} {
		if strings.Contains(page, "nonce=") || strings.Contains(page, scriptNoncePlaceholder) {
			t.Error("a page without a policy has a nonce or its placeholder")
		}
	}
}
