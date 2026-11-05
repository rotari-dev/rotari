package tableview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExportFile(t *testing.T, name, content string) Target {
	t.Helper()
	directory := t.TempDir()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return Target{Path: path, Roots: []string{directory}}
}

func exported(t *testing.T, table *Table, filters []Condition, sort *Sort) string {
	t.Helper()
	export, err := table.PrepareExport(filters, sort)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := export.Stream(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestExportWritesDelimitedRows(t *testing.T) {
	long := strings.Repeat("x", cellLimit+10)
	content := "id,,note\n1,3,\"a,b\"\n2,1,\"line\nbreak\"\n3,2\n\"\"\n4,5," + long + "\n"
	for _, test := range []struct {
		name    string
		format  Format
		filters []Condition
		sort    *Sort
		want    string
	}{
		{
			name:   "keeps the blank header cell, short rows, quoting, and whole long cells",
			format: Format{Separator: ',', Header: true},
			want:   "id,,note\n1,3,\"a,b\"\n2,1,\"line\nbreak\"\n3,2\n\"\"\n4,5," + long + "\n",
		},
		{
			name:   "has no header without one",
			format: Format{Separator: ',', Header: false},
			want:   "id,,note\n1,3,\"a,b\"\n2,1,\"line\nbreak\"\n3,2\n\"\"\n4,5," + long + "\n",
		},
		{
			name:    "filters",
			format:  Format{Separator: ',', Header: true},
			filters: []Condition{{Column: 0, Op: OpGreater, Value: "2"}},
			want:    "id,,note\n3,2\n4,5," + long + "\n",
		},
		{
			name:   "follows the sort",
			format: Format{Separator: ',', Header: true},
			sort:   &Sort{Column: 1, Descending: true},
			want:   "id,,note\n4,5," + long + "\n1,3,\"a,b\"\n3,2\n2,1,\"line\nbreak\"\n\"\"\n",
		},
		{
			name:    "writes only the header when no row matches",
			format:  Format{Separator: ',', Header: true},
			filters: []Condition{{Column: 0, Op: OpEqual, Value: "none"}},
			want:    "id,,note\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			table, err := OpenFile(writeExportFile(t, "data.csv", content), test.format)
			if err != nil {
				t.Fatal(err)
			}
			if got := exported(t, table, test.filters, test.sort); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestExportKeepsTheSeparatorAndReadsBackTheSameCells(t *testing.T) {
	content := "a\tb\n\" padded\"\tq\"uote\n1,2\t\n"
	format := Format{Separator: '\t', Header: true}
	table, err := OpenFile(writeExportFile(t, "data.tsv", content), format)
	if err != nil {
		t.Fatal(err)
	}
	got := exported(t, table, nil, nil)
	if !strings.Contains(got, "a\tb\n") || !strings.Contains(got, "1,2\t\n") {
		t.Fatalf("output = %q", got)
	}
	again, err := OpenFile(writeExportFile(t, "again.tsv", got), format)
	if err != nil {
		t.Fatal(err)
	}
	before, err := table.Page(Query{})
	if err != nil {
		t.Fatal(err)
	}
	after, err := again.Page(Query{})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(before.Rows) != fmt.Sprint(after.Rows) {
		t.Fatalf("rows changed: %v, then %v", before.Rows, after.Rows)
	}
}

func TestExportKeepsJSONLinesOriginalLines(t *testing.T) {
	lines := []string{
		`{"z": 1.50, "a": null, "nested": {"k": [1, 2]}}`,
		``,
		`[1,  2]`,
		`"text"`,
		`{"z": 2}`,
	}
	table, err := OpenJSONLines(writeExportFile(t, "data.jsonl", strings.Join(lines, "\r\n")+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := lines[0] + "\n" + lines[2] + "\n" + lines[3] + "\n" + lines[4] + "\n"
	if got := exported(t, table, nil, nil); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	column := -1
	for index, name := range table.source.columns() {
		if name.Name == "z" {
			column = index
		}
	}
	filters := []Condition{{Column: column, Op: OpGreaterEqual, Value: "2"}}
	if got := exported(t, table, filters, nil); got != lines[4]+"\n" {
		t.Fatalf("filtered = %q", got)
	}
	sorted := exported(t, table, []Condition{{Column: column, Op: OpNotEmpty}}, &Sort{Column: column, Descending: true})
	if sorted != lines[4]+"\n"+lines[0]+"\n" {
		t.Fatalf("sorted = %q", sorted)
	}
}

func TestExportKeepsOrderAcrossBatches(t *testing.T) {
	total := exportBatch*2 + 5
	var content, want strings.Builder
	content.WriteString("n\n")
	want.WriteString("n\n")
	for n := 1; n <= total; n++ {
		fmt.Fprintf(&content, "%d\n", n)
		fmt.Fprintf(&want, "%d\n", total+1-n)
	}
	table, err := OpenFile(writeExportFile(t, "data.csv", content.String()), Format{Separator: ',', Header: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := exported(t, table, nil, &Sort{Column: 0, Descending: true}); got != want.String() {
		t.Fatalf("output differs; got %d bytes, want %d", len(got), want.Len())
	}
}

func TestExportWritesADirectoryListing(t *testing.T) {
	target := writeExportFile(t, "a,b.txt", "x")
	listing, err := OpenDirectory(Target{Path: filepath.Dir(target.Path), Roots: target.Roots}, Listing{})
	if err != nil {
		t.Fatal(err)
	}
	got := exported(t, listing, nil, nil)
	first, rest, _ := strings.Cut(got, "\n")
	if !strings.HasPrefix(first, "path,") || !strings.HasPrefix(rest, `"a,b.txt",`) {
		t.Fatalf("output = %q", got)
	}
}

func TestExportRefusesWhatPagesRefuse(t *testing.T) {
	table, err := OpenFile(writeExportFile(t, "data.csv", "a\n1\n"), Format{Separator: ',', Header: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := table.PrepareExport([]Condition{{Column: 5, Op: OpEqual}}, nil); err == nil {
		t.Error("missing filter column accepted")
	}
	if _, err := table.PrepareExport([]Condition{{Column: 0, Op: OpGreater, Value: "high"}}, nil); err == nil {
		t.Error("ordering against text accepted")
	}
	if _, err := table.PrepareExport(nil, &Sort{Column: 9}); err == nil {
		t.Error("sort by a missing column accepted")
	}
}

// failingSource fails reading rows after its first batch.
type failingSource struct {
	source
	calls int
}

func (source *failingSource) len() int { return exportBatch + 1 }
func (source *failingSource) exportHeader() ([]byte, error) {
	return []byte("header\n"), nil
}
func (source *failingSource) exportRecords(numbers []int) ([][]byte, error) {
	source.calls++
	if source.calls > 1 {
		return nil, errors.New("read failed")
	}
	return make([][]byte, len(numbers)), nil
}

func TestExportStreamReportsAReadFailure(t *testing.T) {
	export := &Export{source: &failingSource{}}
	var output bytes.Buffer
	if err := export.Stream(context.Background(), &output); err == nil || !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestExportStreamStopsWhenCancelled(t *testing.T) {
	source := &failingSource{}
	export := &Export{source: source}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if err := export.Stream(ctx, &output); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if source.calls != 0 {
		t.Fatalf("read %d batches after cancelling", source.calls)
	}
}
