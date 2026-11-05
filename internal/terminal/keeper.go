package terminal

import (
	"bytes"
	"errors"
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
	// ErrKeeperClosed is returned by Attach after Close.
	ErrKeeperClosed = errors.New("terminal keeper is closed")
	// ErrDetached is returned when using an Attachment that has ended.
	ErrDetached = errors.New("terminal attachment has ended")
	// ErrAttachedElsewhere is returned by AttachIfFree when another attachment
	// has the shell.
	ErrAttachedElsewhere = errors.New("terminal is attached elsewhere")
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

// KeeperOptions says how a Keeper runs its shell.
type KeeperOptions struct {
	// Shell starts each shell; its Rows and Cols are ignored, since each
	// Attach gives the size.
	Shell Options
	// IdleLimit is how long a shell runs with nothing attached.
	IdleLimit time.Duration
	// RingSize is how many bytes of recent output a new Attach replays.
	RingSize int
	// Log, if set, is told when a shell starts and ends, and never what it shows.
	Log func(message string)
}

// Keeper holds at most one shell, which outlives the attachments that show it:
// callers come and go, and each new one sees recent output. With nothing attached,
// output goes only to the ring, so a program never waits. An attached reader that
// falls behind makes the program wait, so it misses nothing, unless it stalls.
type Keeper struct {
	options KeeperOptions

	mu         sync.Mutex
	closed     bool
	shell      *keptShell
	attached   *Attachment
	idleTimer  *time.Timer
	idleSerial int
}

type keptShell struct {
	session  *Session
	ring     outputRing
	finished chan struct{}
}

// NewKeeper returns a Keeper with no shell yet.
func NewKeeper(options KeeperOptions) (*Keeper, error) {
	if options.IdleLimit <= 0 {
		return nil, errors.New("terminal idle limit must be above zero")
	}
	if options.RingSize <= 0 {
		return nil, errors.New("terminal output ring size must be above zero")
	}
	return &Keeper{options: options}, nil
}

// Attach shows the shell at the given size, starting one if none is running, and
// ends any earlier Attachment with TakenOver. Next begins with the replay of recent
// output. A program already running is made to redraw itself.
func (keeper *Keeper) Attach(rows, cols uint16) (*Attachment, error) {
	return keeper.attach(rows, cols, true)
}

// AttachIfFree is Attach that leaves another attachment alone and returns
// ErrAttachedElsewhere instead. It is for reconnecting by itself, which must not
// take the shell from whoever is using it.
func (keeper *Keeper) AttachIfFree(rows, cols uint16) (*Attachment, error) {
	return keeper.attach(rows, cols, false)
}

func (keeper *Keeper) attach(rows, cols uint16, takeOver bool) (*Attachment, error) {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	if keeper.closed {
		return nil, ErrKeeperClosed
	}
	if keeper.attached != nil && !takeOver {
		return nil, ErrAttachedElsewhere
	}
	shell := keeper.shell
	if shell != nil {
		if err := redraw(shell.session, rows, cols); err != nil {
			return nil, err
		}
	} else {
		options := keeper.options.Shell
		options.Rows, options.Cols = rows, cols
		session, err := Start(options)
		if err != nil {
			return nil, err
		}
		shell = &keptShell{session: session, ring: outputRing{limit: keeper.options.RingSize}, finished: make(chan struct{})}
		keeper.shell = shell
		keeper.log("terminal: shell started")
		go keeper.pump(shell)
	}
	keeper.stopIdleTimer()
	if keeper.attached != nil {
		keeper.detach(TakenOver)
	}
	attachment := &Attachment{
		keeper:  keeper,
		shell:   shell,
		pending: shell.ring.contents(),
		wake:    make(chan struct{}, 1),
		room:    make(chan struct{}, 1),
		ended:   make(chan struct{}),
	}
	keeper.attached = attachment
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
// reader is behind. Only this goroutine adds output, so a program waits only as
// long as it does.
func (keeper *Keeper) deliver(shell *keptShell, output []byte) {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	shell.ring.write(output)
	attachment := keeper.attached
	if attachment == nil || attachment.shell != shell {
		return
	}
	attachment.pending = append(attachment.pending, output...)
	signal(attachment.wake)
	if len(attachment.pending) < pendingLimit {
		return
	}
	stall := time.NewTimer(stallLimit)
	defer stall.Stop()
	for keeper.attached == attachment && len(attachment.pending) >= pendingLimit {
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
			if keeper.attached == attachment {
				keeper.detach(TooSlow)
				keeper.startIdleTimer(shell)
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
	if keeper.shell == shell {
		keeper.shell = nil
		keeper.stopIdleTimer()
	}
	if keeper.attached != nil && keeper.attached.shell == shell {
		keeper.attached.result = shell.session.Result()
		keeper.detach(ShellExited)
	}
	close(shell.finished)
}

func (keeper *Keeper) log(message string) {
	if keeper.options.Log != nil {
		keeper.options.Log(message)
	}
}

// detach ends the current attachment; the caller holds mu.
func (keeper *Keeper) detach(reason DetachReason) {
	keeper.attached.reason = reason
	close(keeper.attached.ended)
	keeper.attached = nil
}

// startIdleTimer ends shell after the idle limit unless something attaches first;
// the caller holds mu.
func (keeper *Keeper) startIdleTimer(shell *keptShell) {
	keeper.stopIdleTimer()
	serial := keeper.idleSerial
	keeper.idleTimer = time.AfterFunc(keeper.options.IdleLimit, func() {
		keeper.mu.Lock()
		expired := serial == keeper.idleSerial && keeper.shell == shell && keeper.attached == nil
		keeper.mu.Unlock()
		if expired {
			shell.session.Close()
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

// EndShell ends the running shell, if any, and returns once it has. The next
// Attach starts a new one.
func (keeper *Keeper) EndShell() {
	keeper.mu.Lock()
	shell := keeper.shell
	keeper.mu.Unlock()
	if shell != nil {
		shell.session.Close()
		<-shell.finished
	}
}

// Close ends the shell and refuses further attachments.
func (keeper *Keeper) Close() {
	keeper.mu.Lock()
	keeper.closed = true
	if keeper.attached != nil {
		keeper.detach(KeeperClosed)
	}
	keeper.stopIdleTimer()
	keeper.mu.Unlock()
	keeper.EndShell()
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
	return attachment.keeper.attached == attachment
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

// Detach ends the attachment and leaves the shell running; the idle limit starts.
// It does nothing to an attachment that has already ended.
func (attachment *Attachment) Detach() {
	keeper := attachment.keeper
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	if keeper.attached != attachment {
		return
	}
	keeper.detach(DetachedByCaller)
	keeper.startIdleTimer(attachment.shell)
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
