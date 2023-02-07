package decor

import (
	"fmt"
	"strconv"

	"github.com/vbauerster/mpb/v8/internal"
)

type percentageType float64

func (s percentageType) Format(st fmt.State, verb rune) {
	var prec int
	switch verb {
	case 'd':
	case 's':
		prec = -1
	default:
		if p, ok := st.Precision(); ok {
			prec = p
		} else {
			prec = 6
		}
	}

	p := bytesPool.Get().(*[]byte)
	b := strconv.AppendFloat(*p, float64(s), 'f', prec, 64)
	if st.Flag(' ') {
		b = append(b, ' ', '%')
	} else {
		b = append(b, '%')
	}
	_, err := st.Write(b)
	if err != nil {
		panic(err)
	}
	bytesPool.Put(p)
}

// Percentage returns percentage decorator. It's a wrapper of NewPercentage.
func Percentage(wcc ...WC) Decorator {
	return NewPercentage("% d", wcc...)
}

// NewPercentage percentage decorator with custom format string.
//
// format examples:
//
//	format="%.1f"  output: "1.0%"
//	format="% .1f" output: "1.0 %"
//	format="%d"    output: "1%"
//	format="% d"   output: "1 %"
func NewPercentage(format string, wcc ...WC) Decorator {
	if format == "" {
		format = "% d"
	}
	f := func(s Statistics) string {
		p := internal.Percentage(s.Total, s.Current, 100)
		return fmt.Sprintf(format, percentageType(p))
	}
	return Any(f, wcc...)
}
