package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/config"
)

// configLayer is a config file of scope with values, for tests.
func configLayer(scope string, values map[string]any) config.Loaded {
	return config.Loaded{Values: values, Sources: []config.Source{{Scope: scope, Path: "/" + scope + "/config.toml"}}}
}

// useConfigLayers makes layers, lowest scope first, the config files of the
// test's invocation.
func useConfigLayers(t *testing.T, layers ...config.Loaded) {
	t.Helper()
	oldConfig, oldCommand, oldLayers := cliConfig, cliConfigCommand, cliConfigLayers
	t.Cleanup(func() {
		cliConfig, cliConfigCommand, cliConfigLayers = oldConfig, oldCommand, oldLayers
	})
	merged := config.Loaded{Values: map[string]any{}}
	for _, layer := range layers {
		merged.Add(layer)
	}
	cliConfig, cliConfigCommand, cliConfigLayers = merged.Values, "web", layers
}

func web(values map[string]any) map[string]any {
	return map[string]any{"web": values}
}

// TestWebTrustedOptionsComeOnlyFromTrustedConfigs checks that a workspace,
// basedir, or project config cannot turn on the terminal or launchers, add
// allowed hosts, or, with either on, choose the host or token, and that
// rotari web says so; the global config and --config FILE can (WEB-36).
func TestWebTrustedOptionsComeOnlyFromTrustedConfigs(t *testing.T) {
	for _, test := range []struct {
		name   string
		layers []config.Loaded
		env    map[string]string
		args   []string
		check  func(webCommandFlags) bool
		warns  []string
		quiet  []string
	}{
		{
			name:   "workspace cannot turn on the terminal",
			layers: []config.Loaded{configLayer("workspace", web(map[string]any{"allow-terminal": true}))},
			check:  func(flags webCommandFlags) bool { return !flags.allowTerminal },
			warns:  []string{"ignoring allow-terminal = true in workspace config /workspace/config.toml", "ROTARI_WEB_ALLOW_TERMINAL"},
		},
		{
			name:   "a top-level key in a project config is ignored too",
			layers: []config.Loaded{configLayer("project", map[string]any{"allow-exec": true})},
			check:  func(flags webCommandFlags) bool { return !flags.allowExec },
			warns:  []string{"ignoring allow-exec = true in project config"},
		},
		{
			name:   "the global config turns on the terminal",
			layers: []config.Loaded{configLayer("global", web(map[string]any{"allow-terminal": true}))},
			check:  func(flags webCommandFlags) bool { return flags.allowTerminal },
			quiet:  []string{"ignoring"},
		},
		{
			name:   "--config FILE turns on launchers",
			layers: []config.Loaded{configLayer("explicit", web(map[string]any{"allow-exec": true}))},
			check:  func(flags webCommandFlags) bool { return flags.allowExec },
			quiet:  []string{"ignoring"},
		},
		{
			name: "a hidden global value applies",
			layers: []config.Loaded{
				configLayer("global", web(map[string]any{"allow-terminal": true})),
				configLayer("basedir", web(map[string]any{"allow-terminal": false})),
			},
			check: func(flags webCommandFlags) bool { return flags.allowTerminal },
			warns: []string{"ignoring allow-terminal = false in basedir config"},
		},
		{
			name:   "the command line replaces a local value without a warning",
			layers: []config.Loaded{configLayer("workspace", web(map[string]any{"allow-terminal": false}))},
			args:   []string{"--allow-terminal"},
			check:  func(flags webCommandFlags) bool { return flags.allowTerminal },
			quiet:  []string{"ignoring"},
		},
		{
			name:   "the environment replaces a local value without a warning",
			layers: []config.Loaded{configLayer("workspace", web(map[string]any{"allow-terminal": false}))},
			env:    map[string]string{"ROTARI_WEB_ALLOW_TERMINAL": "true"},
			check:  func(flags webCommandFlags) bool { return flags.allowTerminal },
			quiet:  []string{"ignoring"},
		},
		{
			name:   "allowed hosts from a basedir config are ignored",
			layers: []config.Loaded{configLayer("basedir", web(map[string]any{"allowed-host": []any{"gpu01"}}))},
			args:   []string{"--allow-exec"},
			check:  func(flags webCommandFlags) bool { return len(flags.allowedHosts) == 0 },
			warns:  []string{`ignoring allowed-host = ["gpu01"] in basedir config`},
		},
		{
			name:   "allowed hosts from the global config and the command line add up",
			layers: []config.Loaded{configLayer("global", web(map[string]any{"allowed-host": "gpu01"}))},
			args:   []string{"--allow-exec", "--allowed-host", "gpu02"},
			check: func(flags webCommandFlags) bool {
				return slices.Equal(flags.allowedHosts, []string{"gpu01", "gpu02"})
			},
		},
		{
			name: "with the terminal on, the host and token come from the global config",
			layers: []config.Loaded{
				configLayer("global", web(map[string]any{"host": "127.0.0.3"})),
				configLayer("project", web(map[string]any{"host": "0.0.0.0", "auth-token": "project-secret"})),
			},
			args:  []string{"--allow-terminal"},
			check: func(flags webCommandFlags) bool { return flags.host == "127.0.0.3" && flags.authToken == "" },
			warns: []string{`ignoring host = "0.0.0.0" in project config`, "ignoring auth-token in project config", "with --allow-terminal or --allow-exec"},
			quiet: []string{"project-secret"},
		},
		{
			name:   "with launchers on and no trusted value, the host is the default",
			layers: []config.Loaded{configLayer("workspace", web(map[string]any{"host": "0.0.0.0"}))},
			args:   []string{"--allow-exec"},
			check:  func(flags webCommandFlags) bool { return flags.host == "127.0.0.1" },
			warns:  []string{`ignoring host = "0.0.0.0" in workspace config`},
		},
		{
			name:   "with the terminal on, the command line's host applies without a warning",
			layers: []config.Loaded{configLayer("workspace", web(map[string]any{"host": "0.0.0.0"}))},
			args:   []string{"--allow-terminal", "--host", "127.0.0.4"},
			check:  func(flags webCommandFlags) bool { return flags.host == "127.0.0.4" },
			quiet:  []string{"ignoring"},
		},
		{
			name:   "without the terminal or launchers, a local host and token apply",
			layers: []config.Loaded{configLayer("workspace", web(map[string]any{"host": "127.0.0.5", "auth-token": "local"}))},
			check:  func(flags webCommandFlags) bool { return flags.host == "127.0.0.5" && flags.authToken == "local" },
			quiet:  []string{"ignoring"},
		},
		{
			name: "terminal limits come from any config",
			layers: []config.Loaded{configLayer("project", web(map[string]any{
				"terminal-max-shells": int64(3), "terminal-max-shells-per-project": 2, "terminal-idle-timeout": "5m",
				"terminal-replay-size": "1KiB", "terminal-shell": "/bin/sh",
			}))},
			args: []string{"--allow-terminal"},
			check: func(flags webCommandFlags) bool {
				settings := flags.terminal
				return settings.shellLimit == 3 && settings.groupLimit == 2 && settings.idleLimit.String() == "5m0s" &&
					settings.ringSize == 1024 && settings.shell == "/bin/sh"
			},
			quiet: []string{"ignoring"},
		},
		{
			name:   "the command line replaces a configured limit",
			layers: []config.Loaded{configLayer("global", web(map[string]any{"terminal-max-shells": 0}))},
			args:   []string{"--allow-terminal", "--terminal-max-shells", "4"},
			check:  func(flags webCommandFlags) bool { return flags.terminal.shellLimit == 4 },
		},
		{
			name:   "configured limits and allowed hosts are unused with the terminal off",
			layers: []config.Loaded{configLayer("global", web(map[string]any{"terminal-shell": "/no/such/shell", "allowed-host": "gpu01"}))},
			check:  func(flags webCommandFlags) bool { return !flags.allowTerminal && !flags.allowExec },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			useConfigLayers(t, test.layers...)
			for name, value := range test.env {
				t.Setenv(name, value)
			}
			var flags webCommandFlags
			code, stderr := captureStderr(t, func() int {
				var code int
				flags, code = parseWebFlags(test.args)
				return code
			})
			if code != 0 || !test.check(flags) {
				t.Fatalf("parseWebFlags(%v) = %+v, %d; stderr %q", test.args, flags, code, stderr)
			}
			for _, want := range test.warns {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr %q does not say %q", stderr, want)
				}
			}
			for _, unwanted := range test.quiet {
				if strings.Contains(stderr, unwanted) {
					t.Errorf("stderr %q says %q", stderr, unwanted)
				}
			}
		})
	}
}

