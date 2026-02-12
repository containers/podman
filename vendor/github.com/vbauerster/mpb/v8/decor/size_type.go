package decor

import (
	"fmt"
	"strconv"
)

//go:generate stringer -type=SizeB1024 -trimprefix=_i
//go:generate stringer -type=SizeB1000 -trimprefix=_

var (
	_ fmt.Formatter = SizeB1024(0)
	_ fmt.Stringer  = SizeB1024(0)
	_ fmt.Formatter = SizeB1000(0)
	_ fmt.Stringer  = SizeB1000(0)
)

const (
	_ib   SizeB1024 = iota + 1
	_iKiB SizeB1024 = 1 << (iota * 10)
	_iMiB
	_iGiB
	_iTiB
)

// SizeB1024 named type, which implements fmt.Formatter interface. It
// adjusts its value according to byte size multiple by 1024 and appends
// appropriate size marker (KiB, MiB, GiB, TiB).
type SizeB1024 int64

func (self SizeB1024) Format(st fmt.State, verb rune) {
	prec := -1
	switch verb {
	case 'f', 'e', 'E':
		prec = 6 // default prec of fmt.Printf("%f|%e|%E")
		fallthrough
	case 'b', 'g', 'G', 'x', 'X':
		if p, ok := st.Precision(); ok {
			prec = p
		}
	default:
		verb, prec = 'f', 0
	}

	var unit SizeB1024
	switch {
	case self < _iKiB:
		unit = _ib
	case self < _iMiB:
		unit = _iKiB
	case self < _iGiB:
		unit = _iMiB
	case self < _iTiB:
		unit = _iGiB
	default:
		unit = _iTiB
	}

	b := strconv.AppendFloat(make([]byte, 0, 24), float64(self)/float64(unit), byte(verb), prec, 64)
	if st.Flag(' ') {
		b = append(b, ' ')
	}
	b = append(b, []byte(unit.String())...)
	_, err := st.Write(b)
	if err != nil {
		panic(err)
	}
}

const (
	_b  SizeB1000 = 1
	_KB SizeB1000 = _b * 1000
	_MB SizeB1000 = _KB * 1000
	_GB SizeB1000 = _MB * 1000
	_TB SizeB1000 = _GB * 1000
)

// SizeB1000 named type, which implements fmt.Formatter interface. It
// adjusts its value according to byte size multiple by 1000 and appends
// appropriate size marker (KB, MB, GB, TB).
type SizeB1000 int64

func (self SizeB1000) Format(st fmt.State, verb rune) {
	prec := -1
	switch verb {
	case 'f', 'e', 'E':
		prec = 6 // default prec of fmt.Printf("%f|%e|%E")
		fallthrough
	case 'b', 'g', 'G', 'x', 'X':
		if p, ok := st.Precision(); ok {
			prec = p
		}
	default:
		verb, prec = 'f', 0
	}

	var unit SizeB1000
	switch {
	case self < _KB:
		unit = _b
	case self < _MB:
		unit = _KB
	case self < _GB:
		unit = _MB
	case self < _TB:
		unit = _GB
	default:
		unit = _TB
	}

	b := strconv.AppendFloat(make([]byte, 0, 24), float64(self)/float64(unit), byte(verb), prec, 64)
	if st.Flag(' ') {
		b = append(b, ' ')
	}
	b = append(b, []byte(unit.String())...)
	_, err := st.Write(b)
	if err != nil {
		panic(err)
	}
}
