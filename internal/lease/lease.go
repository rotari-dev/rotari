// Package lease holds a process's claim on a file with flock: a process holds
// the exclusive lock for as long as it lives, and others probe it with a
// shared lock. The kernel drops the lock however the holder ends, so a probe
// never mistakes a dead holder, or a process that reused its PID, for a live
// one.
package lease

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// ErrHeld says another process holds the lease.
var ErrHeld = errors.New("the lease is held by another process")

const retryDelay = 10 * time.Millisecond

// TryLock takes the exclusive lock on file without waiting. It returns ErrHeld
// while another process holds it, and any other failure as it is.
func TryLock(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return ErrHeld
	}
	return err
}

// Hold opens path, creating it with mode, and takes its exclusive lock,
// retrying for up to wait so that a probe holding the shared lock for an
// instant does not read as a holder. The lease lasts until the returned file
// is closed or the process ends. The file itself is never removed: a process
// that opened it before a removal could otherwise lock a file no one else
// sees.
func Hold(path string, mode os.FileMode, wait time.Duration) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open lease %s: %w", path, err)
	}
	start := time.Now()
	for {
		err = TryLock(file)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, ErrHeld) || time.Since(start) >= wait {
			_ = file.Close()
			if errors.Is(err, ErrHeld) {
				return nil, ErrHeld
			}
			return nil, fmt.Errorf("failed to lock lease %s: %w", path, err)
		}
		time.Sleep(retryDelay)
	}
}

// Probe reports whether another process holds the lease at path. A missing
// file is a lease no one holds. When no one holds it, whileFree, if set, runs
// while Probe holds the shared lock, so no holder can start meanwhile; its
// error is returned. Any other failure is returned rather than read as free.
func Probe(path string, whileFree func() error) (bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		if whileFree != nil {
			return false, whileFree()
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to open lease %s: %w", path, err)
	}
	defer file.Close()
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to probe lease %s: %w", path, err)
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()
	if whileFree != nil {
		return false, whileFree()
	}
	return false, nil
}
