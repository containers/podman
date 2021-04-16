package decor

import (
	"fmt"
	"strings"
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
//	`pairFmt` printf compatible verbs for current and total pair
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
	producer := func(unit int, pairFmt string) DecorFunc {
		if pairFmt == "" {
			pairFmt = "%d / %d"
		} else if strings.Count(pairFmt, "%") != 2 {
			panic("expected pairFmt with exactly 2 verbs")
		}
		switch unit {
		case UnitKiB:
			return func(s Statistics) string {
				return fmt.Sprintf(pairFmt, SizeB1024(s.Current), SizeB1024(s.Total))
			}
		case UnitKB:
			return func(s Statistics) string {
				return fmt.Sprintf(pairFmt, SizeB1000(s.Current), SizeB1000(s.Total))
			}
		default:
			return func(s Statistics) string {
				return fmt.Sprintf(pairFmt, s.Current, s.Total)
			}
		}
	}
	return Any(producer(unit, pairFmt), wcc...)
}

// TotalNoUnit is a wrapper around Total with no unit param.
func TotalNoUnit(format string, wcc ...WC) Decorator {
	return Total(0, format, wcc...)
}

// TotalKibiByte is a wrapper around Total with predefined unit
// UnitKiB (bytes/1024).
func TotalKibiByte(format string, wcc ...WC) Decorator {
	return Total(UnitKiB, format, wcc...)
}

// TotalKiloByte is a wrapper around Total with predefined unit
// UnitKB (bytes/1000).
func TotalKiloByte(format string, wcc ...WC) Decorator {
	return Total(UnitKB, format, wcc...)
}

// Total decorator with dynamic unit measure adjustment.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`format` printf compatible verb for Total
//
//	`wcc` optional WC config
//
// format example if unit=UnitKiB:
//
//	format="%.1f"  output: "12.0MiB"
//	format="% .1f" output: "12.0 MiB"
//	format="%d"    output: "12MiB"
//	format="% d"   output: "12 MiB"
//
func Total(unit int, format string, wcc ...WC) Decorator {
	producer := func(unit int, format string) DecorFunc {
		if format == "" {
			format = "%d"
		} else if strings.Count(format, "%") != 1 {
			panic("expected format with exactly 1 verb")
		}

		switch unit {
		case UnitKiB:
			return func(s Statistics) string {
				return fmt.Sprintf(format, SizeB1024(s.Total))
			}
		case UnitKB:
			return func(s Statistics) string {
				return fmt.Sprintf(format, SizeB1000(s.Total))
			}
		default:
			return func(s Statistics) string {
				return fmt.Sprintf(format, s.Total)
			}
		}
	}
	return Any(producer(unit, format), wcc...)
}

// CurrentNoUnit is a wrapper around Current with no unit param.
func CurrentNoUnit(format string, wcc ...WC) Decorator {
	return Current(0, format, wcc...)
}

// CurrentKibiByte is a wrapper around Current with predefined unit
// UnitKiB (bytes/1024).
func CurrentKibiByte(format string, wcc ...WC) Decorator {
	return Current(UnitKiB, format, wcc...)
}

// CurrentKiloByte is a wrapper around Current with predefined unit
// UnitKB (bytes/1000).
func CurrentKiloByte(format string, wcc ...WC) Decorator {
	return Current(UnitKB, format, wcc...)
}

// Current decorator with dynamic unit measure adjustment.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`format` printf compatible verb for Current
//
//	`wcc` optional WC config
//
// format example if unit=UnitKiB:
//
//	format="%.1f"  output: "12.0MiB"
//	format="% .1f" output: "12.0 MiB"
//	format="%d"    output: "12MiB"
//	format="% d"   output: "12 MiB"
//
func Current(unit int, format string, wcc ...WC) Decorator {
	producer := func(unit int, format string) DecorFunc {
		if format == "" {
			format = "%d"
		} else if strings.Count(format, "%") != 1 {
			panic("expected format with exactly 1 verb")
		}

		switch unit {
		case UnitKiB:
			return func(s Statistics) string {
				return fmt.Sprintf(format, SizeB1024(s.Current))
			}
		case UnitKB:
			return func(s Statistics) string {
				return fmt.Sprintf(format, SizeB1000(s.Current))
			}
		default:
			return func(s Statistics) string {
				return fmt.Sprintf(format, s.Current)
			}
		}
	}
	return Any(producer(unit, format), wcc...)
}

// InvertedCurrentNoUnit is a wrapper around InvertedCurrent with no unit param.
func InvertedCurrentNoUnit(format string, wcc ...WC) Decorator {
	return InvertedCurrent(0, format, wcc...)
}

// InvertedCurrentKibiByte is a wrapper around InvertedCurrent with predefined unit
// UnitKiB (bytes/1024).
func InvertedCurrentKibiByte(format string, wcc ...WC) Decorator {
	return InvertedCurrent(UnitKiB, format, wcc...)
}

// InvertedCurrentKiloByte is a wrapper around InvertedCurrent with predefined unit
// UnitKB (bytes/1000).
func InvertedCurrentKiloByte(format string, wcc ...WC) Decorator {
	return InvertedCurrent(UnitKB, format, wcc...)
}

// InvertedCurrent decorator with dynamic unit measure adjustment.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`format` printf compatible verb for InvertedCurrent
//
//	`wcc` optional WC config
//
// format example if unit=UnitKiB:
//
//	format="%.1f"  output: "12.0MiB"
//	format="% .1f" output: "12.0 MiB"
//	format="%d"    output: "12MiB"
//	format="% d"   output: "12 MiB"
//
func InvertedCurrent(unit int, format string, wcc ...WC) Decorator {
	producer := func(unit int, format string) DecorFunc {
		if format == "" {
			format = "%d"
		} else if strings.Count(format, "%") != 1 {
			panic("expected format with exactly 1 verb")
		}

		switch unit {
		case UnitKiB:
			return func(s Statistics) string {
				return fmt.Sprintf(format, SizeB1024(s.Total-s.Current))
			}
		case UnitKB:
			return func(s Statistics) string {
				return fmt.Sprintf(format, SizeB1000(s.Total-s.Current))
			}
		default:
			return func(s Statistics) string {
				return fmt.Sprintf(format, s.Total-s.Current)
			}
		}
	}
	return Any(producer(unit, format), wcc...)
}
