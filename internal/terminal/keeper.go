package terminal

import (
	"bytes"
	"cmp"
	"errors"
	"slices"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// pendingLimit is how much output waits for an attached reader before the
	// program is made to wait too, as a real terminal makes it.
	pendingLimit = 1 << 20
	// outputChunk is the most that one read from the terminal takes.
	outputChunk = 32 * 1024
	// exitFlush is how long output may still arrive after the shell exits. A
	// process that outlives the shell holds the terminal open, so the read
	// would otherwise never end.
	exitFlush = 200 * time.Millisecond
)

// stallLimit is how long a reader may take nothing while the program waits for it.
// After that the reader counts as gone, so a dead connection cannot hold a program
// for long.
var stallLimit = 30 * time.Second

var (
	// ErrKeeperClosed is returned by Start and Attach after Close.
	ErrKeeperClosed = errors.New("terminal keeper is closed")
	// ErrDetached is returned when using an Attachment that has ended.
	ErrDetached = errors.New("terminal attachment has ended")
	// ErrAttachedElsewhere is returned by AttachIfFree when another attachment
	// has the shell.
	ErrAttachedElsewhere = errors.New("terminal is attached elsewhere")
	// ErrNoShell is returned for a shell that is not running, which may have ended.
	ErrNoShell = errors.New("no such shell")
	// ErrTooManyShells is returned by Start at the limit on all shells.
	ErrTooManyShells = errors.New("too many shells are running")
	// ErrTooManyInGroup is returned by Start at the limit on a group's shells.
	ErrTooManyInGroup = errors.New("too many shells are running in this group")
)

// DetachReason says why an Attachment ended.
type DetachReason int

const (
	// DetachedByCaller means Detach was called.
	DetachedByCaller DetachReason = iota
	// TakenOver means a newer Attach replaced this one.
	TakenOver
	// ShellExited means the shell ended; Result says how.
	ShellExited
	// TooSlow means the reader took nothing for stallLimit. Attaching again
	// replays recent output.
	TooSlow
	// KeeperClosed means the Keeper was closed.
	KeeperClosed
)

// KeeperOptions says how a Keeper runs its shells.
type KeeperOptions struct {
	// Shell starts each shell; its Rows and Cols are ignored, since Start gives
	// the size.
	Shell Options
	// IdleLimit is how long shells run with nothing attached to any of them.
	IdleLimit time.Duration
	// RingSize is how many bytes of recent output, per shell, a new Attach replays.
	RingSize int
	// ShellLimit is how many shells may run at once.
	ShellLimit int
	// GroupLimit is how many shells of one group may run at once.
	GroupLimit int
	// Log, if set, is told when a shell starts and ends, and never what it shows.
	Log func(message string)
}

// StartOptions say how to start one shell.
type StartOptions struct {
	Rows, Cols uint16
	// Group is what the shell belongs to, which means nothing to the Keeper beyond
	// its numbers and limit.
	Group string
	// Dir is where it starts, instead of the Keeper's Shell.Dir.
	Dir string
	// Banner is shown before the shell's own output, and replayed with it.
	Banner string
	// Env is added to the Keeper's Shell.Env, overriding what is there.
	Env []string
}

// ShellInfo describes a running shell.
type ShellInfo struct {
	// ID names this shell for as long as the Keeper lasts, and is never reused, so
	// that a stale reference cannot reach a later shell.
	ID string
	// Number is what a person calls it within its group: the lowest not in use when
	// it started.
	Number int
	// Name is what someone has called it, if anyone has.
	Name  string
	Group string
	// Dir is where it started.
	Dir string
	// UnseenOutput says it has printed something that no attachment has shown since one
	// last did. A shell nothing has ever shown does not count: it is only starting.
	UnseenOutput bool
}

