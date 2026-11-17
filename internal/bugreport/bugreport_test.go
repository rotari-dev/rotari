package bugreport

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/state"
)

const (
	fixtureRunID      = "20261014-090000-0000abcd"
	fixtureWebhookURL = "https://hooks.example.com/services/T000/B000/secret-webhook"
	fixtureAuthToken  = "web-auth-token-value"
	fixtureJobSecret  = "job-secret-value"
	fixtureEnvToken   = "environment-token-value"
)

type runFixture struct {
	baseDir string
	runDir  string
}

func testTime() time.Time {
	return time.Date(2026, time.October, 14, 9, 0, 0, 0, time.UTC)
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// newRunFixture builds a project "demo" whose run has a failed job "bad"
// (exit 3, a secret environment value, and output) and a successful job
// "good", with a webhook URL and a web auth token in configuration.
func newRunFixture(t *testing.T) runFixture {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	baseDir := filepath.Join(t.TempDir(), "base")
	projectDir := filepath.Join(baseDir, "projects", "demo")
	runDir := filepath.Join(projectDir, "runs", fixtureRunID)
	writeFixtureFile(t, filepath.Join(baseDir, "notifications.toml"), "[webhook]\nurl = \""+fixtureWebhookURL+"\"\n")
	writeFixtureFile(t, filepath.Join(projectDir, "config.toml"), "[web]\nauth-token = \""+fixtureAuthToken+"\"\nport = 8080\n")
	commands := model.Queue{Commands: []model.QueuedCommand{
		{ID: "bad", Command: []string{"train"}, Environment: []string{"API_KEY_VALUE=" + fixtureJobSecret, "ROTARI_EXECUTOR=slurm"}},
		{ID: "good", Command: []string{"true"}},
	}}
	if err := state.WriteJSON(filepath.Join(runDir, "commands.json"), commands); err != nil {
		t.Fatal(err)
	}
	badAttempt := state.MakeAttemptID(fixtureRunID, "bad", 0)
	goodAttempt := state.MakeAttemptID(fixtureRunID, "good", 0)
	summary := model.RunSummary{RunID: fixtureRunID, Status: "failed", ExitCode: 1, Results: []model.JobResult{
		{ID: "bad", AttemptID: badAttempt, ExitCode: 3, Error: "exit status 3", Hosts: []string{"node-17"}},
		{ID: "good", AttemptID: goodAttempt, ExitCode: 0},
	}}
	if err := state.WriteJSON(filepath.Join(runDir, "summary.json"), summary); err != nil {
		t.Fatal(err)
	}
	badDir := filepath.Join(runDir, "bad", "attempts", badAttempt)
	if err := state.WriteJSON(filepath.Join(badDir, "command.json"), model.JobSpec{ID: "bad", AttemptID: badAttempt, Command: []string{"train"}, Executor: "slurm", Environment: commands.Commands[0].Environment}); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(badDir, "job.json"), `{"executor":"slurm","job_id":"bad","slurm_job_id":"4242"}`)
	writeFixtureFile(t, filepath.Join(badDir, state.SchedulerEventsFileName), `{"at":"2026-10-14T09:00:00Z","query":"queue","error":"exit status 1: Socket timed out"}`+"\n"+`{"at":"2026-10-14T09:00:00Z","query":"accounting","state":"running"}`+"\n")
	writeFixtureFile(t, filepath.Join(badDir, "output"), strings.Repeat("x", outputTailBytes)+"\nTraceback: boom in "+baseDir+"\n")
	writeFixtureFile(t, filepath.Join(runDir, "good", "attempts", goodAttempt, "output"), "fine\n")
	return runFixture{baseDir: baseDir, runDir: runDir}
}

