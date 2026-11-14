package state

import (
	"fmt"
	"os"
	"strings"
)

// processStart is field 22 of /proc/PID/stat, in clock ticks since boot.
func processStart(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return ""
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return ""
	}
	fields := strings.Fields(string(data)[end+1:])
	if len(fields) <= 19 {
		return ""
	}
	return fields[19]
}

func pidNamespace() string {
	link, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return ""
	}
	return link
}
