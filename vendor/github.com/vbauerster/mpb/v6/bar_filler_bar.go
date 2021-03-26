package mpb

import (
	"bytes"
	"io"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
	"github.com/vbauerster/mpb/v6/decor"
	"github.com/vbauerster/mpb/v6/internal"
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

// BarDefaultStyle is a style for rendering a progress bar.
// It consist of 7 ordered runes:
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
const BarDefaultStyle string = "[=>-]<+"

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

// NewBarFiller returns a BarFiller implementation which renders a
// progress bar in regular direction. If style is empty string,
// BarDefaultStyle is applied. To be used with `*Progress.Add(...)
// *Bar` method.
func NewBarFiller(style string) BarFiller {
	return newBarFiller(style, false)
}

// NewBarFillerRev returns a BarFiller implementation which renders a
// progress bar in reverse direction. If style is empty string,
// BarDefaultStyle is applied. To be used with `*Progress.Add(...)
// *Bar` method.
func NewBarFillerRev(style string) BarFiller {
	return newBarFiller(style, true)
}

// NewBarFillerPick pick between regular and reverse BarFiller implementation
// based on rev param. To be used with `*Progress.Add(...) *Bar` method.
func NewBarFillerPick(style string, rev bool) BarFiller {
	return newBarFiller(style, rev)
}

func newBarFiller(style string, rev bool) BarFiller {
	bf := &barFiller{
		format:  make([][]byte, len(BarDefaultStyle)),
		rwidth:  make([]int, len(BarDefaultStyle)),
		reverse: rev,
	}
	bf.parse(BarDefaultStyle)
	if style != "" && style != BarDefaultStyle {
		bf.parse(style)
	}
	return bf
}

func (s *barFiller) parse(style string) {
	if !utf8.ValidString(style) {
		panic("invalid bar style")
	}
	srcFormat := make([][]byte, len(BarDefaultStyle))
	srcRwidth := make([]int, len(BarDefaultStyle))
	i := 0
	for gr := uniseg.NewGraphemes(style); i < len(BarDefaultStyle) && gr.Next(); i++ {
		srcFormat[i] = gr.Bytes()
		srcRwidth[i] = runewidth.StringWidth(gr.Str())
	}
	copy(s.format, srcFormat[:i])
	copy(s.rwidth, srcRwidth[:i])
	if s.reverse {
		s.tip = s.format[rRevTip]
		s.flush = reverseFlush
	} else {
		s.tip = s.format[rTip]
		s.flush = regularFlush
	}
}

func (s *barFiller) Fill(w io.Writer, reqWidth int, stat decor.Statistics) {
	width := internal.CheckRequestedWidth(reqWidth, stat.AvailableWidth)
	brackets := s.rwidth[rLeft] + s.rwidth[rRight]
	if width < brackets {
		return
	}
	// don't count brackets as progress
	width -= brackets

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
