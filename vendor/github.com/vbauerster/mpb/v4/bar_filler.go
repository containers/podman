package mpb

import (
	"io"
	"unicode/utf8"

	"github.com/vbauerster/mpb/v4/decor"
	"github.com/vbauerster/mpb/v4/internal"
)

const (
	rLeft = iota
	rFill
	rTip
	rEmpty
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
//	'4th rune' stands for empty rune
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
	tip     []byte
	refill  int64
	reverse bool
	flush   func(w io.Writer, bb [][]byte)
}

// NewBarFiller constucts mpb.Filler, to be used with *Progress.Add(...) *Bar method.
func NewBarFiller(style string, reverse bool) Filler {
	if style == "" {
		style = DefaultBarStyle
	}
	bf := &barFiller{
		format:  make([][]byte, utf8.RuneCountInString(style)),
		reverse: reverse,
	}
	bf.SetStyle(style)
	return bf
}

func (s *barFiller) SetStyle(style string) {
	if !utf8.ValidString(style) {
		return
	}
	src := make([][]byte, 0, utf8.RuneCountInString(style))
	for _, r := range style {
		src = append(src, []byte(string(r)))
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
		s.flush = normalFlush
	}
	s.reverse = reverse
}

func (s *barFiller) SetRefill(amount int64) {
	s.refill = amount
}

func (s *barFiller) Fill(w io.Writer, width int, stat *decor.Statistics) {
	// don't count rLeft and rRight as progress
	width -= 2
	if width < 2 {
		return
	}
	w.Write(s.format[rLeft])
	defer w.Write(s.format[rRight])

	bb := make([][]byte, width)

	cwidth := int(internal.PercentageRound(stat.Total, stat.Current, width))

	for i := 0; i < cwidth; i++ {
		bb[i] = s.format[rFill]
	}

	if s.refill > 0 {
		var rwidth int
		if s.refill > stat.Current {
			rwidth = cwidth
		} else {
			rwidth = int(internal.PercentageRound(stat.Total, int64(s.refill), width))
		}
		for i := 0; i < rwidth; i++ {
			bb[i] = s.format[rRefill]
		}
	}

	if cwidth > 0 && cwidth < width {
		bb[cwidth-1] = s.tip
	}

	for i := cwidth; i < width; i++ {
		bb[i] = s.format[rEmpty]
	}

	s.flush(w, bb)
}

func normalFlush(w io.Writer, bb [][]byte) {
	for i := 0; i < len(bb); i++ {
		w.Write(bb[i])
	}
}

func reverseFlush(w io.Writer, bb [][]byte) {
	for i := len(bb) - 1; i >= 0; i-- {
		w.Write(bb[i])
	}
}
