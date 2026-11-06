package bugreport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rotari-dev/rotari/internal/config"
	"github.com/rotari-dev/rotari/internal/notification"
)

// schedulerCommands are the scheduler client tools rotari runs; their
// presence tells which scheduler this host can reach.
var schedulerCommands = []string{"sbatch", "squeue", "sacct", "qsub", "qstat", "qacct", "bsub", "bjobs", "bhist"}

// schedulerVersionCommands print a scheduler's version.
var schedulerVersionCommands = [][]string{{"sbatch", "--version"}, {"qstat", "--version"}, {"lsid"}}

func (collector *collector) environment() {
	options := collector.options
	builder := &collector.summary
	builder.WriteString("## rotari and host\n\n")
	fmt.Fprintf(builder, "- Version: %s\n", options.Version)
	if options.ExecutableSHA256 != "" {
		fmt.Fprintf(builder, "- Executable SHA-256: %s\n", options.ExecutableSHA256)
	}
	if !options.ExecutableModified.IsZero() {
		fmt.Fprintf(builder, "- Executable modified: %s\n", options.ExecutableModified.UTC().Format(time.RFC3339Nano))
	}
	fmt.Fprintf(builder, "- Go: %s, %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if hostname, err := os.Hostname(); err == nil {
		fmt.Fprintf(builder, "- Host: %s\n", hostname)
	}
	if options.BaseDir != "" {
		fmt.Fprintf(builder, "- Basedir: %s\n", options.BaseDir)
		fmt.Fprintf(builder, "- Basedir filesystem: %s\n", filesystemType(options.BaseDir))
		fmt.Fprintf(builder, "- Basedir clock: %s\n", describeClockSkew(options.BaseDir, options.Now))
	}
	if options.Project != "" {
		fmt.Fprintf(builder, "- Project: %s\n", options.Project)
	}
	builder.WriteString("- Scheduler commands:")
	found := false
	for _, name := range schedulerCommands {
		if path, err := exec.LookPath(name); err == nil {
			fmt.Fprintf(builder, " %s (%s)", name, path)
			found = true
		}
	}
	if !found {
		builder.WriteString(" none found in PATH")
	}
	builder.WriteString("\n")
	for _, command := range schedulerVersionCommands {
		if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}
		output, err := runProbe(command)
		line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(output), "\n", 2)[0])
		if err != nil {
			line = strings.TrimSpace(line + " (" + err.Error() + ")")
		}
		fmt.Fprintf(builder, "- `%s`: %s\n", strings.Join(command, " "), line)
	}
	builder.WriteString("\n")

	variables := redactEnvironment(options.Environ)
	builder.WriteString("## ROTARI_* environment variables\n\n")
	if len(variables) == 0 {
		builder.WriteString("None set.\n\n")
	} else {
		writeFence(builder, strings.Join(variables, "\n"))
		collector.addFile("environment.txt", []byte(strings.Join(variables, "\n")+"\n"))
	}
}

func (collector *collector) webEvidence() {
	builder := &collector.summary
	builder.WriteString("## Web events\n\n")
	if collector.options.WebEvidenceError != nil {
		fmt.Fprintf(builder, "Not collected: %v\n\n", collector.options.WebEvidenceError)
		collector.note("Web evidence not collected: %v", collector.options.WebEvidenceError)
	} else if collector.options.WebLogPath == "" {
		builder.WriteString("Web event log path is unavailable.\n\n")
	} else {
		data, err := readWebLogTail(collector.options.WebLogPath, serverLogBytes)
		switch {
		case errors.Is(err, os.ErrNotExist):
			builder.WriteString("No Web event log in the master directory.\n\n")
			collector.note("Web event log does not exist yet")
		case err != nil:
			fmt.Fprintf(builder, "Web event log not read: %v\n\n", err)
			collector.note("Web event log not read: %v", err)
		default:
			data = sanitizeWebEvents(data, collector.options.Redact)
			collector.addFile("web-events.jsonl", data)
			fmt.Fprintf(builder, "Last %d lines from the Web event log:\n\n", summaryLogLines)
			writeFence(builder, lastLines(string(data), summaryLogLines))
		}
	}

	builder.WriteString("## Registered basedirs\n\n")
	if collector.options.WebEvidenceError != nil {
		builder.WriteString("Not collected because the master directory was unavailable.\n\n")
		return
	}
	if collector.options.RegisteredBaseDirsError != nil {
		fmt.Fprintf(builder, "Not collected: %v\n\n", collector.options.RegisteredBaseDirsError)
		collector.note("registered basedirs not collected: %v", collector.options.RegisteredBaseDirsError)
		return
	}
	if !collector.options.RegisteredBaseDirsKnown {
		builder.WriteString("Not collected.\n\n")
		return
	}
	if len(collector.options.RegisteredBaseDirs) == 0 {
		builder.WriteString("None registered.\n\n")
		return
	}
	paths := append([]string(nil), collector.options.RegisteredBaseDirs...)
	sort.Strings(paths)
	for _, path := range paths {
		status := "exists"
		if _, err := os.Stat(path); os.IsNotExist(err) {
			status = "missing"
		} else if err != nil {
			status = "unreadable (" + err.Error() + ")"
		}
		fmt.Fprintf(builder, "- %s: %s\n", path, status)
	}
	builder.WriteString("\n")
}

