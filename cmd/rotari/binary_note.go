package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/project"
)

// currentBinary is the rotari binary this process runs, as model.RunBinary compares it.
func currentBinary() (model.RunBinary, error) {
	executable, err := os.Executable()
	if err != nil {
		return model.RunBinary{}, err
	}
	info, err := os.Stat(executable)
	if err != nil {
		return model.RunBinary{}, err
	}
	return model.RunBinary{Version: versionString(), Executable: executable, Size: info.Size(), Modified: info.ModTime().UTC().Format(time.RFC3339Nano)}, nil
}

// recordedBinary is currentBinary for a record, nil when this binary cannot be looked
// at; a record without it says nothing later, as one from an older rotari does.
func recordedBinary() *model.RunBinary {
	binary, err := currentBinary()
	if err != nil {
		return nil
	}
	return &binary
}

// binaryNote is the line a command prints when what it attaches to runs another rotari
// binary than this one: subject says what, such as "run R is managed by". It is empty
// when the binary is the same, or when other is not known because an older rotari
// wrote the record. When this binary cannot be looked at, it says so.
func binaryNote(subject string, other *model.RunBinary) string {
	if other == nil {
		return ""
	}
	current, err := currentBinary()
	if err != nil {
		return fmt.Sprintf("Note: cannot tell whether %s this rotari binary: %v", subject, err)
	}
	if !other.Differs(current) {
		return ""
	}
	if other.Version == current.Version {
		return fmt.Sprintf("Note: %s a different build of %s (%s), not this one (%s).", subject, other.Version, other.Executable, current.Executable)
	}
	return fmt.Sprintf("Note: %s %s (%s), not this %s (%s).", subject, other.Version, other.Executable, current.Version, current.Executable)
}

// noteRunBinary tells a command that attached to a running run when the run's
// coordinator runs another rotari binary. It says nothing for a coordinator that is not
// live, or one that recorded no binary.
func noteRunBinary(output *waitOutput, runDir, runID string) {
	assessment, err := project.AssessCoordinator(jsonStore(), runDir)
	if err != nil {
		fmt.Fprintf(output.stderrWriter(), "Note: cannot tell which rotari manages run %s: %v\n", runID, err)
		return
	}
	if assessment.State != project.CoordinatorLive || assessment.Identity == nil {
		return
	}
	if note := binaryNote("run "+runID+" is managed by", assessment.Identity.Binary); note != "" {
		fmt.Fprintln(output.stderrWriter(), note)
	}
}
