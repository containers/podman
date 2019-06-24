package decor

import (
	"fmt"
	"math"
	"time"

	"github.com/VividCortex/ewma"
)

type TimeNormalizer func(time.Duration) time.Duration

// EwmaETA exponential-weighted-moving-average based ETA decorator.
//
//	`style` one of [ET_STYLE_GO|ET_STYLE_HHMMSS|ET_STYLE_HHMM|ET_STYLE_MMSS]
//
//	`age` is the previous N samples to average over.
//
//	`wcc` optional WC config
func EwmaETA(style TimeStyle, age float64, wcc ...WC) Decorator {
	return MovingAverageETA(style, ewma.NewMovingAverage(age), NopNormalizer(), wcc...)
}

// MovingAverageETA decorator relies on MovingAverage implementation to calculate its average.
//
//	`style` one of [ET_STYLE_GO|ET_STYLE_HHMMSS|ET_STYLE_HHMM|ET_STYLE_MMSS]
//
//	`average` available implementations of MovingAverage [ewma.MovingAverage|NewMedian|NewMedianEwma]
//
//	`normalizer` available implementations are [NopNormalizer|FixedIntervalTimeNormalizer|MaxTolerateTimeNormalizer]
//
//	`wcc` optional WC config
func MovingAverageETA(style TimeStyle, average MovingAverage, normalizer TimeNormalizer, wcc ...WC) Decorator {
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	wc.Init()
	d := &movingAverageETA{
		WC:         wc,
		style:      style,
		average:    average,
		normalizer: normalizer,
	}
	return d
}

type movingAverageETA struct {
	WC
	style       TimeStyle
	average     ewma.MovingAverage
	completeMsg *string
	normalizer  TimeNormalizer
}

func (d *movingAverageETA) Decor(st *Statistics) string {
	if st.Completed && d.completeMsg != nil {
		return d.FormatMsg(*d.completeMsg)
	}

	v := math.Round(d.average.Value())
	remaining := d.normalizer(time.Duration((st.Total - st.Current) * int64(v)))
	hours := int64((remaining / time.Hour) % 60)
	minutes := int64((remaining / time.Minute) % 60)
	seconds := int64((remaining / time.Second) % 60)

	var str string
	switch d.style {
	case ET_STYLE_GO:
		str = fmt.Sprint(time.Duration(remaining.Seconds()) * time.Second)
	case ET_STYLE_HHMMSS:
		str = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	case ET_STYLE_HHMM:
		str = fmt.Sprintf("%02d:%02d", hours, minutes)
	case ET_STYLE_MMSS:
		if hours > 0 {
			str = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
		} else {
			str = fmt.Sprintf("%02d:%02d", minutes, seconds)
		}
	}

	return d.FormatMsg(str)
}

func (d *movingAverageETA) NextAmount(n int, wdd ...time.Duration) {
	var workDuration time.Duration
	for _, wd := range wdd {
		workDuration = wd
	}
	lastItemEstimate := float64(workDuration) / float64(n)
	if math.IsInf(lastItemEstimate, 0) || math.IsNaN(lastItemEstimate) {
		return
	}
	d.average.Add(lastItemEstimate)
}

func (d *movingAverageETA) OnCompleteMessage(msg string) {
	d.completeMsg = &msg
}

// AverageETA decorator.
//
//	`style` one of [ET_STYLE_GO|ET_STYLE_HHMMSS|ET_STYLE_HHMM|ET_STYLE_MMSS]
//
//	`wcc` optional WC config
func AverageETA(style TimeStyle, wcc ...WC) Decorator {
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	wc.Init()
	d := &averageETA{
		WC:        wc,
		style:     style,
		startTime: time.Now(),
	}
	return d
}

type averageETA struct {
	WC
	style       TimeStyle
	startTime   time.Time
	completeMsg *string
}

func (d *averageETA) Decor(st *Statistics) string {
	if st.Completed && d.completeMsg != nil {
		return d.FormatMsg(*d.completeMsg)
	}

	var str string
	timeElapsed := time.Since(d.startTime)
	v := math.Round(float64(timeElapsed) / float64(st.Current))
	if math.IsInf(v, 0) || math.IsNaN(v) {
		v = 0
	}
	remaining := time.Duration((st.Total - st.Current) * int64(v))
	hours := int64((remaining / time.Hour) % 60)
	minutes := int64((remaining / time.Minute) % 60)
	seconds := int64((remaining / time.Second) % 60)

	switch d.style {
	case ET_STYLE_GO:
		str = fmt.Sprint(time.Duration(remaining.Seconds()) * time.Second)
	case ET_STYLE_HHMMSS:
		str = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	case ET_STYLE_HHMM:
		str = fmt.Sprintf("%02d:%02d", hours, minutes)
	case ET_STYLE_MMSS:
		if hours > 0 {
			str = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
		} else {
			str = fmt.Sprintf("%02d:%02d", minutes, seconds)
		}
	}

	return d.FormatMsg(str)
}

func (d *averageETA) OnCompleteMessage(msg string) {
	d.completeMsg = &msg
}

func MaxTolerateTimeNormalizer(maxTolerate time.Duration) TimeNormalizer {
	var normalized time.Duration
	var lastCall time.Time
	return func(remaining time.Duration) time.Duration {
		if diff := normalized - remaining; diff <= 0 || diff > maxTolerate || remaining < maxTolerate/2 {
			normalized = remaining
			lastCall = time.Now()
			return remaining
		}
		normalized -= time.Since(lastCall)
		lastCall = time.Now()
		return normalized
	}
}

func FixedIntervalTimeNormalizer(updInterval int) TimeNormalizer {
	var normalized time.Duration
	var lastCall time.Time
	var count int
	return func(remaining time.Duration) time.Duration {
		if count == 0 || remaining <= time.Duration(15*time.Second) {
			count = updInterval
			normalized = remaining
			lastCall = time.Now()
			return remaining
		}
		count--
		normalized -= time.Since(lastCall)
		lastCall = time.Now()
		if normalized > 0 {
			return normalized
		}
		return remaining
	}
}

func NopNormalizer() TimeNormalizer {
	return func(remaining time.Duration) time.Duration {
		return remaining
	}
}