var webEventFields = map[string]bool{
	"time": true, "event": true, "pid": true, "command": true, "outcome": true,
	"transport": true, "cookie": true, "cookie_secure": true, "user_agent": true,
	"suppressed": true, "version": true, "executable_sha256": true,
	"executable_modified": true, "host": true, "port": true, "auth": true,
	"allow_exec": true, "allow_terminal": true, "allow_control": true,
	"notifications": true, "artifact_root_count": true, "registered_basedir_count": true,
}

func sanitizeWebEvents(data []byte, redactHost bool) []byte {
	var builder strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var decoded map[string]any
		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.UseNumber()
		if decoder.Decode(&decoded) != nil {
			builder.WriteString("{\"bug_report\":\"unreadable line omitted\"}\n")
			continue
		}
		for key := range decoded {
			if !webEventFields[key] {
				delete(decoded, key)
			}
		}
		if redactHost {
			if _, exists := decoded["host"]; exists {
				decoded["host"] = "[REDACTED_HOST]"
			}
		}
		encoded, err := encodeJSON(decoded, "")
		if err == nil {
			builder.Write(encoded)
		}
	}
	return []byte(builder.String())
}

func readWebLogTail(path string, limit int64) ([]byte, error) {
	current, err := readTail(path, limit)
	if err != nil {
		return nil, err
	}
	remaining := limit - int64(len(current))
	if remaining <= 0 {
		return current, nil
	}
	rotated, err := readTail(path+".1", remaining)
	if errors.Is(err, os.ErrNotExist) {
		return current, nil
	}
	if err != nil {
		return nil, err
	}
	return append(rotated, current...), nil
}

// describeClockSkew compares the modification time the basedir's
// filesystem gives a new file with this host's clock. A shared filesystem
// whose server clock differs makes recorded times look wrong.
func describeClockSkew(dir string, now func() time.Time) string {
	file, err := os.CreateTemp(dir, ".rotari-bug-report-")
	if err != nil {
		return "not measured (" + err.Error() + ")"
	}
	name := file.Name()
	defer os.Remove(name)
	before := now()
	_, writeErr := file.WriteString("clock probe\n")
	closeErr := file.Close()
	after := now()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return "not measured (" + err.Error() + ")"
	}
	info, err := os.Stat(name)
	if err != nil {
		return "not measured (" + err.Error() + ")"
	}
	local := before.Add(after.Sub(before) / 2)
	skew := info.ModTime().Sub(local).Round(time.Second)
	switch {
	case skew > time.Second:
		return fmt.Sprintf("filesystem time is %s ahead of this host", skew)
	case skew < -time.Second:
		return fmt.Sprintf("filesystem time is %s behind this host", -skew)
	default:
		return "matches this host within 1s"
	}
}

func runProbe(command []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, command[0], command[1:]...).CombinedOutput()
	if ctx.Err() != nil {
		return string(output), fmt.Errorf("timed out after %s", commandTimeout)
	}
	return string(output), err
}

func (collector *collector) configuration() {
	builder := &collector.summary
	builder.WriteString("## Configuration files\n\n")
	common, projects := config.ListPaths(collector.options.BaseDir, collector.options.Project, notification.FileName)
	paths := append([]string(nil), common...)
	projectNames := make([]string, 0, len(projects))
	for name := range projects {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)
	for _, name := range projectNames {
		paths = append(paths, projects[name]...)
	}
	if len(paths) == 0 {
		builder.WriteString("None found.\n\n")
		return
	}
	for index, path := range paths {
		name := fmt.Sprintf("config/%02d-%s.toml", index+1, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		data, err := readLimited(path, stateFileBytes)
		if err == nil {
			data, err = redactConfig(path, data)
		}
		if err != nil {
			fmt.Fprintf(builder, "- %s: content omitted (%v)\n", path, err)
			continue
		}
		collector.addFile(name, data)
		fmt.Fprintf(builder, "- %s: `%s`\n", path, name)
	}
	builder.WriteString("\n")
}

func (collector *collector) facts() {
	if len(collector.options.Facts) == 0 {
		return
	}
	builder := &collector.summary
	builder.WriteString("## rotari's own view\n\n")
	for _, fact := range collector.options.Facts {
		data := fact.Data
		if strings.HasSuffix(fact.Name, ".json") {
			if redacted, err := redactJSON(data); err == nil {
				data = redacted
			}
		}
		collector.addFile(fact.Name, data)
		fmt.Fprintf(builder, "### `%s`\n\n", fact.Name)
		writeFence(builder, string(data))
	}
}

func (collector *collector) serverLog() {
	if collector.options.BaseDir == "" {
		return
	}
	builder := &collector.summary
	data, err := readTail(filepath.Join(collector.options.BaseDir, "server.log"), serverLogBytes)
	if errors.Is(err, os.ErrNotExist) {
		builder.WriteString("## server.log\n\nNo server.log in the basedir.\n\n")
		return
	}
	if err != nil {
		collector.note("server.log not read: %v", err)
		return
	}
	collector.addFile("server.log", data)
	fmt.Fprintf(builder, "## server.log (last %d lines)\n\n", summaryLogLines)
	writeFence(builder, lastLines(string(data), summaryLogLines))
}

func readLimited(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit))
}

// readTail reads at most the last limit bytes of path.
func readTail(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		if _, err := file.Seek(info.Size()-limit, io.SeekStart); err != nil {
			return nil, err
		}
	}
	return io.ReadAll(io.LimitReader(file, limit))
}
