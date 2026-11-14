//go:build !linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// redirectOutput points standard output and standard error at file, as a server
// that detaches from its client does with its log.
func redirectOutput(file *os.File) error {
	for _, fd := range []int{1, 2} {
		if err := unix.Dup2(int(file.Fd()), fd); err != nil {
			return err
		}
	}
	return nil
}