// Keeper holds shells, which outlive the attachments that show them: callers come
// and go, and each new one sees recent output. A shell with nothing attached sends
// its output only to its ring, so a program never waits. An attached reader that falls
// behind makes the program wait, so it misses nothing, unless it stalls.
//
// A person watches one shell at a time, so a shell with nothing attached says nothing
// about whether anyone is there. The idle limit therefore applies to them together: it
// runs only while nothing is attached to any shell, restarts on KeepAlive, and then ends
// them all.
type Keeper struct {
	options KeeperOptions

	mu         sync.Mutex
	closed     bool
	lastID     int
	shells     []*keptShell // By group and then Number.
	idleTimer  *time.Timer
	idleSerial int
}

type keptShell struct {
	info     ShellInfo
	session  *Session
	ring     outputRing
	attached *Attachment
	// shown says something has attached before, so a program may be running that
	// needs to redraw.
	shown    bool
	finished chan struct{}
}

// NewKeeper returns a Keeper with no shells yet.
func NewKeeper(options KeeperOptions) (*Keeper, error) {
	if options.IdleLimit <= 0 {
		return nil, errors.New("terminal idle limit must be above zero")
	}
	if options.RingSize <= 0 {
		return nil, errors.New("terminal output ring size must be above zero")
	}
	if options.ShellLimit <= 0 {
		return nil, errors.New("terminal shell limit must be above zero")
	}
	if options.GroupLimit <= 0 {
		return nil, errors.New("terminal group limit must be above zero")
	}
	return &Keeper{options: options}, nil
}

// Start starts a shell at the given size. A shell nobody attaches to falls under the
// idle limit, which starting one restarts.
func (keeper *Keeper) Start(start StartOptions) (ShellInfo, error) {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	if keeper.closed {
		return ShellInfo{}, ErrKeeperClosed
	}
	if len(keeper.shells) >= keeper.options.ShellLimit {
		return ShellInfo{}, ErrTooManyShells
	}
	inGroup := 0
	for _, shell := range keeper.shells {
		if shell.info.Group == start.Group {
			inGroup++
		}
	}
	if inGroup >= keeper.options.GroupLimit {
		return ShellInfo{}, ErrTooManyInGroup
	}
	options := keeper.options.Shell
	options.Rows, options.Cols = start.Rows, start.Cols
	if start.Dir != "" {
		options.Dir = start.Dir
	}
	options.Env = append(slices.Clone(options.Env), start.Env...)
	session, err := Start(options)
	if err != nil {
		return ShellInfo{}, err
	}
	keeper.lastID++
	number := 1
	for _, shell := range keeper.shells {
		if shell.info.Group == start.Group && shell.info.Number == number {
			number++
		}
	}
	shell := &keptShell{
		info:     ShellInfo{ID: strconv.Itoa(keeper.lastID), Number: number, Group: start.Group, Dir: options.Dir},
		session:  session,
		ring:     outputRing{limit: keeper.options.RingSize},
		finished: make(chan struct{}),
	}
	shell.ring.write([]byte(start.Banner))
	keeper.shells = append(keeper.shells, shell)
	slices.SortFunc(keeper.shells, func(a, b *keptShell) int {
		return cmp.Or(cmp.Compare(a.info.Group, b.info.Group), cmp.Compare(a.info.Number, b.info.Number))
	})
	keeper.log("terminal: shell started")
	go keeper.pump(shell)
	keeper.stopIdleTimer()
	keeper.armIdleTimer()
	return shell.info, nil
}

// Shells lists the running shells, by group and then Number.
func (keeper *Keeper) Shells() []ShellInfo {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	infos := make([]ShellInfo, 0, len(keeper.shells))
	for _, shell := range keeper.shells {
		infos = append(infos, shell.info)
	}
	return infos
}

// Limits are how many shells may run at once, and how many of one group.
func (keeper *Keeper) Limits() (shells, inGroup int) {
	return keeper.options.ShellLimit, keeper.options.GroupLimit
}

// DefaultDir is where a shell starts unless told otherwise.
func (keeper *Keeper) DefaultDir() string {
	return keeper.options.Shell.Dir
}

