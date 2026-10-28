package diagnose

import (
	"regexp"
	"strings"

	"github.com/rotari-dev/rotari/internal/model"
)

type Job struct {
	JobID    string
	ExitCode *int
	Error    string
	Log      string
}

func DiagnoseDefault(job Job) []model.RuleDiagnosis {
	return Diagnose(job.Error, job.Log, DefaultRules())
}

// omittedLogMarker starts a log that TailLog shortened.
const omittedLogMarker = "[earlier log output omitted]"

func TailLog(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return omittedLogMarker + "\n" + value[len(value)-limit:]
}

// streamHeader is the line that introduces each stream of a job that keeps
// stdout and stderr separately, in the log read for diagnosis.
var streamHeader = regexp.MustCompile(`^--- (stdout|stderr) ---$`)

// LastOutputLine returns the last nonblank line of log, without ANSI
// escapes, surrounding whitespace, or text a carriage return overwrote. It
// skips the lines TailLog and the stream headers add, which are not output.
func LastOutputLine(log string) string {
	lines := strings.Split(log, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := ansiEscapeSequence.ReplaceAllString(lines[index], "")
		segments := strings.Split(line, "\r")
		line = ""
		for segment := len(segments) - 1; segment >= 0 && line == ""; segment-- {
			line = strings.TrimSpace(segments[segment])
		}
		if line == "" || line == omittedLogMarker || streamHeader.MatchString(line) {
			continue
		}
		return line
	}
	return ""
}
