package mpb

import (
	"bytes"
	"container/heap"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v7/cwriter"
	"github.com/vbauerster/mpb/v7/decor"
)

const (
	// default RefreshRate
	prr = 150 * time.Millisecond
)

// Progress represents a container that renders one or more progress
// bars.
type Progress struct {
	ctx          context.Context
	uwg          *sync.WaitGroup
	cwg          *sync.WaitGroup
	bwg          *sync.WaitGroup
	operateState chan func(*pState)
	done         chan struct{}
	refreshCh    chan time.Time
	once         sync.Once
}

// pState holds bars in its priorityQueue. It gets passed to
// *Progress.serve(...) monitor goroutine.
type pState struct {
	bHeap            priorityQueue
	heapUpdated      bool
	pMatrix          map[int][]chan int
	aMatrix          map[int][]chan int
	barShutdownQueue []*Bar

	// following are provided/overrided by user
	idCount          int
	reqWidth         int
	popCompleted     bool
	outputDiscarded  bool
	rr               time.Duration
	uwg              *sync.WaitGroup
	externalRefresh  <-chan interface{}
	renderDelay      <-chan struct{}
	shutdownNotifier chan struct{}
	parkedBars       map[*Bar]*Bar
	output           io.Writer
	debugOut         io.Writer
}

// New creates new Progress container instance. It's not possible to
// reuse instance after *Progress.Wait() method has been called.
func New(options ...ContainerOption) *Progress {
	return NewWithContext(context.Background(), options...)
}

