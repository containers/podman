package decor

// Name returns name decorator.
//
//	`name` string to display
//
//	`wcc` optional WC config
func Name(name string, wcc ...WC) Decorator {
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	d := &nameDecorator{
		WC:  wc.Init(),
		msg: name,
	}
	return d
}

type nameDecorator struct {
	WC
	msg string
}

func (d *nameDecorator) Decor(st *Statistics) string {
	return d.FormatMsg(d.msg)
}