func collectFixture(t *testing.T, fixture runFixture, mutate func(*Options)) Bundle {
	t.Helper()
	options := Options{
		Version: "rotari test", BaseDir: fixture.baseDir, Project: "demo", RunID: fixtureRunID, Redact: true,
		Environ: []string{"ROTARI_WEB_AUTH_TOKEN=" + fixtureEnvToken, "ROTARI_PROJECT_NAME=demo", "HOME=/home/someone"},
		Facts:   []File{{Name: "info.json", Data: []byte(`{"basedir":"` + fixture.baseDir + `","auth_token":"` + fixtureAuthToken + `"}`)}},
	}
	if mutate != nil {
		mutate(&options)
	}
	bundle, err := Collect(options)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func bundleFile(bundle Bundle, suffix string) (File, bool) {
	for _, file := range bundle.Files {
		if strings.HasSuffix(file.Name, suffix) {
			return file, true
		}
	}
	return File{}, false
}

func TestCollectNeverIncludesSecrets(t *testing.T) {
	fixture := newRunFixture(t)
	t.Setenv("PATH", t.TempDir())
	for _, redact := range []bool{true, false} {
		bundle := collectFixture(t, fixture, func(options *Options) { options.Redact = redact })
		for _, secret := range []string{fixtureWebhookURL, fixtureAuthToken, fixtureJobSecret, fixtureEnvToken} {
			if strings.Contains(bundle.Summary, secret) {
				t.Fatalf("redact=%v: summary contains %q", redact, secret)
			}
			for _, file := range bundle.Files {
				if strings.Contains(string(file.Data), secret) {
					t.Fatalf("redact=%v: %s contains %q", redact, file.Name, secret)
				}
			}
		}
		command, ok := bundleFile(bundle, "/bad/"+state.MakeAttemptID(fixtureRunID, "bad", 0)+"/command.json")
		if !ok || !strings.Contains(string(command.Data), "API_KEY_VALUE=[REDACTED]") || !strings.Contains(string(command.Data), "ROTARI_EXECUTOR=slurm") {
			t.Fatalf("redact=%v: command.json = %q, want the job secret replaced and rotari's own value kept", redact, command.Data)
		}
		if !strings.Contains(bundle.Summary, "ROTARI_PROJECT_NAME=demo") || !strings.Contains(bundle.Summary, "ROTARI_WEB_AUTH_TOKEN=[REDACTED]") {
			t.Fatalf("redact=%v: summary environment section is wrong:\n%s", redact, bundle.Summary)
		}
	}
}

func TestCollectRedactsPathsAndHostsUnlessAskedNot(t *testing.T) {
	fixture := newRunFixture(t)
	t.Setenv("PATH", t.TempDir())
	redactedBundle := collectFixture(t, fixture, nil)
	kept := collectFixture(t, fixture, func(options *Options) { options.Redact = false })
	for _, text := range []string{fixture.baseDir, "node-17"} {
		if strings.Contains(redactedBundle.Summary, text) {
			t.Fatalf("redacted summary contains %q", text)
		}
		for _, file := range redactedBundle.Files {
			if strings.Contains(string(file.Data), text) {
				t.Fatalf("redacted %s contains %q", file.Name, text)
			}
		}
	}
	if !strings.Contains(kept.Summary, fixture.baseDir) {
		t.Fatalf("summary with redaction off lacks the basedir %q", fixture.baseDir)
	}
}

func TestCollectSelectsJobsAndOutput(t *testing.T) {
	fixture := newRunFixture(t)
	t.Setenv("PATH", t.TempDir())
	badOutput := "/bad/" + state.MakeAttemptID(fixtureRunID, "bad", 0) + "/output"
	goodOutput := "/good/" + state.MakeAttemptID(fixtureRunID, "good", 0) + "/output"

	// Path redaction shortens the text, so sizes are checked without it.
	keepPaths := func(options *Options) { options.Redact = false }
	tail := collectFixture(t, fixture, keepPaths)
	output, ok := bundleFile(tail, badOutput)
	if !ok || len(output.Data) != outputTailBytes || !strings.Contains(string(output.Data), "Traceback: boom") {
		t.Fatalf("tail output = %d bytes, ok=%v; want the last %d bytes with the traceback", len(output.Data), ok, outputTailBytes)
	}
	if _, ok := bundleFile(tail, goodOutput); ok {
		t.Fatal("a successful job was included without --job-id")
	}
	if !strings.Contains(tail.Summary, "Traceback: boom") || !strings.Contains(tail.Summary, "Socket timed out") {
		t.Fatalf("summary lacks the output tail or the scheduler events:\n%s", tail.Summary)
	}

	full := collectFixture(t, fixture, func(options *Options) { keepPaths(options); options.Output = OutputFull })
	if output, ok := bundleFile(full, badOutput); !ok || len(output.Data) <= outputTailBytes {
		t.Fatalf("full output = %d bytes, want the whole file", len(output.Data))
	}
	none := collectFixture(t, fixture, func(options *Options) { options.Output = OutputNone })
	if _, ok := bundleFile(none, badOutput); ok {
		t.Fatal("output was included with OutputNone")
	}

	writeFixtureFile(t, filepath.Join(fixture.runDir, "bad", "cancelled"), "2026-10-14T09:00:00Z\n")
	if _, ok := bundleFile(collectFixture(t, fixture, nil), "/bad/job/cancelled"); !ok {
		t.Fatal("the cancel recorded before the job's first attempt was not included")
	}

	named := collectFixture(t, fixture, func(options *Options) {
		options.JobIDs = []string{state.MakeAttemptID(fixtureRunID, "good", 0)}
	})
	if _, ok := bundleFile(named, goodOutput); !ok {
		t.Fatal("the job named by its attempt ID was not included")
	}
	if _, ok := bundleFile(named, badOutput); ok {
		t.Fatal("a job not named was included")
	}
	if _, err := Collect(Options{BaseDir: fixture.baseDir, Project: "demo", RunID: fixtureRunID, JobIDs: []string{"missing"}}); err == nil || !strings.Contains(err.Error(), `job "missing" not found`) {
		t.Fatalf("Collect with an unknown job = %v, want not found", err)
	}
}

func TestCollectRunsSchedulerQueriesForSchedulerJobs(t *testing.T) {
	fixture := newRunFixture(t)
	binDir := t.TempDir()
	writeFixtureFile(t, filepath.Join(binDir, "squeue"), "#!/bin/sh\necho 'slurm_load_jobs error: Socket timed out' >&2\nexit 1\n")
	writeFixtureFile(t, filepath.Join(binDir, "sacct"), "#!/bin/sh\necho 'RUNNING|0:0'\n")
	for _, name := range []string{"squeue", "sacct"} {
		if err := os.Chmod(filepath.Join(binDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir)
	bundle := collectFixture(t, fixture, nil)
	probe, ok := bundleFile(bundle, "/scheduler_probe.txt")
	if !ok {
		t.Fatal("no scheduler_probe.txt for the Slurm job")
	}
	for _, want := range []string{"$ squeue --noheader --jobs 4242", "Socket timed out", "$ sacct", "RUNNING|0:0"} {
		if !strings.Contains(string(probe.Data), want) || !strings.Contains(bundle.Summary, want) {
			t.Fatalf("probe or summary lacks %q:\n%s", want, probe.Data)
		}
	}
}

func TestCollectWithoutRunReportsEnvironment(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	baseDir := t.TempDir()
	bundle, err := Collect(Options{Version: "rotari test", BaseDir: baseDir})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Version: rotari test", "Basedir filesystem:", "Basedir clock:", "No server.log"} {
		if !strings.Contains(bundle.Summary, want) {
			t.Fatalf("summary lacks %q:\n%s", want, bundle.Summary)
		}
	}
	if entries, _ := os.ReadDir(baseDir); len(entries) != 0 {
		t.Fatalf("basedir holds %v after the clock probe, want nothing left", entries)
	}
}

func TestCollectIncludesWebAndExecutableEvidenceWithoutRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	baseDir := t.TempDir()
	otherBase := filepath.Join(t.TempDir(), "registered")
	webLog := filepath.Join(t.TempDir(), "web-events.jsonl")
	writeFixtureFile(t, webLog+".1", `{"time":"2026-11-05T11:00:00Z","event":"sign_in","pid":122,"command":"web","outcome":"wrong_credentials","user_agent":"rotated-marker"}`+"\n")
	writeFixtureFile(t, webLog, `{"time":"2026-11-05T12:00:00Z","event":"server_start","pid":123,"command":"web","host":"web-host-marker","port":8765,"executable_sha256":"server-hash","forged_secret":"forged-secret-value"}`+"\n")

	bundle, err := Collect(Options{
		Version: "rotari test", BaseDir: baseDir,
		ExecutableSHA256: "report-hash", ExecutableModified: testTime(),
		WebLogPath: webLog, RegisteredBaseDirs: []string{baseDir, otherBase}, RegisteredBaseDirsKnown: true,
		Redact: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bundleFile(bundle, "web-events.jsonl"); !ok {
		t.Fatalf("bundle files = %#v, want web-events.jsonl", bundle.Files)
	}
	for _, want := range []string{"Executable SHA-256: report-hash", "Executable modified: 2026-10-14T09:00:00Z", "## Web events", "server_start", "rotated-marker", "## Registered basedirs", "exists", "missing"} {
		if !strings.Contains(bundle.Summary, want) {
			t.Errorf("summary lacks %q:\n%s", want, bundle.Summary)
		}
	}
	for _, forbidden := range []string{baseDir, otherBase, "web-host-marker", "forged-secret-value"} {
		if strings.Contains(bundle.Summary, forbidden) {
			t.Errorf("redacted summary contains %q", forbidden)
		}
	}
}

func TestCollectNotesMissingAndUnreadableWebEventLog(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, test := range []struct {
		name, path, want string
		prepare          func(string) error
	}{
		{name: "missing", path: filepath.Join(t.TempDir(), "missing.jsonl"), want: "does not exist yet"},
		{name: "unreadable", path: filepath.Join(t.TempDir(), "directory"), want: "not read", prepare: func(path string) error { return os.Mkdir(path, 0o700) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.prepare != nil {
				if err := test.prepare(test.path); err != nil {
					t.Fatal(err)
				}
			}
			bundle, err := Collect(Options{Version: "rotari test", BaseDir: t.TempDir(), WebLogPath: test.path, RegisteredBaseDirsKnown: true})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(bundle.Summary, test.want) {
				t.Fatalf("summary lacks %q:\n%s", test.want, bundle.Summary)
			}
		})
	}
}

func TestWriteTarGz(t *testing.T) {
	target := filepath.Join(t.TempDir(), "report.tar.gz")
	bundle := Bundle{Summary: "# summary\n", Files: []File{{Name: "run/r/summary.json", Data: []byte("{}\n")}}}
	if err := WriteTarGz(target, "root", bundle, testTime()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("bundle mode = %v, %v; want 0600", info, err)
	}
	file, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewReader(compressed)
	var names []string
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, header.Name)
	}
	if strings.Join(names, ",") != "root/README.md,root/run/r/summary.json" {
		t.Fatalf("entries = %v", names)
	}
	if err := WriteTarGz(target, "root", bundle, testTime()); err == nil {
		t.Fatal("WriteTarGz replaced an existing file")
	}
	escaping := filepath.Join(t.TempDir(), "escape.tar.gz")
	if err := WriteTarGz(escaping, "root", Bundle{Files: []File{{Name: "../outside", Data: nil}}}, testTime()); err == nil {
		t.Fatal("WriteTarGz accepted an entry outside the root")
	}
	if _, err := os.Stat(escaping); !os.IsNotExist(err) {
		t.Fatalf("a failed bundle was left behind: %v", err)
	}
}

// Path redaction replaces paths and nothing else: a JSON escape such as \n
// after a word ending in a letter and a colon is not a Windows drive path,
// and a Windows path inside JSON, where its backslashes are escaped, is
// still redacted, as one in plain output is.
func TestCollectRedactsOnlyPaths(t *testing.T) {
	fixture := newRunFixture(t)
	t.Setenv("PATH", t.TempDir())
	writeFixtureFile(t, filepath.Join(fixture.runDir, state.ProgressFileName),
		`{"at":"2026-10-14T09:00:00Z","message":"Job running:\n  ID: bad\nShow:\n  rotari show"}`+"\n"+
			`{"at":"2026-10-14T09:00:01Z","message":"cwd C:\\Users\\alice\\proj"}`+"\n")
	outputs, err := filepath.Glob(filepath.Join(fixture.runDir, "bad", "attempts", "*", "output"))
	if err != nil || len(outputs) != 1 {
		t.Fatalf("bad output: %q, %v", outputs, err)
	}
	writeFixtureFile(t, outputs[0], "Traceback: boom\nopened D:\\data\\bob\\input.csv\nStep A: done\n")
	bundle := collectFixture(t, fixture, func(options *Options) {
		options.Facts = append(options.Facts, File{Name: "windows.json", Data: []byte(`{"cwd":"E:\\work\\carol","label":"Phase:\nB"}`)})
	})

	progress, ok := bundleFile(bundle, state.ProgressFileName)
	if !ok {
		t.Fatal("progress.jsonl was not collected")
	}
	var messages []string
	for _, line := range strings.Split(strings.TrimSpace(string(progress.Data)), "\n") {
		var event struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("progress line %q: %v", line, err)
		}
		messages = append(messages, event.Message)
	}
	if want := []string{"Job running:\n  ID: bad\nShow:\n  rotari show", "cwd [REDACTED_PATH]"}; !slices.Equal(messages, want) {
		t.Fatalf("progress messages = %q, want %q", messages, want)
	}
	output, ok := bundleFile(bundle, "/output")
	if !ok || !strings.Contains(string(output.Data), "opened [REDACTED_PATH]\nStep A: done") {
		t.Fatalf("output = %q, want the Windows path redacted and the rest kept", output.Data)
	}
	fact, ok := bundleFile(bundle, "windows.json")
	var values map[string]string
	if !ok || json.Unmarshal(fact.Data, &values) != nil || values["cwd"] != "[REDACTED_PATH]" || values["label"] != "Phase:\nB" {
		t.Fatalf("windows.json = %s, want its path redacted and its label kept", fact.Data)
	}
	for _, text := range []string{"alice", "bob", "carol", "runnin[", "Sho["} {
		if strings.Contains(bundle.Summary, text) {
			t.Fatalf("summary contains %q", text)
		}
		for _, file := range bundle.Files {
			if strings.Contains(string(file.Data), text) {
				t.Fatalf("%s contains %q:\n%s", file.Name, text, file.Data)
			}
		}
	}
}
