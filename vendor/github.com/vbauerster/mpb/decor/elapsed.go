package decor

import (
	"fmt"
	"time"
)

// Elapsed returns elapsed time decorator.
//
//	`style` one of [ET_STYLE_GO|ET_STYLE_HHMMSS|ET_STYLE_HHMM|ET_STYLE_MMSS]
//
//	`wcc` optional WC config
func Elapsed(style TimeStyle, wcc ...WC) Decorator {
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	wc.Init()
	d := &elapsedDecorator{
		WC:        wc,
		style:     style,
		startTime: time.Now(),
	}
	return d
}

type elapsedDecorator struct {
	WC
	style       TimeStyle
	startTime   time.Time
	msg         string
	completeMsg *string
}

func (d *elapsedDecorator) Decor(st *Statistics) string {
	if st.Completed {
		if d.completeMsg != nil {
			return d.FormatMsg(*d.completeMsg)
		}
		return d.FormatMsg(d.msg)
	}

	timeElapsed := time.Since(d.startTime)
	hours := int64((timeElapsed / time.Hour) % 60)
	minutes := int64((timeElapsed / time.Minute) % 60)
	seconds := int64((timeElapsed / time.Second) % 60)

	switch d.style {
	case ET_STYLE_GO:
		d.msg = fmt.Sprint(time.Duration(timeElapsed.Seconds()) * time.Second)
	case ET_STYLE_HHMMSS:
		d.msg = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	case ET_STYLE_HHMM:
		d.msg = fmt.Sprintf("%02d:%02d", hours, minutes)
	case ET_STYLE_MMSS:
		if hours > 0 {
			d.msg = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
		} else {
			d.msg = fmt.Sprintf("%02d:%02d", minutes, seconds)
		}
	}

	return d.FormatMsg(d.msg)
}

func (d *elapsedDecorator) OnCompleteMessage(msg string) {
	d.completeMsg = &msg
}
