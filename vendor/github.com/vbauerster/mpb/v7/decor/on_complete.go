package decor

// OnComplete returns decorator, which wraps provided decorator with
// sole purpose to display provided message on complete event.
//
//	`decorator` Decorator to wrap
//
//	`message` message to display on complete event
//
func OnComplete(decorator Decorator, message string) Decorator {
	if decorator == nil {
		return nil
	}
	d := &onCompleteWrapper{
		Decorator: decorator,
		msg:       message,
	}
	if md, ok := decorator.(*mergeDecorator); ok {
		d.Decorator, md.Decorator = md.Decorator, d
		return md
	}
	return d
}

type onCompleteWrapper struct {
	Decorator
	msg string
}

func (d *onCompleteWrapper) Decor(s Statistics) string {
	if s.Completed {
		wc := d.GetConf()
		return wc.FormatMsg(d.msg)
	}
	return d.Decorator.Decor(s)
}

func (d *onCompleteWrapper) Base() Decorator {
	return d.Decorator
}
