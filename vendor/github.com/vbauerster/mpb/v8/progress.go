package mpb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8/cwriter"
	"github.com/vbauerster/mpb/v8/decor"
)

const (
	defaultRefreshRate = 150 * time.Millisecond
)

// DoneError represents an error when `*mpb.Progress` is done but its functionality is requested.
var DoneError = fmt.Errorf("%T instance can't be reused after it's done!", (*Progress)(nil))

// Progress represents a container that renders one or more progress bars.
type Progress struct {
	uwg          *sync.WaitGroup
	pwg, bwg     sync.WaitGroup
	operateState chan func(*pState)
	interceptIO  chan func(io.Writer)
	done         <-chan struct{}
	cancel       func()
}

// pState holds bars in its priorityQueue, it gets passed to (*Progress).serve monitor goroutine.
type pState struct {
	ctx          context.Context
	hm           heapManager
	dropS, dropD chan struct{}
	renderReq    chan time.Time
	idCount      int
	popPriority  int

	// following are provided/overrided by user
	refreshRate      time.Duration
	reqWidth         int
	popCompleted     bool
	autoRefresh      bool
	delayRC          <-chan struct{}
	manualRC         <-chan interface{}
	shutdownNotifier chan<- interface{}
	queueBars        map[*Bar]*Bar
	output           io.Writer
	debugOut         io.Writer
	uwg              *sync.WaitGroup
}

// New creates new Progress container instance. It's not possible to
// reuse instance after (*Progress).Wait method has been called.
func New(options ...ContainerOption) *Progress {
	return NewWithContext(context.Background(), options...)
}

// NewWithContext creates new Progress container instance with provided
// context. It's not possible to reuse instance after (*Progress).Wait
// method has been called.
func NewWithContext(ctx context.Context, options ...ContainerOption) *Progress {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &pState{
		ctx:         ctx,
		hm:          make(heapManager),
		dropS:       make(chan struct{}),
		dropD:       make(chan struct{}),
		renderReq:   make(chan time.Time),
		refreshRate: defaultRefreshRate,
		popPriority: math.MinInt32,
		queueBars:   make(map[*Bar]*Bar),
		output:      os.Stdout,
		debugOut:    io.Discard,
	}

	for _, opt := range options {
		if opt != nil {
			opt(s)
		}
	}

	p := &Progress{
		uwg:          s.uwg,
		operateState: make(chan func(*pState)),
		interceptIO:  make(chan func(io.Writer)),
		cancel:       cancel,
	}

	cw := cwriter.New(s.output)
	if s.manualRC != nil {
		done := make(chan struct{})
		p.done = done
		s.autoRefresh = false
		go s.manualRefreshListener(done)
	} else if cw.IsTerminal() || s.autoRefresh {
		done := make(chan struct{})
		p.done = done
		s.autoRefresh = true
		go s.autoRefreshListener(done)
	} else {
		p.done = ctx.Done()
		s.autoRefresh = false
	}

	p.pwg.Add(1)
	go p.serve(s, cw)
	go s.hm.run()
	return p
}

// AddBar creates a bar with default bar filler.
func (p *Progress) AddBar(total int64, options ...BarOption) *Bar {
	return p.New(total, BarStyle(), options...)
}

// AddSpinner creates a bar with default spinner filler.
func (p *Progress) AddSpinner(total int64, options ...BarOption) *Bar {
	return p.New(total, SpinnerStyle(), options...)
}

// New creates a bar by calling `Build` method on provided `BarFillerBuilder`.
func (p *Progress) New(total int64, builder BarFillerBuilder, options ...BarOption) *Bar {
	return p.AddFiller(total, builder.Build(), options...)
}

// AddFiller creates a bar which renders itself by provided filler.
// If `total <= 0` triggering complete event by increment methods is disabled.
// Panics if *Progress instance is done, i.e. called after (*Progress).Wait().
func (p *Progress) AddFiller(total int64, filler BarFiller, options ...BarOption) *Bar {
	if filler == nil {
		filler = NopStyle().Build()
	}
	type result struct {
		bar *Bar
		bs  *bState
	}
	ch := make(chan result)
	select {
	case p.operateState <- func(ps *pState) {
		bs := ps.makeBarState(total, filler, options...)
		bar := newBar(ps.ctx, p, bs)
		if bs.waitBar != nil {
			ps.queueBars[bs.waitBar] = bar
		} else {
			ps.hm.push(bar, true)
		}
		ps.idCount++
		ch <- result{bar, bs}
	}:
		res := <-ch
		bar, bs := res.bar, res.bs
		bar.TraverseDecorators(func(d decor.Decorator) {
			if d, ok := d.(decor.AverageDecorator); ok {
				bs.averageDecorators = append(bs.averageDecorators, d)
			}
			if d, ok := d.(decor.EwmaDecorator); ok {
				bs.ewmaDecorators = append(bs.ewmaDecorators, d)
			}
			if d, ok := d.(decor.ShutdownListener); ok {
				bs.shutdownListeners = append(bs.shutdownListeners, d)
			}
		})
		return bar
	case <-p.done:
		panic(DoneError)
	}
}

func (p *Progress) traverseBars(cb func(b *Bar) bool) {
	iter, drop := make(chan *Bar), make(chan struct{})
	select {
	case p.operateState <- func(s *pState) { s.hm.iter(iter, drop) }:
		for b := range iter {
			if cb(b) {
				close(drop)
				break
			}
		}
	case <-p.done:
	}
}

// UpdateBarPriority same as *Bar.SetPriority(int).
func (p *Progress) UpdateBarPriority(b *Bar, priority int) {
	select {
	case p.operateState <- func(s *pState) { s.hm.fix(b, priority) }:
	case <-p.done:
	}
}

