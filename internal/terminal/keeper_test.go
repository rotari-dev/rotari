package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestKeeper(t *testing.T, idle time.Duration, ring int) *Keeper {
	t.Helper()
	return newTestKeeperWithShell(t, "/bin/sh", idle, ring)
}

func newTestKeeperWithShell(t *testing.T, shell string, idle time.Duration, ring int) *Keeper {
	t.Helper()
	home := t.TempDir()
	keeper, err := NewKeeper(KeeperOptions{
		Shell:      Options{Shell: shell, Dir: home, Env: []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "PS1=$ "}},
		IdleLimit:  idle,
		RingSize:   ring,
		ShellLimit: testShellLimit,
		GroupLimit: testShellLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	return keeper
}

// aShell is the first running shell, started if there is none.
func aShell(t *testing.T, keeper *Keeper) string {
	t.Helper()
	if shells := keeper.Shells(); len(shells) > 0 {
		return shells[0].ID
	}
	shell, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	return shell.ID
}

// watcher collects an attachment's output until it ends.
type watcher struct {
	*Attachment
	mu    sync.Mutex
	text  bytes.Buffer
	ended chan struct{}
}

func watch(t *testing.T, keeper *Keeper) *watcher {
	t.Helper()
	return watchShell(t, keeper, aShell(t, keeper))
}

func watchShell(t *testing.T, keeper *Keeper, id string) *watcher {
	t.Helper()
	attachment, err := keeper.Attach(id, 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	seen := &watcher{Attachment: attachment, ended: make(chan struct{})}
	go func() {
		defer close(seen.ended)
		for {
			output, ok := attachment.Next()
			if !ok {
				return
			}
			seen.mu.Lock()
			seen.text.Write(output)
			seen.mu.Unlock()
		}
	}()
	return seen
}

func (seen *watcher) shown() string {
	seen.mu.Lock()
	defer seen.mu.Unlock()
	return seen.text.String()
}

// await waits until the output has want in it, on a line of the program's own: the
// terminal also echoes what was typed.
func (seen *watcher) await(t *testing.T, want string) {
	t.Helper()
	for deadline := time.Now().Add(testWait); !strings.Contains(seen.shown(), want+"\r\n"); {
		if time.Now().After(deadline) {
			t.Fatalf("output never showed %q:\n%s", want, seen.shown())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// run types a command, built so that its output differs from its echo, and waits.
func (seen *watcher) run(t *testing.T, command, want string) {
	t.Helper()
	if _, err := fmt.Fprintf(seen, "%s\n", command); err != nil {
		t.Fatal(err)
	}
	seen.await(t, want)
}

func (seen *watcher) awaitEnd(t *testing.T, want DetachReason) {
	t.Helper()
	select {
	case <-seen.ended:
	case <-time.After(testWait):
		t.Fatal("the attachment did not end")
	}
	if got := seen.Reason(); got != want {
		t.Errorf("Reason() = %v, want %v", got, want)
	}
}

const (
	testIdle       = time.Hour
	testShellLimit = 3
)

func TestNewKeeperRejectsBadOptions(t *testing.T) {
	for name, options := range map[string]KeeperOptions{
		"no idle limit":  {RingSize: 1, ShellLimit: 1, GroupLimit: 1},
		"no ring":        {IdleLimit: time.Second, ShellLimit: 1, GroupLimit: 1},
		"no shell limit": {IdleLimit: time.Second, RingSize: 1, GroupLimit: 1},
		"no group limit": {IdleLimit: time.Second, RingSize: 1, ShellLimit: 1},
	} {
		if _, err := NewKeeper(options); err == nil {
			t.Errorf("%s: NewKeeper succeeded", name)
		}
	}
}

func TestAttachReplaysRecentOutputOfTheSameShell(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	first := watch(t, keeper)
	first.run(t, `echo "one-$((1+1))"; VALUE=kept`, "one-2")
	first.Detach()
	first.awaitEnd(t, DetachedByCaller)

	second := watch(t, keeper)
	second.await(t, "one-2")
	second.run(t, `echo "value-$VALUE"`, "value-kept")
}

func TestNewAttachmentTakesOverAndOldOneCannotType(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	first := watch(t, keeper)
	first.run(t, `echo "a-$((1+1))"`, "a-2")
	second := watch(t, keeper)
	first.awaitEnd(t, TakenOver)
	if _, err := first.Write([]byte("echo stale\n")); !errors.Is(err, ErrDetached) {
		t.Errorf("Write after take-over = %v, want ErrDetached", err)
	}
	if err := first.Resize(10, 10); !errors.Is(err, ErrDetached) {
		t.Errorf("Resize after take-over = %v, want ErrDetached", err)
	}
	first.Detach() // Must not disturb the attachment that took over.
	second.run(t, `echo "b-$((1+1))"`, "b-2")
}

// Reconnecting by itself must not take the shell from whoever is using it.
func TestAttachIfFreeLeavesAnotherAttachmentAlone(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	first := watch(t, keeper)
	first.run(t, `VALUE=kept; echo "one-$((1+1))"`, "one-2")
	id := aShell(t, keeper)
	if _, err := keeper.AttachIfFree(id, 24, 80); !errors.Is(err, ErrAttachedElsewhere) {
		t.Fatalf("AttachIfFree with another attached = %v", err)
	}
	first.run(t, `echo "still-$((1+1))"`, "still-2")
	first.Detach()
	first.awaitEnd(t, DetachedByCaller)
	second, err := keeper.AttachIfFree(id, 24, 80)
	if err != nil {
		t.Fatalf("AttachIfFree with nothing attached = %v", err)
	}
	second.Detach()
}

func TestShellExitEndsAttachmentAndTheShellIsGone(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	id := aShell(t, keeper)
	first := watch(t, keeper)
	first.run(t, `echo "old-$((1+1))"`, "old-2")
	if _, err := first.Write([]byte("exit 3\n")); err != nil {
		t.Fatal(err)
	}
	first.awaitEnd(t, ShellExited)
	if result := first.Result(); result.ExitCode != 3 {
		t.Errorf("Result() = %+v, want exit 3", result)
	}
	if _, err := keeper.Attach(id, 24, 80); !errors.Is(err, ErrNoShell) {
		t.Errorf("Attach to the ended shell = %v, want ErrNoShell", err)
	}
	if shells := keeper.Shells(); len(shells) != 0 {
		t.Errorf("Shells() = %v, want none", shells)
	}
	second := watch(t, keeper)
	second.run(t, `echo "new-$((1+1))"`, "new-2")
	if strings.Contains(second.shown(), "old-2") {
		t.Errorf("a new shell replayed the old one's output:\n%s", second.shown())
	}
}

// Output printed just before the shell exits still arrives.
func TestLastOutputOfAnExitingShellArrives(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	seen := watch(t, keeper)
	if _, err := fmt.Fprintf(seen, "echo \"last-$((1+1))\"; exit 0\n"); err != nil {
		t.Fatal(err)
	}
	seen.awaitEnd(t, ShellExited)
	if !strings.Contains(seen.shown(), "last-2\r\n") {
		t.Errorf("the last output is missing:\n%s", seen.shown())
	}
}

func TestUnattendedShellEndsAfterTheIdleLimit(t *testing.T) {
	const idle = 150 * time.Millisecond
	keeper := newTestKeeper(t, idle, 1<<16)
	first := watch(t, keeper)
	first.run(t, `VALUE=first; echo "up-$((1+1))"`, "up-2")
	first.Detach()
	first.awaitEnd(t, DetachedByCaller)
	shell := first.shell
	select {
	case <-shell.finished:
	case <-time.After(testWait):
		t.Fatal("the shell outlived the idle limit")
	}
	second := watch(t, keeper)
	second.run(t, `echo "value-[$VALUE]"`, "value-[]")
}

func TestAttachingStopsTheIdleLimit(t *testing.T) {
	const idle = 200 * time.Millisecond
	keeper := newTestKeeper(t, idle, 1<<16)
	first := watch(t, keeper)
	first.run(t, `VALUE=kept; echo "up-$((1+1))"`, "up-2")
	first.Detach()
	second := watch(t, keeper)
	time.Sleep(2 * idle)
	second.run(t, `echo "value-$VALUE"`, "value-kept")
}

// A burst must reach a reader in full, however fast the program writes.
func TestAttachedReaderMissesNothingInABurst(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<12)
	seen := watch(t, keeper)
	const lines = 1000000
	seen.run(t, fmt.Sprintf(`yes line | head -n %d; echo "done-$((1+1))"`, lines), "done-2")
	select {
	case <-seen.ended:
		t.Fatalf("the reader was detached: %v", seen.Reason())
	default:
	}
	if got := strings.Count(seen.shown(), "line\r\n"); got != lines {
		t.Errorf("the reader saw %d lines, want %d", got, lines)
	}
}

// A reader that falls behind slows the program down rather than losing output.
func TestSlowReaderMakesTheProgramWait(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<12)
	attachment, err := keeper.Attach(aShell(t, keeper), 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	finished := filepath.Join(t.TempDir(), "finished")
	// Far more than the terminal and pendingLimit hold.
	if _, err := fmt.Fprintf(attachment, "yes line | head -c %d; touch %q\n", 8*pendingLimit, finished); err != nil {
		t.Fatal(err)
	}
	// Output stops piling up near the limit, rather than the program running on.
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		keeper.mu.Lock()
		pending := len(attachment.pending)
		keeper.mu.Unlock()
		if pending > pendingLimit+outputChunk {
			t.Fatalf("%d bytes piled up with nothing reading them; the limit is %d", pending, pendingLimit)
		}
	}
	if _, err := os.Stat(finished); err == nil {
		t.Fatal("the program finished although nothing read its output")
	}
	if !attachment.current() {
		t.Fatal("the reader was detached before stallLimit")
	}
	total := 0
	for deadline := time.Now().Add(testWait); ; {
		output, ok := attachment.Next()
		if !ok {
			t.Fatalf("the attachment ended: %v", attachment.Reason())
		}
		total += len(output)
		if _, err := os.Stat(finished); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the program did not finish once the reader read")
		}
	}
	if total < 8*pendingLimit {
		t.Errorf("the reader saw %d bytes, want at least %d", total, 8*pendingLimit)
	}
}

// A reader that stops altogether, such as a dead connection, must not hold the
// program for good: it is detached, and the program goes on.
func TestStalledReaderIsDetachedAndTheProgramContinues(t *testing.T) {
	limit := stallLimit
	stallLimit = 200 * time.Millisecond
	t.Cleanup(func() { stallLimit = limit })
	keeper := newTestKeeper(t, testIdle, 1<<12)
	attachment, err := keeper.Attach(aShell(t, keeper), 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	finished := filepath.Join(t.TempDir(), "finished")
	if _, err := fmt.Fprintf(attachment, "VALUE=kept; yes line | head -c %d; touch %q\n", 8*pendingLimit, finished); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(testWait); attachment.current(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the stalled reader stayed attached")
		}
	}
	for deadline := time.Now().Add(testWait); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(finished); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the program stayed stalled after the reader was detached")
		}
	}
	// What was waiting can still be taken before the end is reported.
	for {
		if _, ok := attachment.Next(); !ok {
			break
		}
	}
	if got := attachment.Reason(); got != TooSlow {
		t.Errorf("Reason() = %v, want TooSlow", got)
	}
	second := watch(t, keeper)
	second.run(t, `echo "value-$VALUE"`, "value-kept")
}

