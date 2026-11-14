package state

// NoPIDNamespaces is the PID namespace recorded on a platform that has none,
// such as macOS and the BSDs, where every process on a host shares one PID
// space. Two records naming it on the same host are in the same PID space, so
// Locate verifies them as it verifies two equal Linux namespaces. A record
// written there by an older rotari has an empty namespace and stays
// unverified.
const NoPIDNamespaces = "none"

// ProcessStart is pid's start time, which tells a process from a later one
// reusing its PID. It is compared only with values recorded on the same host,
// so its form is the platform's own. An empty value means the platform cannot
// verify PID reuse, or that pid is not running.
func ProcessStart(pid int) string { return processStart(pid) }

// PIDNamespace names this process's PID namespace, within which alone a PID
// identifies a process: Linux's namespace link, NoPIDNamespaces on a platform
// without namespaces, and empty where neither can be told.
func PIDNamespace() string { return pidNamespace() }
