package mpb

import (
	"io"

	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v8/decor"
	"github.com/vbauerster/mpb/v8/internal"
)

const (
	iLbound = iota
	iRbound
	iRefiller
	iFiller
	iTip
	iPadding
	components
)

var defaultBarStyle = [components]string{"[", "]", "+", "=", ">", "-"}

// BarStyleComposer interface.
type BarStyleComposer interface {
	BarFillerBuilder
	Lbound(string) BarStyleComposer
	LboundMeta(func(string) string) BarStyleComposer
	Rbound(string) BarStyleComposer
	RboundMeta(func(string) string) BarStyleComposer
	Filler(string) BarStyleComposer
	FillerMeta(func(string) string) BarStyleComposer
	Refiller(string) BarStyleComposer
	RefillerMeta(func(string) string) BarStyleComposer
	Padding(string) BarStyleComposer
	PaddingMeta(func(string) string) BarStyleComposer
	Tip(frames ...string) BarStyleComposer
	TipMeta(func(string) string) BarStyleComposer
	TipOnComplete() BarStyleComposer
	Reverse() BarStyleComposer
}

type component struct {
	width int
	bytes []byte
}

type flushSection struct {
	meta  func(io.Writer, []byte) error
	bytes []byte
}

type bFiller struct {
	components    [components]component
	meta          [components]func(io.Writer, []byte) error
	flush         func(io.Writer, ...flushSection) error
	tipOnComplete bool
	tip           struct {
		frames []component
		count  uint
	}
}

type barStyle struct {
	style         [components]string
	metaFuncs     [components]func(io.Writer, []byte) error
	tipFrames     []string
	tipOnComplete bool
	rev           bool
}

// BarStyle constructs default bar style which can be altered via
// BarStyleComposer interface.
func BarStyle() BarStyleComposer {
	bs := barStyle{
		style:     defaultBarStyle,
		tipFrames: []string{defaultBarStyle[iTip]},
	}
	for i := range bs.metaFuncs {
		bs.metaFuncs[i] = defaultMeta
	}
	return bs
}

func (s barStyle) Lbound(bound string) BarStyleComposer {
	s.style[iLbound] = bound
	return s
}

func (s barStyle) LboundMeta(fn func(string) string) BarStyleComposer {
	s.metaFuncs[iLbound] = makeMetaFunc(fn)
	return s
}

func (s barStyle) Rbound(bound string) BarStyleComposer {
	s.style[iRbound] = bound
	return s
}

func (s barStyle) RboundMeta(fn func(string) string) BarStyleComposer {
	s.metaFuncs[iRbound] = makeMetaFunc(fn)
	return s
}

func (s barStyle) Filler(filler string) BarStyleComposer {
	s.style[iFiller] = filler
	return s
}

func (s barStyle) FillerMeta(fn func(string) string) BarStyleComposer {
	s.metaFuncs[iFiller] = makeMetaFunc(fn)
	return s
}

func (s barStyle) Refiller(refiller string) BarStyleComposer {
	s.style[iRefiller] = refiller
	return s
}

func (s barStyle) RefillerMeta(fn func(string) string) BarStyleComposer {
	s.metaFuncs[iRefiller] = makeMetaFunc(fn)
	return s
}

func (s barStyle) Padding(padding string) BarStyleComposer {
	s.style[iPadding] = padding
	return s
}

func (s barStyle) PaddingMeta(fn func(string) string) BarStyleComposer {
	s.metaFuncs[iPadding] = makeMetaFunc(fn)
	return s
}

func (s barStyle) Tip(frames ...string) BarStyleComposer {
	if len(frames) != 0 {
		s.tipFrames = frames
	}
	return s
}

func (s barStyle) TipMeta(fn func(string) string) BarStyleComposer {
	s.metaFuncs[iTip] = makeMetaFunc(fn)
	return s
}

func (s barStyle) TipOnComplete() BarStyleComposer {
	s.tipOnComplete = true
	return s
}

func (s barStyle) Reverse() BarStyleComposer {
	s.rev = true
	return s
}