// Write is implementation of io.Writer.
// Writing to `*mpb.Progress` will print lines above a running bar.
// Writes aren't flushed immediately, but at next refresh cycle.
// If Write is called after `*mpb.Progress` is done, `mpb.DoneError`
// is returned.
func (p *Progress) Write(b []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	ch := make(chan result)
	select {
	case p.interceptIO <- func(w io.Writer) {
		n, err := w.Write(b)
		ch <- result{n, err}
	}:
		res := <-ch
		return res.n, res.err
	case <-p.done:
		return 0, DoneError
	}
}

// Wait waits for all bars to complete and finally shutdowns container. After
// this method has been called, there is no way to reuse (*Progress) instance.
func (p *Progress) Wait() {
	// wait for user wg, if any
	if p.uwg != nil {
		p.uwg.Wait()
	}

	p.bwg.Wait()
	p.Shutdown()
}

// Shutdown cancels any running bar immediately and then shutdowns (*Progress)
// instance. Normally this method shouldn't be called unless you know what you
// are doing. Proper way to shutdown is to call (*Progress).Wait() instead.
func (p *Progress) Shutdown() {
	p.cancel()
	p.pwg.Wait()
}

func (p *Progress) serve(s *pState, cw *cwriter.Writer) {
	defer p.pwg.Done()
	render := func() error { return s.render(cw) }
	var err error

	for {
		select {
		case op := <-p.operateState:
			op(s)
		case fn := <-p.interceptIO:
			fn(cw)
		case <-s.renderReq:
			e := render()
			if e != nil {
				p.cancel() // cancel all bars
				render = func() error { return nil }
				err = e
			}
		case <-p.done:
			update := make(chan bool)
			for s.autoRefresh && err == nil {
				s.hm.state(update)
				if <-update {
					err = render()
				} else {
					break
				}
			}
			if err != nil {
				_, _ = fmt.Fprintln(s.debugOut, err.Error())
			}
			s.hm.end(s.shutdownNotifier)
			return
		}
	}
}

func (s pState) autoRefreshListener(done chan struct{}) {
	if s.delayRC != nil {
		<-s.delayRC
	}
	ticker := time.NewTicker(s.refreshRate)
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			s.renderReq <- t
		case <-s.ctx.Done():
			close(done)
			return
		}
	}
}

func (s pState) manualRefreshListener(done chan struct{}) {
	for {
		select {
		case x := <-s.manualRC:
			if t, ok := x.(time.Time); ok {
				s.renderReq <- t
			} else {
				s.renderReq <- time.Now()
			}
		case <-s.ctx.Done():
			close(done)
			return
		}
	}
}

func (s *pState) render(cw *cwriter.Writer) (err error) {
	s.hm.sync(s.dropS)
	iter := make(chan *Bar)
	go s.hm.iter(iter, s.dropS)

	var width, height int
	if cw.IsTerminal() {
		width, height, err = cw.GetTermSize()
		if err != nil {
			close(s.dropS)
			return err
		}
	} else {
		if s.reqWidth > 0 {
			width = s.reqWidth
		} else {
			width = 100
		}
		height = 100
	}

	for b := range iter {
		go b.render(width)
	}

	return s.flush(cw, height)
}

func (s *pState) flush(cw *cwriter.Writer, height int) error {
	wg := new(sync.WaitGroup)
	defer wg.Wait() // waiting for all s.hm.push to complete

	var popCount int
	var rows []io.Reader

	iter := make(chan *Bar)
	s.hm.drain(iter, s.dropD)

	for b := range iter {
		frame := <-b.frameCh
		if frame.err != nil {
			close(s.dropD)
			b.cancel()
			return frame.err // b.frameCh is buffered it's ok to return here
		}
		var usedRows int
		for i := len(frame.rows) - 1; i >= 0; i-- {
			if row := frame.rows[i]; len(rows) < height {
				rows = append(rows, row)
				usedRows++
			} else {
				_, _ = io.Copy(io.Discard, row)
			}
		}
		if frame.shutdown != 0 && !frame.done {
			if qb, ok := s.queueBars[b]; ok {
				b.cancel()
				delete(s.queueBars, b)
				qb.priority = b.priority
				wg.Add(1)
				go func(b *Bar) {
					s.hm.push(b, true)
					wg.Done()
				}(qb)
				continue
			}
			if s.popCompleted && !frame.noPop {
				switch frame.shutdown {
				case 1:
					b.priority = s.popPriority
					s.popPriority++
				default:
					b.cancel()
					popCount += usedRows
					continue
				}
			} else if frame.rmOnComplete {
				b.cancel()
				continue
			} else {
				b.cancel()
			}
		}
		wg.Add(1)
		go func(b *Bar) {
			s.hm.push(b, false)
			wg.Done()
		}(b)
	}

	for i := len(rows) - 1; i >= 0; i-- {
		_, err := cw.ReadFrom(rows[i])
		if err != nil {
			return err
		}
	}

	return cw.Flush(len(rows) - popCount)
}

func (s pState) makeBarState(total int64, filler BarFiller, options ...BarOption) *bState {
	bs := &bState{
		id:          s.idCount,
		priority:    s.idCount,
		reqWidth:    s.reqWidth,
		total:       total,
		filler:      filler,
		renderReq:   s.renderReq,
		autoRefresh: s.autoRefresh,
	}

	if total > 0 {
		bs.triggerComplete = true
	}

	for _, opt := range options {
		if opt != nil {
			opt(bs)
		}
	}

	for i := 0; i < len(bs.buffers); i++ {
		bs.buffers[i] = bytes.NewBuffer(make([]byte, 0, 512))
	}

	return bs
}
