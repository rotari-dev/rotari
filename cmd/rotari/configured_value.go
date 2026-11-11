package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// configuredValue is an option's value from the environment or a config
// file, and where it came from: a variable name or a config file.
type configuredValue struct {
	raw  any
	from string
	set  bool
}

// configuredFromConfig returns the config value of option name for the
// current command, as configValue chooses it, and the file it is in.
func configuredFromConfig(name string) configuredValue {
	value, ok := configValue(name)
	if !ok {
		return configuredValue{}
	}
	from := "config"
	if setting := configSettingIn(cliConfigLayers, cliConfigCommand, name); setting.found {
		from = setting.where()
	}
	return configuredValue{raw: value, from: from, set: true}
}

// configuredFromEnvironment returns the value of the variable envName. An
// empty variable counts as unset: options whose values are typed read it
// this way.
func configuredFromEnvironment(envName string) configuredValue {
	if envName == "" {
		return configuredValue{}
	}
	if raw, ok := os.LookupEnv(envName); ok && raw != "" {
		return configuredValue{raw: raw, from: envName, set: true}
	}
	return configuredValue{}
}

func (value configuredValue) invalid(name string, err error) error {
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

func (value configuredValue) boolean(name string) (bool, error) {
	switch raw := value.raw.(type) {
	case bool:
		return raw, nil
	case string:
		if parsed, err := strconv.ParseBool(raw); err == nil {
			return parsed, nil
		}
	}
	return false, value.invalid(name, errors.New("want true or false"))
}

func (value configuredValue) integer(name string) (int, error) {
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

// duration parses a duration such as 30s. A number is read as its text, so
// 0 is a duration and 5 is not.
func (value configuredValue) duration(name string) (time.Duration, error) {
	switch value.raw.(type) {
	case string, int, int64, uint64, float64:
		if parsed, err := time.ParseDuration(fmt.Sprint(value.raw)); err == nil {
			return parsed, nil
		}
	}
	return 0, value.invalid(name, errors.New("want a duration such as 30s or 1h"))
}

// text returns a string, or a whole number as text.
func (value configuredValue) text(name string) (string, error) {
	switch raw := value.raw.(type) {
	case string:
		return raw, nil
	case int, int64, uint64:
		return fmt.Sprint(raw), nil
	}
	return "", value.invalid(name, errors.New("want a string"))
}

// scalarText returns any single value as text, as string options have
// always read config numbers and booleans; a list or table is an error.
func (value configuredValue) scalarText(name string) (string, error) {
	switch raw := value.raw.(type) {
	case string:
		return raw, nil
	case []any, map[string]any:
		return "", value.invalid(name, errors.New("want a single value, not a list or table"))
	default:
		return fmt.Sprint(raw), nil
	}
}

// texts returns a string, or each string of a list.
func (value configuredValue) texts(name string) ([]string, error) {
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

// scalarTexts returns a single value, or each value of a list, as text, as
// repeated options have always read them; a table, or a list or table in a
// list, is an error.
func (value configuredValue) scalarTexts(name string) ([]string, error) {
	items, ok := value.raw.([]any)
	if !ok {
		text, err := value.scalarText(name)
		if err != nil {
			return nil, value.invalid(name, errors.New("want a value or a list of values, not a table"))
		}
		return []string{text}, nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		switch item.(type) {
		case []any, map[string]any:
			return nil, value.invalid(name, errors.New("want a list of single values"))
		}
		result = append(result, fmt.Sprint(item))
	}
	return result, nil
}

// choice returns text if it is one of choices.
func (value configuredValue) choice(name, text string, choices []string) (string, error) {
	for _, choice := range choices {
		if text == choice {
			return text, nil
		}
	}
	return "", value.invalid(name, fmt.Errorf("choose from %s", strings.Join(choices, ", ")))
}

// configuredProblems holds, for each flag set, the environment and config
// values of its options that could not be used. parseCLIFlagSet reports them
// once the command line has parsed: an error for a command, so a bad value
// is never replaced silently by a default, and a warning for --help.
var configuredProblems = struct {
	sync.Mutex
	byFlagSet map[*flag.FlagSet][]error
}{byFlagSet: map[*flag.FlagSet][]error{}}

func addConfiguredProblem(fs *flag.FlagSet, err error) {
	configuredProblems.Lock()
	defer configuredProblems.Unlock()
	configuredProblems.byFlagSet[fs] = append(configuredProblems.byFlagSet[fs], err)
}

// takeConfiguredProblems returns fs's problems, joined, and forgets them.
func takeConfiguredProblems(fs *flag.FlagSet) error {
	configuredProblems.Lock()
	defer configuredProblems.Unlock()
	problems := configuredProblems.byFlagSet[fs]
	delete(configuredProblems.byFlagSet, fs)
	return errors.Join(problems...)
}

// configuredSources returns option name's config value, then the value of
// each of envNames, leaving out those not set; the last one set applies.
func configuredSources(name string, envNames ...string) []configuredValue {
	var values []configuredValue
	for _, value := range append([]configuredValue{configuredFromConfig(name)}, environmentValues(envNames)...) {
		if value.set {
			values = append(values, value)
		}
	}
	return values
}

func environmentValues(envNames []string) []configuredValue {
	values := make([]configuredValue, 0, len(envNames))
	for _, envName := range envNames {
		values = append(values, configuredFromEnvironment(envName))
	}
	return values
}

// configuredString returns the default of the string option spec from config
// and then the environment. A plain string option keeps an empty variable as
// its value; an option with choices reads it as unset and must get one of
// its choices.
func configuredString(fs *flag.FlagSet, spec cliFlagSpec, defaultValue string) string {
	name := spec.Name
	apply := func(value configuredValue, text string) {
		if len(spec.Values) > 0 {
			chosen, err := value.choice(name, text, spec.Values)
			if err != nil {
				addConfiguredProblem(fs, err)
				return
			}
			text = chosen
		}
		defaultValue = text
	}
	if value := configuredFromConfig(name); value.set {
		if text, err := value.scalarText(name); err != nil {
			addConfiguredProblem(fs, err)
		} else {
			apply(value, text)
		}
	}
	envName := cliCommandEnvironmentVariable(fs.Name(), name)
	if envName == "" {
		return defaultValue
	}
	if len(spec.Values) > 0 {
		if value := configuredFromEnvironment(envName); value.set {
			apply(value, value.raw.(string))
		}
	} else if raw, ok := os.LookupEnv(envName); ok {
		defaultValue = raw
	}
	return defaultValue
}

// configuredRepeated adds the config values of the repeated or custom option
// name to target, or else the environment's, which replaces them.
func configuredRepeated(fs *flag.FlagSet, target flag.Value, name string) {
	if value := configuredFromConfig(name); value.set {
		texts, err := value.scalarTexts(name)
		if err != nil {
			addConfiguredProblem(fs, err)
		}
		for _, text := range texts {
			if err := target.Set(text); err != nil {
				addConfiguredProblem(fs, configuredValue{raw: text, from: value.from, set: true}.invalid(name, err))
			}
		}
	}
	value := configuredFromEnvironment(cliCommandEnvironmentVariable(fs.Name(), name))
	if !value.set {
		return
	}
	if resettable, ok := target.(interface{ Reset() }); ok {
		resettable.Reset()
	}
	if err := target.Set(value.raw.(string)); err != nil {
		addConfiguredProblem(fs, value.invalid(name, err))
	}
}