// TestWebConfiguredBadValuesAreErrors checks that a value from the environment
// or a config that does not parse, or is out of range, stops rotari web and
// says where it came from.
func TestWebConfiguredBadValuesAreErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		layers []config.Loaded
		env    map[string]string
		args   []string
		want   string
	}{
		{
			name: "a variable that is not a boolean",
			env:  map[string]string{"ROTARI_WEB_ALLOW_TERMINAL": "maybe"},
			want: `invalid allow-terminal "maybe" in ROTARI_WEB_ALLOW_TERMINAL`,
		},
		{
			name:   "a config value that is not a number",
			layers: []config.Loaded{configLayer("global", web(map[string]any{"terminal-max-shells": "lots"}))},
			want:   `invalid terminal-max-shells "lots" in global config /global/config.toml`,
		},
		{
			name:   "a config value that is not a duration",
			layers: []config.Loaded{configLayer("workspace", web(map[string]any{"terminal-idle-timeout": int64(30)}))},
			want:   "invalid terminal-idle-timeout 30 in workspace config",
		},
		{
			name:   "an allowed host with a port",
			layers: []config.Loaded{configLayer("global", web(map[string]any{"allowed-host": []any{"gpu01:80"}}))},
			want:   `invalid allowed-host ["gpu01:80"] in global config`,
		},
		{
			name: "a limit out of range",
			env:  map[string]string{"ROTARI_WEB_TERMINAL_MAX_SHELLS": "0"},
			args: []string{"--allow-terminal"},
			want: "--terminal-max-shells 0 must be at least 1 (from ROTARI_WEB_TERMINAL_MAX_SHELLS)",
		},
		{
			name:   "a replay size out of range",
			layers: []config.Loaded{configLayer("project", web(map[string]any{"terminal-replay-size": "1GiB"}))},
			args:   []string{"--allow-terminal"},
			want:   "(from project config /project/config.toml)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			useConfigLayers(t, test.layers...)
			for name, value := range test.env {
				t.Setenv(name, value)
			}
			code, stderr := captureStderr(t, func() int {
				_, code := parseWebFlags(test.args)
				return code
			})
			if code == 0 || !strings.Contains(stderr, test.want) {
				t.Fatalf("parseWebFlags(%v) = %d, stderr %q; want an error containing %q", test.args, code, stderr, test.want)
			}
		})
	}
}

