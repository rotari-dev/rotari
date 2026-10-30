package notification

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/rotari-dev/rotari/internal/config"
	"github.com/rotari-dev/rotari/internal/state"
)

const FileName = "notifications.toml"

// fieldOrder lists every field from most to least useful at a glance: what
// happened to which job first, then run context and details, and opaque IDs
// last. Defaults follow it, and the Web UI editor offers fields in this order.
var fieldOrder = []string{
	"project", "job_name", "job_status", "exit_code", "error",
	"diagnosis_name", "diagnosis_suggestion", "diagnosis_evidence",
	"diagnosis_status", "diagnosis_rules", "diagnosis_outdated",
	"run_name", "run_status", "success_count", "failure_count", "total_count",
	"duration", "started_at", "finished_at", "stage", "array_task_id",
	"executor", "hosts", "working_directory", "command",
	"run_id", "job_id", "attempt_id", "link",
}

var fields = func() map[string]bool {
	known := make(map[string]bool, len(fieldOrder))
	for _, name := range fieldOrder {
		known[name] = true
	}
	return known
}()

// AvailableFields returns every field in its default display order.
func AvailableFields() []string {
	return slices.Clone(fieldOrder)
}

type ChannelSettings struct {
	JobFailure bool     `json:"job_failure"`
	JobSuccess bool     `json:"job_success"`
	RunFailure bool     `json:"run_failure"`
	RunSuccess bool     `json:"run_success"`
	Fields     []string `json:"fields"`
	MaxJobs    int      `json:"max_jobs"`
}

type WebhookSettings struct {
	ChannelSettings
	URL    string `json:"url,omitempty"`
	WebURL string `json:"web_url,omitempty"`
	Format string `json:"format"`
}

type Settings struct {
	Webhook WebhookSettings `json:"webhook"`
	Browser ChannelSettings `json:"browser"`
}

type Loaded struct {
	Settings Settings
	Path     string
}

type rawChannel struct {
	JobFailure *bool    `toml:"job_failure"`
	JobSuccess *bool    `toml:"job_success"`
	RunFailure *bool    `toml:"run_failure"`
	RunSuccess *bool    `toml:"run_success"`
	Fields     []string `toml:"fields"`
	MaxJobs    *int     `toml:"max_jobs"`
}

type rawWebhook struct {
	rawChannel
	URL    *string `toml:"url"`
	WebURL *string `toml:"web_url"`
	Format *string `toml:"format"`
}

type rawSettings struct {
	Webhook rawWebhook `toml:"webhook"`
	Browser rawChannel `toml:"browser"`
}

func Defaults() Settings {
	return Settings{
		Webhook: WebhookSettings{
			ChannelSettings: ChannelSettings{
				JobFailure: true, RunFailure: true, RunSuccess: true, MaxJobs: 20,
				Fields: []string{"project", "job_name", "job_status", "exit_code", "diagnosis_name", "diagnosis_suggestion", "run_name", "run_status", "success_count", "failure_count", "duration", "run_id", "attempt_id", "link"},
			},
			Format: "json",
		},
		Browser: ChannelSettings{
			JobFailure: true, RunFailure: true, RunSuccess: true, MaxJobs: 10,
			Fields: []string{"project", "job_name", "job_status", "exit_code", "diagnosis_name", "diagnosis_suggestion", "run_name", "run_status", "run_id", "attempt_id", "link"},
		},
	}
}

