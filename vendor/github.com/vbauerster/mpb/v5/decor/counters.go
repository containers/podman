package decor

import (
	"fmt"
)

const (
	_ = iota
	UnitKiB
	UnitKB
)

// CountersNoUnit is a wrapper around Counters with no unit param.
func CountersNoUnit(pairFmt string, wcc ...WC) Decorator {
	return Counters(0, pairFmt, wcc...)
}

// CountersKibiByte is a wrapper around Counters with predefined unit
// UnitKiB (bytes/1024).
func CountersKibiByte(pairFmt string, wcc ...WC) Decorator {
	return Counters(UnitKiB, pairFmt, wcc...)
}

// CountersKiloByte is a wrapper around Counters with predefined unit
// UnitKB (bytes/1000).
func CountersKiloByte(pairFmt string, wcc ...WC) Decorator {
	return Counters(UnitKB, pairFmt, wcc...)
}

// Counters decorator with dynamic unit measure adjustment.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`pairFmt` printf compatible verbs for current and total, like "%f" or "%d"
//
//	`wcc` optional WC config
//
// pairFmt example if unit=UnitKB:
//
//	pairFmt="%.1f / %.1f"   output: "1.0MB / 12.0MB"
//	pairFmt="% .1f / % .1f" output: "1.0 MB / 12.0 MB"
//	pairFmt="%d / %d"       output: "1MB / 12MB"
//	pairFmt="% d / % d"     output: "1 MB / 12 MB"
//
func Counters(unit int, pairFmt string, wcc ...WC) Decorator {
	return Any(chooseSizeProducer(unit, pairFmt), wcc...)
}

func chooseSizeProducer(unit int, format string) DecorFunc {
	if format == "" {
		format = "%d / %d"
	}
	switch unit {
	case UnitKiB:
		return func(s Statistics) string {
			return fmt.Sprintf(format, SizeB1024(s.Current), SizeB1024(s.Total))
		}
	case UnitKB:
		return func(s Statistics) string {
			return fmt.Sprintf(format, SizeB1000(s.Current), SizeB1000(s.Total))
		}
	default:
		return func(s Statistics) string {
			return fmt.Sprintf(format, s.Current, s.Total)
		}
	}
}
