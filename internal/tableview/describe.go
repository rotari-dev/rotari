package tableview

import (
	"cmp"
	"encoding/json"
	"hash/maphash"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

// uniqueLimit bounds the distinct values counted per column.
const uniqueLimit = 100_000

// Bounds on Describe's distributions, which are exact or absent. These are
// variables for tests.
var (
	// distributionLimit bounds the numbers one Describe holds across all
	// columns, for quartiles and histograms: about 80 MiB.
	distributionLimit = 10_000_000
	// topTrackLimit bounds the distinct values tracked per column for its top
	// values: with topValueBytes, about 2.5 MiB.
	topTrackLimit = 10_000
)

const (
	topValues     = 5
	topValueBytes = 256
	// histogramBins is how many equal-width bins a histogram has.
	histogramBins = 20
)

// Histogram counts a column's numbers in equal-width bins from Min to Max.
type Histogram struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Counts []int   `json:"counts"`
}

// TopValue is one of a column's most frequent values. Cut reports a value
// shortened to topValueBytes.
type TopValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
	Cut   bool   `json:"cut,omitempty"`
}

// ColumnStats summarizes one column's non-empty cells, like pandas'
// describe: how many, how many distinct, and for those that read as
// numbers, their mean, sample standard deviation, extremes, and sum.
type ColumnStats struct {
	Count  int `json:"count"`
	Unique int `json:"unique"`
	// UniqueOver reports more than uniqueLimit distinct values; Unique then
	// stops at the limit.
	UniqueOver bool     `json:"unique_over,omitempty"`
	Numeric    int      `json:"numeric"`
	Mean       *float64 `json:"mean,omitempty"`
	Std        *float64 `json:"std,omitempty"`
	Min        *float64 `json:"min,omitempty"`
	Max        *float64 `json:"max,omitempty"`
	Sum        *float64 `json:"sum,omitempty"`
	// The quartiles of the numbers, interpolated linearly as pandas does.
	Q25       *float64   `json:"q25,omitempty"`
	Q50       *float64   `json:"q50,omitempty"`
	Q75       *float64   `json:"q75,omitempty"`
	Histogram *Histogram `json:"histogram,omitempty"`
	// Top is the most frequent non-blank values, most frequent and then first
	// seen first, unless every value appears once.
	Top []TopValue `json:"top,omitempty"`
	// TopOver reports too many distinct values to say which are most frequent.
	TopOver bool `json:"top_over,omitempty"`
}

// Description is Describe's result: one entry per column, over the rows
// the filters select.
type Description struct {
	Columns []ColumnStats `json:"columns"`
	Matched int           `json:"matched"`
	Total   int           `json:"total"`
	// DistributionOver reports more numbers than distributionLimit, which leaves
	// out every column's quartiles and histogram.
	DistributionOver bool `json:"distribution_over,omitempty"`
}

// columnTally accumulates one column's statistics in one pass.
type columnTally struct {
	count, numeric int
	seen           map[uint64]struct{}
	over           bool
	// Welford's running mean and sum of squared differences.
	mean, squares, sum, min, max float64
	// values holds the numbers, for the quartiles and histogram.
	values []float64
	// tops counts each distinct value, until there are too many to track.
	tops    map[uint64]*topCount
	topOver bool
}

// topCount is a distinct value's count, and when it was first seen.
type topCount struct {
	value TopValue
	order int
}

// Describe summarizes every column over the rows the filters select. It
// reads the whole source once, and is cached per source version and
// filters.
func (table *Table) Describe(filters []Condition) (Description, error) {
	columns := table.source.columns()
	conditions, err := compileConditions(filters, columns)
	if err != nil {
		return Description{}, err
	}
	key, _ := json.Marshal(struct {
		Source  string
		Filters []condition
	}{table.source.key(), conditions})
	value, err := results.get("describe\x00"+string(key), func() (any, error) {
		return table.describe(columns, conditions)
	})
	if err != nil {
		return Description{}, err
	}
	return value.(Description), nil
}

func (table *Table) describe(columns []Column, conditions []condition) (Description, error) {
	seed := maphash.MakeSeed()
	tallies := make([]columnTally, len(columns))
	for index := range tallies {
		tallies[index].seen = map[uint64]struct{}{}
		tallies[index].tops = map[uint64]*topCount{}
	}
	matched := 0
	held, over := 0, false
	err := table.source.scan(func(number int, cells []string) bool {
		if !matchesAll(conditions, cells) {
			return true
		}
		matched++
		for index := range min(len(cells), len(tallies)) {
			value, numeric := tallies[index].add(seed, cells[index])
			if !numeric || over {
				continue
			}
			if held == distributionLimit {
				over = true
				for other := range tallies {
					tallies[other].values = nil
				}
				continue
			}
			tallies[index].values = append(tallies[index].values, value)
			held++
		}
		return true
	})
	if err != nil {
		return Description{}, err
	}
	description := Description{Columns: make([]ColumnStats, len(tallies)), Matched: matched, Total: table.source.len(), DistributionOver: over}
	for index, tally := range tallies {
		description.Columns[index] = tally.stats()
	}
	return description, nil
}

