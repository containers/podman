package mpb

import (
	"container/heap"
	"time"

	"github.com/vbauerster/mpb/v8/decor"
)

type heapManager chan heapRequest

type heapCmd int

const (
	h_sync heapCmd = iota
	h_push
	h_iter
	h_fix
	h_state
	h_end
)

type heapRequest struct {
	cmd  heapCmd
	data interface{}
}

type iterRequest chan (<-chan *Bar)

type pushData struct {
	bar  *Bar
	sync bool
}

type fixData struct {
	bar      *Bar
	priority int
	lazy     bool
}

func (m heapManager) run() {
	var bHeap barHeap
	var pMatrix map[int][]*decor.Sync
	var aMatrix map[int][]*decor.Sync

	var sync bool
	var prevLen int

	for req := range m {
		switch req.cmd {
		case h_sync:
			if sync || prevLen != bHeap.Len() {
				pMatrix = make(map[int][]*decor.Sync)
				aMatrix = make(map[int][]*decor.Sync)
				for _, b := range bHeap {
					table := b.wSyncTable()
					for i, s := range table[0] {
						pMatrix[i] = append(pMatrix[i], s)
					}
					for i, s := range table[1] {
						aMatrix[i] = append(aMatrix[i], s)
					}
				}
				sync, prevLen = false, bHeap.Len()
			}
			syncWidth(pMatrix)
			syncWidth(aMatrix)
		case h_push:
			data := req.data.(pushData)
			heap.Push(&bHeap, data.bar)
			sync = sync || data.sync
		case h_iter:
			for i, req := range req.data.([]iterRequest) {
				ch := make(chan *Bar, bHeap.Len())
				req <- ch
				switch i {
				case 0:
					rangeOverSlice(bHeap, ch)
				case 1:
					popOverHeap(&bHeap, ch)
				}
			}
		case h_fix:
			data := req.data.(fixData)
			if data.bar.index < 0 {
				break
			}
			data.bar.priority = data.priority
			if !data.lazy {
				heap.Fix(&bHeap, data.bar.index)
			}
		case h_state:
			ch := req.data.(chan<- bool)
			ch <- sync || prevLen != bHeap.Len()
		case h_end:
			ch := req.data.(chan<- interface{})
			if ch != nil {
				go func() {
					select {
					case ch <- []*Bar(bHeap):
					case <-time.After(time.Second):
					}
				}()
			}
			return
		}
	}
}

func (m heapManager) sync() {
	m <- heapRequest{cmd: h_sync}
}

func (m heapManager) push(b *Bar, sync bool) {
	data := pushData{b, sync}
	req := heapRequest{cmd: h_push, data: data}
	select {
	case m <- req:
	default:
		go func() {
			m <- req
		}()
	}
}

func (m heapManager) iter(req ...iterRequest) {
	m <- heapRequest{cmd: h_iter, data: req}
}

func (m heapManager) fix(b *Bar, priority int, lazy bool) {
	data := fixData{b, priority, lazy}
	m <- heapRequest{cmd: h_fix, data: data}
}

func (m heapManager) state(ch chan<- bool) {
	m <- heapRequest{cmd: h_state, data: ch}
}

func (m heapManager) end(ch chan<- interface{}) {
	m <- heapRequest{cmd: h_end, data: ch}
}

func syncWidth(matrix map[int][]*decor.Sync) {
	for _, column := range matrix {
		go maxWidthDistributor(column)
	}
}

func maxWidthDistributor(column []*decor.Sync) {
	var maxWidth int
	for _, s := range column {
		w := <-s.Tx
		if w > maxWidth {
			maxWidth = w
		}
	}
	for _, s := range column {
		s.Rx <- maxWidth
	}
}

// unordered iteration
func rangeOverSlice(s barHeap, dst chan<- *Bar) {
	defer close(dst)
	for _, b := range s {
		dst <- b
	}
}

// ordered iteration
func popOverHeap(h heap.Interface, dst chan<- *Bar) {
	defer close(dst)
	for h.Len() != 0 {
		dst <- heap.Pop(h).(*Bar)
	}
}
