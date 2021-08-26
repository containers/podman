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
	TipOnComplete(string) BarStyleComposer
	Tip(frames ...string) BarStyleComposer
	Reverse() BarStyleComposer
}

type bFiller struct {
	components [components]*component
	tip        struct {
		count      uint
		onComplete *component
		frames     []*component
	}
	flush func(dst io.Writer, filling, padding [][]byte)
}

type component struct {
	width int
	bytes []byte
}

type barStyle struct {
	lbound        string
	rbound        string
	filler        string
	refiller      string
	padding       string
	tipOnComplete string
	tipFrames     []string
	rev           bool
}

// BarStyle constructs default bar style which can be altered via
// BarStyleComposer interface.
func BarStyle() BarStyleComposer {
	return &barStyle{
		lbound:    "[",
		rbound:    "]",
		filler:    "=",
		refiller:  "+",
		padding:   "-",
		tipFrames: []string{">"},
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

func (s *barStyle) TipOnComplete(tip string) BarStyleComposer {
	s.tipOnComplete = tip
	return s
}

func (s *barStyle) Tip(frames ...string) BarStyleComposer {
	if len(frames) != 0 {
		s.tipFrames = append(s.tipFrames[:0], frames...)
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
	bf.tip.onComplete = &component{
		width: runewidth.StringWidth(stripansi.Strip(s.tipOnComplete)),
		bytes: []byte(s.tipOnComplete),
	}
	bf.tip.frames = make([]*component, len(s.tipFrames))
	for i, t := range s.tipFrames {
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
	// don't count brackets as progress
	width -= brackets
	if width < 0 {
		return
	}

	w.Write(s.components[iLbound].bytes)
	defer w.Write(s.components[iRbound].bytes)

	if width == 0 {
		return
	}

	var filling [][]byte
	var padding [][]byte
	var tip *component
	var filled int
	var refWidth int
	curWidth := int(internal.PercentageRound(stat.Total, stat.Current, uint(width)))

	if stat.Current >= stat.Total {
		tip = s.tip.onComplete
	} else {
		tip = s.tip.frames[s.tip.count%uint(len(s.tip.frames))]
	}

	if curWidth > 0 {
		filling = append(filling, tip.bytes)
		filled += tip.width
		s.tip.count++
	}

	if stat.Refill > 0 {
		refWidth = int(internal.PercentageRound(stat.Total, stat.Refill, uint(width)))
		curWidth -= refWidth
		refWidth += curWidth
	}

	for filled < curWidth {
		if curWidth-filled >= s.components[iFiller].width {
			filling = append(filling, s.components[iFiller].bytes)
			if s.components[iFiller].width == 0 {
				break
			}
			filled += s.components[iFiller].width
		} else {
			filling = append(filling, []byte("…"))
			filled++
		}
	}

	for filled < refWidth {
		if refWidth-filled >= s.components[iRefiller].width {
			filling = append(filling, s.components[iRefiller].bytes)
			if s.components[iRefiller].width == 0 {
				break
			}
			filled += s.components[iRefiller].width
		} else {
			filling = append(filling, []byte("…"))
			filled++
		}
	}

	padWidth := width - filled
	for padWidth > 0 {
		if padWidth >= s.components[iPadding].width {
			padding = append(padding, s.components[iPadding].bytes)
			if s.components[iPadding].width == 0 {
				break
			}
			padWidth -= s.components[iPadding].width
		} else {
			padding = append(padding, []byte("…"))
			padWidth--
		}
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