func Parse(data []byte) (Settings, error) {
	var raw rawSettings
	metadata, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&raw)
	if err != nil {
		return Settings{}, err
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		return Settings{}, fmt.Errorf("unknown notification setting %q", undecoded[0].String())
	}
	settings := Defaults()
	applyChannel(&settings.Webhook.ChannelSettings, raw.Webhook.rawChannel)
	applyChannel(&settings.Browser, raw.Browser)
	if raw.Webhook.URL != nil {
		settings.Webhook.URL = *raw.Webhook.URL
	}
	if raw.Webhook.WebURL != nil {
		settings.Webhook.WebURL = *raw.Webhook.WebURL
	}
	if raw.Webhook.Format != nil {
		settings.Webhook.Format = *raw.Webhook.Format
	}
	if err := validate(settings); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func Load(baseDir, projectName string) (Loaded, error) {
	paths := make([]string, 0, 3)
	if projectName != "" {
		projectDir, err := state.SafeJoin(filepath.Join(baseDir, "projects"), projectName)
		if err != nil {
			return Loaded{}, err
		}
		paths = append(paths, filepath.Join(projectDir, FileName))
	}
	paths = append(paths, filepath.Join(baseDir, FileName))
	configHome, err := config.HomeDir()
	if err != nil {
		return Loaded{}, err
	}
	paths = append(paths, filepath.Join(configHome, FileName))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Loaded{}, err
		}
		settings, err := Parse(data)
		if err != nil {
			return Loaded{}, fmt.Errorf("parse %s: %w", path, err)
		}
		return Loaded{Settings: settings, Path: path}, nil
	}
	return Loaded{Settings: Defaults()}, nil
}

func applyChannel(settings *ChannelSettings, raw rawChannel) {
	if raw.JobFailure != nil {
		settings.JobFailure = *raw.JobFailure
	}
	if raw.JobSuccess != nil {
		settings.JobSuccess = *raw.JobSuccess
	}
	if raw.RunFailure != nil {
		settings.RunFailure = *raw.RunFailure
	}
	if raw.RunSuccess != nil {
		settings.RunSuccess = *raw.RunSuccess
	}
	if raw.Fields != nil {
		settings.Fields = raw.Fields
	}
	if raw.MaxJobs != nil {
		settings.MaxJobs = *raw.MaxJobs
	}
}

func validate(settings Settings) error {
	if settings.Webhook.Format != "json" && settings.Webhook.Format != "slack" && settings.Webhook.Format != "teams" && settings.Webhook.Format != "discord" {
		return fmt.Errorf("invalid webhook format %q", settings.Webhook.Format)
	}
	if settings.Webhook.WebURL != "" {
		parsed, err := url.Parse(settings.Webhook.WebURL)
		if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("webhook web_url must be an absolute HTTP or HTTPS URL without credentials, query, or fragment")
		}
	}
	for name, channel := range map[string]ChannelSettings{"webhook": settings.Webhook.ChannelSettings, "browser": settings.Browser} {
		if err := validateChannel(name, channel); err != nil {
			return err
		}
	}
	return nil
}

func validateChannel(name string, channel ChannelSettings) error {
	if channel.MaxJobs < 1 {
		return fmt.Errorf("%s max_jobs must be positive", name)
	}
	seen := make(map[string]bool)
	for _, field := range channel.Fields {
		if !fields[field] {
			return fmt.Errorf("unknown %s field %q", name, field)
		}
		if seen[field] {
			return fmt.Errorf("duplicate %s field %q", name, field)
		}
		seen[field] = true
	}
	return nil
}

func Validate(settings Settings) error {
	return validate(settings)
}

func Marshal(settings Settings) ([]byte, error) {
	if err := validate(settings); err != nil {
		return nil, err
	}
	var output strings.Builder
	writeChannel := func(channel ChannelSettings) {
		fmt.Fprintf(&output, "job_failure = %t\njob_success = %t\nrun_failure = %t\nrun_success = %t\n", channel.JobFailure, channel.JobSuccess, channel.RunFailure, channel.RunSuccess)
		output.WriteString("fields = [")
		for index, field := range channel.Fields {
			if index > 0 {
				output.WriteString(", ")
			}
			output.WriteString(strconv.Quote(field))
		}
		fmt.Fprintf(&output, "]\nmax_jobs = %d\n", channel.MaxJobs)
	}
	output.WriteString("[webhook]\nurl = ")
	output.WriteString(strconv.Quote(settings.Webhook.URL))
	output.WriteString("\nformat = ")
	output.WriteString(strconv.Quote(settings.Webhook.Format))
	output.WriteString("\nweb_url = ")
	output.WriteString(strconv.Quote(settings.Webhook.WebURL))
	output.WriteByte('\n')
	writeChannel(settings.Webhook.ChannelSettings)
	output.WriteString("\n[browser]\n")
	writeChannel(settings.Browser)
	return []byte(output.String()), nil
}

func Template() []byte {
	data, _ := Marshal(Defaults())
	return data
}