func (keeper *Keeper) find(id string) *keptShell {
	for _, shell := range keeper.shells {
		if shell.info.ID == id {
			return shell
		}
	}
	return nil
}

// Attach shows the shell at the given size, and ends any earlier Attachment to it
// with TakenOver. Next begins with the replay of recent output. A program already
// running is made to redraw itself.
func (keeper *Keeper) Attach(id string, rows, cols uint16) (*Attachment, error) {
	return keeper.attach(id, rows, cols, true)
}

// AttachIfFree is Attach that leaves another attachment alone and returns
// ErrAttachedElsewhere instead. It is for reconnecting by itself, which must not
// take the shell from whoever is using it.
func (keeper *Keeper) AttachIfFree(id string, rows, cols uint16) (*Attachment, error) {
	return keeper.attach(id, rows, cols, false)
}

func (keeper *Keeper) attach(id string, rows, cols uint16, takeOver bool) (*Attachment, error) {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	if keeper.closed {
		return nil, ErrKeeperClosed
	}
	shell := keeper.find(id)
	if shell == nil {
		return nil, ErrNoShell
	}
	if shell.attached != nil && !takeOver {
		return nil, ErrAttachedElsewhere
	}
	resize := redraw
	if !shell.shown {
		// A shell still starting has nothing to redraw, and telling it would have
		// some shells print their prompt twice.
		resize = (*Session).Resize
	}
	if err := resize(shell.session, rows, cols); err != nil {
		return nil, err
	}
	shell.shown = true
	shell.info.UnseenOutput = false
	keeper.stopIdleTimer()
	if shell.attached != nil {
		shell.detach(TakenOver)
	}
	attachment := &Attachment{
		keeper:  keeper,
		shell:   shell,
		pending: shell.ring.contents(),
		wake:    make(chan struct{}, 1),
		room:    make(chan struct{}, 1),
		ended:   make(chan struct{}),
	}
	shell.attached = attachment
	return attachment, nil
}

// redraw sets the size and tells the program even when the size is unchanged: the
// system only signals a change, and replayed output alone may not restore a
// full-screen program.
func redraw(session *Session, rows, cols uint16) error {
	if rows == 0 || cols == 0 {
		return session.Resize(rows, cols)
	}
	nudged := cols + 1
	if nudged == 0 {
		nudged = cols - 1
	}
	if err := session.Resize(rows, nudged); err != nil {
		return err
	}
	return session.Resize(rows, cols)
}

// pump reads the terminal for as long as it can, and then records the shell's end.
func (keeper *Keeper) pump(shell *keptShell) {
	readEnded := make(chan struct{})
	go func() {
		defer close(readEnded)
		buffer := make([]byte, outputChunk)
		for {
			count, err := shell.session.Read(buffer)
			if count > 0 {
				keeper.deliver(shell, buffer[:count])
			}
			if err != nil {
				return
			}
		}
	}()
	<-shell.session.Done()
	select {
	case <-readEnded:
	case <-time.After(exitFlush):
	}
	shell.session.Close()
	<-readEnded
	keeper.finish(shell)
}

// deliver records output and passes it to the attached reader, waiting while that
// reader is behind. Only this shell's goroutine adds its output, so a program waits
// only as long as it does.
func (keeper *Keeper) deliver(shell *keptShell, output []byte) {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	shell.ring.write(output)
	attachment := shell.attached
	if attachment == nil {
		shell.info.UnseenOutput = shell.info.UnseenOutput || shell.shown
		return
	}
	attachment.pending = append(attachment.pending, output...)
	signal(attachment.wake)
	if len(attachment.pending) < pendingLimit {
		return
	}
	stall := time.NewTimer(stallLimit)
	defer stall.Stop()
	for shell.attached == attachment && len(attachment.pending) >= pendingLimit {
		keeper.mu.Unlock()
		select {
		case <-attachment.room:
			keeper.mu.Lock()
			if !stall.Stop() {
				select {
				case <-stall.C:
				default:
				}
			}
			stall.Reset(stallLimit)
		case <-attachment.ended:
			keeper.mu.Lock()
		case <-stall.C:
			keeper.mu.Lock()
			if shell.attached == attachment {
				shell.detach(TooSlow)
				keeper.armIdleTimer()
			}
		}
	}
}

