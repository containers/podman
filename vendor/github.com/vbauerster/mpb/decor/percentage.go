package decor

import (
	"fmt"

	"github.com/vbauerster/mpb/internal"
)

// Percentage returns percentage decorator.
//
//	`wcc` optional WC config
func Percentage(wcc ...WC) Decorator {
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	wc.Init()
	d := &percentageDecorator{
		WC: wc,
	}
	return d
}

type percentageDecorator struct {
	WC
	completeMsg *string
}

func (d *percentageDecorator) Decor(st *Statistics) string {
	if st.Completed && d.completeMsg != nil {
		return d.FormatMsg(*d.completeMsg)
	}
	str := fmt.Sprintf("%d %%", internal.Percentage(st.Total, st.Current, 100))
	return d.FormatMsg(str)
}

func (d *percentageDecorator) OnCompleteMessage(msg string) {
	d.completeMsg = &msg
}
