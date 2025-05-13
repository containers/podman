package mpb

import "container/heap"

var _ heap.Interface = (*priorityQueue)(nil)

type priorityQueue []*Bar

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// greater priority pops first
	return pq[i].priority > pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	s := *pq
	bar := x.(*Bar)
	bar.index = len(s)
	*pq = append(s, bar)
}

func (pq *priorityQueue) Pop() interface{} {
	s := *pq
	l := len(s)
	bar := s[l-1]
	bar.index = -1 // for safety
	s[l-1] = nil   // avoid memory leak
	*pq = s[:l-1]
	return bar
}
