//go:build !linux && !darwin

package projectrun

import "github.com/rotari-dev/rotari/internal/model"

func platformLoadAverage() *model.LoadAverage { return nil }