// With nothing attached, output is still read, so a program never stalls.
func TestProgramsDoNotStallWithNothingAttached(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<12)
	finished := filepath.Join(t.TempDir(), "finished")
	first := watch(t, keeper)
	first.run(t, `echo "up-$((1+1))"`, "up-2")
	if _, err := fmt.Fprintf(first, "yes line | head -c 5000000; touch %q\n", finished); err != nil {
		t.Fatal(err)
	}
	first.Detach()
	for deadline := time.Now().Add(testWait); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(finished); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the program stalled with nothing attached")
		}
	}
}

// Attaching tells a running program to redraw, even at an unchanged size.
func TestAttachMakesARunningProgramRedraw(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not installed")
	}
	keeper := newTestKeeperWithShell(t, bash, testIdle, 1<<16)
	first := watch(t, keeper)
	first.run(t, `trap 'echo "win""ched"' WINCH; echo "set-$((1+1))"`, "set-2")
	first.Detach()
	second := watch(t, keeper)
	// Bash runs a trap when it next returns to its prompt.
	if _, err := fmt.Fprintf(second, ":\n"); err != nil {
		t.Fatal(err)
	}
	second.await(t, "winched")
	second.run(t, `echo "size-$(stty size)"`, "size-24 80")
}

func TestEndAndCloseEndTheShell(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	id := aShell(t, keeper)
	first := watch(t, keeper)
	first.run(t, `VALUE=first; echo "up-$((1+1))"`, "up-2")
	if err := keeper.End(id); err != nil {
		t.Fatal(err)
	}
	first.awaitEnd(t, ShellExited)
	if err := keeper.End(id); !errors.Is(err, ErrNoShell) {
		t.Errorf("End of an ended shell = %v, want ErrNoShell", err)
	}
	second := watch(t, keeper)
	second.run(t, `echo "value-[$VALUE]"`, "value-[]")
	keeper.Close()
	keeper.Close()
	second.awaitEnd(t, KeeperClosed)
	select {
	case <-second.shell.finished:
	default:
		t.Error("Close returned with the shell still running")
	}
	if _, err := keeper.Attach(id, 24, 80); !errors.Is(err, ErrKeeperClosed) {
		t.Errorf("Attach after Close = %v, want ErrKeeperClosed", err)
	}
	if _, err := keeper.Start(StartOptions{Rows: 24, Cols: 80}); !errors.Is(err, ErrKeeperClosed) {
		t.Errorf("Start after Close = %v, want ErrKeeperClosed", err)
	}
}

