package decor

import (
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/mattn/go-runewidth"
)

var (
	_ Decorator = (*mergeDecorator)(nil)
	_ Wrapper   = (*mergeDecorator)(nil)
	_ Decorator = (*placeHolderDecorator)(nil)
)

// Merge wraps its decorator argument with intention to sync width
// with several decorators of another bar. Visual example:
//
//	+----+--------+---------+--------+
//	| B1 |      MERGE(D, P1, Pn)     |
//	+----+--------+---------+--------+
//	| B2 |   D0   |   D1    |   Dn   |
//	+----+--------+---------+--------+
func Merge(decorator Decorator, placeholders ...WC) Decorator {
	if decorator == nil {
		return nil
	}
	if _, ok := decorator.Sync(); !ok || len(placeholders) == 0 {
		return decorator
	}
	md := &mergeDecorator{
		Decorator:    decorator,
		wc:           decorator.GetConf(),
		placeHolders: make([]Decorator, len(placeholders)),
	}
	decorator.SetConf(WC{})
	for i, wc := range placeholders {
		if (wc.C & DSyncWidth) == 0 {
			return decorator
		}
		md.placeHolders[i] = &placeHolderDecorator{wc.Init()}
	}
	return md
}

type mergeDecorator struct {
	Decorator
	wc           WC
	placeHolders []Decorator
}

func (d *mergeDecorator) GetConf() WC {
	return d.wc
}

func (d *mergeDecorator) SetConf(conf WC) {
	d.wc = conf.Init()
}

func (d *mergeDecorator) PlaceHolders() []Decorator {
	return d.placeHolders
}

func (d *mergeDecorator) Sync() (chan int, bool) {
	return d.wc.Sync()
}

func (d *mergeDecorator) Unwrap() Decorator {
	return d.Decorator
}

func (d *mergeDecorator) Decor(s Statistics) string {
	msg := d.Decorator.Decor(s)
	pureWidth := runewidth.StringWidth(msg)
	stripWidth := runewidth.StringWidth(stripansi.Strip(msg))
	cellCount := stripWidth
	if (d.wc.C & DextraSpace) != 0 {
		cellCount++
	}

	total := runewidth.StringWidth(d.placeHolders[0].GetConf().FormatMsg(""))
	pw := (cellCount - total) / len(d.placeHolders)
	rem := (cellCount - total) % len(d.placeHolders)

	var diff int
	for i := 1; i < len(d.placeHolders); i++ {
		wc := d.placeHolders[i].GetConf()
		width := pw - diff
		if (wc.C & DextraSpace) != 0 {
			width--
			if width < 0 {
				width = 0
			}
		}
		max := runewidth.StringWidth(wc.FormatMsg(strings.Repeat(" ", width)))
		total += max
		diff = max - pw
	}

	d.wc.wsync <- pw + rem
	max := <-d.wc.wsync
	return d.wc.fill(msg, max+total+(pureWidth-stripWidth))
}

type placeHolderDecorator struct {
	WC
}

func (d *placeHolderDecorator) Decor(Statistics) string {
	return ""
}
