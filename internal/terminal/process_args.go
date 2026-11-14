package terminal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// parseDarwinProcessArgs reads argv from kern.procargs2 without including the
// environment that follows it.
func parseDarwinProcessArgs(data []byte) ([]string, error) {
	if len(data) < 4 {
		return nil, errors.New("process argument record is too short")
	}
	count := binary.LittleEndian.Uint32(data[:4])
	data = data[4:]
	if count == 0 || uint64(count) > uint64(len(data)) {
		return nil, fmt.Errorf("invalid process argument count %d", count)
	}
	end := bytes.IndexByte(data, 0)
	if end <= 0 {
		return nil, errors.New("process executable path is missing")
	}
	start := end + 1
	// macOS aligns a 64-bit process's argv after the executable path.
	padding := (8 - start%8) % 8
	if len(data) < start+padding {
		return nil, errors.New("process executable padding is truncated")
	}
	for _, value := range data[start : start+padding] {
		if value != 0 {
			return nil, errors.New("invalid process executable padding")
		}
	}
	data = data[start+padding:]
	if len(data) == 0 {
		return nil, errors.New("process arguments are missing")
	}
	args := make([]string, 0, int(count))
	for range count {
		end = bytes.IndexByte(data, 0)
		if end < 0 {
			return nil, errors.New("process argument record is truncated")
		}
		args = append(args, string(data[:end]))
		data = data[end+1:]
	}
	return args, nil
}
