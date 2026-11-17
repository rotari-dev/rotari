package bugreport

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// OutputMode says how much of each included job's own output to collect.
type OutputMode int

const (
	// OutputTail collects the last outputTailBytes of each output file.
	OutputTail OutputMode = iota
	// OutputFull collects each output file up to outputFullBytes.
	OutputFull
	// OutputNone collects no job output.
	OutputNone
)

const (
	outputTailBytes    = 64 << 10
	outputFullBytes    = 64 << 20
	stateFileBytes     = 1 << 20
	serverLogBytes     = 1 << 20
	maxJobs            = 50
	maxProbedJobs      = 10
	summaryLogLines    = 40
	summaryOutputLines = 20
	summaryOutputJobs  = 5
	commandTimeout     = 10 * time.Second
)

// File is one entry of a bundle, named by a slash-separated relative path.
type File struct {
	Name string
	Data []byte
}

// Bundle is a collected bug report: a Markdown summary and the files it
// draws on.
type Bundle struct {
	Summary string
	Files   []File
}

// Options select what Collect gathers.
type Options struct {
	Version            string
	ExecutableSHA256   string
	ExecutableModified time.Time
	BaseDir            string
	// Project and RunID select the run to collect; either may be empty.
	Project string
	RunID   string
	// JobIDs selects jobs of the run; empty selects its jobs that did not
	// succeed.
	JobIDs []string
	Output OutputMode
	// Redact replaces paths and hostnames. Secrets are removed regardless.
	Redact bool
	// Facts are files the caller produced, such as `rotari info --json`.
	Facts   []File
	Environ []string
	// WebLogPath and RegisteredBaseDirs are master-directory evidence. When
	// resolution fails, WebEvidenceError explains why neither is available.
	WebLogPath              string
	RegisteredBaseDirs      []string
	RegisteredBaseDirsKnown bool
	RegisteredBaseDirsError error
	WebEvidenceError        error
	Now                     func() time.Time
}

type collector struct {
	options Options
	summary strings.Builder
	files   []File
	notes   []string
	// known maps paths and hostnames to their placeholders for Redact.
	known map[string]string
	// fences are the summary's fenced copies of JSON and JSON Lines files,
	// which Collect fills in at its markers once it knows how to redact them.
	fences []fence
}

// fence is a fenced copy of a file's data in the summary: its last lines, or
// all of it when lines is 0. name says how the data is encoded, as a bundle
// file's name does.
type fence struct {
	name  string
	data  []byte
	lines int
}

// fenceMarker stands in the summary for collector.fences[index]. Redaction
// of the summary's text leaves it alone.
func fenceMarker(index int) string {
	return fmt.Sprintf("\x00fence %d\x00", index)
}

// writeFileFence adds a fenced copy of data, encoded as name says, to builder
// at a marker, so that Collect fills it in redacted as the file is: by its
// decoded strings rather than its encoded text.
func (collector *collector) writeFileFence(builder *strings.Builder, name string, data []byte, lines int) {
	builder.WriteString(fenceMarker(len(collector.fences)))
	collector.fences = append(collector.fences, fence{name: name, data: data, lines: lines})
}

// Collect gathers a bug report. It fails only for a selection that cannot
// be read, such as a missing run or job; anything else it cannot collect is
// noted in the summary.
func Collect(options Options) (Bundle, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	collector := &collector{options: options, known: make(map[string]string)}
	collector.rememberKnownLocations()
	fmt.Fprintf(&collector.summary, "# rotari bug report\n\nGenerated: %s\n\n", options.Now().UTC().Format(time.RFC3339))
	collector.environment()
	collector.webEvidence()
	collector.configuration()
	collector.facts()
	collector.serverLog()
	if options.Project != "" && options.RunID != "" {
		if err := collector.run(); err != nil {
			return Bundle{}, err
		}
	}
	return collector.finish(), nil
}

func (collector *collector) rememberKnownLocations() {
	if collector.options.BaseDir != "" {
		collector.known[collector.options.BaseDir] = "[BASEDIR]"
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" {
		collector.known[home] = "[HOME]"
	}
	if cwd, err := os.Getwd(); err == nil && cwd != "/" {
		collector.known[cwd] = "[CWD]"
	}
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		collector.known[hostname] = "[REDACTED_HOST]"
	}
	registered := append([]string(nil), collector.options.RegisteredBaseDirs...)
	sort.Strings(registered)
	for index, path := range registered {
		if _, exists := collector.known[path]; !exists && path != "" {
			collector.known[path] = fmt.Sprintf("[REGISTERED_BASEDIR_%d]", index+1)
		}
	}
}

func (collector *collector) addFile(name string, data []byte) {
	collector.files = append(collector.files, File{Name: name, Data: data})
}

func (collector *collector) note(format string, args ...any) {
	collector.notes = append(collector.notes, fmt.Sprintf(format, args...))
}

func (collector *collector) finish() Bundle {
	var header strings.Builder
	header.WriteString("> Configuration values that look secret, `ROTARI_*` variables that look secret, and job environment values are replaced with `[REDACTED]`.")
	if collector.options.Redact {
		header.WriteString(" Paths and hostnames are redacted where detected.")
	} else {
		header.WriteString(" Paths and hostnames are kept (`--no-redact`).")
	}
	header.WriteString(" Job output and command lines are not checked for secrets; review them before sharing.\n\n")
	summary := collector.summary.String()
	if index := strings.Index(summary, "\n\n"); index >= 0 {
		summary = summary[:index+2] + header.String() + summary[index+2:]
	}
	if len(collector.notes) > 0 {
		summary += "## Collection notes\n\n"
		for _, note := range collector.notes {
			summary += "- " + note + "\n"
		}
		summary += "\n"
	}
	bundle := Bundle{Summary: summary, Files: collector.files}
	redactor := pathRedactor{}
	if collector.options.Redact {
		redactor = newPathRedactor(collector.known)
		bundle.Summary = redactor.apply(bundle.Summary)
		for index := range bundle.Files {
			bundle.Files[index].Data = redactor.applyFile(bundle.Files[index].Name, bundle.Files[index].Data)
		}
	}
	for index, fenced := range collector.fences {
		text := string(redactor.applyFile(fenced.name, fenced.data))
		if fenced.lines > 0 {
			text = lastLines(text, fenced.lines)
		}
		var block strings.Builder
		writeFence(&block, text)
		bundle.Summary = strings.Replace(bundle.Summary, fenceMarker(index), block.String(), 1)
	}
	return bundle
}

func writeFence(builder *strings.Builder, text string) {
	fence := "```"
	for strings.Contains(text, fence) {
		fence += "`"
	}
	builder.WriteString(fence + "\n" + strings.TrimRight(text, "\n") + "\n" + fence + "\n\n")
}

func lastLines(text string, count int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) <= count {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-count:], "\n")
}
