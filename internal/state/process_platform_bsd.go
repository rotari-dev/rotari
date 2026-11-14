//go:build dragonfly || freebsd || netbsd || openbsd

package state

// processStart is empty: these systems' kinfo_proc differs from macOS's, and
// no CI runs them, so PID reuse is not verified there.
func processStart(int) string { return "" }

func pidNamespace() string { return NoPIDNamespaces }
