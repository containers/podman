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
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	d := &countersDecorator{
		WC:       wc.Init(),
		producer: chooseSizeProducer(unit, pairFmt),
	}
	return d
}

type countersDecorator struct {
	WC
	producer func(*Statistics) string
}

func (d *countersDecorator) Decor(st *Statistics) string {
	return d.FormatMsg(d.producer(st))
}

func chooseSizeProducer(unit int, format string) func(*Statistics) string {
	if format == "" {
		format = "%d / %d"
	}
	switch unit {
	case UnitKiB:
		return func(st *Statistics) string {
			return fmt.Sprintf(format, SizeB1024(st.Current), SizeB1024(st.Total))
		}
	case UnitKB:
		return func(st *Statistics) string {
			return fmt.Sprintf(format, SizeB1000(st.Current), SizeB1000(st.Total))
		}
	default:
		return func(st *Statistics) string {
			return fmt.Sprintf(format, st.Current, st.Total)
		}
	}
}
