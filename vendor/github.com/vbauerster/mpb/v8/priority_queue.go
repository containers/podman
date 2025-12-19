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
	n := len(*pq)
	bar := x.(*Bar)
	bar.index = n
	*pq = append(*pq, bar)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	bar := old[n-1]
	old[n-1] = nil // avoid memory leak
	bar.index = -1 // for safety
	*pq = old[:n-1]
	return bar
}
