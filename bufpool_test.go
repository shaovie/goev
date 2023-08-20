package goev

import (
	"math/rand"
	"runtime"
	"sync"
	"testing"
)

func TestBufpool0(t *testing.T) {
	bf := BMalloc(333)
	bf[0] = 'a'
	t.Logf("len: %d, cap: %d", len(bf), cap(bf))
	BFree(bf)

	bf2 := BMalloc(333)
	t.Logf("bf2 len: %d, cap: %d, alloc %p %p %v", len(bf2), cap(bf2), &bf[0], &bf2[0], string(bf2[0]))
	BFree(bf2)
}
func goFunc(bf []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := int64(1); i < 100; i++ {
		if (rand.Int63() % i) == i {
		}
	}
	BFree(bf)
}
func TestBufpool1(t *testing.T) {
	// test btPool
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		s := int(rand.Int63()%(128*7-16) + 16)
		bf := BMalloc(s)
		if len(bf) != s {
			t.Errorf("btpool allock len err %d", len(bf))
		}
		wg.Add(1)
		go goFunc(bf, &wg)
	}
	wg.Wait()
	runtime.GC()

	// test kbPool
	for i := 0; i < 16; i++ {
		s := int(rand.Int63()%(1024*1023-16) + 16)
		bf := BMalloc(s)
		if len(bf) != s {
			t.Errorf("kbpool allock len err")
		}
		wg.Add(1)
		go goFunc(bf, &wg)
	}
	wg.Wait()
	runtime.GC()

	// test mbPool
	for i := 0; i < 16; i++ {
		s := int(rand.Int63()%(1024*1024*int64(bufPoolMaxMBytes)-16) + 16)
		bf := BMalloc(s)
		if len(bf) != s {
			t.Errorf("mbpool allock len err")
		}
		wg.Add(1)
		go goFunc(bf, &wg)
	}
	wg.Wait()
	runtime.GC()

	buffPoolAdjust()
}
