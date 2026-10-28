package server

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Acquire waits for a held lease in steps of leaseRetryDelay. A supervisor
// that has recorded its PID is live, so Acquire waits only leaseProbeWait:
// Running holds a shared lock on the lease for an instant, which must not
// make a starting supervisor report that another one is running. Without a
// PID record, the holder is a supervisor that Withdraw marked as exiting
// after its run, whose client may already have returned, or one that is
// about to record its PID, so Acquire waits up to finishingLeaseWait.
const (
	leaseRetryDelay    = 10 * time.Millisecond
	leaseProbeWait     = 200 * time.Millisecond
	finishingLeaseWait = 10 * time.Second
)

// Acquire takes the lease of dir, a project directory, and records the
// supervisor PID. The returned function releases them. It fails with
// ErrAlreadyRunning while another supervisor holds the lease.
func Acquire(dir string, fileMode os.FileMode) (func(), error) {
	lease, err := os.OpenFile(LockPath(dir), os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return nil, fmt.Errorf("failed to open server lock: %w", err)
	}
	start := time.Now()
	for {
		err = syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		limit := leaseProbeWait
		if _, statErr := os.Lstat(PIDPath(dir)); errors.Is(statErr, os.ErrNotExist) {
			limit = finishingLeaseWait
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || time.Since(start) >= limit {
			_ = lease.Close()
			if errors.Is(err, syscall.EWOULDBLOCK) {
				return nil, ErrAlreadyRunning
			}
			return nil, fmt.Errorf("failed to lock server lease: %w", err)
		}
		time.Sleep(leaseRetryDelay)
	}
	if err := os.WriteFile(PIDPath(dir), []byte(strconv.Itoa(os.Getpid())+"\n"), fileMode); err != nil {
		_ = lease.Close()
		return nil, fmt.Errorf("failed to write server pid: %w", err)
	}
	return func() {
		_ = os.Remove(PIDPath(dir))
		_ = lease.Close()
	}, nil
}

// Withdraw removes the PID record of dir's supervisor, whose run has ended,
// before the run lock is removed and the run's clients return. The
// supervisor keeps its lease until it exits, and a supervisor starting
// meanwhile waits for it instead of reporting that a run is starting or
// running.
func Withdraw(dir string) error {
	if err := os.Remove(PIDPath(dir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove server pid: %w", err)
	}
	return nil
}

// Running reports whether a supervisor holds the lease of dir, a project
// directory, and its recorded PID, which is 0 while it is being written.
// It never creates the lease file.
func Running(dir string) (int, bool) {
	lease, err := os.Open(LockPath(dir))
	if err != nil {
		return 0, false
	}
	defer lease.Close()
	if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		if err == nil {
			_ = syscall.Flock(int(lease.Fd()), syscall.LOCK_UN)
		}
		return 0, false
	}
	data, err := os.ReadFile(PIDPath(dir))
	if err != nil {
		return 0, true
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, true
	}
	return pid, true
}
