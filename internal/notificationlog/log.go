// Package notificationlog keeps the notifications that runs produced, so the
// Web inbox can show those that happened while no page was open.
//
// The master directory holds one file per run, <masterdir>/notifications/
// <run-id>.jsonl, with one record per notification batch. Only the run's own
// process, or a later unlock that finalizes it, appends to a file, so
// concurrent runs never write the same file, even on NFS. Records are kept
// for Retention; inbox-state.json holds the shared read and cleared cursors.
package notificationlog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rotari-dev/rotari/internal/notification"
	"github.com/rotari-dev/rotari/internal/state"
)

// Retention is how long records are kept after their run's last record.
const Retention = 10 * 24 * time.Hour

const (
	recordSchemaVersion = 1
	recordExtension     = ".jsonl"
	stateFileName       = "inbox-state.json"
	stateLockFileName   = "inbox-state.lock"
	// timeLayout has a fixed width, so record times order as strings.
	timeLayout = "2006-01-02T15:04:05.000000000Z"
)

// Record is one notification batch of one run, with every event the run
// produced in it before any channel's selection.
type Record struct {
	SchemaVersion int                  `json:"schema_version"`
	ID            string               `json:"id"`
	CreatedAt     string               `json:"created_at"`
	BaseDir       string               `json:"base_dir"`
	Project       string               `json:"project"`
	RunID         string               `json:"run_id"`
	RunName       string               `json:"run_name,omitempty"`
	Events        []notification.Event `json:"events"`
}

// Cursor is a position in the record order: by creation time, then ID.
type Cursor struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

// Cursor returns the record's position.
func (record Record) Cursor() Cursor {
	return Cursor{CreatedAt: record.CreatedAt, ID: record.ID}
}

// After reports whether cursor comes after other. The zero cursor comes
// before every record.
func (cursor Cursor) After(other Cursor) bool {
	if cursor.CreatedAt != other.CreatedAt {
		return cursor.CreatedAt > other.CreatedAt
	}
	return cursor.ID > other.ID
}

// Valid reports whether cursor is the zero cursor or a record position.
func (cursor Cursor) Valid() bool {
	if cursor == (Cursor{}) {
		return true
	}
	_, err := time.Parse(timeLayout, cursor.CreatedAt)
	return err == nil && cursor.ID != ""
}

// InboxState is the inbox's shared state: records up to Read are read, and
// records up to Cleared are no longer shown.
type InboxState struct {
	Read    Cursor `json:"read"`
	Cleared Cursor `json:"cleared"`
}

// Log is the record store under one master directory.
type Log struct {
	dir   string
	now   func() time.Time
	cache *fileCache
}

// fileCache keeps parsed run files, so repeated polls read only the files
// that changed.
type fileCache struct {
	mu    sync.Mutex
	files map[string]cachedFile
}

type cachedFile struct {
	size     int64
	modified time.Time
	records  []Record
	problems []string
}

// Open returns the log under masterDir.
func Open(masterDir string) Log {
	return Log{dir: filepath.Join(masterDir, "notifications"), now: time.Now, cache: &fileCache{files: map[string]cachedFile{}}}
}

// Default returns the log under the resolved master directory; see
// state.ResolveMasterDir.
func Default() (Log, error) {
	masterDir, err := state.ResolveMasterDir("")
	if err != nil {
		return Log{}, err
	}
	return Open(masterDir), nil
}

