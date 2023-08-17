package goev

import (
	"container/list"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall"
)

type MBuff struct {
	Len int
	idx int
	sp  *span
	sg  *spanGroup
	Buf []byte
}

func (m *MBuff) Write(p []byte) (n int, err error) {
	if cap(m.Buf)-m.Len < len(p) {
		return 0, io.ErrShortBuffer
	}
	n = copy(m.Buf[m.Len:], p)
	m.Len += n
	return
}

type memPool struct {
	getInitSize func(int) (idx, size, n int)
	spanMap     map[int]*spanGroup
}

func newMemPool(f func(int) (idx, size, n int)) *memPool {
	return &memPool{
		getInitSize: f,
		spanMap:     make(map[int]*spanGroup, 512+2),
	}
}

var lockedMemPool = newMemPool(getMallocSizeInfo)
var lockedMemPoolMtx sync.Mutex

func Malloc(s int) MBuff {
	lockedMemPoolMtx.Lock()
	defer lockedMemPoolMtx.Unlock()
	return lockedMemPool.Malloc(s)
}
func Free(mb MBuff) {
	lockedMemPoolMtx.Lock()
	defer lockedMemPoolMtx.Unlock()
	lockedMemPool.Free(mb)
}
func MemPoolStat() string {
	lockedMemPoolMtx.Lock()
	defer lockedMemPoolMtx.Unlock()
	return lockedMemPool.Stat()
}

// init val
func getMallocSizeInfo(s int) (idx, size, n int) {
	//   s                      idx    size      n
	//
	// { (0-256],               0      256      1024*2 },  // span size 256*1024*2         0.5M
	// { (256-512],             1      512      1024*2 },  // span size 512*1024*2         1M
	// { (512-1024*1],          2      1024*1   1024*2 },  // span size (1024*1)*(1024*2)  2M
	//
	// { (1024*1-1024*2],       3      1024*2   1024*1 },  // span size (1024*2)*(1024*1)  2M
	// { (1024*2-1024*3],       4      1024*3   1024*1 },  // span size (1024*3)*(1024*1)  3M
	// ...
	// { (1024*15-1024*16],     17     1024*16  256    },  // span size (1024*16)*(256)    4M
	// ...
	// { (1024*63-1024*64],     65     1024*64  64     },  // span size (1024*64)*(64)     4M
	// { (1024*64-1024*65],     66     1024*65  64     },  // span size (1024*65)*(64)     4.1M
	// ...
	// { (1024*256-1024*512],   513    1024*512 16     },  // span size (1024*512)*(16)    8M
	// ...
	// { (1024*1023-1024*1024], 1025   1024*1024 16    },  // span size (1024*1024)*(16)   16M

	if s <= 1024 {
		n = 2048
		if s > 512 {
			size = 1024
			idx = 2
		} else if s > 256 {
			size = 512
			idx = 1
		} else {
			idx = 0
			size = 256
		}
	} else {
		if s > 1024*1024*1 { // use make([]byte, n)
			return 0, 0, 0
		}

		size = (s + 1024 - 1) / 1024 * 1024
		idx = size/1024 + 1
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
func (mp *memPool) Malloc(s int) MBuff {
	idx, size, n := mp.getInitSize(s)
	if n == 0 {
		return MBuff{Buf: make([]byte, s)}
	}

	sg, ok := mp.spanMap[idx]
	if !ok {
		sg = newSpanGroup(mp, size, n)
		mp.spanMap[idx] = sg
	}
	return sg.alloc()
}
func (mp *memPool) Free(mb MBuff) {
	if mb.sg == nil {
		return
	}
	if mb.sg.mp != mp {
		panic("goev: memPool.Free mb not belong this mempool")
	}
	mb.sg.free(mb.idx, mb.sp)
}
func (mp *memPool) Stat() string {
	var stat strings.Builder
	stat.Grow(1024)
	for idx, sg := range mp.spanMap {
		ssize := fmt.Sprintf("%dB", sg.size)
		if sg.size >= 1024 {
			ssize = fmt.Sprintf("%dK", sg.size/1024)
		}
		stat.WriteString(
			fmt.Sprintf("idx:%d\t size:%s\t idleL:%d\t fullL:%d\t n:%d\t allocTimess:%d\t fullTimes:%d\n",
				idx, ssize, sg.idleL.Len(), sg.fullL.Len(), sg.n, sg.allocTimes, sg.fullTimes),
		)
		for e := sg.idleL.Front(); e != nil; e = e.Next() {
			sp := e.Value.(*span)
			stat.WriteString(fmt.Sprintf("\t idles. n:%d\t sliceSize:%d\t freeN:%d\t\n",
				sp.bitmap.Size(), sp.sliceSize, sp.freeN))
		}
		for e := sg.fullL.Front(); e != nil; e = e.Next() {
			sp := e.Value.(*span)
			stat.WriteString(fmt.Sprintf("\t fulls. n:%d\t sliceSize:%d\t freeN:%d\t\n",
				sp.bitmap.Size(), sp.sliceSize, sp.freeN))
		}
		//stat.WriteString("\n")
	}
	return stat.String()
}

// span group
type spanGroup struct {
	size int // readonly
	n    int // Dynamically increase n=n*2 based on the number of times spanGroup full.

	fullTimes  int // Accumulated allocation times within the 'gc' cycle
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
func (sg *spanGroup) alloc() MBuff {
	for {
		if sg.idleL.Len() == 0 {
			sg.n *= 2 // Dynamically increase
			sg.fullTimes++
			sg.addSpan()
		}
		for e := sg.idleL.Front(); e != nil; e = e.Next() {
			sp := e.Value.(*span)
			idx, mm := sp.alloc()
			if mm == nil { // empty
				sg.idleL.Remove(e)
				sg.fullL.PushBack(e.Value)
				break
			}
			sg.allocTimes++ //
			return MBuff{Buf: mm, sg: sg, sp: sp, idx: idx}
		}
	}
}
func (sg *spanGroup) free(idx int, sp *span) {
	if sp.list == sg.fullL {
		for e := sg.fullL.Front(); e != nil; e = e.Next() {
			tsp := e.Value.(*span)
			if tsp == sp {
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
	idleIdx := s.bitmap.firstUnSet()
	if idleIdx < 0 { // full
		panic("idle idx < 0") // TODO for debug
		return 0, nil
	}
	s.bitmap.Set(idleIdx)
	s.freeN--
	return int(idleIdx), s.mm[idleIdx*s.sliceSize : (idleIdx+1)*s.sliceSize]
}
func (s *span) free(idx int) {
	s.freeN++
	s.bitmap.Unset(idx)
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