// signal notes an event on a channel with room for one.
func signal(events chan struct{}) {
	select {
	case events <- struct{}{}:
	default:
	}
}

func (keeper *Keeper) finish(shell *keptShell) {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	keeper.log("terminal: shell ended: " + shell.session.Result().String())
	keeper.shells = slices.DeleteFunc(keeper.shells, func(other *keptShell) bool { return other == shell })
	if shell.attached != nil {
		shell.attached.result = shell.session.Result()
		shell.detach(ShellExited)
	}
	keeper.armIdleTimer()
	close(shell.finished)
}

func (keeper *Keeper) log(message string) {
	if keeper.options.Log != nil {
		keeper.options.Log(message)
	}
}

// detach ends the current attachment; the caller holds the keeper's mu.
func (shell *keptShell) detach(reason DetachReason) {
	shell.attached.reason = reason
	close(shell.attached.ended)
	shell.attached = nil
}

func (keeper *Keeper) anyAttached() bool {
	return slices.ContainsFunc(keeper.shells, func(shell *keptShell) bool { return shell.attached != nil })
}

// armIdleTimer starts the idle limit if nothing is attached to any shell and it is not
// running already, and stops it if no shell is left; the caller holds mu.
func (keeper *Keeper) armIdleTimer() {
	if len(keeper.shells) == 0 || keeper.closed {
		keeper.stopIdleTimer()
		return
	}
	if keeper.idleTimer != nil || keeper.anyAttached() {
		return
	}
	serial := keeper.idleSerial
	keeper.idleTimer = time.AfterFunc(keeper.options.IdleLimit, func() {
		keeper.mu.Lock()
		expired := serial == keeper.idleSerial && !keeper.anyAttached()
		if expired {
			keeper.idleTimer = nil
		}
		keeper.mu.Unlock()
		if expired {
			keeper.EndAll()
		}
	})
}

// stopIdleTimer cancels the timer, including one already firing; the caller holds mu.
func (keeper *Keeper) stopIdleTimer() {
	keeper.idleSerial++
	if keeper.idleTimer != nil {
		keeper.idleTimer.Stop()
		keeper.idleTimer = nil
	}
}

// Rename sets a shell's name, which means nothing to the Keeper.
func (keeper *Keeper) Rename(id, name string) (ShellInfo, error) {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	shell := keeper.find(id)
	if shell == nil {
		return ShellInfo{}, ErrNoShell
	}
	shell.info.Name = name
	return shell.info, nil
}

// KeepAlive says someone is still there, attached or not, and restarts the idle limit.
func (keeper *Keeper) KeepAlive() {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	if keeper.idleTimer != nil {
		keeper.stopIdleTimer()
		keeper.armIdleTimer()
	}
}

// End ends a shell and returns once it has.
func (keeper *Keeper) End(id string) error {
	keeper.mu.Lock()
	shell := keeper.find(id)
	keeper.mu.Unlock()
	if shell == nil {
		return ErrNoShell
	}
	shell.session.Close()
	<-shell.finished
	return nil
}

// EndAll ends every shell and returns once they have. Each may take a moment to hang
// up, so they end together rather than in turn.
func (keeper *Keeper) EndAll() {
	keeper.mu.Lock()
	shells := slices.Clone(keeper.shells)
	keeper.mu.Unlock()
	var ending sync.WaitGroup
	for _, shell := range shells {
		ending.Go(func() {
			shell.session.Close()
			<-shell.finished
		})
	}
	ending.Wait()
}

// Close ends the shells and refuses further ones.
func (keeper *Keeper) Close() {
	keeper.mu.Lock()
	keeper.closed = true
	for _, shell := range keeper.shells {
		if shell.attached != nil {
			shell.detach(KeeperClosed)
		}
	}
	keeper.stopIdleTimer()
	keeper.mu.Unlock()
	keeper.EndAll()
}

