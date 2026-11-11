package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestStaticExportsIgnoreConfiguredToken checks that rotari web and rotari
// tableview write a static export when the environment or config gives an
// auth-token, which protects only a live server, saying where it is ignored
// without printing it, and still refuse --auth-token on the command line
// (WEB-3, WEB-22).
func TestStaticExportsIgnoreConfiguredToken(t *testing.T) {
	covers(t, "WEB-3", "WEB-22")
	e := support.NewEnv(t)
	e.MustRotari("add", "--", "true")
	table := filepath.Join(e.Root, "data", "list.csv")
	if err := os.MkdirAll(filepath.Dir(table), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(table, []byte("name,value\na,1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const secret = "kept-secret-token"
	globalPath := globalConfig(e)
	exports := map[string]func(output string) []string{
		"web":       func(output string) []string { return []string{"web", "--static-dir", output} },
		"tableview": func(output string) []string { return []string{"tableview", "--static-dir", output, table} },
	}
	for command, args := range exports {
		for _, source := range []struct {
			name  string
			env   *support.Env
			where string
		}{
			{name: "environment", env: e.WithVar("ROTARI_WEB_AUTH_TOKEN", secret), where: "ROTARI_WEB_AUTH_TOKEN"},
			{name: "config", env: e, where: "global config " + globalPath},
		} {
			t.Run(command+"/"+source.name, func(t *testing.T) {
				if err := os.RemoveAll(filepath.Dir(globalPath)); err != nil {
					t.Fatal(err)
				}
				if source.name == "config" {
					writeConfig(t, globalPath, "auth-token = \""+secret+"\"\n")
				}
				output := filepath.Join(e.Root, "export-"+command+"-"+source.name)
				r := source.env.Rotari(args(output)...)
				if r.Code != 0 || !strings.Contains(r.Stderr, "ignoring auth-token in "+source.where) || strings.Contains(r.Stderr+r.Stdout, secret) {
					t.Errorf("%s static export with a token in the %s: %s", command, source.name, r)
				}
				if _, err := os.Stat(filepath.Join(output, "index.html")); err != nil {
					t.Errorf("%s static export: %v", command, err)
				}
			})
		}
		if err := os.RemoveAll(filepath.Dir(globalPath)); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(e.Root, "export-"+command+"-flag")
		if r := e.Rotari(append(args(output), "--auth-token", secret)...); r.Code != 1 || !strings.Contains(r.Stderr, "--auth-token cannot be combined with --static-dir") {
			t.Errorf("%s --static-dir --auth-token must be refused: %s", command, r)
		}
	}
}
