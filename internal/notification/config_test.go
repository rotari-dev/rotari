package notification

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestParseAppliesDefaultsAndOverrides(t *testing.T) {
	settings, err := Parse([]byte("[webhook]\njob_success = true\nfields = []\n\n[browser]\nrun_success = false\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Webhook.JobFailure || !settings.Webhook.JobSuccess || len(settings.Webhook.Fields) != 0 {
		t.Fatalf("webhook settings = %#v", settings.Webhook)
	}
	if settings.Browser.RunSuccess || !settings.Browser.RunFailure {
		t.Fatalf("browser settings = %#v", settings.Browser)
	}
}

func TestParseRejectsUnknownSettingsAndFields(t *testing.T) {
	for _, data := range []string{
		"[browser]\nunknown = true\n",
		"[browser]\nfields = [\"unknown\"]\n",
		"[webhook]\nweb_url = \"javascript:alert(1)\"\n",
		"[webhook]\nweb_url = \"https://user:pass@example.invalid/rotari\"\n",
	} {
		if _, err := Parse([]byte(data)); err == nil {
			t.Fatalf("Parse(%q) succeeded", data)
		}
	}
}

func TestLoadUsesOnlyHighestPriorityFile(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	baseDir := t.TempDir()
	projectDir := filepath.Join(baseDir, "projects", "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(configHome, "rotari", FileName), "[webhook]\nurl = 'https://example.invalid/global'\n[browser]\nmax_jobs = 1\n")
	write(filepath.Join(baseDir, FileName), "[browser]\nmax_jobs = 2\n")
	write(filepath.Join(projectDir, FileName), "[browser]\nmax_jobs = 3\n")

	loaded, err := Load(baseDir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Path != filepath.Join(projectDir, FileName) || loaded.Settings.Browser.MaxJobs != 3 {
		t.Fatalf("loaded = %#v", loaded)
	}
	if loaded.Settings.Webhook.URL != "" {
		t.Fatal("project notifications inherited global webhook URL")
	}
}

func TestTemplateParses(t *testing.T) {
	if _, err := Parse(Template()); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultNotificationFieldsIncludeRunAndAttemptIDs(t *testing.T) {
	settings := Defaults()
	for channel, fields := range map[string][]string{
		"webhook": settings.Webhook.Fields,
		"browser": settings.Browser.Fields,
	} {
		selected := make(map[string]bool, len(fields))
		for _, field := range fields {
			selected[field] = true
		}
		for _, field := range []string{"run_id", "attempt_id"} {
			if !selected[field] {
				t.Errorf("%s default fields do not include %q: %v", channel, field, fields)
			}
		}
		if !selected["job_name"] {
			t.Errorf("%s default fields do not include job_name: %v", channel, fields)
		}
		if selected["job_id"] {
			t.Errorf("%s default fields unexpectedly include job_id: %v", channel, fields)
		}
		if channel == "webhook" && !selected["link"] {
			t.Errorf("%s default fields do not include link: %v", channel, fields)
		}
		position := make(map[string]int, len(fields))
		for index, field := range fields {
			position[field] = index
		}
		for _, name := range []string{"project", "job_name"} {
			for _, id := range []string{"run_id", "attempt_id"} {
				if position[name] > position[id] {
					t.Errorf("%s default fields list %q after %q: %v", channel, name, id, fields)
				}
			}
		}
		available := AvailableFields()
		ordered := slices.DeleteFunc(slices.Clone(available), func(field string) bool { return !selected[field] })
		if !slices.Equal(ordered, fields) {
			t.Errorf("%s default fields %v do not follow the available field order %v", channel, fields, available)
		}
	}
}

func TestMarshalRoundTripsSettings(t *testing.T) {
	want := Defaults()
	want.Webhook.URL = "https://example.invalid/hook?token=secret"
	want.Webhook.WebURL = "https://rotari.example.invalid/rotari"
	want.Browser.JobSuccess = true
	data, err := Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Webhook.URL != want.Webhook.URL || got.Webhook.WebURL != want.Webhook.WebURL || !got.Browser.JobSuccess {
		t.Fatalf("settings = %#v", got)
	}
}

// The [terminal] section: its defaults, its keys, its check, and its round trip.
func TestTerminalSettings(t *testing.T) {
	if got := Defaults().Terminal; got != (TerminalSettings{CommandFailure: true, CommandSuccess: true, MinSeconds: 30}) {
		t.Fatalf("defaults %+v", got)
	}
	settings, err := Parse([]byte("[terminal]\ncommand_success = false\nmin_seconds = 5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if settings.Terminal != (TerminalSettings{CommandFailure: true, CommandSuccess: false, MinSeconds: 5}) {
		t.Fatalf("parsed %+v", settings.Terminal)
	}
	for _, bad := range []string{"[terminal]\nmin_seconds = -1\n", "[terminal]\nevery = true\n"} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	data, err := Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(data)
	if err != nil || again.Terminal != settings.Terminal {
		t.Fatalf("round trip %+v, %v:\n%s", again.Terminal, err, data)
	}
	if !bytes.Contains(Template(), []byte("[terminal]\ncommand_failure = true\ncommand_success = true\nmin_seconds = 30\n")) {
		t.Errorf("the template has no [terminal]:\n%s", Template())
	}
	zero, one := 0, 1
	for _, test := range []struct {
		settings TerminalSettings
		exit     *int
		took     time.Duration
		want     bool
	}{
		{Defaults().Terminal, &one, 30 * time.Second, true},
		{Defaults().Terminal, &zero, 30 * time.Second, true},
		{Defaults().Terminal, nil, time.Minute, true},
		{Defaults().Terminal, &one, 29 * time.Second, false},
		{TerminalSettings{CommandFailure: true, MinSeconds: 0}, &zero, time.Hour, false},
		{TerminalSettings{CommandSuccess: true}, nil, time.Hour, false},
	} {
		if got := test.settings.Notifies(test.exit, test.took); got != test.want {
			t.Errorf("%+v, exit %v, after %v: %v", test.settings, test.exit, test.took, got)
		}
	}
}

// With no basedir, as for a Web terminal shell of no project, only the global file counts.
func TestLoadWithNoBasedirReadsTheGlobalFile(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if err := os.MkdirAll(filepath.Join(configHome, "rotari"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configHome, "rotari", FileName), []byte("[terminal]\nmin_seconds = 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A notifications.toml where the server runs is no basedir's.
	t.Chdir(t.TempDir())
	if err := os.WriteFile(FileName, []byte("[terminal]\nmin_seconds = 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load("", "")
	if err != nil || loaded.Settings.Terminal.MinSeconds != 7 {
		t.Fatalf("loaded %+v, %v", loaded, err)
	}
}
