package web

import (
	"bufio"
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rotari-dev/rotari/conformance/support"
)

// lockedBuffer collects a process's stderr while the test reads it.
type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

var listeningAt = regexp.MustCompile(`listening at (http://\S+)`)

// startWebWithStderr starts rotari web in e and returns its address and what
// it has written to stderr by then.
func startWebWithStderr(t *testing.T, e *support.Env, args ...string) (string, string) {
	t.Helper()
	command := e.Command(append([]string{"web", "--port", "0"}, args...)...)
	stderr := &lockedBuffer{}
	command.Stderr = stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	found := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if match := listeningAt.FindStringSubmatch(scanner.Text()); match != nil {
				found <- match[1]
				return
			}
		}
	}()
	select {
	case address := <-found:
		return address, stderr.String()
	case <-time.After(10 * time.Second):
		t.Fatalf("rotari web %v did not report its address; stderr:\n%s", args, stderr.String())
		return "", ""
	}
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func globalConfig(e *support.Env) string {
	return filepath.Join(e.Root, "config", "rotari", "config.toml")
}

// TestTerminalOptionsFromTrustedConfigs checks that the global config and the
// environment can turn on the terminal, and that a workspace or basedir
// config cannot, nor choose the host or token of a server with it on; rotari
// web warns of each value it ignores.
func TestTerminalOptionsFromTrustedConfigs(t *testing.T) {
	covers(t, "WEB-36")
	token := map[string]string{"X-Rotari-Token": launchToken}

	t.Run("a workspace config cannot turn on the terminal", func(t *testing.T) {
		e := support.NewEnv(t)
		workspace := filepath.Join(e.Root, "workspace")
		writeConfig(t, filepath.Join(workspace, ".rotari.toml"), "[web]\nallow-terminal = true\nallow-exec = true\n")
		base, stderr := startWebWithStderr(t, e.In(workspace))
		for _, want := range []string{"ignoring allow-terminal = true in workspace config", "ignoring allow-exec = true in workspace config", "ROTARI_WEB_ALLOW_TERMINAL"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("stderr does not say %q:\n%s", want, stderr)
			}
		}
		if got := e.HTTPGet(base + "/_terminal/xterm.js"); got.Status != http.StatusNotFound {
			t.Errorf("GET /_terminal/xterm.js: status %d, want 404", got.Status)
		}
	})

	t.Run("the global config turns on the terminal", func(t *testing.T) {
		e := support.NewEnv(t)
		e.Setenv("ROTARI_WEB_AUTH_TOKEN", launchToken)
		writeConfig(t, globalConfig(e), "[web]\nallow-terminal = true\nterminal-max-shells = 3\n")
		base, stderr := startWebWithStderr(t, e)
		if strings.Contains(stderr, "ignoring") {
			t.Errorf("stderr warns of a trusted value:\n%s", stderr)
		}
		if got := e.HTTPDo(http.MethodGet, base+"/_terminal/shells", token, ""); got.Status != http.StatusOK {
			t.Errorf("GET /_terminal/shells: status %d, want 200", got.Status)
		}
	})

	t.Run("the environment turns on the terminal", func(t *testing.T) {
		e := support.NewEnv(t)
		e.Setenv("ROTARI_WEB_AUTH_TOKEN", launchToken)
		e.Setenv("ROTARI_WEB_ALLOW_TERMINAL", "true")
		base, _ := startWebWithStderr(t, e)
		if got := e.HTTPDo(http.MethodGet, base+"/_terminal/shells", token, ""); got.Status != http.StatusOK {
			t.Errorf("GET /_terminal/shells: status %d, want 200", got.Status)
		}
	})

	t.Run("a basedir config cannot choose the host or token", func(t *testing.T) {
		e := support.NewEnv(t)
		writeConfig(t, filepath.Join(e.Base, "config.toml"), "[web]\nhost = \"127.0.0.2\"\nauth-token = \"basedir-secret\"\n")
		base, stderr := startWebWithStderr(t, e, "--allow-terminal")
		for _, want := range []string{`ignoring host = "127.0.0.2" in basedir config`, "ignoring auth-token in basedir config", "and this token as the password"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("stderr does not say %q:\n%s", want, stderr)
			}
		}
		if strings.Contains(stderr, "basedir-secret") {
			t.Errorf("stderr shows the ignored token:\n%s", stderr)
		}
		if !strings.HasPrefix(base, "http://127.0.0.1:") {
			t.Errorf("listening at %s, want 127.0.0.1", base)
		}
		ignored := map[string]string{"X-Rotari-Token": "basedir-secret"}
		if got := e.HTTPDo(http.MethodGet, base+"/_terminal/shells", ignored, ""); got.Status != http.StatusUnauthorized {
			t.Errorf("GET /_terminal/shells with the basedir's token: status %d, want 401", got.Status)
		}
	})

	t.Run("without the terminal, a basedir config chooses the host", func(t *testing.T) {
		e := support.NewEnv(t)
		writeConfig(t, filepath.Join(e.Base, "config.toml"), "[web]\nhost = \"127.0.0.2\"\n")
		base, stderr := startWebWithStderr(t, e)
		if !strings.HasPrefix(base, "http://127.0.0.2:") || strings.Contains(stderr, "ignoring") {
			t.Errorf("listening at %s, stderr:\n%s", base, stderr)
		}
	})
}

