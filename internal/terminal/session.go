// Package terminal runs interactive shells on pseudo-terminals. It knows nothing
// of HTTP, jobs, or runs.
package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// termType is what the shell's programs are told the terminal understands.
const termType = "xterm-256color"

// closeGrace is how long a shell has to exit after its terminal closes.
var closeGrace = 5 * time.Second

// Options says how to start a shell.
type Options struct {
	// Shell is the program to run, as a login shell.
	Shell string
	// Integration, if set, is a directory WriteIntegration filled: bash and zsh then
	// start with its files, which read the user's and mark prompts and commands.
	Integration string
	// Dir is the working directory.
	Dir string
	// Env is the shell's whole environment; nothing is inherited, so a caller
	// cannot pass on a secret by leaving it out. TERM is always set.
	Env []string
	// Rows and Cols are the terminal's first size, both above zero.
	Rows, Cols uint16
}

// Result says how a shell ended.
type Result struct {
	// ExitCode is the shell's exit status, or -1 when a signal ended it.
	ExitCode int
	// Signal is the signal that ended the shell, or 0.
	Signal syscall.Signal
}

// String describes how the shell ended.
func (result Result) String() string {
	if result.Signal != 0 {
		return "signal " + result.Signal.String()
	}
	return "exit status " + strconv.Itoa(result.ExitCode)
}

// Session is a started shell and its terminal. Read and Write may run at the
// same time as each other and as Resize and Close.
type Session struct {
	terminal  *os.File
	command   *exec.Cmd
	done      chan struct{}
	result    Result
	closeOnce sync.Once
}

// Start starts a shell in a session of its own, on a new terminal.
func Start(options Options) (*Session, error) {
	if options.Shell == "" {
		return nil, errors.New("no shell to start")
	}
	if options.Rows == 0 || options.Cols == 0 {
		return nil, fmt.Errorf("invalid terminal size %dx%d", options.Cols, options.Rows)
	}
	args, env := shellCommand(options)
	command := exec.Command(options.Shell, args...)
	command.Dir = options.Dir
	command.Env = append(env, "TERM="+termType)
	started, err := pty.StartWithSize(command, &pty.Winsize{Rows: options.Rows, Cols: options.Cols})
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", options.Shell, err)
	}
	terminal, err := pollable(started)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("prepare the terminal of %s: %w", options.Shell, err)
	}
	session := &Session{terminal: terminal, command: command, done: make(chan struct{})}
	go session.wait()
	return session, nil
}

// pollable returns file as a File whose Close interrupts a Read in progress and
// really closes the terminal. The pty package leaves its file blocking, and
// closing a blocking file waits for its Read to end, which only the hangup that
// closing would send can bring about. Session must therefore never call Fd on
// the result, which would make it blocking again.
func pollable(file *os.File) (*os.File, error) {
	defer file.Close()
	descriptor, err := unix.Dup(int(file.Fd()))
	if err != nil {
		return nil, err
	}
	unix.CloseOnExec(descriptor)
	if err := unix.SetNonblock(descriptor, true); err != nil {
		_ = unix.Close(descriptor)
		return nil, err
	}
	return os.NewFile(uintptr(descriptor), file.Name()), nil
}

func (session *Session) wait() {
	defer close(session.done)
	err := session.command.Wait()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		session.result = Result{ExitCode: -1}
		return
	}
	state := session.command.ProcessState
	session.result = Result{ExitCode: state.ExitCode()}
	if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		session.result.Signal = status.Signal()
	}
}

// Read reads what the shell and its programs wrote. It returns io.EOF once the
// terminal has closed or nothing still has it open; Linux reports the latter as
// EIO, which is not an error here.
func (session *Session) Read(buffer []byte) (int, error) {
	count, err := session.terminal.Read(buffer)
	if errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed) {
		err = io.EOF
	}
	return count, err
}

