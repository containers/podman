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
// When argument bar completes or aborts queued bar replaces its place.
// If sync is true queued bar is suspended until argument bar completes
// or aborts.
func BarQueueAfter(bar *Bar, sync bool) BarOption {
	if bar == nil {
		return nil
	}
	return func(s *bState) {
		s.afterBar = bar
		s.sync = sync
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
// will be on top. If `BarReplaceOnComplete` option is supplied, this
// option is ignored.
func BarPriority(priority int) BarOption {
	return func(s *bState) {
		s.priority = priority
	}
}

// BarExtender provides a way to extend bar to the next new line.
func BarExtender(filler BarFiller) BarOption {
	if filler == nil {
		return nil
	}
	return func(s *bState) {
		s.extender = makeExtenderFunc(filler)
	}
}

func makeExtenderFunc(filler BarFiller) extenderFunc {
	buf := new(bytes.Buffer)
	return func(r io.Reader, reqWidth int, st decor.Statistics) (io.Reader, int) {
		filler.Fill(buf, reqWidth, st)
		return io.MultiReader(r, buf), bytes.Count(buf.Bytes(), []byte("\n"))
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