func TestStartReportsAFailureAndKeepsNoShell(t *testing.T) {
	keeper, err := NewKeeper(KeeperOptions{Shell: Options{Shell: filepath.Join(t.TempDir(), "missing"), Dir: t.TempDir()}, IdleLimit: testIdle, RingSize: 1, ShellLimit: 1, GroupLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	if _, err := keeper.Start(StartOptions{Rows: 24, Cols: 80}); err == nil {
		t.Fatal("Start succeeded with a missing shell")
	}
	if shells := keeper.Shells(); len(shells) != 0 {
		t.Errorf("a failed start left %v behind", shells)
	}
}

func TestStartAndAttachRejectABadSize(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	if _, err := keeper.Start(StartOptions{Rows: 0, Cols: 80}); err == nil {
		t.Error("Start accepted no rows")
	}
	id := aShell(t, keeper)
	if _, err := keeper.Attach(id, 0, 80); err == nil {
		t.Error("the first Attach accepted no rows")
	}
	first := watch(t, keeper)
	first.run(t, `echo "up-$((1+1))"`, "up-2")
	if _, err := keeper.Attach(id, 24, 0); err == nil {
		t.Error("a later Attach accepted no columns")
	}
	first.run(t, `echo "still-$((1+1))"`, "still-2")
}

// Each shell has its own state, output, and take-over.
func TestShellsAreSeparate(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	one, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	two, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	first, second := watchShell(t, keeper, one.ID), watchShell(t, keeper, two.ID)
	first.run(t, `VALUE=first; echo "one-$((1+1))"`, "one-2")
	second.run(t, `VALUE=second; echo "two-$((1+1))"`, "two-2")
	first.run(t, `echo "value-$VALUE"`, "value-first")
	second.run(t, `echo "value-$VALUE"`, "value-second")
	if strings.Contains(second.shown(), "one-2") {
		t.Errorf("one shell's output reached the other:\n%s", second.shown())
	}

	if err := keeper.End(one.ID); err != nil {
		t.Fatal(err)
	}
	first.awaitEnd(t, ShellExited)
	second.run(t, `echo "still-$((1+1))"`, "still-2")
	if err := keeper.End("no such id"); !errors.Is(err, ErrNoShell) {
		t.Errorf("End of an unknown shell = %v, want ErrNoShell", err)
	}
}

func TestShellsAreNumberedAndLimited(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	var started []ShellInfo
	for range testShellLimit {
		shell, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
		if err != nil {
			t.Fatal(err)
		}
		started = append(started, shell)
	}
	if _, err := keeper.Start(StartOptions{Rows: 24, Cols: 80}); !errors.Is(err, ErrTooManyShells) {
		t.Fatalf("Start past the limit = %v, want ErrTooManyShells", err)
	}
	if err := keeper.End(started[0].ID); err != nil {
		t.Fatal(err)
	}
	// The freed number comes back, under an ID that does not.
	replacement, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	if replacement.Number != 1 || replacement.ID == started[0].ID {
		t.Errorf("the replacement is %+v, after %+v", replacement, started[0])
	}
	var numbers []int
	for _, shell := range keeper.Shells() {
		numbers = append(numbers, shell.Number)
	}
	if want := []int{1, 2, 3}; !slices.Equal(numbers, want) {
		t.Errorf("numbers = %v, want %v", numbers, want)
	}
	keeper.EndAll()
	if shells := keeper.Shells(); len(shells) != 0 {
		t.Errorf("EndAll left %v", shells)
	}
}

// Numbers and the group limit are per group, while the shell limit covers them all.
func TestGroupsNumberAndLimitTheirShells(t *testing.T) {
	home := t.TempDir()
	keeper, err := NewKeeper(KeeperOptions{
		Shell:     Options{Shell: "/bin/sh", Dir: home, Env: []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "PS1=$ "}},
		IdleLimit: testIdle, RingSize: 1 << 16, ShellLimit: 3, GroupLimit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	start := func(group string) ShellInfo {
		t.Helper()
		shell, err := keeper.Start(StartOptions{Rows: 24, Cols: 80, Group: group})
		if err != nil {
			t.Fatal(err)
		}
		return shell
	}
	a1, a2, b1 := start("a"), start("a"), start("b")
	if a1.Number != 1 || a2.Number != 2 || b1.Number != 1 {
		t.Errorf("numbers: %+v %+v %+v", a1, a2, b1)
	}
	if _, err := keeper.Start(StartOptions{Rows: 24, Cols: 80, Group: "c"}); !errors.Is(err, ErrTooManyShells) {
		t.Errorf("past the shell limit = %v, want ErrTooManyShells", err)
	}
	if err := keeper.End(b1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.Start(StartOptions{Rows: 24, Cols: 80, Group: "a"}); !errors.Is(err, ErrTooManyInGroup) {
		t.Errorf("past the group limit = %v, want ErrTooManyInGroup", err)
	}
	if b := start("b"); b.Number != 1 {
		t.Errorf("after the group's only shell ended: %+v", b)
	}
	if shells := keeper.Shells(); shells[0].Group != "a" || shells[1].Group != "a" || shells[2].Group != "b" {
		t.Errorf("shells are not by group: %+v", shells)
	}
}

func TestRenameNamesOneShell(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	one, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.Start(StartOptions{Rows: 24, Cols: 80}); err != nil {
		t.Fatal(err)
	}
	renamed, err := keeper.Rename(one.ID, "build")
	if err != nil || renamed.Name != "build" {
		t.Fatalf("Rename = %+v, %v", renamed, err)
	}
	if shells := keeper.Shells(); shells[0].Name != "build" || shells[1].Name != "" {
		t.Errorf("shells = %+v", shells)
	}
	if _, err := keeper.Rename("no such id", "x"); !errors.Is(err, ErrNoShell) {
		t.Errorf("Rename of an unknown shell = %v, want ErrNoShell", err)
	}
}

// A shell can start elsewhere, and what it is told first stays with its output.
func TestStartPlacesTheShellAndKeepsItsBanner(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	shell, err := keeper.Start(StartOptions{Rows: 24, Cols: 80, Dir: dir, Banner: "first-word\r\n", Env: []string{"ADDED=added", "HOME=/elsewhere"}})
	if err != nil {
		t.Fatal(err)
	}
	if shell.Dir != dir {
		t.Errorf("Dir = %q, want %q", shell.Dir, dir)
	}
	seen := watchShell(t, keeper, shell.ID)
	seen.await(t, "first-word")
	seen.run(t, `echo "in-$(pwd -P)"`, "in-"+dir)
	seen.run(t, `echo "env-$ADDED-$HOME"`, "env-added-/elsewhere")
	seen.Detach()
	seen.awaitEnd(t, DetachedByCaller)
	again := watchShell(t, keeper, shell.ID)
	again.await(t, "first-word")
	if other, err := keeper.Start(StartOptions{Rows: 24, Cols: 80}); err != nil || other.Dir == dir {
		t.Errorf("a shell told nothing: %+v, %v", other, err)
	}
}

// A shell nobody is looking at is not idle while another one is in use, since a
// person watches one at a time. With nothing attached to any, they all end.
func TestIdleLimitAppliesToTheShellsTogether(t *testing.T) {
	const idle = 200 * time.Millisecond
	keeper := newTestKeeper(t, idle, 1<<16)
	background, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	foreground, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	attachment, err := keeper.Attach(foreground.ID, 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * idle)
	if shells := keeper.Shells(); len(shells) != 2 {
		t.Fatalf("with one shell in use, %v are left; %s did not count as in use", shells, background.ID)
	}
	attachment.Detach()
	for deadline := time.Now().Add(testWait); len(keeper.Shells()) != 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%v outlived the idle limit", keeper.Shells())
		}
	}
}

// Someone still there keeps the shells with nothing attached, until they stop saying so.
func TestKeepAliveHoldsTheIdleLimitOff(t *testing.T) {
	const idle = 300 * time.Millisecond
	keeper := newTestKeeper(t, idle, 1<<16)
	if _, err := keeper.Start(StartOptions{Rows: 24, Cols: 80}); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(3 * idle); time.Now().Before(deadline); time.Sleep(idle / 6) {
		keeper.KeepAlive()
	}
	if shells := keeper.Shells(); len(shells) != 1 {
		t.Fatalf("the shell ended with someone there: %v", shells)
	}
	for deadline := time.Now().Add(testWait); len(keeper.Shells()) != 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%v outlived the idle limit once no one said anything", keeper.Shells())
		}
	}
	// With no shells, it starts nothing.
	keeper.KeepAlive()
	if keeper.idleTimer != nil {
		t.Error("KeepAlive with no shells started the idle limit")
	}
}

