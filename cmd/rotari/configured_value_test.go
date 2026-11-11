package main

// configInt and configBool read an option's config value for tests, with the
// parsing the option helpers use. A value that does not parse is a test bug.
func configInt(name string, defaultValue int) int {
	value := configuredFromConfig(name)
	if !value.set {
		return defaultValue
	}
	parsed, err := value.integer(name)
	if err != nil {
		panic(err)
	}
	return parsed
}

func configBool(name string, defaultValue bool) bool {
	value := configuredFromConfig(name)
	if !value.set {
		return defaultValue
	}
	parsed, err := value.boolean(name)
	if err != nil {
		panic(err)
	}
	return parsed
}
