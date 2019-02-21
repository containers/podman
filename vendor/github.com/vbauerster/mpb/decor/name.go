package decor

// StaticName returns name decorator.
//
//	`name` string to display
//
//	`wcc` optional WC config
func StaticName(name string, wcc ...WC) Decorator {
	return Name(name, wcc...)
}

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
	wc.Init()
	d := &nameDecorator{
		WC:  wc,
		msg: name,
	}
	return d
}

type nameDecorator struct {
	WC
	msg      string
	complete *string
}

func (d *nameDecorator) Decor(st *Statistics) string {
	if st.Completed && d.complete != nil {
		return d.FormatMsg(*d.complete)
	}
	return d.FormatMsg(d.msg)
}

func (d *nameDecorator) OnCompleteMessage(msg string) {
	d.complete = &msg
}
