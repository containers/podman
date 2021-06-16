package mpb

import (
	"bytes"
	"io"

	"github.com/vbauerster/mpb/v7/decor"
	"github.com/vbauerster/mpb/v7/internal"
)

// BarOption is a func option to alter default behavior of a bar.
type BarOption func(*bState)

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
		s.addDecorators(&s.aDecorators, decorators...)
	}
}

// PrependDecorators let you inject decorators to the bar's left side.
func PrependDecorators(decorators ...decor.Decorator) BarOption {
	return func(s *bState) {
		s.addDecorators(&s.pDecorators, decorators...)
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

// BarQueueAfter queues this (being constructed) bar to relplace
// runningBar after it has been completed.
func BarQueueAfter(runningBar *Bar) BarOption {
	if runningBar == nil {
		return nil
	}
	return func(s *bState) {
		s.runningBar = runningBar
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
				io.WriteString(w, message)
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

// BarOptional will invoke provided option only when pick is true.
func BarOptional(option BarOption, pick bool) BarOption {
	return BarOptOn(option, internal.Predicate(pick))
}

// BarOptOn will invoke provided option only when higher order predicate
// evaluates to true.
func BarOptOn(option BarOption, predicate func() bool) BarOption {
	if predicate() {
		return option
	}
	return nil
}