// TestWebStaticDirFollowsTrustedConfigs checks that --static-dir rejects the
// terminal turned on by a trusted config, as WEB-3 says of other live-server
// options, and not one a local config tried to turn on.
func TestWebStaticDirFollowsTrustedConfigs(t *testing.T) {
	useConfigLayers(t, configLayer("global", web(map[string]any{"allow-terminal": true})))
	code, stderr := captureStderr(t, func() int {
		_, code := parseWebFlags([]string{"--static-dir", t.TempDir()})
		return code
	})
	if code == 0 || !strings.Contains(stderr, "--allow-terminal cannot be combined with --static-dir") {
		t.Fatalf("global allow-terminal with --static-dir = %d, %q", code, stderr)
	}
	useConfigLayers(t, configLayer("workspace", web(map[string]any{"allow-terminal": true, "allow-exec": true})))
	code, stderr = captureStderr(t, func() int {
		_, code := parseWebFlags([]string{"--static-dir", t.TempDir()})
		return code
	})
	if code != 0 || !strings.Contains(stderr, "ignoring allow-terminal = true in workspace config") || !strings.Contains(stderr, "ignoring allow-exec = true") {
		t.Fatalf("workspace allow-terminal with --static-dir = %d, %q", code, stderr)
	}
}

// TestLocalConfigTemplatesLeaveOutTrustedWebOptions checks that only the
// global template offers the options a local config cannot set.
func TestLocalConfigTemplatesLeaveOutTrustedWebOptions(t *testing.T) {
	for _, scope := range []string{"", "global", "workspace", "basedir", "project"} {
		var scopes []string
		if scope != "" {
			scopes = []string{scope}
		}
		data, err := configTemplate("toml", scopes...)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		local := scope == "workspace" || scope == "basedir" || scope == "project"
		for _, name := range webTrustedOptions {
			if offered := strings.Contains(text, "# "+name+" = "); offered == local {
				t.Errorf("%q template offers %s: %v", scope, name, offered)
			}
		}
		if !strings.Contains(text, "# terminal-max-shells = ") {
			t.Errorf("%q template does not offer terminal-max-shells", scope)
		}
	}
}
