package decor

// OnAbort returns decorator, which wraps provided decorator with sole
// purpose to display provided message on abort event. It has no effect
// if bar.Abort(drop bool) is called with true argument.
//
//	`decorator` Decorator to wrap
//
//	`message` message to display on abort event
//
func OnAbort(decorator Decorator, message string) Decorator {
	if decorator == nil {
		return nil
	}
	d := &onAbortWrapper{
		Decorator: decorator,
		msg:       message,
	}
	if md, ok := decorator.(*mergeDecorator); ok {
		d.Decorator, md.Decorator = md.Decorator, d
		return md
	}
	return d
}

type onAbortWrapper struct {
	Decorator
	msg string
}

func (d *onAbortWrapper) Decor(s Statistics) string {
	if s.Aborted {
		wc := d.GetConf()
		return wc.FormatMsg(d.msg)
	}
	return d.Decorator.Decor(s)
}

func (d *onAbortWrapper) Base() Decorator {
	return d.Decorator
}
