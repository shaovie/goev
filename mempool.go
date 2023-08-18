package goev

import (
	"container/list"
	"fmt"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// mmItem save alloc info
type mmItem struct {
	idx int
	sp  *span
	sg  *spanGroup
	bp  []byte
}

var lockedMemPool = newMemPool(getMallocSizeInfo)
var lockedMemPoolMtx sync.Mutex

func init() {
	go memPoolGCTiming(time.Second * 60 * 3)
}

// Malloc alloc []byte
func Malloc(s int) []byte {
	lockedMemPoolMtx.Lock()
	defer lockedMemPoolMtx.Unlock()
	return lockedMemPool.Malloc(s)
}

// Free release []byte
func Free(bf []byte) {
	lockedMemPoolMtx.Lock()
	defer lockedMemPoolMtx.Unlock()
	lockedMemPool.Free(bf)
}

// MemPoolStat return some statistical information
func MemPoolStat() string {
	lockedMemPoolMtx.Lock()
	defer lockedMemPoolMtx.Unlock()
	return lockedMemPool.Stat()
}

// MemPoolGC can reclaim memory blocks that have not been used for a period of time
func MemPoolGC() {
	lockedMemPoolMtx.Lock()
	defer lockedMemPoolMtx.Unlock()
	lockedMemPool.GC()
}
func memPoolGCTiming(period time.Duration) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			MemPoolGC()
		}
	}
}

// This method can provide a size class along with the initial quantity 'n' for each size.
//
// Reimplement this method to enable customizing the size class.
func getMallocSizeInfo(s int) (size, n int) {
	//   s                        size      n
	//
	// { (0-256],                 256           1024*2 },  // span size 256*(1024*2)       0.5M
	// { (256-512],               512           1024*2 },  // span size 512*(1024*2)       1M
	// { (512-1024*1],            1024*1        1024*2 },  // span size (1024*1)*(1024*2)  2M
	//
	// { (1024*1-1024*2],         1024*2        1024*1 },  // span size (1024*2)*(1024*1)  2M
	// { (1024*2-1024*3],         1024*3        1024*1 },  // span size (1024*3)*(1024*1)  3M
	// ...
	// { (1024*15-1024*16],       1024*16       256    },  // span size (1024*16)*(256)    4M
	// ...
	// { (1024*63-1024*64],       1024*64       64     },  // span size (1024*64)*(64)     4M
	// { (1024*64-1024*65],       1024*65       64     },  // span size (1024*65)*(64)     4.1M
	// ...
	// { (1024*256-1024*512],     1024*512      16     },  // span size (1024*512)*(16)    8M
	// ...
	// { (1024*1023-1024*1024],   1024*1024     16     },  // span size (1024*1024)*(16)   16M

	if s <= 1024 {
		n = 2048
		if s > 512 {
			size = 1024
		} else if s > 256 {
			size = 512
		} else {
			size = 256
		}
	} else {
		if s > 1024*1024*1 { // use make([]byte, n)
			return 0, 0
		}

		size = (s + 1024 - 1) / 1024 * 1024
		if size >= 1024*256 {
			n = 16
		} else if size >= 1024*64 {
			n = 64
		} else if size >= 1024*16 {
			n = 256
		} else {
			n = 256
		}
		// special case (hot size)
		switch size {
		case 1024 * 2:
			n = 1024
		case 1024 * 32:
			n = 256
		}
	}
	return
}

type memPool struct {
	getInitSize func(int) (size, n int)
	spanMap     map[int]*spanGroup
	active      map[*byte]mmItem
}

func newMemPool(f func(int) (size, n int)) *memPool {
	return &memPool{
		getInitSize: f,
		spanMap:     make(map[int]*spanGroup, 512+2),
		active:      make(map[*byte]mmItem, 1024),
	}
}
func (mp *memPool) Malloc(s int) []byte {
	size, n := mp.getInitSize(s)
	if n == 0 {
		return make([]byte, s)
	}

	sg, ok := mp.spanMap[size]
	if !ok {
		sg = newSpanGroup(mp, size, n)
		mp.spanMap[size] = sg
	}
	mm := sg.alloc()
	mp.active[&(mm.bp[0])] = mm
	return mm.bp
}
func (mp *memPool) Free(bf []byte) {
	if len(bf) == 0 {
		panic("goev: memPool.Free bf is illegal")
	}
	p := &bf[0]
	mm, ok := mp.active[p]
	if ok {
		if &(mm.bp[0]) != p {
			panic("goev: memPool.Free bf is invalid")
		}
		if mm.sg.mp != mp {
			panic("goev: memPool.Free mb not belong this mempool")
		}
		mm.sg.free(mm.idx, mm.sp)
		delete(mp.active, p)
	}
	// !ok maybe alloc by make([]byte, n)
}
func (mp *memPool) GC() {
	for _, sg := range mp.spanMap {
		sg.gc()
	}
}
func (mp *memPool) Stat() string {
	var stat strings.Builder
	stat.Grow(2048)
	spanL := make([]*spanGroup, 0, len(mp.spanMap))
	for _, sg := range mp.spanMap {
		spanL = append(spanL, sg)
	}
	sort.Slice(spanL, func(i, j int) bool {
		if spanL[i].allocTimes == spanL[j].allocTimes {
			return spanL[i].size < spanL[j].size
		}
		return spanL[i].allocTimes > spanL[j].allocTimes
	})
	for i, sg := range spanL {
		ssize := fmt.Sprintf("%dB", sg.size)
		if sg.size >= 1024 {
			ssize = fmt.Sprintf("%dK", sg.size/1024)
		}
		stat.WriteString(
			fmt.Sprintf("Top%d\t size:%s\t allocN:%d\t fullTimes:%d\t idleL:%d\t fullL:%d\t n:%d\n",
				i, ssize, sg.allocTimes, sg.fullTimes, sg.idleL.Len(), sg.fullL.Len(), sg.n),
		)
		for e := sg.idleL.Front(); e != nil; e = e.Next() {
			sp := e.Value.(*span)
			stat.WriteString(fmt.Sprintf("\t idles-> n:%d\t freeN:%d\n",
				sp.bitmap.Size(), sp.freeN))
		}
		for e := sg.fullL.Front(); e != nil; e = e.Next() {
			sp := e.Value.(*span)
			stat.WriteString(fmt.Sprintf("\t fulls-> n:%d\t freeN:%d\n",
				sp.bitmap.Size(), sp.freeN))
		}
		if sg.idleL.Len() > 0 || sg.fullL.Len() > 0 {
			stat.WriteString("\n")
		}
	}
	return stat.String()
}

