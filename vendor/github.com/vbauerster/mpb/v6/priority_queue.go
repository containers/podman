package mpb

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
	s := *pq
	bar := x.(*Bar)
	bar.index = len(s)
	s = append(s, bar)
	*pq = s
}

func (pq *priorityQueue) Pop() interface{} {
	s := *pq
	*pq = s[0 : len(s)-1]
	bar := s[len(s)-1]
	bar.index = -1 // for safety
	return bar
}
