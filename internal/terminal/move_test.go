package terminal

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// movingKeeper is a Keeper of shell, with rotari's integration, and limits for moving.
func movingKeeper(t *testing.T, shell string, env ...string) *Keeper {
	t.Helper()
	path, err := exec.LookPath(shell)
	if err != nil {
		t.Skipf("no %s here", shell)
	}
	useCache(t)
	home := t.TempDir()
	// zsh asks a new user to set it up without a .zshrc, and the host's /etc/zshrc may run
	// compinit and ask about insecure directories, taking what the test types as the answer.
	writeFiles(t, home, map[string]string{".zshenv": "unsetopt GLOBAL_RCS\n", ".zshrc": "PS1='$ '\n"})
	keeper, err := NewKeeper(KeeperOptions{
		Shell:    Options{Shell: path, Dir: home, Env: append([]string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "PS1=$ "}, env...)},
		RingSize: 1 << 16, ShellLimit: 4, GroupLimit: 2, ShellIntegration: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(keeper.Close)
	return keeper
}

// Move puts a shell after the shells of its new group, with the lowest number free there,
// keeping its name, within that group's limit; its directories from then on are the new
// group's. A shell without rotari's integration does not move.
func TestMoveTakesAShellToAnotherGroup(t *testing.T) {
	keeper := movingKeeper(t, "bash")
	start := func(group string) ShellInfo {
		t.Helper()
		shell, err := keeper.Start(StartOptions{Rows: 24, Cols: 80, Group: group, Dir: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		if !shell.FollowsMoves {
			t.Fatalf("a bash with the integration does not follow moves: %+v", shell)
		}
		return shell
	}
	a1, a2, b2 := start("a"), start("a"), start("b")
	if _, err := keeper.Rename(a1.ID, "mine"); err != nil {
		t.Fatal(err)
	}
	order := func() []string {
		var got []string
		for _, shell := range keeper.Shells() {
			got = append(got, shell.Group+":"+shell.ID)
		}
		return got
	}
	moved, err := keeper.Move(a1.ID, "b", []string{"ROTARI_PROJECT_NAME=b"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Group != "b" || moved.Number != 2 || moved.Name != "mine" {
		t.Errorf("moved = %+v, want group b, number 2, its name kept", moved)
	}
	if got, want := order(), []string{"a:" + a2.ID, "b:" + b2.ID, "b:" + a1.ID}; !slices.Equal(got, want) {
		t.Errorf("order after the move = %q, want %q", got, want)
	}
	// Where it went before stays its old group's history.
	if !slices.ContainsFunc(keeper.History("a"), func(entry DirEntry) bool { return entry.Dir == mustResolve(t, a1.Dir) }) ||
		slices.ContainsFunc(keeper.History("b"), func(entry DirEntry) bool { return entry.Dir == mustResolve(t, a1.Dir) }) {
		t.Errorf("histories after the move: a %v, b %v", keeper.History("a"), keeper.History("b"))
	}
	// Its own group: nothing changes.
	if same, err := keeper.Move(a1.ID, "b", nil, nil); err != nil || same.Number != 2 {
		t.Errorf("moving to its own group = %+v, %v", same, err)
	}
	if _, err := keeper.Move(a2.ID, "b", nil, nil); !errors.Is(err, ErrTooManyInGroup) {
		t.Errorf("past the group limit = %v, want ErrTooManyInGroup", err)
	}
	if _, err := keeper.Move("gone", "b", nil, nil); !errors.Is(err, ErrNoShell) {
		t.Errorf("an unknown shell = %v, want ErrNoShell", err)
	}
	for _, bad := range []struct {
		set, unset []string
	}{
		{set: []string{"PATH=/x"}},
		{set: []string{"ROTARI_PROJECT_NAME"}},
		{set: []string{"ROTARI_x=1"}},
		{set: []string{"ROTARI_PROJECT_NAME=a\nb"}},
		{unset: []string{"HOME"}},
	} {
		if _, err := keeper.Move(a2.ID, "c", bad.set, bad.unset); err == nil {
			t.Errorf("Move with set %q, unset %q succeeded", bad.set, bad.unset)
		}
	}
	if got := keeper.Shells()[0]; got.ID != a2.ID || got.Group != "a" {
		t.Errorf("a refused move moved the shell: %+v", got)
	}
	// A shell moved into an emptied group takes its first number.
	if again, err := keeper.Move(a2.ID, "c", nil, []string{"ROTARI_PROJECT_NAME"}); err != nil || again.Number != 1 {
		t.Errorf("moving into an empty group = %+v, %v", again, err)
	}

	plain := movingKeeper(t, "sh")
	shell, err := plain.Start(StartOptions{Rows: 24, Cols: 80, Group: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if shell.FollowsMoves {
		t.Error("sh follows moves")
	}
	if _, err := plain.Move(shell.ID, "b", nil, nil); !errors.Is(err, ErrCannotMove) {
		t.Errorf("moving sh = %v, want ErrCannotMove", err)
	}
}

func mustResolve(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// A shell started with the place file of a shell the server runs in does not take it.
func TestShellsDoNotInheritAPlaceFile(t *testing.T) {
	keeper := movingKeeper(t, "bash", placeVariable+"=/outer/place", placeApplied+"=7")
	info, err := keeper.Start(StartOptions{Rows: 24, Cols: 80, Group: "a"})
	if err != nil {
		t.Fatal(err)
	}
	watch := watchMoving(t, keeper, info.ID)
	watch.waitFor(t, "the first prompt", "\x1b]133;B\a")
	if got := watch.ask(t, `"$ROTARI_TERMINAL_PLACE:${ROTARI_TERMINAL_PLACE_APPLIED-none}"`); got != filepath.Join(keeper.placeDir, info.ID)+":none" {
		t.Errorf("the shell's place file = %q, want its own and nothing applied", got)
	}
}

// watchedShell is an attached shell's output as it arrives.
type watchedShell struct {
	attachment *Attachment
	mu         sync.Mutex
	output     strings.Builder
	asked      int
}

func watchMoving(t *testing.T, keeper *Keeper, id string) *watchedShell {
	t.Helper()
	attachment, err := keeper.Attach(id, 24, 200)
	if err != nil {
		t.Fatal(err)
	}
	watch := &watchedShell{attachment: attachment}
	go func() {
		for {
			output, ok := attachment.Next()
			watch.mu.Lock()
			watch.output.Write(output)
			watch.mu.Unlock()
			if !ok {
				return
			}
		}
	}()
	return watch
}

func (watch *watchedShell) text() string {
	watch.mu.Lock()
	defer watch.mu.Unlock()
	return watch.output.String()
}

func (watch *watchedShell) waitFor(t *testing.T, what, text string) {
	t.Helper()
	eventually(t, what, func() (bool, string) {
		got := watch.text()
		return strings.Contains(got, text), got
	})
}

func (watch *watchedShell) typeLine(t *testing.T, line string) {
	t.Helper()
	if _, err := watch.attachment.Write([]byte(line + "\n")); err != nil {
		t.Fatal(err)
	}
}

// ask types a command that prints value, a shell word, and returns what it printed. The
// answer is told apart from the line typed, which shows the word unexpanded.
func (watch *watchedShell) ask(t *testing.T, value string) string {
	t.Helper()
	watch.asked++
	tag := "answer" + strings.Repeat("x", watch.asked)
	watch.typeLine(t, "printf '%s=[%s]\\n' "+tag+" "+value)
	pattern := regexp.MustCompile(regexp.QuoteMeta(tag) + `=\[([^\]]*)\]\r?\n`)
	var got string
	eventually(t, "the answer to "+value, func() (bool, string) {
		text := watch.text()
		match := pattern.FindStringSubmatch(text)
		if match == nil {
			return false, text
		}
		got = match[1]
		return true, text
	})
	return got
}

// A moved bash or zsh takes its new place's variables before the next command, typed at the
// prompt it waited at, and when a command ends; it takes each move once, so that what is set
// by hand afterwards stays; a shell started inside takes a move too, and so does the outer
// one when it comes back. Values are assigned as they are, never evaluated.
func TestMovedShellTakesItsPlaceVariables(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			keeper := movingKeeper(t, shell)
			info, err := keeper.Start(StartOptions{Rows: 24, Cols: 200, Group: "b\x00p1",
				Env: []string{"ROTARI_BASEDIR=/b", "ROTARI_PROJECT_NAME=p1"}})
			if err != nil {
				t.Fatal(err)
			}
			watch := watchMoving(t, keeper, info.ID)
			watch.waitFor(t, "the first prompt", "\x1b]133;B\a")
			project := `"$ROTARI_BASEDIR:${ROTARI_PROJECT_NAME-unset}"`
			if got := watch.ask(t, project); got != "/b:p1" {
				t.Fatalf("before a move: %q", got)
			}
			move := func(group string, set, unset []string) {
				t.Helper()
				// The prompt after the last answer has been drawn.
				time.Sleep(200 * time.Millisecond)
				if _, err := keeper.Move(info.ID, group, set, unset); err != nil {
					t.Fatal(err)
				}
			}
			move("b\x00p2", []string{"ROTARI_PROJECT_NAME=p 2;$(touch evaluated)`id`"}, nil)
			if got := watch.ask(t, project); got != "/b:p 2;$(touch evaluated)`id`" {
				t.Errorf("after a move at the prompt: %q", got)
			}
			if _, err := os.Stat(filepath.Join(keeper.options.Shell.Dir, "evaluated")); err == nil {
				t.Error("a value was evaluated")
			}
			// To a basedir's own shells: the project unset.
			move("c\x00", []string{"ROTARI_BASEDIR=/c"}, []string{"ROTARI_PROJECT_NAME"})
			if got := watch.ask(t, project); got != "/c:unset" {
				t.Errorf("after a move to a basedir's own: %q", got)
			}
			// What is set by hand after a move stays.
			watch.typeLine(t, "export ROTARI_PROJECT_NAME=mine")
			if got := watch.ask(t, project); got != "/c:mine" {
				t.Errorf("a value set by hand after the move: %q", got)
			}
			// A move while a command runs: that command keeps the old place, and the shell
			// takes the new one when it ends.
			watch.typeLine(t, "sleep 1; printf 'slept=[%s]\\n' \"$ROTARI_PROJECT_NAME\"")
			time.Sleep(300 * time.Millisecond)
			if _, err := keeper.Move(info.ID, "c\x00p3", []string{"ROTARI_PROJECT_NAME=p3"}, nil); err != nil {
				t.Fatal(err)
			}
			watch.waitFor(t, "the command's end", "slept=[mine]")
			if got := watch.ask(t, project); got != "/c:p3" {
				t.Errorf("after a move while a command ran: %q", got)
			}
			// A shell started inside takes a move, and the outer one when it comes back.
			watch.typeLine(t, shell)
			// What is typed meanwhile waits for the inner shell to read it.
			time.Sleep(500 * time.Millisecond)
			if got := watch.ask(t, `"${ROTARI_TERMINAL_PLACE_APPLIED-}"`); got != "3" {
				t.Errorf("the inner shell inherited move %q, want 3", got)
			}
			move("c\x00p4", []string{"ROTARI_PROJECT_NAME=p4"}, nil)
			if got := watch.ask(t, project); got != "/c:p4" {
				t.Errorf("the inner shell after a move: %q", got)
			}
			watch.typeLine(t, "exit")
			time.Sleep(300 * time.Millisecond)
			if got := watch.ask(t, project); got != "/c:p4" {
				t.Errorf("the outer shell after the inner one took the move: %q", got)
			}
		})
	}
}
