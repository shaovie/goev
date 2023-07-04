package goev

import (
	"container/heap"
)

type PriorityQueueItem struct {
	Value    any
	Priority int64    // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	Index int // The index of the item in the heap.
}
func NewPriorityQueueItem(value any, priority int64) *PriorityQueueItem {
    return &PriorityQueueItem{
        Value: value,
        Priority: priority,
    }
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*PriorityQueueItem

func NewPriorityQueue(capacity int) PriorityQueue {
	return make(PriorityQueue, 0, capacity)
}

// heap Interface
func (pq PriorityQueue) Len() int { return len(pq) }

// heap Interface
func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Priority < pq[j].Priority
}

// heap Interface
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

// heap Interface
func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*PriorityQueueItem)
	item.Index = n
	*pq = append(*pq, item)
}

// heap Interface
func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.Index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) PushOne(item *PriorityQueueItem) {
    heap.Push(pq, item)
}
func (pq *PriorityQueue) PopOne(priority, errorVal int64) (*PriorityQueueItem, int64) {
	if pq.Len() == 0 {
		return nil, 0
	}

	item := (*pq)[0]
    delta := item.Priority - priority
    if delta > errorVal { // The error is errorVal
		return nil, delta
	}
	heap.Remove(pq, 0)
	return item, 0
}