// TestTerminalOptionsRejectBadConfiguredValues checks that a bad value from
// the environment or a config stops rotari web before it serves, saying where
// the value came from, and that --static-dir rejects the terminal turned on
// by the global config but not one a workspace config tried to turn on.
func TestTerminalOptionsRejectBadConfiguredValues(t *testing.T) {
	covers(t, "WEB-36", "WEB-3")
	e := support.NewEnv(t)
	for _, test := range []struct {
		env    *support.Env
		config string
		args   []string
		want   string
	}{
		{env: e.WithVar("ROTARI_WEB_ALLOW_TERMINAL", "maybe"), want: `invalid allow-terminal "maybe" in ROTARI_WEB_ALLOW_TERMINAL`},
		{env: e.WithVar("ROTARI_WEB_TERMINAL_MAX_SHELLS", "0"), args: []string{"--allow-terminal"}, want: "--terminal-max-shells 0 must be at least 1 (from ROTARI_WEB_TERMINAL_MAX_SHELLS)"},
		{env: e, config: "[web]\nterminal-max-shells = \"lots\"\n", want: `invalid terminal-max-shells "lots" in global config`},
		{env: e, config: "[web]\nallow-terminal = true\nterminal-idle-timeout = \"1m\"\n", want: "must be at least 2m0s (from global config"},
		{env: e, config: "[web]\nallow-terminal = true\n", args: []string{"--static-dir", filepath.Join(e.Root, "export")}, want: "--allow-terminal cannot be combined with --static-dir"},
	} {
		if err := os.RemoveAll(filepath.Dir(globalConfig(e))); err != nil {
			t.Fatal(err)
		}
		if test.config != "" {
			writeConfig(t, globalConfig(e), test.config)
		}
		args := append([]string{"web"}, test.args...)
		if !strings.Contains(strings.Join(test.args, " "), "--static-dir") {
			// Should it start after all, it does not take the default port.
			args = append(args, "--port", "0")
		}
		if r := test.env.Rotari(args...); r.Code == 0 || !strings.Contains(r.Stderr, test.want) {
			t.Errorf("web %v with config %q: %s, want an error with %q", test.args, test.config, r, test.want)
		}
	}
	if err := os.RemoveAll(filepath.Dir(globalConfig(e))); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(e.Root, "workspace")
	writeConfig(t, filepath.Join(workspace, ".rotari.toml"), "[web]\nallow-terminal = true\n")
	export := filepath.Join(e.Root, "workspace-export")
	r := e.In(workspace).Rotari("web", "--static-dir", export)
	if r.Code != 0 || !strings.Contains(r.Stderr, "ignoring allow-terminal = true in workspace config") {
		t.Errorf("web --static-dir with a workspace allow-terminal: %s", r)
	}
	if _, err := os.Stat(filepath.Join(export, "index.html")); err != nil {
		t.Errorf("static export: %v", err)
	}
}

// TestWebConfigSaveKeepsAuthToken checks that saving the global config from
// the Web UI cannot set auth-token, which would choose the token of the next
// rotari web with the terminal on, but can change the file otherwise.
func TestWebConfigSaveKeepsAuthToken(t *testing.T) {
	covers(t, "WEB-36")
	e := support.NewEnv(t)
	path := globalConfig(e)
	original := "[web]\nterminal-max-shells = 3\n"
	writeConfig(t, path, original)
	base := e.StartWeb()
	save := func(content string) support.HTTPResult {
		return e.HTTPPostJSON(base+"/api/save-config", map[string]any{"scope": "global", "path": path, "content": content})
	}
	for _, content := range []string{
		"auth-token = \"theirs\"\n[web]\nterminal-max-shells = 3\n",
		"[web]\nallow-terminal = true\nauth-token = \"theirs\"\n",
	} {
		if got := save(content); got.Status == http.StatusOK || !strings.Contains(got.Body, "auth-token cannot be changed from the Web UI") {
			t.Errorf("save %q: status %d, body %q", content, got.Status, got.Body)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != original {
			t.Fatalf("global config after a refused save = %q, err = %v", data, err)
		}
	}
	changed := "[web]\nterminal-max-shells = 4\n"
	if got := save(changed); got.Status != http.StatusOK {
		t.Fatalf("save without auth-token: status %d, body %q", got.Status, got.Body)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != changed {
		t.Fatalf("global config = %q, err = %v", data, err)
	}
}
