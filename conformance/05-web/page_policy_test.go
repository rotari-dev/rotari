package web

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

var (
	scriptTag      = regexp.MustCompile(`(?m)^\s*<script\b[^>]*>`)
	pageNonce      = regexp.MustCompile(`script-src 'nonce-([^']+)'`)
	inlineHandlers = regexp.MustCompile(`[\s"']on(click|change|toggle|load|error|submit)=\\?["']`)
)

// TestLivePagesSendContentSecurityPolicy checks each live page's policy and that
// its scripts carry a nonce that changes with every response. Whether a browser
// then enforces it is TestPagePolicyBlocksPlantedScript.
func TestLivePagesSendContentSecurityPolicy(t *testing.T) {
	covers(t, "WEB-29")
	e := support.NewEnv(t)
	e.CreateFinishedRun()
	table := filepath.Join(e.Root, "data.csv")
	if err := os.WriteFile(table, []byte("a,b\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := e.StartWeb()
	for name, address := range map[string]string{
		"projects":     live + "/",
		"recent jobs":  live + "/jobs/",
		"Table viewer": e.StartTableView(table),
	} {
		t.Run(name, func(t *testing.T) {
			nonces := map[string]bool{}
			for range 2 {
				response, err := http.Get(address)
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				body := string(content)
				policy := response.Header.Get("Content-Security-Policy")
				match := pageNonce.FindStringSubmatch(policy)
				if match == nil {
					t.Fatalf("policy %q has no script nonce", policy)
				}
				nonces[match[1]] = true
				for _, directive := range []string{"default-src 'none'", "connect-src 'self'", "base-uri 'none'", "object-src 'none'"} {
					if !strings.Contains(policy, directive) {
						t.Errorf("policy %q lacks %s", policy, directive)
					}
				}
				if scripts := policy[strings.Index(policy, "script-src"):]; strings.Contains(scripts[:strings.Index(scripts, ";")], "unsafe") {
					t.Errorf("policy %q lets inline script run", policy)
				}
				tags := scriptTag.FindAllString(body, -1)
				if len(tags) == 0 {
					t.Fatal("the page has no script tags")
				}
				for _, tag := range tags {
					if !strings.Contains(tag, `nonce="`+match[1]+`"`) {
						t.Errorf("%s does not have the response's nonce %s", tag, match[1])
					}
				}
				if found := inlineHandlers.FindString(body); found != "" {
					t.Errorf("the page has an inline handler: %s", found)
				}
			}
			if len(nonces) != 2 {
				t.Errorf("two responses shared a nonce: %v", nonces)
			}
		})
	}
}

// TestStaticExportHasNoScriptNonce checks that an export, which has no response
// headers to carry a policy, opens from plain files with plain script tags.
func TestStaticExportHasNoScriptNonce(t *testing.T) {
	covers(t, "WEB-29")
	e := support.NewEnv(t)
	e.CreateFinishedRun()
	export := filepath.Join(e.Root, "export")
	e.MustRotari("web", "--static-dir", export)
	index, err := os.ReadFile(filepath.Join(export, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(index), "nonce=") || strings.Contains(string(index), "__ROTARI_SCRIPT_NONCE__") {
		t.Error("a static export has a script nonce or its placeholder")
	}
	if found := inlineHandlers.FindString(string(index)); found != "" {
		t.Errorf("a static export has an inline handler: %s", found)
	}
}
