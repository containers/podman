package decor

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Merge wraps its decorator argument with intention to sync width
// with several decorators of another bar. Visual example:
//
//    +----+--------+---------+--------+
//    | B1 |      MERGE(D, P1, Pn)     |
//    +----+--------+---------+--------+
//    | B2 |   D0   |   D1    |   Dn   |
//    +----+--------+---------+--------+
//
func Merge(decorator Decorator, placeholders ...WC) Decorator {
	if _, ok := decorator.Sync(); !ok || len(placeholders) == 0 {
		return decorator
	}
	md := &mergeDecorator{
		Decorator:    decorator,
		wc:           decorator.GetConf(),
		placeHolders: make([]*placeHolderDecorator, len(placeholders)),
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
	placeHolders []*placeHolderDecorator
}

func (d *mergeDecorator) GetConf() WC {
	return d.wc
}

func (d *mergeDecorator) SetConf(conf WC) {
	d.wc = conf.Init()
}

func (d *mergeDecorator) MergeUnwrap() []Decorator {
	decorators := make([]Decorator, len(d.placeHolders))
	for i, ph := range d.placeHolders {
		decorators[i] = ph
	}
	return decorators
}

func (d *mergeDecorator) Sync() (chan int, bool) {
	return d.wc.Sync()
}

func (d *mergeDecorator) Base() Decorator {
	return d.Decorator
}

func (d *mergeDecorator) Decor(s *Statistics) string {
	msg := d.Decorator.Decor(s)
	msgLen := utf8.RuneCountInString(msg)
	if (d.wc.C & DextraSpace) != 0 {
		msgLen++
	}

	var total int
	max := utf8.RuneCountInString(d.placeHolders[0].FormatMsg(""))
	total += max
	pw := (msgLen - max) / len(d.placeHolders)
	rem := (msgLen - max) % len(d.placeHolders)

	var diff int
	for i := 1; i < len(d.placeHolders); i++ {
		ph := d.placeHolders[i]
		width := pw - diff
		if (ph.WC.C & DextraSpace) != 0 {
			width--
			if width < 0 {
				width = 0
			}
		}
		max = utf8.RuneCountInString(ph.FormatMsg(strings.Repeat(" ", width)))
		total += max
		diff = max - pw
	}

	d.wc.wsync <- pw + rem
	max = <-d.wc.wsync
	return fmt.Sprintf(fmt.Sprintf(d.wc.dynFormat, max+total), msg)
}

type placeHolderDecorator struct {
	WC
}

func (d *placeHolderDecorator) Decor(*Statistics) string {
	return ""
}
