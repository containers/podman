package decor

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/VividCortex/ewma"
)

type SpeedKiB float64

func (s SpeedKiB) Format(st fmt.State, verb rune) {
	prec, ok := st.Precision()

	if verb == 'd' || !ok {
		prec = 0
	}
	if verb == 'f' && !ok {
		prec = 6
	}
	// retain old beahavior if s verb used
	if verb == 's' {
		prec = 1
	}

	var res, unit string
	switch {
	case s >= TiB:
		unit = "TiB/s"
		res = strconv.FormatFloat(float64(s)/TiB, 'f', prec, 64)
	case s >= GiB:
		unit = "GiB/s"
		res = strconv.FormatFloat(float64(s)/GiB, 'f', prec, 64)
	case s >= MiB:
		unit = "MiB/s"
		res = strconv.FormatFloat(float64(s)/MiB, 'f', prec, 64)
	case s >= KiB:
		unit = "KiB/s"
		res = strconv.FormatFloat(float64(s)/KiB, 'f', prec, 64)
	default:
		unit = "b/s"
		res = strconv.FormatInt(int64(s), 10)
	}

	if st.Flag(' ') {
		res += " "
	}
	res += unit

	if w, ok := st.Width(); ok {
		if len(res) < w {
			pad := strings.Repeat(" ", w-len(res))
			if st.Flag(int('-')) {
				res += pad
			} else {
				res = pad + res
			}
		}
	}

	io.WriteString(st, res)
}

type SpeedKB float64

func (s SpeedKB) Format(st fmt.State, verb rune) {
	prec, ok := st.Precision()

	if verb == 'd' || !ok {
		prec = 0
	}
	if verb == 'f' && !ok {
		prec = 6
	}
	// retain old beahavior if s verb used
	if verb == 's' {
		prec = 1
	}

	var res, unit string
	switch {
	case s >= TB:
		unit = "TB/s"
		res = strconv.FormatFloat(float64(s)/TB, 'f', prec, 64)
	case s >= GB:
		unit = "GB/s"
		res = strconv.FormatFloat(float64(s)/GB, 'f', prec, 64)
	case s >= MB:
		unit = "MB/s"
		res = strconv.FormatFloat(float64(s)/MB, 'f', prec, 64)
	case s >= KB:
		unit = "kB/s"
		res = strconv.FormatFloat(float64(s)/KB, 'f', prec, 64)
	default:
		unit = "b/s"
		res = strconv.FormatInt(int64(s), 10)
	}

	if st.Flag(' ') {
		res += " "
	}
	res += unit

	if w, ok := st.Width(); ok {
		if len(res) < w {
			pad := strings.Repeat(" ", w-len(res))
			if st.Flag(int('-')) {
				res += pad
			} else {
				res = pad + res
			}
		}
	}

	io.WriteString(st, res)
}

// EwmaSpeed exponential-weighted-moving-average based speed decorator,
// with dynamic unit measure adjustment.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`unitFormat` printf compatible verb for value, like "%f" or "%d"
//
//	`average` MovingAverage implementation
//
//	`wcc` optional WC config
//
// unitFormat example if UnitKiB is chosen:
//
//	"%.1f" = "1.0MiB/s" or "% .1f" = "1.0 MiB/s"
func EwmaSpeed(unit int, unitFormat string, age float64, wcc ...WC) Decorator {
	return MovingAverageSpeed(unit, unitFormat, ewma.NewMovingAverage(age), wcc...)
}

// MovingAverageSpeed decorator relies on MovingAverage implementation
// to calculate its average.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`unitFormat` printf compatible verb for value, like "%f" or "%d"
//
//	`average` MovingAverage implementation
//
//	`wcc` optional WC config
func MovingAverageSpeed(unit int, unitFormat string, average MovingAverage, wcc ...WC) Decorator {
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	wc.Init()
	d := &movingAverageSpeed{
		WC:         wc,
		unit:       unit,
		unitFormat: unitFormat,
		average:    average,
	}
	return d
}

type movingAverageSpeed struct {
	WC
	unit        int
	unitFormat  string
	average     ewma.MovingAverage
	msg         string
	completeMsg *string
}

func (d *movingAverageSpeed) Decor(st *Statistics) string {
	if st.Completed {
		if d.completeMsg != nil {
			return d.FormatMsg(*d.completeMsg)
		}
		return d.FormatMsg(d.msg)
	}

	speed := d.average.Value()
	switch d.unit {
	case UnitKiB:
		d.msg = fmt.Sprintf(d.unitFormat, SpeedKiB(speed))
	case UnitKB:
		d.msg = fmt.Sprintf(d.unitFormat, SpeedKB(speed))
	default:
		d.msg = fmt.Sprintf(d.unitFormat, speed)
	}

	return d.FormatMsg(d.msg)
}

func (s *movingAverageSpeed) NextAmount(n int, wdd ...time.Duration) {
	var workDuration time.Duration
	for _, wd := range wdd {
		workDuration = wd
	}
	speed := float64(n) / workDuration.Seconds() / 1000
	if math.IsInf(speed, 0) || math.IsNaN(speed) {
		return
	}
	s.average.Add(speed)
}

func (d *movingAverageSpeed) OnCompleteMessage(msg string) {
	d.completeMsg = &msg
}

// AverageSpeed decorator with dynamic unit measure adjustment.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`unitFormat` printf compatible verb for value, like "%f" or "%d"
//
//	`wcc` optional WC config
//
// unitFormat example if UnitKiB is chosen:
//
//	"%.1f" = "1.0MiB/s" or "% .1f" = "1.0 MiB/s"
func AverageSpeed(unit int, unitFormat string, wcc ...WC) Decorator {
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	wc.Init()
	d := &averageSpeed{
		WC:         wc,
		unit:       unit,
		unitFormat: unitFormat,
		startTime:  time.Now(),
	}
	return d
}

type averageSpeed struct {
	WC
	unit        int
	unitFormat  string
	startTime   time.Time
	msg         string
	completeMsg *string
}

func (d *averageSpeed) Decor(st *Statistics) string {
	if st.Completed {
		if d.completeMsg != nil {
			return d.FormatMsg(*d.completeMsg)
		}
		return d.FormatMsg(d.msg)
	}

	timeElapsed := time.Since(d.startTime)
	speed := float64(st.Current) / timeElapsed.Seconds()

	switch d.unit {
	case UnitKiB:
		d.msg = fmt.Sprintf(d.unitFormat, SpeedKiB(speed))
	case UnitKB:
		d.msg = fmt.Sprintf(d.unitFormat, SpeedKB(speed))
	default:
		d.msg = fmt.Sprintf(d.unitFormat, speed)
	}

	return d.FormatMsg(d.msg)
}

func (d *averageSpeed) OnCompleteMessage(msg string) {
	d.completeMsg = &msg
}