// The last attached shell ending leaves the others under the idle limit.
func TestIdleLimitStartsWhenTheAttachedShellEnds(t *testing.T) {
	const idle = 150 * time.Millisecond
	keeper := newTestKeeper(t, idle, 1<<16)
	if _, err := keeper.Start(StartOptions{Rows: 24, Cols: 80}); err != nil {
		t.Fatal(err)
	}
	attached, err := keeper.Start(StartOptions{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.Attach(attached.ID, 24, 80); err != nil {
		t.Fatal(err)
	}
	if err := keeper.End(attached.ID); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(testWait); len(keeper.Shells()) != 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%v outlived the idle limit", keeper.Shells())
		}
	}
}

// Cleared output does not come back, while what follows it does.
func TestClearOutputForgetsWhatWouldBeReplayed(t *testing.T) {
	keeper := newTestKeeper(t, testIdle, 1<<16)
	first := watch(t, keeper)
	first.run(t, `echo "old-$((1+1))"`, "old-2")
	if err := first.ClearOutput(); err != nil {
		t.Fatal(err)
	}
	first.run(t, `echo "new-$((1+1))"`, "new-2")
	first.Detach()
	first.awaitEnd(t, DetachedByCaller)
	if err := first.ClearOutput(); !errors.Is(err, ErrDetached) {
		t.Errorf("ClearOutput after Detach = %v, want ErrDetached", err)
	}
	second := watch(t, keeper)
	second.await(t, "new-2")
	if shown := second.shown(); strings.Contains(shown, "old-2") {
		t.Errorf("cleared output came back:\n%s", shown)
	}
}

