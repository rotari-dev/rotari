package tableview

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// jsonLinesCheckpointInterval is how many records lie between file offsets
// kept by a JSON Lines index.
const jsonLinesCheckpointInterval = 256

type jsonLinesIndex struct {
	checkpoints []int64
	columns     []Column
	records     int
}

type jsonLinesTable struct {
	target      Target
	id          string
	index       *jsonLinesIndex
	columnNames []string
}

// OpenJSONLines reads a file of one JSON value per line as a table. Object
// keys become columns; arrays and scalar values use a "value" column.
func OpenJSONLines(target Target) (*Table, error) {
	info, err := target.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", target.Path)
	}
	key := fmt.Sprintf("jsonl\x00%s\x00%d\x00%d", target.Path, info.Size(), info.ModTime().UnixNano())
	value, err := results.get(key, func() (any, error) {
		return buildJSONLinesIndex(target)
	})
	if err != nil {
		return nil, err
	}
	index := value.(*jsonLinesIndex)
	columns := make([]string, len(index.columns))
	for i, column := range index.columns {
		columns[i] = column.Name
	}
	return &Table{
		source: &jsonLinesTable{
			target: target, id: key, index: index, columnNames: columns,
		},
		target: target,
	}, nil
}

func buildJSONLinesIndex(target Target) (*jsonLinesIndex, error) {
	file, err := target.open(target.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64<<10)
	index := &jsonLinesIndex{}
	columns := map[string]struct{}{}
	var offset int64
	for {
		line, err := readJSONLinesRecord(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		start := offset
		offset += int64(len(line))
		line = trimJSONLinesEnding(line)
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		value, err := decodeJSONLinesRecord(line, index.records+1)
		if err != nil {
			return nil, err
		}
		if index.records%jsonLinesCheckpointInterval == 0 {
			index.checkpoints = append(index.checkpoints, start)
		}
		if object, ok := value.(map[string]any); ok {
			for name := range object {
				columns[name] = struct{}{}
			}
		} else {
			columns["value"] = struct{}{}
		}
		index.records++
	}
	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names)
	index.columns = make([]Column, len(names))
	for i, name := range names {
		index.columns[i] = Column{Name: name}
	}
	return index, nil
}

func readJSONLinesRecord(reader *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')
		if int64(len(line))+int64(len(part)) > maxRecordBytes {
			return nil, fmt.Errorf("a JSON Lines record is longer than %d MiB", maxRecordBytes>>20)
		}
		line = append(line, part...)
		switch err {
		case nil, io.EOF:
			if err == io.EOF && len(line) == 0 {
				return nil, io.EOF
			}
			return line, nil
		case bufio.ErrBufferFull:
		default:
			return nil, err
		}
	}
}

func trimJSONLinesEnding(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte{'\n'})
	return bytes.TrimSuffix(line, []byte{'\r'})
}

func decodeJSONLinesRecord(line []byte, number int) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("invalid JSON Lines record %d: %w", number, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("invalid JSON Lines record %d: more than one JSON value", number)
		}
		return nil, fmt.Errorf("invalid JSON Lines record %d: %w", number, err)
	}
	return value, nil
}

func (table *jsonLinesTable) key() string { return table.id }
func (table *jsonLinesTable) notesOf() []string { return nil }
func (table *jsonLinesTable) columns() []Column { return table.index.columns }
func (table *jsonLinesTable) len() int { return table.index.records }

func (table *jsonLinesTable) scan(visit func(number int, cells []string) bool) error {
	file, err := table.target.open(table.target.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64<<10)
	for number := 0; ; {
		line, err := readJSONLinesRecord(reader)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		line = trimJSONLinesEnding(line)
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		value, err := decodeJSONLinesRecord(line, number+1)
		if err != nil {
			return err
		}
		if !visit(number, jsonLinesCells(value, table.columnNames)) {
			return nil
		}
		number++
	}
}

func (table *jsonLinesTable) rows(numbers []int) ([][]string, error) {
	if len(numbers) == 0 {
		return [][]string{}, nil
	}
	first := numbers[0]
	checkpoint := first / jsonLinesCheckpointInterval
	start := checkpoint * jsonLinesCheckpointInterval
	file, err := table.target.open(table.target.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err := file.Seek(table.index.checkpoints[checkpoint], io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReaderSize(file, 64<<10)
	rows := make([][]string, 0, len(numbers))
	wanted := 0
	for number := start; wanted < len(numbers); {
		line, err := readJSONLinesRecord(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		line = trimJSONLinesEnding(line)
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		value, err := decodeJSONLinesRecord(line, number+1)
		if err != nil {
			return nil, err
		}
		if number == numbers[wanted] {
			rows = append(rows, jsonLinesCells(value, table.columnNames))
			wanted++
		}
		number++
	}
	if wanted != len(numbers) {
		return nil, fmt.Errorf("JSON Lines file changed while reading its index")
	}
	return rows, nil
}

func jsonLinesCells(value any, columns []string) []string {
	cells := make([]string, len(columns))
	if object, ok := value.(map[string]any); ok {
		for index, name := range columns {
			if field, exists := object[name]; exists {
				cells[index] = jsonLinesCell(field)
			}
		}
		return cells
	}
	for index, name := range columns {
		if name == "value" {
			cells[index] = jsonLinesCell(value)
			break
		}
	}
	return cells
}

func jsonLinesCell(value any) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return value
	case json.Number:
		return string(value)
	case bool:
		return fmt.Sprint(value)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

var _ source = (*jsonLinesTable)(nil)
