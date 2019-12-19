package decor

import (
	"fmt"
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
		md.placeHolders[i] = &placeHolderDecorator{
			WC:  wc.Init(),
			wch: make(chan int),
		}
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

func (d *mergeDecorator) Decor(st *Statistics) string {
	msg := d.Decorator.Decor(st)
	msgLen := utf8.RuneCountInString(msg)

	var space int
	for _, ph := range d.placeHolders {
		space += <-ph.wch
	}

	d.wc.wsync <- msgLen - space

	max := <-d.wc.wsync
	if (d.wc.C & DextraSpace) != 0 {
		max++
	}
	return fmt.Sprintf(fmt.Sprintf(d.wc.dynFormat, max+space), msg)
}

type placeHolderDecorator struct {
	WC
	wch chan int
}

func (d *placeHolderDecorator) Decor(st *Statistics) string {
	go func() {
		d.wch <- utf8.RuneCountInString(d.FormatMsg(""))
	}()
	return ""
}
