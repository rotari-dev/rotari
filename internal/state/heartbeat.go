package state

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"
)

// HeartbeatFileName is the run-directory file a run's coordinator rewrites
// while the run is active, so a reader on any host can tell whether it is
// still writing.
const HeartbeatFileName = "heartbeat.json"

// HeartbeatInterval is how often a coordinator rewrites its heartbeat.
const HeartbeatInterval = 30 * time.Second

// Heartbeat is one beat of a run's coordinator. Seq changes with every beat
// a coordinator writes; it starts again at 1 when another coordinator takes
// the run over, so a reader compares it for change, not order. At is the
// coordinator's own clock, which another host's clock need not agree with.
type Heartbeat struct {
	Seq          uint64 `json:"seq"`
	At           string `json:"at"`
	Host         string `json:"host,omitempty"`
	PID          int    `json:"pid,omitempty"`
	PIDNamespace string `json:"pid_namespace,omitempty"`
}

// WriteHeartbeat replaces runDir's heartbeat. The write goes through a
// temporary file and a rename, so a reader sees one whole beat or another.
func (s Store) WriteHeartbeat(runDir string, beat Heartbeat) error {
	return s.WriteJSON(filepath.Join(runDir, HeartbeatFileName), beat)
}

// LoadHeartbeat reads runDir's heartbeat. found is false when the run has
// none, such as a run started by a rotari that wrote no heartbeat; a
// heartbeat that cannot be read or decoded is an error naming the file.
func (s Store) LoadHeartbeat(runDir string) (beat Heartbeat, found bool, err error) {
	path := filepath.Join(runDir, HeartbeatFileName)
	err = s.ReadJSON(path, &beat)
	if errors.Is(err, fs.ErrNotExist) {
		return Heartbeat{}, false, nil
	}
	if errors.Is(err, ErrInvalidJSON) {
		return Heartbeat{}, false, fmt.Errorf("%s: %w", path, err)
	}
	if err != nil {
		return Heartbeat{}, false, err
	}
	return beat, true, nil
}