// Write sends input to the shell, as typing would.
func (session *Session) Write(input []byte) (int, error) {
	return session.terminal.Write(input)
}

// Resize changes the terminal's size, which tells the running program.
func (session *Session) Resize(rows, cols uint16) error {
	if rows == 0 || cols == 0 {
		return fmt.Errorf("invalid terminal size %dx%d", cols, rows)
	}
	connection, err := session.terminal.SyscallConn()
	if err != nil {
		return err
	}
	var resizeErr error
	if err := connection.Control(func(descriptor uintptr) {
		resizeErr = unix.IoctlSetWinsize(int(descriptor), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols})
	}); err != nil {
		return err
	}
	return resizeErr
}

// Dir is the shell's working directory now, which programs it runs do not change.
func (session *Session) Dir() (string, error) {
	select {
	case <-session.done:
		return "", errors.New("the shell has exited")
	default:
	}
	return processDir(session.command.Process.Pid)
}

// Foreground is the command line of the program the shell runs in the foreground, or
// "" while the shell itself is, at its prompt. A pipeline gives its first program.
func (session *Session) Foreground() (string, error) {
	group, err := session.foregroundGroup()
	if err != nil {
		return "", err
	}
	if group == session.command.Process.Pid {
		return "", nil
	}
	return processCommand(group)
}

// Process is a process a shell started, directly or through others.
type Process struct {
	PID, PPID int
	// Depth is how far below the shell it is: 1 for the shell's own children.
	Depth   int
	Command string
	// group is its process group; sessionLeader and hasTerminal say whether it leads
	// its session and has a controlling terminal, and terminalGroup is that terminal's
	// foreground process group. ServerShells reads them.
	group, terminalGroup       int
	sessionLeader, hasTerminal bool
}

// Descendants are the processes below the shell, its children first, each followed by
// its own. A process that left the shell's tree, as a daemon does, is not among them.
func (session *Session) Descendants() ([]Process, error) {
	select {
	case <-session.done:
		return nil, errors.New("the shell has exited")
	default:
	}
	all, err := processes()
	if err != nil {
		return nil, err
	}
	return descendantsOf(all, session.command.Process.Pid), nil
}

// busy says something other than the shell holds the terminal's foreground: a command
// it runs, as opposed to its prompt.
func (session *Session) busy() (bool, error) {
	group, err := session.foregroundGroup()
	return err == nil && group != session.command.Process.Pid, err
}

// foregroundGroup is the terminal's foreground process group.
func (session *Session) foregroundGroup() (int, error) {
	select {
	case <-session.done:
		return 0, errors.New("the shell has exited")
	default:
	}
	connection, err := session.terminal.SyscallConn()
	if err != nil {
		return 0, err
	}
	var group int
	var groupErr error
	if err := connection.Control(func(descriptor uintptr) {
		group, groupErr = unix.IoctlGetInt(int(descriptor), unix.TIOCGPGRP)
	}); err != nil {
		return 0, err
	}
	if groupErr != nil {
		return 0, fmt.Errorf("find the terminal's foreground program: %w", groupErr)
	}
	return group, nil
}

// Done is closed when the shell has exited.
func (session *Session) Done() <-chan struct{} {
	return session.done
}

// Result is how the shell ended; it is valid once Done is closed.
func (session *Session) Result() Result {
	return session.result
}

// Close closes the terminal, which sends the shell and its foreground program
// SIGHUP, and returns once the shell has exited. A shell still running after
// closeGrace is killed with its process group. What has left the shell's session
// or ignores SIGHUP, such as a nohup command or a rotari supervisor, keeps running,
// as after closing any terminal.
func (session *Session) Close() {
	session.closeOnce.Do(func() {
		_ = session.terminal.Close()
		select {
		case <-session.done:
		case <-time.After(closeGrace):
			_ = syscall.Kill(-session.command.Process.Pid, syscall.SIGKILL)
		}
	})
	<-session.done
}
