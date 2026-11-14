package projectrun

import (
	"golang.org/x/sys/unix"

	"github.com/rotari-dev/rotari/internal/model"
)

func platformLoadAverage() *model.LoadAverage {
	data, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return nil
	}
	return decodeDarwinLoadAverage(data)
}