// Attachment is one caller's view of the shell.
type Attachment struct {
	keeper  *Keeper
	shell   *keptShell
	pending []byte
	wake    chan struct{}
	room    chan struct{}
	ended   chan struct{}
	reason  DetachReason
	result  Result
}

// Next returns the output so far, waiting until there is some. It returns false once
// the attachment has ended and its last output has been taken; Reason and Result are
// valid after that.
func (attachment *Attachment) Next() ([]byte, bool) {
	keeper := attachment.keeper
	for {
		keeper.mu.Lock()
		output := attachment.pending
		attachment.pending = nil
		keeper.mu.Unlock()
		if len(output) > 0 {
			signal(attachment.room)
			return output, true
		}
		select {
		case <-attachment.wake:
		case <-attachment.ended:
			keeper.mu.Lock()
			output := attachment.pending
			attachment.pending = nil
			keeper.mu.Unlock()
			return output, len(output) > 0
		}
	}
}

// Reason says why the attachment ended; it is valid once Next has returned false.
func (attachment *Attachment) Reason() DetachReason {
	return attachment.reason
}

// Result is how the shell ended, when Reason is ShellExited.
func (attachment *Attachment) Result() Result {
	return attachment.result
}

// current reports whether this is still the attachment in use.
func (attachment *Attachment) current() bool {
	attachment.keeper.mu.Lock()
	defer attachment.keeper.mu.Unlock()
	return attachment.shell.attached == attachment
}

// Write sends input to the shell. A tab that lost the shell cannot type into it.
func (attachment *Attachment) Write(input []byte) (int, error) {
	if !attachment.current() {
		return 0, ErrDetached
	}
	return attachment.shell.session.Write(input)
}

// Resize changes the terminal's size.
func (attachment *Attachment) Resize(rows, cols uint16) error {
	if !attachment.current() {
		return ErrDetached
	}
	return attachment.shell.session.Resize(rows, cols)
}

// ClearOutput forgets the output kept to replay, so what has been cleared from the
// screen does not come back with the next Attach. What already waits for this reader
// still arrives.
func (attachment *Attachment) ClearOutput() error {
	keeper := attachment.keeper
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	if attachment.shell.attached != attachment {
		return ErrDetached
	}
	attachment.shell.ring = outputRing{limit: keeper.options.RingSize}
	return nil
}

// Detach ends the attachment and leaves the shell running; the idle limit starts if
// this was the last attachment to any shell.
// It does nothing to an attachment that has already ended.
func (attachment *Attachment) Detach() {
	keeper := attachment.keeper
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	if attachment.shell.attached != attachment {
		return
	}
	attachment.shell.detach(DetachedByCaller)
	keeper.armIdleTimer()
}

// outputRing keeps the most recent output. It holds up to twice its limit, so that
// most writes do not move data.
type outputRing struct {
	limit     int
	data      []byte
	truncated bool
}

func (ring *outputRing) write(output []byte) {
	ring.data = append(ring.data, output...)
	if len(ring.data) > 2*ring.limit {
		ring.data = append(ring.data[:0], ring.data[len(ring.data)-ring.limit:]...)
		ring.truncated = true
	}
}

// contents returns a copy of the recent output. Output cut at the limit starts
// after a line break when one is near, and otherwise at a whole character, so that
// a replay does not begin in the middle of one.
func (ring *outputRing) contents() []byte {
	view := ring.data
	cut := ring.truncated
	if len(view) > ring.limit {
		view = view[len(view)-ring.limit:]
		cut = true
	}
	if cut {
		if line := bytes.IndexByte(view, '\n'); line >= 0 && line < len(view)/2 {
			view = view[line+1:]
		} else {
			for skipped := 0; skipped < utf8.UTFMax-1 && len(view) > 0 && !utf8.RuneStart(view[0]); skipped++ {
				view = view[1:]
			}
		}
	}
	return bytes.Clone(view)
}