// add counts a cell, and returns its number if it reads as one.
func (tally *columnTally) add(seed maphash.Seed, cell string) (float64, bool) {
	if strings.TrimSpace(cell) == "" {
		return 0, false
	}
	tally.count++
	hash := maphash.String(seed, cell)
	if !tally.over {
		tally.seen[hash] = struct{}{}
		if len(tally.seen) > uniqueLimit {
			tally.over = true
		}
	}
	if !tally.topOver {
		if found := tally.tops[hash]; found != nil {
			found.value.Count++
		} else if len(tally.tops) == topTrackLimit {
			tally.topOver, tally.tops = true, nil
		} else {
			shown, cut := cutTopValue(cell)
			tally.tops[hash] = &topCount{value: TopValue{Value: shown, Count: 1, Cut: cut}, order: len(tally.tops)}
		}
	}
	value, ok := parseNumber(cell)
	if !ok {
		return 0, false
	}
	tally.numeric++
	if tally.numeric == 1 {
		tally.min, tally.max = value, value
	}
	tally.min, tally.max = math.Min(tally.min, value), math.Max(tally.max, value)
	tally.sum += value
	delta := value - tally.mean
	tally.mean += delta / float64(tally.numeric)
	tally.squares += delta * (value - tally.mean)
	return value, true
}

// cutTopValue shortens a value to topValueBytes, on a rune boundary.
func cutTopValue(cell string) (string, bool) {
	if len(cell) <= topValueBytes {
		return cell, false
	}
	end := topValueBytes
	for end > 0 && !utf8.RuneStart(cell[end]) {
		end--
	}
	return cell[:end], true
}

// mostFrequent is the column's top values.
func (tally columnTally) mostFrequent() []TopValue {
	counts := make([]*topCount, 0, len(tally.tops))
	repeated := false
	for _, count := range tally.tops {
		counts = append(counts, count)
		repeated = repeated || count.value.Count > 1
	}
	if !repeated {
		return nil
	}
	slices.SortFunc(counts, func(a, b *topCount) int {
		return cmp.Or(cmp.Compare(b.value.Count, a.value.Count), cmp.Compare(a.order, b.order))
	})
	top := make([]TopValue, 0, topValues)
	for _, count := range counts[:min(topValues, len(counts))] {
		top = append(top, count.value)
	}
	return top
}

// quantile interpolates linearly between sorted values, as pandas does.
func quantile(sorted []float64, fraction float64) float64 {
	position := fraction * float64(len(sorted)-1)
	low := int(position)
	if low == len(sorted)-1 || sorted[low] == sorted[low+1] {
		return sorted[low]
	}
	return sorted[low] + (sorted[low+1]-sorted[low])*(position-float64(low))
}

// histogramOf counts sorted values in equal-width bins, with the largest in the
// last. Equal values make one bin. A range past float64's has none.
func histogramOf(sorted []float64) *Histogram {
	low, high := sorted[0], sorted[len(sorted)-1]
	width := high - low
	if math.IsInf(width, 0) {
		return nil
	}
	if width == 0 {
		return &Histogram{Min: low, Max: high, Counts: []int{len(sorted)}}
	}
	histogram := &Histogram{Min: low, Max: high, Counts: make([]int, histogramBins)}
	for _, value := range sorted {
		histogram.Counts[min(int((value-low)/width*histogramBins), histogramBins-1)]++
	}
	return histogram
}

func (tally columnTally) stats() ColumnStats {
	stats := ColumnStats{Count: tally.count, Unique: min(len(tally.seen), uniqueLimit), UniqueOver: tally.over, Numeric: tally.numeric, Top: tally.mostFrequent(), TopOver: tally.topOver}
	if tally.numeric == 0 {
		return stats
	}
	// A sum or spread past float64's range is left out rather than shown
	// as infinite.
	number := func(value float64) *float64 {
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return nil
		}
		return &value
	}
	stats.Mean, stats.Min, stats.Max, stats.Sum = number(tally.mean), number(tally.min), number(tally.max), number(tally.sum)
	if tally.numeric > 1 {
		stats.Std = number(math.Sqrt(tally.squares / float64(tally.numeric-1)))
	}
	if len(tally.values) > 0 {
		slices.Sort(tally.values)
		stats.Q25 = number(quantile(tally.values, 0.25))
		stats.Q50 = number(quantile(tally.values, 0.5))
		stats.Q75 = number(quantile(tally.values, 0.75))
		stats.Histogram = histogramOf(tally.values)
	}
	return stats
}
