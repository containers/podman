package mpb

import "container/heap"

// A priorityQueue implements heap.Interface
type priorityQueue []*Bar

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
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
	bar.index = -1 // for safety
	*pq = old[0 : n-1]
	return bar
}

// update modifies the priority of a Bar in the queue.
func (pq *priorityQueue) update(bar *Bar, priority int) {
	bar.priority = priority
	heap.Fix(pq, bar.index)
}
