package decor

var defaultSpinnerStyle = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner returns spinner decorator.
//
//	`frames` spinner frames, if nil or len==0, default is used
//
//	`wcc` optional WC config
func Spinner(frames []string, wcc ...WC) Decorator {
	var wc WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	if len(frames) == 0 {
		frames = defaultSpinnerStyle
	}
	d := &spinnerDecorator{
		WC:     wc.Init(),
		frames: frames,
	}
	return d
}

type spinnerDecorator struct {
	WC
	frames []string
	count  uint
}

func (d *spinnerDecorator) Decor(st *Statistics) string {
	frame := d.frames[d.count%uint(len(d.frames))]
	d.count++
	return d.FormatMsg(frame)
}
