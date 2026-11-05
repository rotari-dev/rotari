package tableview

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// scores has a numeric column with a blank and a non-number in it, a text
// column with repeated values, and a short row.
const scores = `id,score,text
a,0.9,meeting room
b,0.75,Meeting notes
c,,thanks
d,n/a,meeting
e,10,meeting
f,0.80
`

func openScores(t *testing.T) *Table {
	t.Helper()
	table, err := OpenFile(fileTarget(t, "scores.csv", scores), Format{Separator: ',', Header: true})
	if err != nil {
		t.Fatal(err)
	}
	return table
}

func ids(page Page) string {
	var result []string
	for _, row := range page.Rows {
		result = append(result, row.Cells[0])
	}
	return strings.Join(result, "")
}

func TestFiltersSelectRows(t *testing.T) {
	table := openScores(t)
	for _, test := range []struct {
		name    string
		filters []Condition
		want    string
	}{
		{"numbers compare by value", []Condition{{Column: 1, Op: OpGreaterEqual, Value: "0.8"}}, "aef"},
		{"orderings skip blanks and non-numbers", []Condition{{Column: 1, Op: OpLess, Value: "100"}}, "abef"},
		{"equality compares numbers as numbers", []Condition{{Column: 1, Op: OpEqual, Value: "0.8"}}, "f"},
		{"equality compares text otherwise", []Condition{{Column: 1, Op: OpEqual, Value: "n/a"}}, "d"},
		{"not equal keeps blanks", []Condition{{Column: 1, Op: OpNotEqual, Value: "0.9"}}, "bcdef"},
		{"contains is case-sensitive", []Condition{{Column: 2, Op: OpContains, Value: "meeting"}}, "ade"},
		{"does not contain", []Condition{{Column: 2, Op: OpNotContains, Value: "meeting"}}, "bcf"},
		{"a short row's missing cell is empty", []Condition{{Column: 2, Op: OpEmpty}}, "f"},
		{"not empty", []Condition{{Column: 1, Op: OpNotEmpty}}, "abdef"},
		{"every condition must match", []Condition{{Column: 2, Op: OpContains, Value: "meeting"}, {Column: 1, Op: OpGreater, Value: "1"}}, "e"},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, err := table.Page(Query{Filters: test.filters})
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(page); got != test.want || page.Matched != len(test.want) || page.Total != 6 {
				t.Fatalf("rows %q matched %d of %d, want %q", got, page.Matched, page.Total, test.want)
			}
		})
	}
}