// NewWithContext creates new Progress container instance with provided
// context. It's not possible to reuse instance after *Progress.Wait()
// method has been called.
func NewWithContext(ctx context.Context, options ...ContainerOption) *Progress {
	s := &pState{
		bHeap:      priorityQueue{},
		rr:         prr,
		parkedBars: make(map[*Bar]*Bar),
		output:     os.Stdout,
	}

	for _, opt := range options {
		if opt != nil {
			opt(s)
		}
	}

	p := &Progress{
		ctx:          ctx,
		uwg:          s.uwg,
		cwg:          new(sync.WaitGroup),
		bwg:          new(sync.WaitGroup),
		operateState: make(chan func(*pState)),
		done:         make(chan struct{}),
	}

	p.cwg.Add(1)
	go p.serve(s, cwriter.New(s.output))
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

// New creates a bar with provided BarFillerBuilder.
func (p *Progress) New(total int64, builder BarFillerBuilder, options ...BarOption) *Bar {
	return p.Add(total, builder.Build(), options...)
}

// Add creates a bar which renders itself by provided filler.
// If `total <= 0` trigger complete event is disabled until reset with *bar.SetTotal(int64, bool).
// Panics if *Progress instance is done, i.e. called after *Progress.Wait().
func (p *Progress) Add(total int64, filler BarFiller, options ...BarOption) *Bar {
	if filler == nil {
		filler = NopStyle().Build()
	}
	p.bwg.Add(1)
	result := make(chan *Bar)
	select {
	case p.operateState <- func(ps *pState) {
		bs := ps.makeBarState(total, filler, options...)
		bar := newBar(p, bs)
		if bs.runningBar != nil {
			bs.runningBar.noPop = true
			ps.parkedBars[bs.runningBar] = bar
		} else {
			heap.Push(&ps.bHeap, bar)
			ps.heapUpdated = true
		}
		ps.idCount++
		result <- bar
	}:
		bar := <-result
		bar.subscribeDecorators()
		return bar
	case <-p.done:
		p.bwg.Done()
		panic(fmt.Sprintf("%T instance can't be reused after it's done!", p))
	}
}

func (p *Progress) dropBar(b *Bar) {
	select {
	case p.operateState <- func(s *pState) {
		if b.index < 0 {
			return
		}
		heap.Remove(&s.bHeap, b.index)
		s.heapUpdated = true
	}:
	case <-p.done:
	}
}

func (p *Progress) traverseBars(cb func(b *Bar) bool) {
	done := make(chan struct{})
	select {
	case p.operateState <- func(s *pState) {
		for i := 0; i < s.bHeap.Len(); i++ {
			bar := s.bHeap[i]
			if !cb(bar) {
				break
			}
		}
		close(done)
	}:
		<-done
	case <-p.done:
	}
}

// UpdateBarPriority same as *Bar.SetPriority(int).
func (p *Progress) UpdateBarPriority(b *Bar, priority int) {
	select {
	case p.operateState <- func(s *pState) {
		if b.index < 0 {
			return
		}
		b.priority = priority
		heap.Fix(&s.bHeap, b.index)
	}:
	case <-p.done:
	}
}

// BarCount returns bars count.
func (p *Progress) BarCount() int {
	result := make(chan int)
	select {
	case p.operateState <- func(s *pState) { result <- s.bHeap.Len() }:
		return <-result
	case <-p.done:
		return 0
	}
}

// Wait waits for all bars to complete and finally shutdowns container.
// After this method has been called, there is no way to reuse *Progress
// instance.
func (p *Progress) Wait() {
	if p.uwg != nil {
		// wait for user wg
		p.uwg.Wait()
	}

	// wait for bars to quit, if any
	p.bwg.Wait()

	p.once.Do(p.shutdown)

	// wait for container to quit
	p.cwg.Wait()
}

func (p *Progress) shutdown() {
	close(p.done)
}

func (p *Progress) serve(s *pState, cw *cwriter.Writer) {
	defer p.cwg.Done()

	p.refreshCh = s.newTicker(p.done)

	for {
		select {
		case op := <-p.operateState:
			op(s)
		case <-p.refreshCh:
			if err := s.render(cw); err != nil {
				if s.debugOut != nil {
					_, e := fmt.Fprintln(s.debugOut, err)
					if e != nil {
						panic(err)
					}
				} else {
					panic(err)
				}
			}
		case <-s.shutdownNotifier:
			for s.heapUpdated {
				if err := s.render(cw); err != nil {
					if s.debugOut != nil {
						_, e := fmt.Fprintln(s.debugOut, err)
						if e != nil {
							panic(err)
						}
					} else {
						panic(err)
					}
				}
			}
			return
		}
	}
}

func (s *pState) newTicker(done <-chan struct{}) chan time.Time {
	ch := make(chan time.Time)
	if s.shutdownNotifier == nil {
		s.shutdownNotifier = make(chan struct{})
	}
	go func() {
		if s.renderDelay != nil {
			<-s.renderDelay
		}
		var internalRefresh <-chan time.Time
		if !s.outputDiscarded {
			if s.externalRefresh == nil {
				ticker := time.NewTicker(s.rr)
				defer ticker.Stop()
				internalRefresh = ticker.C
			}
		} else {
			s.externalRefresh = nil
		}
		for {
			select {
			case t := <-internalRefresh:
				ch <- t
			case x := <-s.externalRefresh:
				if t, ok := x.(time.Time); ok {
					ch <- t
				} else {
					ch <- time.Now()
				}
			case <-done:
				close(s.shutdownNotifier)
				return
			}
		}
	}()
	return ch
}

func (s *pState) render(cw *cwriter.Writer) error {
	if s.heapUpdated {
		s.updateSyncMatrix()
		s.heapUpdated = false
	}
	syncWidth(s.pMatrix)
	syncWidth(s.aMatrix)

	tw, err := cw.GetWidth()
	if err != nil {
		tw = s.reqWidth
	}
	for i := 0; i < s.bHeap.Len(); i++ {
		bar := s.bHeap[i]
		go bar.render(tw)
	}

	return s.flush(cw)
}

func (s *pState) flush(cw *cwriter.Writer) error {
	var totalLines int
	bm := make(map[*Bar]int, s.bHeap.Len())
	for s.bHeap.Len() > 0 {
		b := heap.Pop(&s.bHeap).(*Bar)
		frame := <-b.frameCh
		_, err := cw.ReadFrom(frame.reader)
		if err != nil {
			return err
		}
		if b.toShutdown {
			if b.recoveredPanic != nil {
				s.barShutdownQueue = append(s.barShutdownQueue, b)
				b.toShutdown = false
			} else {
				// shutdown at next flush
				// this ensures no bar ends up with less than 100% rendered
				defer func() {
					s.barShutdownQueue = append(s.barShutdownQueue, b)
				}()
			}
		}
		bm[b] = frame.lines
		totalLines += frame.lines
	}

	for _, b := range s.barShutdownQueue {
		if parkedBar := s.parkedBars[b]; parkedBar != nil {
			parkedBar.priority = b.priority
			heap.Push(&s.bHeap, parkedBar)
			delete(s.parkedBars, b)
			b.toDrop = true
		}
		if s.popCompleted && !b.noPop {
			totalLines -= bm[b]
			b.toDrop = true
		}
		if b.toDrop {
			delete(bm, b)
			s.heapUpdated = true
		}
		b.cancel()
	}
	s.barShutdownQueue = s.barShutdownQueue[0:0]

	for b := range bm {
		heap.Push(&s.bHeap, b)
	}

	return cw.Flush(totalLines)
}

func (s *pState) updateSyncMatrix() {
	s.pMatrix = make(map[int][]chan int)
	s.aMatrix = make(map[int][]chan int)
	for i := 0; i < s.bHeap.Len(); i++ {
		bar := s.bHeap[i]
		table := bar.wSyncTable()
		pRow, aRow := table[0], table[1]

		for i, ch := range pRow {
			s.pMatrix[i] = append(s.pMatrix[i], ch)
		}

		for i, ch := range aRow {
			s.aMatrix[i] = append(s.aMatrix[i], ch)
		}
	}
}

func (s *pState) makeBarState(total int64, filler BarFiller, options ...BarOption) *bState {
	bs := &bState{
		id:       s.idCount,
		priority: s.idCount,
		reqWidth: s.reqWidth,
		total:    total,
		filler:   filler,
		extender: func(r io.Reader, _ int, _ decor.Statistics) (io.Reader, int) { return r, 0 },
		debugOut: s.debugOut,
	}

	if total > 0 {
		bs.triggerComplete = true
	}

	for _, opt := range options {
		if opt != nil {
			opt(bs)
		}
	}

	if bs.middleware != nil {
		bs.filler = bs.middleware(filler)
		bs.middleware = nil
	}

	if s.popCompleted && !bs.noPop {
		bs.priority = -(math.MaxInt32 - s.idCount)
	}

	for i := 0; i < len(bs.buffers); i++ {
		bs.buffers[i] = bytes.NewBuffer(make([]byte, 0, 512))
	}

	return bs
}

func syncWidth(matrix map[int][]chan int) {
	for _, column := range matrix {
		go maxWidthDistributor(column)
	}
}

var maxWidthDistributor = func(column []chan int) {
	var maxWidth int
	for _, ch := range column {
		if w := <-ch; w > maxWidth {
			maxWidth = w
		}
	}
	for _, ch := range column {
		ch <- maxWidth
	}
}
