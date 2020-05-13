package mpb

import (
	"bytes"
	"io"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v5/decor"
	"github.com/vbauerster/mpb/v5/internal"
)

const (
	rLeft = iota
	rFill
	rTip
	rSpace
	rRight
	rRevTip
	rRefill
)

// DefaultBarStyle is a string containing 7 runes.
// Each rune is a building block of a progress bar.
//
//	'1st rune' stands for left boundary rune
//
//	'2nd rune' stands for fill rune
//
//	'3rd rune' stands for tip rune
//
//	'4th rune' stands for space rune
//
//	'5th rune' stands for right boundary rune
//
//	'6th rune' stands for reverse tip rune
//
//	'7th rune' stands for refill rune
//
const DefaultBarStyle string = "[=>-]<+"

type barFiller struct {
	format  [][]byte
	rwidth  []int
	tip     []byte
	refill  int64
	reverse bool
	flush   func(io.Writer, *space, [][]byte)
}

type space struct {
	space  []byte
	rwidth int
	count  int
}

// NewBarFiller constucts mpb.BarFiller, to be used with *Progress.Add(...) *Bar method.
func NewBarFiller(style string, reverse bool) BarFiller {
	bf := &barFiller{
		format:  make([][]byte, len(DefaultBarStyle)),
		rwidth:  make([]int, len(DefaultBarStyle)),
		reverse: reverse,
	}
	bf.SetStyle(style)
	return bf
}

func (s *barFiller) SetStyle(style string) {
	if !utf8.ValidString(style) {
		panic("invalid bar style")
	}
	if style == "" {
		style = DefaultBarStyle
	}
	src := make([][]byte, utf8.RuneCountInString(style))
	i := 0
	for _, r := range style {
		s.rwidth[i] = runewidth.RuneWidth(r)
		src[i] = []byte(string(r))
		i++
	}
	copy(s.format, src)
	s.SetReverse(s.reverse)
}

func (s *barFiller) SetReverse(reverse bool) {
	if reverse {
		s.tip = s.format[rRevTip]
		s.flush = reverseFlush
	} else {
		s.tip = s.format[rTip]
		s.flush = regularFlush
	}
	s.reverse = reverse
}

func (s *barFiller) Fill(w io.Writer, reqWidth int, stat decor.Statistics) {
	width := internal.WidthForBarFiller(reqWidth, stat.AvailableWidth)

	if brackets := s.rwidth[rLeft] + s.rwidth[rRight]; width < brackets {
		return
	} else {
		// don't count brackets as progress
		width -= brackets
	}
	w.Write(s.format[rLeft])
	defer w.Write(s.format[rRight])

	cwidth := int(internal.PercentageRound(stat.Total, stat.Current, width))
	space := &space{
		space:  s.format[rSpace],
		rwidth: s.rwidth[rSpace],
		count:  width - cwidth,
	}

	index, refill := 0, 0
	bb := make([][]byte, cwidth)

	if cwidth > 0 && cwidth != width {
		bb[index] = s.tip
		cwidth -= s.rwidth[rTip]
		index++
	}

	if stat.Refill > 0 {
		refill = int(internal.PercentageRound(stat.Total, int64(stat.Refill), width))
		if refill > cwidth {
			refill = cwidth
		}
		cwidth -= refill
	}

	for cwidth > 0 {
		bb[index] = s.format[rFill]
		cwidth -= s.rwidth[rFill]
		index++
	}

	for refill > 0 {
		bb[index] = s.format[rRefill]
		refill -= s.rwidth[rRefill]
		index++
	}

	if cwidth+refill < 0 || space.rwidth > 1 {
		buf := new(bytes.Buffer)
		s.flush(buf, space, bb[:index])
		io.WriteString(w, runewidth.Truncate(buf.String(), width, "…"))
		return
	}

	s.flush(w, space, bb)
}

func regularFlush(w io.Writer, space *space, bb [][]byte) {
	for i := len(bb) - 1; i >= 0; i-- {
		w.Write(bb[i])
	}
	for space.count > 0 {
		w.Write(space.space)
		space.count -= space.rwidth
	}
}

func reverseFlush(w io.Writer, space *space, bb [][]byte) {
	for space.count > 0 {
		w.Write(space.space)
		space.count -= space.rwidth
	}
	for i := 0; i < len(bb); i++ {
		w.Write(bb[i])
	}
}
