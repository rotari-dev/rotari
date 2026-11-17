package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rotari-dev/rotari/internal/model"
)

// FinalResultFileName marks an attempt whose job will not be retried.
const FinalResultFileName = "final_result.json"

// ArtifactsFileName holds the artifact candidates discovered for an attempt
// when it started. It is best-effort metadata: an attempt without one has no
// discovery information, not no associated files.
const ArtifactsFileName = "artifacts.json"

type AttemptIDPayload struct {
	RunID  string
	JobID  string
	Number int
}

func MakeAttemptID(runID, jobID string, number int) string {
	if !IsValidPathElement(runID) || !IsValidPathElement(jobID) || number < 0 {
		panic(fmt.Sprintf("invalid attempt ID components: run=%q job=%q number=%d", runID, jobID, number))
	}
	return fmt.Sprintf("att_%s-%s-%d", runID, jobID, number)
}

func DecodeAttemptID(attemptID string) (AttemptIDPayload, error) {
	if !strings.HasPrefix(attemptID, "att_") {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	value := strings.TrimPrefix(attemptID, "att_")
	separator := strings.LastIndexByte(value, '-')
	if separator <= 0 || separator == len(value)-1 {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	number, err := strconv.Atoi(value[separator+1:])
	if err != nil || number < 0 {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	core := value[:separator]
	if len(core) <= runIDLength || core[runIDLength] != '-' {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	payload := AttemptIDPayload{RunID: core[:runIDLength], JobID: core[runIDLength+1:], Number: number}
	if !IsValidPathElement(payload.RunID) || !IsValidPathElement(payload.JobID) {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	return payload, nil
}

func AttemptJobDir(runDir string, job model.JobSpec) (string, error) {
	jobDir, err := SafeJoin(runDir, job.ID)
	if err != nil || job.AttemptID == "" {
		return jobDir, err
	}
	if !IsValidPathElement(job.AttemptID) {
		return "", fmt.Errorf("invalid attempt ID %q", job.AttemptID)
	}
	return filepath.Join(jobDir, "attempts", job.AttemptID), nil
}

func LatestAttemptJobDir(runDir, jobID string) (string, error) {
	_, jobDir, err := LatestAttempt(runDir, jobID)
	return jobDir, err
}

// LatestAttempt lists jobID's attempts once and returns the latest attempt's
// ID, "" when there is none, and the directory holding the job's state: that
// attempt's directory, or the job directory without one.
func LatestAttempt(runDir, jobID string) (attemptID, jobDir string, err error) {
	jobDir, err = SafeJoin(runDir, jobID)
	if err != nil {
		return "", "", err
	}
	attemptID, err = LatestAttemptID(runDir, jobID)
	if err != nil || attemptID == "" {
		return "", jobDir, nil
	}
	return attemptID, filepath.Join(jobDir, "attempts", attemptID), nil
}

func LatestAttemptID(runDir, jobID string) (string, error) {
	jobDir, err := SafeJoin(runDir, jobID)
	if err != nil {
		return "", err
	}
	runID := filepath.Base(runDir)
	// codeql[go/path-injection]: jobDir is produced by SafeJoin and attempts is a fixed directory.
	entries, err := os.ReadDir(filepath.Join(jobDir, "attempts"))
	if err != nil {
		return "", err
	}
	latest := ""
	latestNumber := -1
	for _, entry := range entries {
		if !entry.IsDir() || !IsValidPathElement(entry.Name()) {
			continue
		}
		payload, decodeErr := DecodeAttemptIDForRun(entry.Name(), runID)
		if decodeErr != nil {
			payload, decodeErr = DecodeAttemptID(entry.Name())
		}
		if decodeErr != nil || payload.JobID != jobID {
			continue
		}
		if payload.Number > latestNumber || (payload.Number == latestNumber && entry.Name() > latest) {
			latest = entry.Name()
			latestNumber = payload.Number
		}
	}
	return latest, nil
}

// DecodeAttemptIDForRun decodes an attempt belonging to runID, including
// imported runs whose IDs do not have the usual timestamp format.
func DecodeAttemptIDForRun(attemptID, runID string) (AttemptIDPayload, error) {
	prefix := "att_" + runID + "-"
	if !strings.HasPrefix(attemptID, prefix) {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	value := strings.TrimPrefix(attemptID, prefix)
	separator := strings.LastIndexByte(value, '-')
	if separator <= 0 || separator == len(value)-1 {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	number, err := strconv.Atoi(value[separator+1:])
	if err != nil || number < 0 {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	jobID := value[:separator]
	if !IsValidPathElement(runID) || !IsValidPathElement(jobID) {
		return AttemptIDPayload{}, fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	return AttemptIDPayload{RunID: runID, JobID: jobID, Number: number}, nil
}

// ListAttemptIDs lists a job's attempt IDs in the run, oldest first.
func ListAttemptIDs(runDir, jobID string) []string {
	jobDir, err := SafeJoin(runDir, jobID)
	if err != nil {
		return nil
	}
	// codeql[go/path-injection]: jobDir is validated and attempts is a fixed directory.
	entries, err := os.ReadDir(filepath.Join(jobDir, "attempts"))
	if err != nil {
		return nil
	}
	attempts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && IsValidPathElement(entry.Name()) {
			attempts = append(attempts, entry.Name())
		}
	}
	sort.SliceStable(attempts, func(left, right int) bool {
		leftPayload, leftErr := DecodeAttemptID(attempts[left])
		rightPayload, rightErr := DecodeAttemptID(attempts[right])
		if leftErr == nil && rightErr == nil && leftPayload.Number != rightPayload.Number {
			return leftPayload.Number < rightPayload.Number
		}
		return attempts[left] < attempts[right]
	})
	return attempts
}

func SpecificAttemptJobDir(runDir, jobID, attemptID string) (string, error) {
	jobDir, err := SafeJoin(runDir, jobID)
	if err != nil {
		return "", err
	}
	if !IsValidPathElement(attemptID) {
		return "", fmt.Errorf("invalid attempt ID %q", attemptID)
	}
	return filepath.Join(jobDir, "attempts", attemptID), nil
}

// ReadJobName returns the job name recorded in jobDir, or "" when there is
// none.
func ReadJobName(jobDir string) string {
	data, err := os.ReadFile(filepath.Join(jobDir, "name"))
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// ReadAttemptExecutor returns the executor an attempt ran on, as its
// command.json records it once submitted, or "" for an attempt that has
// none. It reflects a run-level --executor and the queue's default, which
// the job's definition alone does not.
func ReadAttemptExecutor(attemptDir string) string {
	var recorded struct {
		Executor string `json:"executor"`
	}
	data, err := os.ReadFile(filepath.Join(attemptDir, "command.json"))
	if err != nil || json.Unmarshal(data, &recorded) != nil {
		return ""
	}
	return recorded.Executor
}

func ReadJobTimestamp(runDir, jobID, name string) string {
	jobDir, err := LatestAttemptJobDir(runDir, jobID)
	if err != nil {
		return ""
	}
	return ResolveAttemptTimestamp(jobDir, name)
}

// attemptTimestampSource is one place an attempt records a timestamp: a
// file that holds only the timestamp, or a field of a JSON file.
type attemptTimestampSource struct {
	file, field string
}

// attemptTimestampSources lists where an attempt's submitted_at or
// finished_at is recorded, in the order they are read: the timestamp file,
// then the time job.json or status.json records.
func attemptTimestampSources(name string) []attemptTimestampSource {
	switch name {
	case "submitted_at":
		return []attemptTimestampSource{{file: "submitted_at"}, {file: "job.json", field: "submitted_at"}}
	case "finished_at":
		return []attemptTimestampSource{{file: "finished_at"}, {file: "status.json", field: "finished_at"}}
	}
	return nil
}

// read returns the timestamp source records in jobDir and the file read. A
// timestamp file that was read ends the search even when empty; a JSON
// field ends it only when set.
func (source attemptTimestampSource) read(jobDir string) (value, path string, found bool, err error) {
	path, err = ValidatedStateFile(jobDir, source.file)
	if err != nil {
		return "", "", false, err
	}
	// codeql[go/path-injection]: path is restricted by ValidatedStateFile to a fixed state file.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", path, false, err
	}
	if source.field == "" {
		return strings.TrimSpace(string(data)), path, true, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return "", path, false, fmt.Errorf("%s: %w: %v", path, ErrInvalidJSON, err)
	}
	raw, ok := fields[source.field]
	if !ok {
		return "", path, false, nil
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", path, false, fmt.Errorf("%s: %w: field %s: %v", path, ErrInvalidJSON, source.field, err)
	}
	return value, path, value != "", nil
}

// ResolveAttemptTimestamp reads an attempt's submitted_at or finished_at,
// falling back to the times job.json and status.json record. A source that
// cannot be read is skipped; LoadAttemptTimestamp reports it instead.
func ResolveAttemptTimestamp(jobDir, name string) string {
	for _, source := range attemptTimestampSources(name) {
		if value, _, found, err := source.read(jobDir); err == nil && found {
			return value
		}
	}
	return ""
}

// LoadAttemptTimestamp reads an attempt's submitted_at or finished_at from
// the sources ResolveAttemptTimestamp reads, and returns it with the file it
// came from. It returns "" when no source records it. A source that exists
// but cannot be read or decoded is an error naming the file.
func LoadAttemptTimestamp(jobDir, name string) (value, path string, err error) {
	for _, source := range attemptTimestampSources(name) {
		value, path, found, err := source.read(jobDir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", path, err
		}
		if found {
			return value, path, nil
		}
	}
	return "", "", nil
}

func ReadAttemptTimestamp(jobDir, name string) string {
	path, err := ValidatedStateFile(jobDir, name)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func LoadLocalJobResult(jobDir string, job model.JobSpec) (model.JobResult, bool) {
	finishedPath, err := ValidatedStateFile(jobDir, "finished_at")
	if err != nil {
		return model.JobResult{}, false
	}
	if _, err := os.Stat(finishedPath); err != nil {
		return model.JobResult{}, false
	}
	path, err := ValidatedStateFile(jobDir, "status")
	if err != nil {
		return model.JobResult{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return model.JobResult{}, false
	}
	exitCode, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return model.JobResult{}, false
	}
	return model.JobResult{ID: job.ID, Command: job.Command, ExitCode: exitCode}, true
}