// Append records events, all of one run under baseDir, as one record, then
// removes run files past Retention. The record is written with a single
// write, so a reader sees it whole or not at all.
func (log Log) Append(baseDir string, events []notification.Event) error {
	if len(events) == 0 {
		return nil
	}
	if !filepath.IsAbs(baseDir) {
		return fmt.Errorf("notification basedir %q is not absolute", baseDir)
	}
	first := events[0]
	path, err := log.runFile(first.RunID)
	if err != nil {
		return err
	}
	created := log.now().UTC()
	record := Record{SchemaVersion: recordSchemaVersion, ID: fmt.Sprintf("%s-%d", first.RunID, created.UnixNano()), CreatedAt: created.Format(timeLayout),
		BaseDir: filepath.Clean(baseDir), Project: first.Project, RunID: first.RunID, RunName: first.RunName}
	for _, event := range events {
		if event.RunID != first.RunID || event.Project != first.Project {
			return fmt.Errorf("notification record mixes runs %q and %q", first.Project+"/"+first.RunID, event.Project+"/"+event.RunID)
		}
		record.Events = append(record.Events, event.Recordable())
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(log.dir, state.DirectoryMode()); err != nil {
		return err
	}
	// codeql[go/path-injection]: path is the validated run file path.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, state.FileMode()) // NOSONAR: path is validated by runFile.
	if err != nil {
		return err
	}
	// A record left incomplete by a killed writer must not swallow this one.
	if info, statErr := file.Stat(); statErr == nil && info.Size() > 0 && !endsWithNewline(path, info.Size()) {
		line = append([]byte("\n"), line...)
	}
	_, writeErr := file.Write(append(line, '\n'))
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	if err := log.prune(); err != nil {
		return fmt.Errorf("remove expired notification records: %w", err)
	}
	return nil
}

func endsWithNewline(path string, size int64) bool {
	// codeql[go/path-injection]: path is the validated run file path.
	file, err := os.Open(path) // NOSONAR: path is validated by runFile.
	if err != nil {
		return false
	}
	defer file.Close()
	last := make([]byte, 1)
	_, err = file.ReadAt(last, size-1)
	return err == nil && last[0] == '\n'
}

func (log Log) runFile(runID string) (string, error) {
	if !state.IsValidPathElement(runID) || !state.IsValidPathElement(runID+recordExtension) {
		return "", fmt.Errorf("invalid run id %q", runID)
	}
	return state.SafeJoin(log.dir, runID+recordExtension)
}

// expired reports whether a run file last written at modified is past
// Retention.
func (log Log) expired(modified time.Time) bool {
	return log.now().Sub(modified) > Retention
}

// prune removes run files past Retention.
func (log Log) prune() error {
	entries, err := os.ReadDir(log.dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != recordExtension {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !log.expired(info.ModTime()) {
			continue
		}
		if err := os.Remove(filepath.Join(log.dir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Records returns the kept records, newest first. A record that cannot be
// read is left out and described in problems; an incomplete last line, a
// record still being written, is not a problem.
func (log Log) Records() (records []Record, problems []string, err error) {
	entries, err := os.ReadDir(log.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != recordExtension {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if log.expired(info.ModTime()) {
			continue
		}
		seen[entry.Name()] = true
		file, err := log.readFile(entry.Name(), info)
		if err != nil {
			return nil, nil, err
		}
		records = append(records, file.records...)
		problems = append(problems, file.problems...)
	}
	log.cache.mu.Lock()
	for name := range log.cache.files {
		if !seen[name] {
			delete(log.cache.files, name)
		}
	}
	log.cache.mu.Unlock()
	cutoff := log.now().Add(-Retention).UTC().Format(timeLayout)
	kept := records[:0]
	for _, record := range records {
		if record.CreatedAt >= cutoff {
			kept = append(kept, record)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Cursor().After(kept[j].Cursor()) })
	return kept, problems, nil
}

func (log Log) readFile(name string, info os.FileInfo) (cachedFile, error) {
	log.cache.mu.Lock()
	cached, ok := log.cache.files[name]
	log.cache.mu.Unlock()
	if ok && cached.size == info.Size() && cached.modified.Equal(info.ModTime()) {
		return cached, nil
	}
	data, err := os.ReadFile(filepath.Join(log.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return cachedFile{}, nil
	}
	if err != nil {
		return cachedFile{}, err
	}
	parsed := cachedFile{size: info.Size(), modified: info.ModTime()}
	runID := strings.TrimSuffix(name, recordExtension)
	lines := bytes.Split(data, []byte("\n"))
	// The last element follows the last newline: empty, or a record still
	// being written.
	for number, line := range lines[:len(lines)-1] {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var record Record
		if err := json.Unmarshal(line, &record); err != nil {
			parsed.problems = append(parsed.problems, fmt.Sprintf("%s line %d: %v", name, number+1, err))
			continue
		}
		if problem := record.problem(runID); problem != "" {
			parsed.problems = append(parsed.problems, fmt.Sprintf("%s line %d: %s", name, number+1, problem))
			continue
		}
		parsed.records = append(parsed.records, record)
	}
	log.cache.mu.Lock()
	log.cache.files[name] = parsed
	log.cache.mu.Unlock()
	return parsed, nil
}

func (record Record) problem(runID string) string {
	switch {
	case record.SchemaVersion != recordSchemaVersion:
		return fmt.Sprintf("unsupported schema version %d", record.SchemaVersion)
	case record.RunID != runID:
		return fmt.Sprintf("record of run %q in the file of run %q", record.RunID, runID)
	case !record.Cursor().Valid() || record.Cursor() == (Cursor{}):
		return "invalid record position"
	case !filepath.IsAbs(record.BaseDir) || !state.IsValidPathElement(record.Project):
		return "invalid record location"
	}
	return ""
}

// State returns the shared inbox state.
func (log Log) State() (InboxState, error) {
	var inbox InboxState
	err := state.NewStore(state.DirectoryMode(), state.FileMode()).ReadJSON(filepath.Join(log.dir, stateFileName), &inbox)
	if errors.Is(err, os.ErrNotExist) {
		return InboxState{}, nil
	}
	if err != nil {
		return InboxState{}, fmt.Errorf("read inbox state: %w", err)
	}
	return inbox, nil
}

// Advance moves the read and cleared cursors forward to read and cleared;
// a cursor never moves back, so concurrent pages cannot undo each other.
// Clearing a record also reads it.
func (log Log) Advance(read, cleared Cursor) (InboxState, error) {
	if !read.Valid() || !cleared.Valid() {
		return InboxState{}, errors.New("invalid inbox cursor")
	}
	if err := os.MkdirAll(log.dir, state.DirectoryMode()); err != nil {
		return InboxState{}, err
	}
	release, err := state.AcquireStateLock(filepath.Join(log.dir, stateLockFileName))
	if err != nil {
		return InboxState{}, fmt.Errorf("lock inbox state: %w", err)
	}
	defer release()
	inbox, err := log.State()
	if err != nil {
		return InboxState{}, err
	}
	if cleared.After(inbox.Cleared) {
		inbox.Cleared = cleared
	}
	for _, cursor := range []Cursor{read, inbox.Cleared} {
		if cursor.After(inbox.Read) {
			inbox.Read = cursor
		}
	}
	if err := state.WriteJSON(filepath.Join(log.dir, stateFileName), inbox); err != nil {
		return InboxState{}, fmt.Errorf("write inbox state: %w", err)
	}
	return inbox, nil
}