// span group
type spanGroup struct {
	size int // readonly
	n    int // Dynamically increase n=n*2 based on the number of times spanGroup full.

	fullTimes  int // Accumulated times
	allocTimes int // Accumulated allocation times within the 'gc' cycle

	idleL *list.List
	fullL *list.List
	mp    *memPool
}

func newSpanGroup(mp *memPool, s, n int) *spanGroup {
	sg := &spanGroup{
		size:  s,
		n:     n, // init val
		idleL: list.New(),
		fullL: list.New(),
		mp:    mp,
	}
	sg.addSpan() // init
	return sg
}
func (sg *spanGroup) addSpan() *span {
	sp := newSpan(sg.size, sg.n)
	sg.idleL.PushFront(sp)
	sp.list = sg.idleL
	return sp
}
func (sg *spanGroup) alloc() mmItem {
	for {
		if sg.idleL.Len() == 0 {
			sg.n *= 2 // Dynamically increase
			sg.fullTimes++
			sg.addSpan()
		}
		for e := sg.idleL.Front(); e != nil; e = e.Next() {
			sp := e.Value.(*span)
			idx, bp := sp.alloc()
			if bp == nil { // empty
				sg.idleL.Remove(e)
				sg.fullL.PushBack(sp)
				sp.list = sg.fullL
				break
			}
			sg.allocTimes++ //
			return mmItem{bp: bp, sg: sg, sp: sp, idx: idx}
		}
	}
}
func (sg *spanGroup) free(idx int, sp *span) {
	if sp.list == sg.fullL {
		for e := sg.fullL.Front(); e != nil; e = e.Next() {
			if e.Value.(*span) == sp {
				sg.fullL.Remove(e)
				break
			}
		}
		sg.idleL.PushBack(sp)
		sp.list = sg.idleL
	}
	sp.free(idx)
}

func (sg *spanGroup) gc() {
	if sg.allocTimes < 1 {
		var next *list.Element
		for e := sg.idleL.Front(); e != nil; e = next {
			sp := e.Value.(*span)
			next = e.Next()
			if sp.gc() {
				sg.idleL.Remove(e)
			}
		}
	}
	sg.fullTimes = 0
	sg.allocTimes = 0
}

// span item
type span struct {
	sliceSize int
	freeN     int
	mm        []byte
	list      *list.List
	bitmap    *Bitmap
}

func newSpan(sliceSize, n int) *span {
	s := &span{
		sliceSize: sliceSize,
		freeN:     n,
		bitmap:    NewBitMap(n),
	}
	s.newMemChunk(s.sliceSize * n)
	return s
}
func (s *span) alloc() (int, []byte) {
	if s.freeN < 1 {
		return 0, nil // full
	}
	idleIdx := s.bitmap.FirstUnSet()
	if idleIdx < 0 { // full
		panic(fmt.Errorf("goev: span:%d idle idx < 0", s.sliceSize)) // TODO for debug
		return 0, nil
	}
	s.bitmap.Set(idleIdx)
	s.freeN--
	return int(idleIdx), s.mm[idleIdx*s.sliceSize : (idleIdx+1)*s.sliceSize]
}
func (s *span) free(idx int) {
	s.freeN++
	s.bitmap.UnSet(idx)
}
func (s *span) gc() bool {
	if s.freeN == s.bitmap.Size() { // all are free
		syscall.Munmap(s.mm)
		s.bitmap = nil
		s.mm = nil
		s.list = nil
		s.freeN = -1
		return true
	}
	return false
}
func (s *span) newMemChunk(n int) {
	pageSize := syscall.Getpagesize()
	length := (n + pageSize - 1) / pageSize * pageSize
	data, err := syscall.Mmap(-1, 0, length,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		panic(fmt.Errorf("goev: cannot allocate %d bytes via mmap: %s", length, err))
	}
	if len(data) != length {
		panic(fmt.Errorf("goev: cannot allocate %d bytes via mmap", length))
	}
	s.mm = data
}
