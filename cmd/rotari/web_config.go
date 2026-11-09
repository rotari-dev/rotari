package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/rotari-dev/rotari/internal/config"
)

// webTrustedOptions let other people reach this host, so rotari web reads
// them only from the command line, the environment, and trusted configs: the
// global config and a file named with --config. A config in the working
// directory, the basedir, or a project can be written by someone else, or
// arrive with a cloned repository; its values for them are ignored, with a
// warning.
var webTrustedOptions = []string{"allow-exec", "allow-terminal", "allowed-host"}

// webExposedOptions are read like webTrustedOptions when the terminal or
// launchers are on, and from every config otherwise.
var webExposedOptions = []string{"auth-token", "host"}

// trustedConfigScope reports whether a config file of scope may set
// webTrustedOptions.
func trustedConfigScope(scope string) bool {
	return scope == "global" || scope == "explicit"
}

// configSetting is a value found in one config file.
type configSetting struct {
	value  any
	source config.Source
	found  bool
}

func (setting configSetting) where() string {
	return fmt.Sprintf("%s config %s", setting.source.Scope, setting.source.Path)
}

// configSettingIn returns the value configValue would give for command's
// option name had only layers been read, and the file it is in: the
// command's section of any layer first, then the top level, the highest
// scope winning in each.
func configSettingIn(layers []config.Loaded, command, name string) configSetting {
	for _, inSection := range []bool{true, false} {
		for index := len(layers) - 1; index >= 0; index-- {
			values := layers[index].Values
			if inSection {
				section, ok := values[command].(map[string]any)
				if !ok {
					continue
				}
				values = section
			}
			if value, ok := values[name]; ok && value != nil && len(layers[index].Sources) > 0 {
				return configSetting{value: value, source: layers[index].Sources[0], found: true}
			}
		}
	}
	return configSetting{}
}

// webConfigSetting returns the config value of a rotari web option. With
// trustedOnly it reads only trusted configs, and also returns the local
// config's value it left out, if that value would otherwise have applied.
func webConfigSetting(name string, trustedOnly bool) (setting, ignored configSetting) {
	all := configSettingIn(cliConfigLayers, "web", name)
	if !trustedOnly || !all.found || trustedConfigScope(all.source.Scope) {
		return all, configSetting{}
	}
	var trusted []config.Loaded
	for _, layer := range cliConfigLayers {
		if len(layer.Sources) > 0 && trustedConfigScope(layer.Sources[0].Scope) {
			trusted = append(trusted, layer)
		}
	}
	return configSettingIn(trusted, "web", name), all
}

// webOptionConfigured reports whether the environment or a config that may
// set it supplies a rotari web option, for --static-dir's check (WEB-3).
func webOptionConfigured(name string) bool {
	if isWebTrustedOption(name) {
		_, configured := webEnvironmentOrConfig(name, true)
		return configured.set
	}
	if envName := cliCommandEnvironmentVariable("web", name); envName != "" {
		if _, ok := os.LookupEnv(envName); ok {
			return true
		}
	}
	_, ok := configValue(name)
	return ok
}

func isWebTrustedOption(name string) bool {
	for _, trusted := range webTrustedOptions {
		if name == trusted {
			return true
		}
	}
	return false
}

// webValue is a value of a rotari web option from the environment or a
// config file, and where it came from.
type webValue struct {
	raw  any
	from string
	set  bool
}

// webEnvironmentOrConfig returns the environment's value of a rotari web
// option, or else the config's, and a local config's value left out under
// trustedOnly. An empty variable counts as unset.
func webEnvironmentOrConfig(name string, trustedOnly bool) (ignored configSetting, value webValue) {
	envName := cliCommandEnvironmentVariable("web", name)
	if raw, ok := os.LookupEnv(envName); ok && raw != "" {
		return configSetting{}, webValue{raw: raw, from: envName, set: true}
	}
	setting, ignored := webConfigSetting(name, trustedOnly)
	if !setting.found {
		return ignored, webValue{}
	}
	return ignored, webValue{raw: setting.value, from: setting.where(), set: true}
}

func (value webValue) invalid(name string, err error) error {
	return fmt.Errorf("invalid %s %s in %s: %v", name, configValueText(value.raw), value.from, err)
}

// configValueText shows a config or environment value as JSON would.
func configValueText(raw any) string {
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Sprint(raw)
	}
	return string(data)
}

func (value webValue) boolean(name string) (bool, error) {
	switch raw := value.raw.(type) {
	case bool:
		return raw, nil
	case string:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return false, value.invalid(name, errors.New("want true or false"))
		}
		return parsed, nil
	}
	return false, value.invalid(name, errors.New("want true or false"))
}

func (value webValue) integer(name string) (int, error) {
	switch raw := value.raw.(type) {
	case int:
		return raw, nil
	case int64:
		if raw >= math.MinInt && raw <= math.MaxInt {
			return int(raw), nil
		}
	case uint64:
		if raw <= math.MaxInt {
			return int(raw), nil
		}
	case float64:
		if raw == math.Trunc(raw) && raw >= math.MinInt && raw <= math.MaxInt {
			return int(raw), nil
		}
	case string:
		if parsed, err := strconv.Atoi(raw); err == nil {
			return parsed, nil
		}
	}
	return 0, value.invalid(name, errors.New("want a whole number"))
}

