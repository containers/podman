package mpb

import (
	"io"

	"github.com/acarl005/stripansi"
	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v7/decor"
	"github.com/vbauerster/mpb/v7/internal"
)

const (
	iLbound = iota
	iRbound
	iFiller
	iRefiller
	iPadding
	components
)

// BarStyleComposer interface.
type BarStyleComposer interface {
	BarFillerBuilder
	Lbound(string) BarStyleComposer
	Rbound(string) BarStyleComposer
	Filler(string) BarStyleComposer
	Refiller(string) BarStyleComposer
	Padding(string) BarStyleComposer
	Tip(...string) BarStyleComposer
	Reverse() BarStyleComposer
}

type bFiller struct {
	components [components]*component
	tip        struct {
		count  uint
		frames []*component
	}
	flush func(dst io.Writer, filling, padding [][]byte)
}

type component struct {
	width int
	bytes []byte
}

type barStyle struct {
	lbound   string
	rbound   string
	filler   string
	refiller string
	padding  string
	tip      []string
	rev      bool
}

// BarStyle constructs default bar style which can be altered via
// BarStyleComposer interface.
func BarStyle() BarStyleComposer {
	return &barStyle{
		lbound:   "[",
		rbound:   "]",
		filler:   "=",
		refiller: "+",
		padding:  "-",
		tip:      []string{">"},
	}
}

func (s *barStyle) Lbound(bound string) BarStyleComposer {
	s.lbound = bound
	return s
}

func (s *barStyle) Rbound(bound string) BarStyleComposer {
	s.rbound = bound
	return s
}

func (s *barStyle) Filler(filler string) BarStyleComposer {
	s.filler = filler
	return s
}

func (s *barStyle) Refiller(refiller string) BarStyleComposer {
	s.refiller = refiller
	return s
}

func (s *barStyle) Padding(padding string) BarStyleComposer {
	s.padding = padding
	return s
}

func (s *barStyle) Tip(tip ...string) BarStyleComposer {
	if len(tip) != 0 {
		s.tip = append(s.tip[:0], tip...)
	}
	return s
}

func (s *barStyle) Reverse() BarStyleComposer {
	s.rev = true
	return s
}

func (s *barStyle) Build() BarFiller {
	bf := new(bFiller)
	if s.rev {
		bf.flush = func(dst io.Writer, filling, padding [][]byte) {
			flush(dst, padding, filling)
		}
	} else {
		bf.flush = flush
	}
	bf.components[iLbound] = &component{
		width: runewidth.StringWidth(stripansi.Strip(s.lbound)),
		bytes: []byte(s.lbound),
	}
	bf.components[iRbound] = &component{
		width: runewidth.StringWidth(stripansi.Strip(s.rbound)),
		bytes: []byte(s.rbound),
	}
	bf.components[iFiller] = &component{
		width: runewidth.StringWidth(stripansi.Strip(s.filler)),
		bytes: []byte(s.filler),
	}
	bf.components[iRefiller] = &component{
		width: runewidth.StringWidth(stripansi.Strip(s.refiller)),
		bytes: []byte(s.refiller),
	}
	bf.components[iPadding] = &component{
		width: runewidth.StringWidth(stripansi.Strip(s.padding)),
		bytes: []byte(s.padding),
	}
	bf.tip.frames = make([]*component, len(s.tip))
	for i, t := range s.tip {
		bf.tip.frames[i] = &component{
			width: runewidth.StringWidth(stripansi.Strip(t)),
			bytes: []byte(t),
		}
	}
	return bf
}

func (s *bFiller) Fill(w io.Writer, width int, stat decor.Statistics) {
	width = internal.CheckRequestedWidth(width, stat.AvailableWidth)
	brackets := s.components[iLbound].width + s.components[iRbound].width
	if width < brackets {
		return
	}
	// don't count brackets as progress
	width -= brackets

	w.Write(s.components[iLbound].bytes)
	defer w.Write(s.components[iRbound].bytes)

	curWidth := int(internal.PercentageRound(stat.Total, stat.Current, width))
	refWidth, filled := 0, curWidth
	filling := make([][]byte, 0, curWidth)

	if curWidth > 0 && curWidth != width {
		tipFrame := s.tip.frames[s.tip.count%uint(len(s.tip.frames))]
		filling = append(filling, tipFrame.bytes)
		curWidth -= tipFrame.width
		s.tip.count++
	}

	if stat.Refill > 0 && curWidth > 0 {
		refWidth = int(internal.PercentageRound(stat.Total, int64(stat.Refill), width))
		if refWidth > curWidth {
			refWidth = curWidth
		}
		curWidth -= refWidth
	}

	for curWidth > 0 && curWidth >= s.components[iFiller].width {
		filling = append(filling, s.components[iFiller].bytes)
		curWidth -= s.components[iFiller].width
		if s.components[iFiller].width == 0 {
			break
		}
	}

	for refWidth > 0 && refWidth >= s.components[iRefiller].width {
		filling = append(filling, s.components[iRefiller].bytes)
		refWidth -= s.components[iRefiller].width
		if s.components[iRefiller].width == 0 {
			break
		}
	}

	filled -= curWidth + refWidth
	padWidth := width - filled
	padding := make([][]byte, 0, padWidth)
	for padWidth > 0 && padWidth >= s.components[iPadding].width {
		padding = append(padding, s.components[iPadding].bytes)
		padWidth -= s.components[iPadding].width
		if s.components[iPadding].width == 0 {
			break
		}
	}

	for padWidth > 0 {
		padding = append(padding, []byte("…"))
		padWidth--
	}

	s.flush(w, filling, padding)
}

func flush(dst io.Writer, filling, padding [][]byte) {
	for i := len(filling) - 1; i >= 0; i-- {
		dst.Write(filling[i])
	}
	for i := 0; i < len(padding); i++ {
		dst.Write(padding[i])
	}
}