func TestFilteredPagesKeepFileNumbers(t *testing.T) {
	table := openScores(t)
	page, err := table.Page(Query{Filters: []Condition{{Column: 2, Op: OpContains, Value: "meeting"}}, Offset: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if ids(page) != "d" || page.Rows[0].Number != 4 || page.Matched != 3 || page.Offset != 1 {
		t.Fatalf("page = %+v", page)
	}
}

func TestSortOrdersNumbersThenText(t *testing.T) {
	table := openScores(t)
	for _, test := range []struct {
		sort Sort
		want string
	}{
		// Numbers by value, then non-numbers by text with blanks first;
		// descending keeps numbers first.
		{Sort{Column: 1}, "bfaecd"},
		{Sort{Column: 1, Descending: true}, "eafbdc"},
		{Sort{Column: 2}, "fbdeac"},
	} {
		sort := test.sort
		page, err := table.Page(Query{Sort: &sort})
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(page); got != test.want {
			t.Errorf("sort %+v = %q, want %q", test.sort, got, test.want)
		}
	}
	// A sort and a filter combine, and a page of the sorted rows keeps the
	// file's numbers.
	page, err := table.Page(Query{Filters: []Condition{{Column: 1, Op: OpGreater, Value: "0"}}, Sort: &Sort{Column: 1, Descending: true}, Offset: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if ids(page) != "af" || page.Rows[0].Number != 1 || page.Rows[1].Number != 6 || page.Matched != 4 {
		t.Fatalf("sorted page = %+v", page)
	}
}

func TestRegexFiltersSearchCellText(t *testing.T) {
	content := "id,name,duration\n1,alpha,90\n2,beta,\n3,Gamma\n4,日本語,120\n5,\"a\nb\",1\n"
	table, err := OpenFile(fileTarget(t, "regex.csv", content), Format{Separator: ',', Header: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		filters []Condition
		want    []int
	}{
		{"searches anywhere", []Condition{{Column: 1, Op: OpRegex, Value: "et"}}, []int{2}},
		{"anchors", []Condition{{Column: 1, Op: OpRegex, Value: "^a"}}, []int{1, 5}},
		{"whole cell", []Condition{{Column: 1, Op: OpRegex, Value: "^a$"}}, nil},
		{"line mode", []Condition{{Column: 1, Op: OpRegex, Value: "(?m)^b$"}}, []int{5}},
		{"alternation", []Condition{{Column: 1, Op: OpRegex, Value: "^(alpha|beta)$"}}, []int{1, 2}},
		{"case-sensitive", []Condition{{Column: 1, Op: OpRegex, Value: "gamma"}}, nil},
		{"ignoring case", []Condition{{Column: 1, Op: OpRegex, Value: "(?i)gamma"}}, []int{3}},
		{"multi-byte", []Condition{{Column: 1, Op: OpRegex, Value: "^日.語$"}}, []int{4}},
		{"negated", []Condition{{Column: 1, Op: OpNotRegex, Value: "a"}}, []int{4}},
		{"a blank cell reads as empty", []Condition{{Column: 2, Op: OpNotRegex, Value: "."}}, []int{2, 3}},
		{"a pattern matching empty keeps blanks", []Condition{{Column: 2, Op: OpRegex, Value: "^$"}}, []int{2, 3}},
		{"an empty pattern matches everything", []Condition{{Column: 1, Op: OpRegex, Value: ""}}, []int{1, 2, 3, 4, 5}},
		{"all conditions must match", []Condition{{Column: 1, Op: OpRegex, Value: "a"}, {Column: 2, Op: OpRegex, Value: "^9"}}, []int{1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, err := table.Page(Query{Filters: test.filters})
			if err != nil {
				t.Fatal(err)
			}
			var got []int
			for _, row := range page.Rows {
				got = append(got, row.Number)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("rows = %v, want %v", got, test.want)
			}
			// Describe covers the same rows.
			description, err := table.Describe(test.filters)
			if err != nil || description.Matched != len(test.want) {
				t.Fatalf("describe = %+v, %v", description, err)
			}
		})
	}
}

func TestBadFiltersAreRefused(t *testing.T) {
	table := openScores(t)
	for _, filters := range [][]Condition{
		{{Column: 1, Op: OpRegex, Value: "(unclosed"}},
		{{Column: 1, Op: OpNotRegex, Value: "(?=lookahead)"}},
		{{Column: 1, Op: OpRegex, Value: strings.Repeat("(a{1000}){1000}", 10)}},
		{{Column: 3, Op: OpEqual, Value: "x"}},
		{{Column: -1, Op: OpEqual, Value: "x"}},
		{{Column: 1, Op: "~", Value: "x"}},
		{{Column: 1, Op: OpGreater, Value: "high"}},
	} {
		if _, err := table.Page(Query{Filters: filters}); err == nil {
			t.Errorf("filters %+v accepted", filters)
		}
		if _, err := table.Describe(filters); err == nil {
			t.Errorf("describe with %+v accepted", filters)
		}
	}
	if _, err := table.Page(Query{Sort: &Sort{Column: 9}}); err == nil {
		t.Error("sort by a missing column accepted")
	}
	if _, err := ParseConditions(`{"column":1}`); err == nil {
		t.Error("a filter object instead of a list accepted")
	}
	parsed, err := ParseConditions(`[{"column":1,"op":">","value":"2"}]`)
	if err != nil || !reflect.DeepEqual(parsed, []Condition{{Column: 1, Op: OpGreater, Value: "2"}}) {
		t.Errorf("parsed %+v, %v", parsed, err)
	}
}

func TestDescribeSummarizesColumns(t *testing.T) {
	table := openScores(t)
	description, err := table.Describe(nil)
	if err != nil {
		t.Fatal(err)
	}
	if description.Matched != 6 || description.Total != 6 || len(description.Columns) != 3 {
		t.Fatalf("description = %+v", description)
	}
	score := description.Columns[1]
	// Four numbers (0.9, 0.75, 10, 0.80) of five non-empty cells.
	if score.Count != 5 || score.Numeric != 4 || score.Unique != 5 || *score.Min != 0.75 || *score.Max != 10 || math.Abs(*score.Sum-12.45) > 1e-9 || math.Abs(*score.Mean-3.1125) > 1e-9 {
		t.Fatalf("score = %s", describeJSON(score))
	}
	// pandas' sample standard deviation of the same values.
	if math.Abs(*score.Std-4.59210) > 1e-4 {
		t.Fatalf("std = %v", *score.Std)
	}
	text := description.Columns[2]
	if text.Count != 5 || text.Unique != 4 || text.Numeric != 0 || text.Mean != nil || text.Sum != nil {
		t.Fatalf("text = %s", describeJSON(text))
	}

	// Filters choose the rows described.
	description, err = table.Describe([]Condition{{Column: 2, Op: OpContains, Value: "meeting"}})
	if err != nil {
		t.Fatal(err)
	}
	if description.Matched != 3 || description.Columns[1].Numeric != 2 || *description.Columns[1].Sum != 10.9 || description.Columns[1].Std == nil {
		t.Fatalf("filtered = %s", describeJSON(description))
	}
	// One number has no spread.
	description, err = table.Describe([]Condition{{Column: 0, Op: OpEqual, Value: "e"}})
	if err != nil {
		t.Fatal(err)
	}
	if description.Columns[1].Std != nil || *description.Columns[1].Mean != 10 {
		t.Fatalf("one row = %s", describeJSON(description))
	}
}

func TestDescribeShowsDistributions(t *testing.T) {
	table := openScores(t)
	description, err := table.Describe(nil)
	if err != nil {
		t.Fatal(err)
	}
	score := description.Columns[1]
	// Linear interpolation of 0.75, 0.80, 0.9, 10, as pandas' quantile gives.
	for _, quartile := range []struct {
		got  *float64
		want float64
	}{{score.Q25, 0.7875}, {score.Q50, 0.85}, {score.Q75, 3.175}} {
		if quartile.got == nil || math.Abs(*quartile.got-quartile.want) > 1e-9 {
			t.Fatalf("score = %s", describeJSON(score))
		}
	}
	// Bins are 0.04625 wide: 0.75 in the first, 0.80 in the second, 0.9 in the
	// fourth, and the largest in the last. A cell that is not a number is not
	// counted.
	histogram := score.Histogram
	if histogram == nil || len(histogram.Counts) != 200 || histogram.Min != 0.75 || histogram.Max != 10 ||
		histogram.Counts[0] != 1 || histogram.Counts[1] != 1 || histogram.Counts[3] != 1 || histogram.Counts[199] != 1 {
		t.Fatalf("histogram = %s", describeJSON(histogram))
	}
	text := description.Columns[2]
	if text.Q50 != nil || text.Histogram != nil || len(text.Top) == 0 || text.Top[0].Count != 2 {
		t.Fatalf("text = %s", describeJSON(text))
	}
}

func TestDescribeDistributionEdges(t *testing.T) {
	var column strings.Builder
	column.WriteString("same,once,mixed,blank,wide,top\n")
	values := []string{"a", "b", "a", "c", "b", "a"}
	for row := range 6 {
		wide := "1"
		if row == 5 {
			wide = "1.7e308"
		} else if row == 4 {
			wide = "-1.7e308"
		}
		column.WriteString("7,id" + strconv.Itoa(row) + ",")
		if row%2 == 0 {
			column.WriteString("x")
		} else {
			column.WriteString(strconv.Itoa(row))
		}
		column.WriteString(",," + wide + "," + values[row] + "\n")
	}
	table, err := OpenFile(fileTarget(t, "edges.csv", column.String()), Format{Separator: ',', Header: true})
	if err != nil {
		t.Fatal(err)
	}
	description, err := table.Describe(nil)
	if err != nil {
		t.Fatal(err)
	}
	same, once, mixed, blank, wide, top := description.Columns[0], description.Columns[1], description.Columns[2], description.Columns[3], description.Columns[4], description.Columns[5]
	if *same.Q25 != 7 || *same.Q75 != 7 || len(same.Histogram.Counts) != 1 || same.Histogram.Counts[0] != 6 {
		t.Fatalf("equal values = %s", describeJSON(same))
	}
	if once.Top != nil || once.TopOver {
		t.Fatalf("all unique = %s", describeJSON(once))
	}
	// Numbers count only the numeric cells; top values, all of them.
	if mixed.Numeric != 3 || *mixed.Q50 != 3 || mixed.Histogram == nil || mixed.Top[0].Value != "x" || mixed.Top[0].Count != 3 {
		t.Fatalf("mixed = %s", describeJSON(mixed))
	}
	if blank.Count != 0 || blank.Top != nil || blank.Q50 != nil || blank.Histogram != nil {
		t.Fatalf("blank = %s", describeJSON(blank))
	}
	// A range past float64's has no histogram, and encodes as JSON.
	if wide.Histogram != nil || wide.Q50 == nil {
		t.Fatalf("wide = %s", describeJSON(wide))
	}
	if _, err := json.Marshal(description); err != nil {
		t.Fatal(err)
	}
	// Most frequent first; ties go to the value seen first.
	if len(top.Top) != 3 || top.Top[0].Value != "a" || top.Top[1].Value != "b" || top.Top[2].Value != "c" {
		t.Fatalf("top = %s", describeJSON(top))
	}
}

func TestDescribeLimitsDistributions(t *testing.T) {
	restore := []int{distributionLimit, topTrackLimit}
	t.Cleanup(func() { distributionLimit, topTrackLimit = restore[0], restore[1] })
	content := "n,m\n1,1\n2,2\n3,3\n4,4\n"
	describe := func(name string) Description {
		table, err := OpenFile(fileTarget(t, name, content), Format{Separator: ',', Header: true})
		if err != nil {
			t.Fatal(err)
		}
		description, err := table.Describe(nil)
		if err != nil {
			t.Fatal(err)
		}
		return description
	}
	distributionLimit = 7
	description := describe("over.csv")
	if !description.DistributionOver {
		t.Fatalf("over = %s", describeJSON(description))
	}
	for _, column := range description.Columns {
		if column.Q50 != nil || column.Histogram != nil || *column.Sum != 10 {
			t.Fatalf("column over the limit = %s", describeJSON(column))
		}
	}
	distributionLimit = 8
	if description := describe("fits.csv"); description.DistributionOver || description.Columns[1].Q50 == nil {
		t.Fatalf("at the limit = %s", describeJSON(description))
	}
	distributionLimit, topTrackLimit = restore[0], 3
	content = "v\na\na\nb\nc\nd\n"
	if column := describe("tops.csv").Columns[0]; !column.TopOver || column.Top != nil || column.Unique != 4 {
		t.Fatalf("too many values = %s", describeJSON(column))
	}
	topTrackLimit = restore[1]
	long := strings.Repeat("あ", 100)
	content = "v\n" + long + "\n" + long + "\n"
	column := describe("long.csv").Columns[0]
	if len(column.Top) != 1 || !column.Top[0].Cut || !utf8.ValidString(column.Top[0].Value) || len(column.Top[0].Value) != 255 {
		t.Fatalf("long value = %s", describeJSON(column))
	}
}

func TestDescribeCountsUniqueValuesUpToALimit(t *testing.T) {
	var content strings.Builder
	content.WriteString("n,big\n")
	for n := range uniqueLimit + 5 {
		content.WriteString(strconv.Itoa(n) + ",1e308\n")
	}
	table, err := OpenFile(fileTarget(t, "many.csv", content.String()), Format{Separator: ',', Header: true})
	if err != nil {
		t.Fatal(err)
	}
	description, err := table.Describe(nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := description.Columns[0]; n.Unique != uniqueLimit || !n.UniqueOver || n.Count != uniqueLimit+5 {
		t.Fatalf("n = %s", describeJSON(n))
	}
	// A sum past float64's range is left out; the description still
	// encodes as JSON.
	if big := description.Columns[1]; big.Sum != nil || big.Max == nil || *big.Max != 1e308 {
		t.Fatalf("big = %s", describeJSON(big))
	}
	if _, err := json.Marshal(description); err != nil {
		t.Fatal(err)
	}
}

func describeJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
