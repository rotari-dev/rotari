package interfaces

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

// TestBugReportNeverIncludesSecrets runs a failing job with a secret in its
// environment, a webhook URL in notifications.toml, and a Web auth token in
// the caller's environment, then checks the Markdown summary and every file
// of the bundle, with and without --no-redact.
func TestBugReportNeverIncludesSecrets(t *testing.T) {
	covers(t, "CLI-26", "WEB-35")
	const (
		webhook    = "https://hooks.example.com/services/conformance-webhook"
		token      = "conformance-web-token"
		jobSecret  = "conformance-job-secret"
		wrong      = "conformance-wrong-password"
		nextMarker = "/conformance-private-next"
		bearer     = "conformance-wrong-bearer"
	)
	e := support.NewEnv(t)
	e.MustRotari("add", "-p", "secrets", "--env", "API_TOKEN_VALUE="+jobSecret, "--", "sh", "-c", "echo failing-output; exit 3")
	if run := e.RotariRun("run", "-p", "secrets"); run.Code != 1 {
		t.Fatalf("run of a failing job: %s", run)
	}
	if err := os.WriteFile(filepath.Join(e.Base, "notifications.toml"), []byte("[webhook]\nurl = \""+webhook+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.Setenv("ROTARI_WEB_AUTH_TOKEN", token)
	webAddress := e.StartWeb()
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	form := url.Values{"username": {token}, "password": {wrong}, "next": {nextMarker}}
	request, err := http.NewRequest(http.MethodPost, webAddress+"/_auth/login?query-secret", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", webAddress)
	request.Header.Set("User-Agent", "conformance-browser")
	if response, err := client.Do(request); err != nil {
		t.Fatal(err)
	} else {
		_ = response.Body.Close()
	}
	form = url.Values{"username": {"rotari"}, "password": {token}, "next": {nextMarker}}
	request, _ = http.NewRequest(http.MethodPost, webAddress+"/_auth/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", webAddress)
	request.Header.Set("User-Agent", "conformance-browser")
	if response, err := client.Do(request); err != nil {
		t.Fatal(err)
	} else {
		_ = response.Body.Close()
	}
	request, _ = http.NewRequest(http.MethodGet, webAddress+"/api/state", nil)
	request.Header.Set("Authorization", "Bearer "+bearer)
	if response, err := client.Do(request); err != nil {
		t.Fatal(err)
	} else {
		_ = response.Body.Close()
	}
	tablePath := filepath.Join(e.Root, "evidence.csv")
	if err := os.WriteFile(tablePath, []byte("value\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tableAddress := e.StartTableView(tablePath)
	form = url.Values{"username": {token}, "password": {wrong}, "next": {nextMarker}}
	request, _ = http.NewRequest(http.MethodPost, tableAddress+"/_auth/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", tableAddress)
	request.Header.Set("User-Agent", "conformance-browser")
	if response, err := client.Do(request); err != nil {
		t.Fatal(err)
	} else {
		_ = response.Body.Close()
	}
	webLog := filepath.Join(e.Master, "web-events.jsonl")
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, readErr := os.ReadFile(webLog)
		if readErr == nil && strings.Contains(string(data), `"event":"credential_rejected"`) && strings.Contains(string(data), `"command":"tableview"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Web evidence not written to %s: %v, %s", webLog, readErr, data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	secrets := []string{webhook, token, jobSecret, wrong, nextMarker, bearer, "query-secret"}
	assertNoSecrets(t, "web event log", string(mustReadFile(t, webLog)), secrets)

	for _, variant := range []struct {
		name  string
		extra []string
	}{{name: "redacted"}, {name: "no-redact", extra: []string{"--no-redact"}}} {
		t.Run(variant.name, func(t *testing.T) {
			summary := e.MustRotari(append([]string{"bug-report", "-p", "secrets"}, variant.extra...)...).Stdout
			assertNoSecrets(t, "summary", summary, secrets)
			for _, want := range []string{"## Run ", "failing-output", "ROTARI_WEB_AUTH_TOKEN=[REDACTED]", "not checked for secrets", "## Web events", "sign_in", "Executable SHA-256:", "## Registered basedirs"} {
				if !strings.Contains(summary, want) {
					t.Fatalf("summary lacks %q:\n%s", want, summary)
				}
			}
			if keeps := strings.Contains(summary, e.Base); keeps != (variant.name == "no-redact") {
				t.Fatalf("summary contains the basedir: %v, want %v", keeps, variant.name == "no-redact")
			}

			bundle := filepath.Join(e.Root, variant.name+".tar.gz")
			e.MustRotari(append([]string{"bug-report", "-p", "secrets", "--output", bundle}, variant.extra...)...)
			files := readBugReportBundle(t, bundle)
			commandFound := false
			for name, data := range files {
				assertNoSecrets(t, name, data, secrets)
				if strings.HasSuffix(name, "/command.json") && strings.Contains(data, "API_TOKEN_VALUE=[REDACTED]") {
					commandFound = true
				}
			}
			if !commandFound {
				t.Fatalf("no command.json with the job's environment name kept and value replaced in %v", bundleNames(files))
			}
			if again := e.Rotari("bug-report", "-p", "secrets", "--output", bundle); again.Code == 0 {
				t.Fatalf("bug-report replaced an existing bundle: %s", again)
			}
		})
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertNoSecrets(t *testing.T, name, text string, secrets []string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("%s contains %q", name, secret)
		}
	}
}

func readBugReportBundle(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewReader(compressed)
	files := make(map[string]string)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(archive)
		if err != nil {
			t.Fatal(err)
		}
		files[header.Name] = string(data)
	}
}

func bundleNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	return names
}
