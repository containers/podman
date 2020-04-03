package mpb

import (
	"io"
	"io/ioutil"
	"sync"
	"time"
)

// ContainerOption is a function option which changes the default
// behavior of progress container, if passed to mpb.New(...ContainerOption).
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

// WithWidth sets container width. Default is 80. Bars inherit this
// width, as long as no BarWidth is applied.
func WithWidth(w int) ContainerOption {
	return func(s *pState) {
		if w < 0 {
			return
		}
		s.width = w
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
func WithManualRefresh(ch <-chan time.Time) ContainerOption {
	return func(s *pState) {
		s.refreshSrc = ch
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
		s.shutdownNotifier = ch
	}
}

// WithOutput overrides default os.Stdout output. Setting it to nil
// will effectively disable auto refresh rate and discard any output,
// useful if you want to disable progress bars with little overhead.
func WithOutput(w io.Writer) ContainerOption {
	return func(s *pState) {
		if w == nil {
			s.refreshSrc = make(chan time.Time)
			s.output = ioutil.Discard
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

// ContainerOptOn returns option when condition evaluates to true.
func ContainerOptOn(option ContainerOption, condition func() bool) ContainerOption {
	if condition() {
		return option
	}
	return nil
}
