package decor

// Any decorator displays text, that can be changed during decorator's
// lifetime via provided DecorFunc.
//
//	`fn` DecorFunc callback
//
//	`wcc` optional WC config
//
func Any(fn DecorFunc, wcc ...WC) Decorator {
	return &any{initWC(wcc...), fn}
}

type any struct {
	WC
	fn DecorFunc
}

func (d *any) Decor(s Statistics) string {
	return d.FormatMsg(d.fn(s))
}
