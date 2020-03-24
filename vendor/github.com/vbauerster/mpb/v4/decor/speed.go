package decor

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/VividCortex/ewma"
)

// FmtAsSpeed adds "/s" to the end of the input formatter. To be
// used with SizeB1000 or SizeB1024 types, for example:
//
//	fmt.Printf("%.1f", FmtAsSpeed(SizeB1024(2048)))
//
func FmtAsSpeed(input fmt.Formatter) fmt.Formatter {
	return &speedFormatter{input}
}

type speedFormatter struct {
	fmt.Formatter
}

func (self *speedFormatter) Format(st fmt.State, verb rune) {
	self.Formatter.Format(st, verb)
	io.WriteString(st, "/s")
}

// EwmaSpeed exponential-weighted-moving-average based speed decorator.
// Note that it's necessary to supply bar.Incr* methods with incremental
// work duration as second argument, in order for this decorator to
// work correctly. This decorator is a wrapper of MovingAverageSpeed.
func EwmaSpeed(unit int, format string, age float64, wcc ...WC) Decorator {
	var average MovingAverage
	if age == 0 {
		average = ewma.NewMovingAverage()
	} else {
		average = ewma.NewMovingAverage(age)
	}
	return MovingAverageSpeed(unit, format, NewThreadSafeMovingAverage(average), wcc...)
}

// MovingAverageSpeed decorator relies on MovingAverage implementation
// to calculate its average.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`format` printf compatible verb for value, like "%f" or "%d"
//
//	`average` MovingAverage implementation
//
//	`wcc` optional WC config
//
// format examples:
//
//	unit=UnitKiB, format="%.1f"  output: "1.0MiB/s"
//	unit=UnitKiB, format="% .1f" output: "1.0 MiB/s"
//	unit=UnitKB,  format="%.1f"  output: "1.0MB/s"
//	unit=UnitKB,  format="% .1f" output: "1.0 MB/s"
//
func MovingAverageSpeed(unit int, format string, average MovingAverage, wcc ...WC) Decorator {
	if format == "" {
		format = "%.0f"
	}
	d := &movingAverageSpeed{
		WC:       initWC(wcc...),
		average:  average,
		producer: chooseSpeedProducer(unit, format),
	}
	return d
}

type movingAverageSpeed struct {
	WC
	producer func(float64) string
	average  ewma.MovingAverage
	msg      string
}

func (d *movingAverageSpeed) Decor(s *Statistics) string {
	if !s.Completed {
		var speed float64
		if v := d.average.Value(); v > 0 {
			speed = 1 / v
		}
		d.msg = d.producer(speed * 1e9)
	}
	return d.FormatMsg(d.msg)
}

func (d *movingAverageSpeed) NextAmount(n int64, wdd ...time.Duration) {
	var workDuration time.Duration
	for _, wd := range wdd {
		workDuration = wd
	}
	durPerByte := float64(workDuration) / float64(n)
	if math.IsInf(durPerByte, 0) || math.IsNaN(durPerByte) {
		return
	}
	d.average.Add(durPerByte)
}

// AverageSpeed decorator with dynamic unit measure adjustment. It's
// a wrapper of NewAverageSpeed.
func AverageSpeed(unit int, format string, wcc ...WC) Decorator {
	return NewAverageSpeed(unit, format, time.Now(), wcc...)
}

// NewAverageSpeed decorator with dynamic unit measure adjustment and
// user provided start time.
//
//	`unit` one of [0|UnitKiB|UnitKB] zero for no unit
//
//	`format` printf compatible verb for value, like "%f" or "%d"
//
//	`startTime` start time
//
//	`wcc` optional WC config
//
// format examples:
//
//	unit=UnitKiB, format="%.1f"  output: "1.0MiB/s"
//	unit=UnitKiB, format="% .1f" output: "1.0 MiB/s"
//	unit=UnitKB,  format="%.1f"  output: "1.0MB/s"
//	unit=UnitKB,  format="% .1f" output: "1.0 MB/s"
//
func NewAverageSpeed(unit int, format string, startTime time.Time, wcc ...WC) Decorator {
	if format == "" {
		format = "%.0f"
	}
	d := &averageSpeed{
		WC:        initWC(wcc...),
		startTime: startTime,
		producer:  chooseSpeedProducer(unit, format),
	}
	return d
}

type averageSpeed struct {
	WC
	startTime time.Time
	producer  func(float64) string
	msg       string
}

func (d *averageSpeed) Decor(s *Statistics) string {
	if !s.Completed {
		speed := float64(s.Current) / float64(time.Since(d.startTime))
		d.msg = d.producer(speed * 1e9)
	}

	return d.FormatMsg(d.msg)
}

func (d *averageSpeed) AverageAdjust(startTime time.Time) {
	d.startTime = startTime
}

func chooseSpeedProducer(unit int, format string) func(float64) string {
	switch unit {
	case UnitKiB:
		return func(speed float64) string {
			return fmt.Sprintf(format, FmtAsSpeed(SizeB1024(math.Round(speed))))
		}
	case UnitKB:
		return func(speed float64) string {
			return fmt.Sprintf(format, FmtAsSpeed(SizeB1000(math.Round(speed))))
		}
	default:
		return func(speed float64) string {
			return fmt.Sprintf(format, speed)
		}
	}
}