func TestOutputRingKeepsRecentOutputFromAWholeLineOrCharacter(t *testing.T) {
	for _, test := range []struct {
		name   string
		limit  int
		writes []string
		want   string
	}{
		{"nothing", 8, nil, ""},
		{"under the limit", 8, []string{"ab", "cd"}, "abcd"},
		{"exactly the limit", 4, []string{"abcd"}, "abcd"},
		{"cut, then no line break", 4, []string{"abcdefg"}, "defg"},
		{"cut, from the next line", 8, []string{"123456\nabcdef"}, "abcdef"},
		{"cut, a line break too late to use", 8, []string{"123456789abcd\nef"}, "9abcd\nef"},
		{"cut, inside a character", 3, []string{"xx\u3042y"}, "y"},
		{"data moved, then more", 4, []string{"abcdefghi", "j"}, "ghij"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ring := outputRing{limit: test.limit}
			for _, write := range test.writes {
				ring.write([]byte(write))
			}
			if got := string(ring.contents()); got != test.want {
				t.Errorf("contents() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestOutputRingContentsIsACopy(t *testing.T) {
	ring := outputRing{limit: 8}
	ring.write([]byte("abcd"))
	replay := ring.contents()
	replay[0] = 'X'
	if got := string(ring.contents()); got != "abcd" {
		t.Errorf("contents() after changing a copy = %q", got)
	}
}
