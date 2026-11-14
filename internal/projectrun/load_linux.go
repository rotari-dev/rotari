package projectrun

import (
	"os"

	"github.com/rotari-dev/rotari/internal/model"
)

func platformLoadAverage() *model.LoadAverage {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil
	}
	return parseLoadAverage(string(data))
}
