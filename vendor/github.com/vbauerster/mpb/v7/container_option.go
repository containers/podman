package mpb

import (
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v7/internal"
)

// ContainerOption is a func option to alter default behavior of a bar
// container. Container term refers to a Progress struct which can
// hold one or more Bars.
type ContainerOption func(*pState)

// WithWaitGroup provides means to have a single joint point. If
// *sync.WaitGroup is provided, you can safely call just p.Wait()
// without calling Wait() on provided *sync.WaitGroup. Makes sense
// when there are more than one bar to render.
func WithWaitGroup(wg *sync.WaitGroup) ContainerOption {
	return func(s *pState) {
		s.uwg = wg
	}
}

// WithWidth sets container width. If not set it defaults to terminal
// width. A bar added to the container will inherit its width, unless
// overridden by `func BarWidth(int) BarOption`.
func WithWidth(width int) ContainerOption {
	return func(s *pState) {
		s.reqWidth = width
	}
}

// WithRefreshRate overrides default 120ms refresh rate.
func WithRefreshRate(d time.Duration) ContainerOption {
	return func(s *pState) {
		s.rr = d
	}
}

// WithManualRefresh disables internal auto refresh time.Ticker.
// Refresh will occur upon receive value from provided ch.
func WithManualRefresh(ch <-chan interface{}) ContainerOption {
	return func(s *pState) {
		s.externalRefresh = ch
	}
}

// WithRenderDelay delays rendering. By default rendering starts as
// soon as bar is added, with this option it's possible to delay
// rendering process by keeping provided chan unclosed. In other words
// rendering will start as soon as provided chan is closed.
func WithRenderDelay(ch <-chan struct{}) ContainerOption {
	return func(s *pState) {
		s.renderDelay = ch
	}
}

// WithShutdownNotifier provided chanel will be closed, after all bars
// have been rendered.
func WithShutdownNotifier(ch chan struct{}) ContainerOption {
	return func(s *pState) {
		select {
		case <-ch:
		default:
			s.shutdownNotifier = ch
		}
	}
}

// WithOutput overrides default os.Stdout output. Setting it to nil
// will effectively disable auto refresh rate and discard any output,
// useful if you want to disable progress bars with little overhead.
func WithOutput(w io.Writer) ContainerOption {
	return func(s *pState) {
		if w == nil {
			s.output = ioutil.Discard
			s.outputDiscarded = true
			return
		}
		s.output = w
	}
}

// WithDebugOutput sets debug output.
func WithDebugOutput(w io.Writer) ContainerOption {
	if w == nil {
		return nil
	}
	return func(s *pState) {
		s.debugOut = w
	}
}

// PopCompletedMode will pop and stop rendering completed bars.
func PopCompletedMode() ContainerOption {
	return func(s *pState) {
		s.popCompleted = true
	}
}

// ContainerOptional will invoke provided option only when pick is true.
func ContainerOptional(option ContainerOption, pick bool) ContainerOption {
	return ContainerOptOn(option, internal.Predicate(pick))
}

// ContainerOptOn will invoke provided option only when higher order
// predicate evaluates to true.
func ContainerOptOn(option ContainerOption, predicate func() bool) ContainerOption {
	if predicate() {
		return option
	}
	return nil
}
