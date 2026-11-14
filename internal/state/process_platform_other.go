//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package state

func processStart(int) string { return "" }

func pidNamespace() string { return "" }
