package decor

// Any decorator displays text, that can be changed during decorator's
// lifetime via provided func call back.
//
//	`f` call back which provides string to display
//
//	`wcc` optional WC config
//
func Any(f func(*Statistics) string, wcc ...WC) Decorator {
	return &any{initWC(wcc...), f}
}

type any struct {
	WC
	f func(*Statistics) string
}

func (d *any) Decor(s *Statistics) string {
	return d.FormatMsg(d.f(s))
}
