package projectrun

import (
	"strconv"
	"strings"

	"github.com/rotari-dev/rotari/internal/model"
)

// readLoadAverage returns the host's load average: from /proc/loadavg on
// Linux and sysctl vm.loadavg on macOS, or nil where it is unavailable or
// malformed. Load is observational, so its absence is not an error.
func readLoadAverage() *model.LoadAverage { return platformLoadAverage() }

func parseLoadAverage(data string) *model.LoadAverage {
	fields := strings.Fields(data)
	if len(fields) < 3 {
		return nil
	}
	one, oneErr := strconv.ParseFloat(fields[0], 64)
	five, fiveErr := strconv.ParseFloat(fields[1], 64)
	fifteen, fifteenErr := strconv.ParseFloat(fields[2], 64)
	if oneErr != nil || fiveErr != nil || fifteenErr != nil {
		return nil
	}
	return &model.LoadAverage{One: one, Five: five, Fifteen: fifteen}
}
