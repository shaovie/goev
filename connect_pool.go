package goev

import (
    "time"
	"sync"
	"errors"
	"runtime"
	"sync/atomic"
	"container/list"

    "goev/netfd"
)

type ConnHandler EvHandler

type conntionItem struct {
    fd int
    ch ConnHandler
}

type ConnectPool struct {
	minSize   int
	addSize   int
	maxSize   int
	addr      string
	connector *Connector

    ticker   *time.Ticker
	conns    *list.List
	connsMtx sync.Mutex
	toNewNum atomic.Int32
    newConnHandlerFunc func() ConnHandler

    emptySig chan struct{}
    newConnChan chan conntionItem
}

// The addr format 192.168.0.1:8080
func NewConnectPool(c *Connector, addr string, minSize, addSize, maxSize int,
	newConnHandlerFunc func() ConnHandler) (*ConnectPool, error) {

    if minSize < 1 || minSize >= maxSize || maxSize < addSize {
        panic("NewConnectPool min/add/max size invalid")
    }
	r := c.GetReactor()
	if r == nil {
		return nil, errors.New("connector invalid")
	}
	cp := &ConnectPool{
		minSize: minSize,
        addSize: addSize,
		maxSize: maxSize,
		addr:    addr,
        connector: c,
		conns:   list.New(),
        newConnHandlerFunc: newConnHandlerFunc,
        ticker: time.NewTicker(time.Millisecond * 200),
        newConnChan: make(chan conntionItem, runtime.NumCPU() * 2),
        emptySig: make(chan struct{}, runtime.NumCPU() * 2),
	}

    go cp.keepSizeTiming()
    go cp.handleNewConn()
	return cp, nil
}
func (cp *ConnectPool) Acquire() ConnHandler {
	cp.connsMtx.Lock()
	defer cp.connsMtx.Unlock()
	item := cp.conns.Front()
	if item == nil {
        cp.emptySig <- struct{}{}
		return nil
	}
	cp.conns.Remove(item)
	return item.Value.(ConnHandler)
}
func (cp *ConnectPool) Release(ch ConnHandler) {
	if ch == nil {
		panic("ConnectPool.Release ch is nil")
	}
	cp.connsMtx.Lock()
	cp.conns.PushBack(ch)
	cp.connsMtx.Unlock()
}
func (cp *ConnectPool) Size() int {
	cp.connsMtx.Lock()
	defer cp.connsMtx.Unlock()
	return cp.conns.Len()
}

func (cp *ConnectPool) keepSizeTiming() {
    for {
        select {
        case <-cp.emptySig:
                cp.keepSize()
        case <-cp.ticker.C:
                cp.keepSize()
        }
    }
}
func (cp *ConnectPool) keepSize() {
	// 1. keep min size
	nowSize := cp.Size()
	toNewNum := 0
	if nowSize < cp.minSize {
		toNewNum = cp.addSize
	}
	if toNewNum == 0 {
		return
	}

	if !cp.toNewNum.CompareAndSwap(0, int32(toNewNum)) {
		return
	}
	for i := 0; i < toNewNum; i++ {
        if err := cp.connector.Connect(cp.addr, &ConnectPoolConn{cp: cp}, 1000); err != nil {
            Error("connect fail! " + err.Error())
			cp.toNewNum.Add(-1)
		}
	}
}
func (cp *ConnectPool) handleNewConn() {
    for {
        select {
        case conn := <-cp.newConnChan:
            cp.onNewConn(conn.fd, conn.ch)
        }
    }
}
func (cp *ConnectPool) onNewConn(fd int, ch ConnHandler) {
    if ch.OnOpen(fd, time.Now().UnixMilli()) == false {
        return 
    }
	cp.Release(ch)
}
//= 
type ConnectPoolConn struct {
	Event

	cp *ConnectPool
}
func (cpc *ConnectPoolConn) OnOpen(fd int, now int64) bool {
	cpc.cp.toNewNum.Add(-1)

    netfd.SetKeepAlive(fd, 60, 40, 3)
    
    connHandler := cpc.cp.newConnHandlerFunc()
    connHandler.setReactor(cpc.GetReactor())
    cpc.cp.newConnChan <- conntionItem{fd: fd, ch: connHandler}

	return false
}
func (cpc *ConnectPoolConn) OnConnectFail(err error) {
    Debug("connect fail")
	cpc.cp.toNewNum.Add(-1)
}
func (cpc *ConnectPoolConn) OnClose(fd int) {
}
