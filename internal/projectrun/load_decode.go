package projectrun

import (
	"encoding/binary"

	"github.com/rotari-dev/rotari/internal/model"
)

// decodeDarwinLoadAverage reads macOS's struct loadavg, as sysctl vm.loadavg
// returns it on its 64-bit, little-endian platforms: three uint32 fixed-point
// averages (fixpt_t), four bytes of padding, and the int64 scale (long fscale).
// It is nil for a record of another size or a zero scale.
func decodeDarwinLoadAverage(data []byte) *model.LoadAverage {
	if len(data) != 24 {
		return nil
	}
	scale := int64(binary.LittleEndian.Uint64(data[16:24]))
	if scale <= 0 {
		return nil
	}
	average := func(index int) float64 {
		return float64(binary.LittleEndian.Uint32(data[4*index:])) / float64(scale)
	}
	return &model.LoadAverage{One: average(0), Five: average(1), Fifteen: average(2)}
}
