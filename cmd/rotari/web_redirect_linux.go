package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// redirectOutput points standard output and standard error at file, as a server
// that detaches from its client does with its log. Dup3, unlike Dup2, exists on
// every Linux architecture.
func redirectOutput(file *os.File) error {
	for _, fd := range []int{1, 2} {
		if err := unix.Dup3(int(file.Fd()), fd, 0); err != nil {
			return err
		}
	}
	return nil
}
