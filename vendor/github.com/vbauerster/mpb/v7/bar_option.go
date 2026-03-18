package mpb

import (
	"bytes"
	"io"

	"github.com/vbauerster/mpb/v7/decor"
)

// BarOption is a func option to alter default behavior of a bar.
type BarOption func(*bState)

func skipNil(decorators []decor.Decorator) (filtered []decor.Decorator) {
	for _, d := range decorators {
		if d != nil {
			filtered = append(filtered, d)
		}
	}
	return
}

func (s *bState) addDecorators(dest *[]decor.Decorator, decorators ...decor.Decorator) {
	type mergeWrapper interface {
		MergeUnwrap() []decor.Decorator
	}
	for _, decorator := range decorators {
		if mw, ok := decorator.(mergeWrapper); ok {
			*dest = append(*dest, mw.MergeUnwrap()...)
		}
		*dest = append(*dest, decorator)
	}
}

// AppendDecorators let you inject decorators to the bar's right side.
func AppendDecorators(decorators ...decor.Decorator) BarOption {
	return func(s *bState) {
		s.addDecorators(&s.aDecorators, skipNil(decorators)...)
	}
}

// PrependDecorators let you inject decorators to the bar's left side.
func PrependDecorators(decorators ...decor.Decorator) BarOption {
	return func(s *bState) {
		s.addDecorators(&s.pDecorators, skipNil(decorators)...)
	}
}

// BarID sets bar id.
func BarID(id int) BarOption {
	return func(s *bState) {
		s.id = id
	}
}

// BarWidth sets bar width independent of the container.
func BarWidth(width int) BarOption {
	return func(s *bState) {
		s.reqWidth = width
	}
}

// BarQueueAfter puts this (being constructed) bar into the queue.
// BarPriority will be inherited from the argument bar.
// When argument bar completes or aborts queued bar replaces its place.
// If sync is true queued bar is suspended until argument bar completes
// or aborts.
func BarQueueAfter(bar *Bar, sync bool) BarOption {
	if bar == nil {
		return nil
	}
	return func(s *bState) {
		s.wait.bar = bar
		s.wait.sync = sync
	}
}

// BarRemoveOnComplete removes both bar's filler and its decorators
// on complete event.
func BarRemoveOnComplete() BarOption {
	return func(s *bState) {
		s.dropOnComplete = true
	}
}

// BarFillerClearOnComplete clears bar's filler on complete event.
// It's shortcut for BarFillerOnComplete("").
func BarFillerClearOnComplete() BarOption {
	return BarFillerOnComplete("")
}

// BarFillerOnComplete replaces bar's filler with message, on complete event.
func BarFillerOnComplete(message string) BarOption {
	return BarFillerMiddleware(func(base BarFiller) BarFiller {
		return BarFillerFunc(func(w io.Writer, reqWidth int, st decor.Statistics) {
			if st.Completed {
				_, err := io.WriteString(w, message)
				if err != nil {
					panic(err)
				}
			} else {
				base.Fill(w, reqWidth, st)
			}
		})
	})
}

// BarFillerMiddleware provides a way to augment the underlying BarFiller.
func BarFillerMiddleware(middle func(BarFiller) BarFiller) BarOption {
	return func(s *bState) {
		s.middleware = middle
	}
}

// BarPriority sets bar's priority. Zero is highest priority, i.e. bar
// will be on top. This option isn't effective with `BarQueueAfter` option.
func BarPriority(priority int) BarOption {
	return func(s *bState) {
		s.priority = priority
	}
}

// BarExtender extends bar with arbitrary lines. Provided BarFiller will be
// called at each render/flush cycle. Any lines written to the underlying
// io.Writer will be printed after the bar itself.
func BarExtender(filler BarFiller) BarOption {
	return barExtender(filler, false)
}

// BarExtenderRev extends bar with arbitrary lines in reverse order. Provided
// BarFiller will be called at each render/flush cycle. Any lines written
// to the underlying io.Writer will be printed before the bar itself.
func BarExtenderRev(filler BarFiller) BarOption {
	return barExtender(filler, true)
}

func barExtender(filler BarFiller, rev bool) BarOption {
	if filler == nil {
		return nil
	}
	return func(s *bState) {
		s.extender = makeExtenderFunc(filler, rev)
	}
}

func makeExtenderFunc(filler BarFiller, rev bool) extenderFunc {
	buf := new(bytes.Buffer)
	base := func(rows []io.Reader, width int, stat decor.Statistics) []io.Reader {
		buf.Reset()
		filler.Fill(buf, width, stat)
		for {
			b, err := buf.ReadBytes('\n')
			if err != nil {
				break
			}
			rows = append(rows, bytes.NewReader(b))
		}
		return rows
	}

	if !rev {
		return base
	} else {
		return func(rows []io.Reader, width int, stat decor.Statistics) []io.Reader {
			rows = base(rows, width, stat)
			for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
				rows[left], rows[right] = rows[right], rows[left]
			}
			return rows
		}
	}
}

// BarFillerTrim removes leading and trailing space around the underlying BarFiller.
func BarFillerTrim() BarOption {
	return func(s *bState) {
		s.trimSpace = true
	}
}

// BarNoPop disables bar pop out of container. Effective when
// PopCompletedMode of container is enabled.
func BarNoPop() BarOption {
	return func(s *bState) {
		s.noPop = true
	}
}

// BarOptional will invoke provided option only when cond is true.
func BarOptional(option BarOption, cond bool) BarOption {
	if cond {
		return option
	}
	return nil
}

// BarOptOn will invoke provided option only when higher order predicate
// evaluates to true.
func BarOptOn(option BarOption, predicate func() bool) BarOption {
	if predicate() {
		return option
	}
	return nil
}