func (value webValue) duration(name string) (time.Duration, error) {
	if raw, ok := value.raw.(string); ok {
		if parsed, err := time.ParseDuration(raw); err == nil {
			return parsed, nil
		}
	}
	return 0, value.invalid(name, errors.New("want a duration such as 30m or 1h"))
}

func (value webValue) text(name string) (string, error) {
	switch raw := value.raw.(type) {
	case string:
		return raw, nil
	case int, int64, uint64:
		return fmt.Sprint(raw), nil
	}
	return "", value.invalid(name, errors.New("want a string"))
}

func (value webValue) texts(name string) ([]string, error) {
	switch raw := value.raw.(type) {
	case string:
		return []string{raw}, nil
	case []any:
		result := make([]string, 0, len(raw))
		for _, item := range raw {
			text, ok := item.(string)
			if !ok {
				return nil, value.invalid(name, errors.New("want a string or a list of strings"))
			}
			result = append(result, text)
		}
		return result, nil
	}
	return nil, value.invalid(name, errors.New("want a string or a list of strings"))
}

// webConfigured holds the defaults of the terminal and launch options from the
// environment and config, before the command line.
type webConfigured struct {
	allowExec, allowTerminal bool
	allowedHosts             []string
	idleLimit                time.Duration
	shellLimit, groupLimit   int
	replaySize, shell        string
	// from says where each option not left at its built-in default came from.
	from map[string]string
	// ignored are local config values left out of webTrustedOptions.
	ignored map[string]configSetting
}

// readWebConfigured reads the terminal and launch options of rotari web from
// the environment and config. A value that does not parse is an error naming
// where it came from.
func readWebConfigured() (webConfigured, error) {
	configured := webConfigured{
		idleLimit: webTerminalIdleLimit, shellLimit: webTerminalShellLimit, groupLimit: webTerminalGroupLimit,
		replaySize: webTerminalRingSize, from: map[string]string{}, ignored: map[string]configSetting{},
	}
	var problems []error
	read := func(name string, apply func(webValue) error) {
		ignored, value := webEnvironmentOrConfig(name, isWebTrustedOption(name))
		if ignored.found {
			configured.ignored[name] = ignored
		}
		if !value.set {
			return
		}
		if err := apply(value); err != nil {
			problems = append(problems, err)
			return
		}
		configured.from[name] = value.from
	}
	read("allow-exec", func(value webValue) (err error) {
		configured.allowExec, err = value.boolean("allow-exec")
		return err
	})
	read("allow-terminal", func(value webValue) (err error) {
		configured.allowTerminal, err = value.boolean("allow-terminal")
		return err
	})
	read("allowed-host", func(value webValue) error {
		hosts, err := value.texts("allowed-host")
		if err != nil {
			return err
		}
		for _, host := range hosts {
			if !validAllowedHost(host) {
				return value.invalid("allowed-host", fmt.Errorf("%q must be a host name or IP address without a port", host))
			}
		}
		configured.allowedHosts = hosts
		return nil
	})
	read("terminal-idle-timeout", func(value webValue) (err error) {
		configured.idleLimit, err = value.duration("terminal-idle-timeout")
		return err
	})
	read("terminal-max-shells", func(value webValue) (err error) {
		configured.shellLimit, err = value.integer("terminal-max-shells")
		return err
	})
	read("terminal-max-shells-per-project", func(value webValue) (err error) {
		configured.groupLimit, err = value.integer("terminal-max-shells-per-project")
		return err
	})
	read("terminal-replay-size", func(value webValue) (err error) {
		configured.replaySize, err = value.text("terminal-replay-size")
		return err
	})
	read("terminal-shell", func(value webValue) (err error) {
		configured.shell, err = value.text("terminal-shell")
		return err
	})
	return configured, errors.Join(problems...)
}

// webFlagUsage is the help text of a rotari web option.
func webFlagUsage(name string) string {
	return cliFlagDescriptionFor("web", cliCommandFlag("web", name))
}

// warnIgnoredWebConfig says that a local config's value of a rotari web
// option was left out, unless the command line replaced it anyway.
func warnIgnoredWebConfig(fs *flag.FlagSet, name string, ignored configSetting, exposed bool) {
	if !ignored.found || cliOptionSet(fs, name) {
		return
	}
	setting := name
	if name != "auth-token" {
		setting += " = " + configValueText(ignored.value)
	}
	when := ""
	if exposed {
		when = "with --allow-terminal or --allow-exec, "
	}
	printWarningf("WARNING: ignoring %s in %s; %srotari web reads it only from the command line, %s, the global config, or --config FILE",
		setting, ignored.where(), when, cliCommandEnvironmentVariable("web", name))
}

// trustedWebExposedValue returns host or auth-token for a server with the
// terminal or launchers on: the command line's or environment's value as
// parsed, or else a trusted config's, or else builtin. A local config's value
// is ignored with a warning.
func trustedWebExposedValue(fs *flag.FlagSet, name, parsed, builtin string) string {
	if cliOptionSet(fs, name) {
		return parsed
	}
	if _, ok := os.LookupEnv(cliCommandEnvironmentVariable("web", name)); ok {
		return parsed
	}
	setting, ignored := webConfigSetting(name, true)
	warnIgnoredWebConfig(fs, name, ignored, true)
	if !setting.found {
		return builtin
	}
	if text, ok := setting.value.(string); ok {
		return text
	}
	return fmt.Sprint(setting.value)
}