func (s barStyle) Build() BarFiller {
	bf := &bFiller{
		meta:          s.metaFuncs,
		tipOnComplete: s.tipOnComplete,
	}
	bf.components[iLbound] = component{
		width: runewidth.StringWidth(s.style[iLbound]),
		bytes: []byte(s.style[iLbound]),
	}
	bf.components[iRbound] = component{
		width: runewidth.StringWidth(s.style[iRbound]),
		bytes: []byte(s.style[iRbound]),
	}
	bf.components[iFiller] = component{
		width: runewidth.StringWidth(s.style[iFiller]),
		bytes: []byte(s.style[iFiller]),
	}
	bf.components[iRefiller] = component{
		width: runewidth.StringWidth(s.style[iRefiller]),
		bytes: []byte(s.style[iRefiller]),
	}
	bf.components[iPadding] = component{
		width: runewidth.StringWidth(s.style[iPadding]),
		bytes: []byte(s.style[iPadding]),
	}
	bf.tip.frames = make([]component, len(s.tipFrames))
	for i, t := range s.tipFrames {
		bf.tip.frames[i] = component{
			width: runewidth.StringWidth(t),
			bytes: []byte(t),
		}
	}
	if s.rev {
		bf.flush = func(w io.Writer, sections ...flushSection) error {
			for i := len(sections) - 1; i >= 0; i-- {
				if s := sections[i]; len(s.bytes) != 0 {
					err := s.meta(w, s.bytes)
					if err != nil {
						return err
					}
				}
			}
			return nil
		}
	} else {
		bf.flush = func(w io.Writer, sections ...flushSection) error {
			for _, s := range sections {
				if len(s.bytes) != 0 {
					err := s.meta(w, s.bytes)
					if err != nil {
						return err
					}
				}
			}
			return nil
		}
	}
	return bf
}

func (s *bFiller) Fill(w io.Writer, stat decor.Statistics) error {
	width := internal.CheckRequestedWidth(stat.RequestedWidth, stat.AvailableWidth)
	// don't count brackets as progress
	width -= (s.components[iLbound].width + s.components[iRbound].width)
	if width < 0 {
		return nil
	}

	err := s.meta[iLbound](w, s.components[iLbound].bytes)
	if err != nil {
		return err
	}

	if width == 0 {
		return s.meta[iRbound](w, s.components[iRbound].bytes)
	}

	var tip component
	var refilling, filling, padding []byte
	var fillCount int
	curWidth := int(internal.PercentageRound(stat.Total, stat.Current, uint(width)))

	if curWidth != 0 {
		if !stat.Completed || s.tipOnComplete {
			tip = s.tip.frames[s.tip.count%uint(len(s.tip.frames))]
			s.tip.count++
			fillCount += tip.width
		}
		if stat.Refill != 0 {
			refWidth := int(internal.PercentageRound(stat.Total, stat.Refill, uint(width)))
			curWidth -= refWidth
			refWidth += curWidth
			for w := s.components[iFiller].width; curWidth-fillCount >= w; fillCount += w {
				filling = append(filling, s.components[iFiller].bytes...)
			}
			for w := s.components[iRefiller].width; refWidth-fillCount >= w; fillCount += w {
				refilling = append(refilling, s.components[iRefiller].bytes...)
			}
		} else {
			for w := s.components[iFiller].width; curWidth-fillCount >= w; fillCount += w {
				filling = append(filling, s.components[iFiller].bytes...)
			}
		}
	}

	for w := s.components[iPadding].width; width-fillCount >= w; fillCount += w {
		padding = append(padding, s.components[iPadding].bytes...)
	}

	for w := 1; width-fillCount >= w; fillCount += w {
		padding = append(padding, "…"...)
	}

	err = s.flush(w,
		flushSection{s.meta[iRefiller], refilling},
		flushSection{s.meta[iFiller], filling},
		flushSection{s.meta[iTip], tip.bytes},
		flushSection{s.meta[iPadding], padding},
	)
	if err != nil {
		return err
	}
	return s.meta[iRbound](w, s.components[iRbound].bytes)
}

func makeMetaFunc(fn func(string) string) func(io.Writer, []byte) error {
	return func(w io.Writer, p []byte) (err error) {
		_, err = io.WriteString(w, fn(string(p)))
		return err
	}
}

func defaultMeta(w io.Writer, p []byte) (err error) {
	_, err = w.Write(p)
	return err
}
