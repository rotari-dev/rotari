package interfaces

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestBadConfiguredValuesAreErrors checks that an option value from the
// environment or a config file that the option cannot use fails the command
// with the value and its source, even where the command line gives the
// option, and that --help still shows with a warning (CLI-30, CLI-17).
func TestBadConfiguredValuesAreErrors(t *testing.T) {
	covers(t, "CLI-30", "CLI-17")
	e := support.NewEnv(t)
	workspace := filepath.Join(e.Root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	globalPath := filepath.Join(e.Root, "config", "rotari", "config.toml")
	workspacePath := filepath.Join(workspace, ".rotari.toml")
	for _, test := range []struct {
		name      string
		env       *support.Env
		global    string
		workspace string
		args      []string
		want      string
		// helpWarning is what --help warns of: a configured option value, or
		// a config that cannot load (CLI-17).
		helpWarning string
	}{
		{name: "bool from a command variable", env: e.WithVar("ROTARI_ADD_QUIET", "yes"), args: []string{"add", "--", "true"},
			want: `invalid quiet "yes" in ROTARI_ADD_QUIET`},
		{name: "bool from ROTARI_QUIET", env: e.WithVar("ROTARI_QUIET", "maybe"), args: []string{"add", "--", "true"},
			want: `invalid quiet "maybe" in ROTARI_QUIET`},
		{name: "integer from a variable, with the option given", env: e.WithVar("ROTARI_RUN_RETRY", "lots"), args: []string{"run", "--retry", "2"},
			want: `invalid retry "lots" in ROTARI_RUN_RETRY`},
		{name: "duration from a variable", env: e.WithVar("ROTARI_RUN_SLURM_SUBMIT_INTERVAL", "soon"), args: []string{"run"},
			want: `invalid slurm-submit-interval "soon" in ROTARI_RUN_SLURM_SUBMIT_INTERVAL`},
		{name: "choice from a variable", env: e.WithVar("ROTARI_EXPORT_FORMAT", "xml"), args: []string{"export", "--output", "-"},
			want: `invalid format "xml" in ROTARI_EXPORT_FORMAT: choose from yaml, toml, json`},
		{name: "integer with a fraction in the global config", env: e, global: "retry = 1.5\n", args: []string{"run"},
			want: "invalid retry 1.5 in global config " + globalPath},
		{name: "bool text in a command section", env: e, global: "[add]\nquiet = \"maybe\"\n", args: []string{"add", "--", "true"},
			want: `invalid quiet "maybe" in global config`},
		{name: "duration in the workspace config", env: e.In(workspace), workspace: "[run]\nslurm-submit-interval = \"soon\"\n", args: []string{"run"},
			want: `invalid slurm-submit-interval "soon" in workspace config ` + workspacePath},
		{name: "choice in the global config", env: e, global: "[wait]\ndisconnect-action = \"explode\"\n", args: []string{"wait"},
			want: `invalid disconnect-action "explode" in global config`},
		{name: "table for a repeated option", env: e, global: "[add]\nenv = {A = 1}\n", args: []string{"add", "--", "true"},
			want: `invalid env {"A":1} in global config`},
		{name: "list for the basedir", env: e.Without("ROTARI_BASEDIR"), global: "basedir = [\"a\", \"b\"]\n", args: []string{"check"},
			want: `invalid basedir ["a","b"] in global config`, helpWarning: "WARNING: failed to load config"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, path := range []string{globalPath, workspacePath} {
				if err := os.RemoveAll(path); err != nil {
					t.Fatal(err)
				}
			}
			for path, content := range map[string]string{globalPath: test.global, workspacePath: test.workspace} {
				if content == "" {
					continue
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := test.env.Rotari(test.args...)
			if got.Code == 0 || !strings.Contains(got.Stderr, test.want) || strings.Contains(got.Stdout, "added") {
				t.Errorf("want a failure naming %q: %s", test.want, got)
			}
			help := test.env.Rotari(test.args[0], "--help")
			warning := test.helpWarning
			if warning == "" {
				warning = "WARNING: " + test.want
			}
			if help.Code != 0 || !strings.Contains(help.Stdout, "usage: rotari ") || !strings.Contains(help.Stderr, warning) {
				t.Errorf("help must show with a warning %q: %s", warning, help)
			}
		})
	}
	for _, path := range []string{globalPath, workspacePath} {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}

	// A bad value of an option the command does not have is not its problem.
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("retry = \"lots\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := e.Rotari("projects"); got.Code != 0 || got.Stderr != "" {
		t.Errorf("projects reads no retry: %s", got)
	}
	if err := os.RemoveAll(globalPath); err != nil {
		t.Fatal(err)
	}

	// An empty variable leaves a typed option unset.
	if got := e.WithVar("ROTARI_RUN_RETRY", "").Rotari("run", "--help"); got.Code != 0 || got.Stderr != "" {
		t.Errorf("an empty ROTARI_RUN_RETRY must count as unset: %s", got)
	}
}
