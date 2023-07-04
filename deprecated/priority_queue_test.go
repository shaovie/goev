package goev

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue(64)
	if len(pq) != 0 {
		t.Fatalf("init len error:%d", len(pq))
	}
	for i := 0; i < 200; i++ {
		r := rand.Int63() % 200
		pq.PushOne(NewPriorityQueueItem(r, r))
	}
	fmt.Printf("len:%d cap:%d\n", len(pq), cap(pq))
	for i := 0; i < 200; i++ {
		item, _ := pq.PopOne(200, 0)
		fmt.Printf("#%d  %d\n", i, item.Value.(int64))
	}
	fmt.Printf("len:%d cap:%d\n", len(pq), cap(pq))
}
