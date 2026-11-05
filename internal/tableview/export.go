package tableview

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"slices"
)

// exportBatch is how many rows an export reads at a time, which bounds its
// memory as a page's limit does.
const exportBatch = MaxLimit

// Export is the rows a query selects, ready to write in the source's own
// format. Preparing it finds whatever would refuse the query, so that nothing
// has been written when that happens.
type Export struct {
	source source
	// numbers are the selected rows in order, or nil for every row.
	numbers []int
}

// PrepareExport selects the rows filters and sort do, as Page does.
func (table *Table) PrepareExport(filters []Condition, sort *Sort) (*Export, error) {
	conditions, err := compileConditions(filters, table.source.columns())
	if err != nil {
		return nil, err
	}
	selected, err := table.selection(conditions, sort)
	if err != nil {
		return nil, err
	}
	return &Export{source: table.source, numbers: selected}, nil
}

// Stream writes the header, if the format has one, and every selected row in
// order. An error after the first write leaves the output short, so the caller
// must not let it pass for a whole file.
func (export *Export) Stream(ctx context.Context, writer io.Writer) error {
	header, err := export.source.exportHeader()
	if err != nil {
		return err
	}
	if _, err := writer.Write(header); err != nil {
		return err
	}
	total := len(export.numbers)
	if export.numbers == nil {
		total = export.source.len()
	}
	shown := make([]int, 0, min(total, exportBatch))
	for start := 0; start < total; start += exportBatch {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(start+exportBatch, total)
		shown = shown[:0]
		for position := start; position < end; position++ {
			if export.numbers == nil {
				shown = append(shown, position)
			} else {
				shown = append(shown, export.numbers[position])
			}
		}
		// The source reads ascending numbers; a sorted batch is put back in
		// order after.
		ascending := slices.Sorted(slices.Values(shown))
		records, err := export.source.exportRecords(ascending)
		if err != nil {
			return err
		}
		byNumber := make(map[int][]byte, len(records))
		for index, record := range records {
			byNumber[ascending[index]] = record
		}
		for _, number := range shown {
			if _, err := writer.Write(byNumber[number]); err != nil {
				return err
			}
		}
	}
	return nil
}

// encodeRecord writes cells as one delimited record, which reads back as the
// same cells.
func encodeRecord(separator rune, cells []string) ([]byte, error) {
	// A writer leaves one empty cell as a blank line, which reading skips.
	if len(cells) == 1 && cells[0] == "" {
		return []byte("\"\"\n"), nil
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.Comma = separator
	if err := writer.Write(cells); err != nil {
		return nil, err
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

// encodeRecords writes each row of cells as one delimited record.
func encodeRecords(separator rune, rows [][]string) ([][]byte, error) {
	records := make([][]byte, len(rows))
	for index, cells := range rows {
		record, err := encodeRecord(separator, cells)
		if err != nil {
			return nil, err
		}
		records[index] = record
	}
	return records, nil
}
