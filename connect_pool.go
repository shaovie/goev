package goev

import (
	"container/list"
	"errors"
	"sync"
	"sync/atomic"
)

type ConnectPool struct {
	Event

	minSize   int
	maxSize   int
	addr      string
	connector *Connector

	conns    *list.List
	connsMtx sync.Mutex
	toNewNum atomic.Int32
}

// The addr format 192.168.0.1:8080
func NewConnectPool(c *Connector, addr string, minSize, maxSize int,
	newConnHandlerFunc func() ConnectPoolConn) (*ConnectPool, error) {

    if minSize < 1 || minSize >= maxSize {
        panic("NewConnectPool min/max size invalid")
    }
	panic("ConnectPool has not been fully implemented yet")
	r := c.GetReactor()
	if r == nil {
		return nil, errors.New("connector invalid")
	}
	cp := &ConnectPool{
		minSize: minSize,
		maxSize: maxSize,
		addr:    addr,
		conns:   list.New(),
	}
	if err := r.SchedueTimer(cp, 0, 200); err != nil {
		return nil, errors.New("schedule timer fail. " + err.Error())
	}

	return cp, nil
}
func (cp *ConnectPool) Acquire() *ConnectPoolConn {
	cp.connsMtx.Lock()
	defer cp.connsMtx.Unlock()
	item := cp.conns.Front()
	if item == nil {
		return nil
	}
	cp.conns.Remove(item)
	return item.Value.(*ConnectPoolConn)
}
func (cp *ConnectPool) Release(cpc *ConnectPoolConn) {
	if cpc == nil {
		panic("ConnectPool.Release cpc is nil")
	}
	cp.connsMtx.Lock()
	cp.conns.PushBack(cpc)
	cp.connsMtx.Unlock()
}
func (cp *ConnectPool) Size() int {
	cp.connsMtx.Lock()
	defer cp.connsMtx.Unlock()
	return cp.conns.Len()
}

func (cp *ConnectPool) OnTimeout(now int64) bool {
	// 1. keep min size
	nowSize := cp.Size()
	toNewNum := 0
	if nowSize < cp.minSize {
        toSize := (cp.maxSize + cp.minSize) / 2 + 1 // n = (min + max) / 2
		toNewNum = toSize - nowSize
	}
	if toNewNum == 0 {
		return true
	}

	if !cp.toNewNum.CompareAndSwap(0, int32(toNewNum)) {
		return true
	}
	for i := 0; i < toNewNum; i++ {
		if err := cp.connector.Connect(cp.addr, &ConnectPoolConn{}, 1000); err != nil {
			cp.toNewNum.Add(-1)
		}
	}
	return true
}

type ConnectPoolConn struct {
	Event

	cp *ConnectPool
}

func (cpc *ConnectPoolConn) OnOpen(fd *Fd, now int64) bool {
	cpc.cp.toNewNum.Add(-1)
	fd.SetKeepAlive(60, 40, 3)
	cpc.cp.Release(cpc)
	return true
}
func (cpc *ConnectPoolConn) OnConnectFail(err error) {
	cpc.cp.toNewNum.Add(-1)
}
