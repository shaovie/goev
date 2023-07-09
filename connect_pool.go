package goev

import (
	"container/list"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shaovie/goev/netfd"
)

type ConnectPoolHandler interface {
	EvHandler

	setPool(cp *ConnectPool)

	GetPool() *ConnectPool

	Closed()
}
type ConnectPoolItem struct {
	Event

	cp *ConnectPool
}

func (cph *ConnectPoolItem) setPool(cp *ConnectPool) {
	cph.cp = cp
}
func (cph *ConnectPoolItem) GetPool() *ConnectPool {
	return cph.cp
}
func (cph *ConnectPoolItem) Closed() {
	cph.cp.closed()
}

type ConnectPool struct {
	minIdleNum     int
	addNumOnceTime int
	maxLiveNum     int
	addr           string
	connector      *Connector

	ticker                    *time.Ticker
	conns                     *list.List
	connsMtx                  sync.Mutex
	toNewNum                  atomic.Int32
	liveNum                   atomic.Int32
	newConnectPoolHandlerFunc func() ConnectPoolHandler

	emptySig    chan struct{}
	newConnChan chan newConnInPool
}
type newConnInPool struct {
	fd int
	ch ConnectPoolHandler
}

// The addr format 192.168.0.1:8080
func NewConnectPool(c *Connector, addr string, minIdleNum, addNumOnceTime, maxLiveNum int,
	newConnectPoolHandlerFunc func() ConnectPoolHandler) (*ConnectPool, error) {

	if minIdleNum < 1 || minIdleNum >= maxLiveNum || maxLiveNum < addNumOnceTime {
		panic("NewConnectPool min/add/max  invalid")
	}
	r := c.GetReactor()
	if r == nil {
		return nil, errors.New("connector invalid")
	}
	cp := &ConnectPool{
		minIdleNum:                minIdleNum,
		addNumOnceTime:            addNumOnceTime,
		maxLiveNum:                maxLiveNum,
		addr:                      addr,
		connector:                 c,
		conns:                     list.New(),
		newConnectPoolHandlerFunc: newConnectPoolHandlerFunc,
		ticker:                    time.NewTicker(time.Millisecond * 200),
		newConnChan:               make(chan newConnInPool, runtime.NumCPU()*2),
		emptySig:                  make(chan struct{}),
	}

	go cp.keepNumTiming()
	go cp.handleNewConn()
	return cp, nil
}
func (cp *ConnectPool) Acquire() ConnectPoolHandler {
	cp.connsMtx.Lock()
	item := cp.conns.Front()
	if item == nil {
		cp.connsMtx.Unlock()
		cp.emptySig <- struct{}{}
		return nil
	}
	cp.conns.Remove(item)
	cp.connsMtx.Unlock()
	return item.Value.(ConnectPoolHandler)
}
func (cp *ConnectPool) Release(ch ConnectPoolHandler) {
	if ch.GetPool() != cp {
		panic("ConnectPool.Release ch doesn't belong to this pool")
	}
	if ch == nil {
		panic("ConnectPool.Release ch is nil")
	}
	cp.connsMtx.Lock()
	cp.conns.PushBack(ch)
	cp.connsMtx.Unlock()
}
func (cp *ConnectPool) IdleNum() int {
	cp.connsMtx.Lock()
	defer cp.connsMtx.Unlock()
	return cp.conns.Len()
}
func (cp *ConnectPool) LiveNum() int {
	return int(cp.liveNum.Load())
}
func (cp *ConnectPool) keepNumTiming() {
	for {
		select {
		case <-cp.emptySig:
			cp.keepNum()
		case <-cp.ticker.C:
			cp.keepNum()
		}
	}
}
func (cp *ConnectPool) keepNum() {
	// 1. keep min size
	idleNum := cp.IdleNum()
	toNewNum := 0
	if idleNum < cp.minIdleNum {
		toNewNum = cp.addNumOnceTime
		liveNum := cp.LiveNum()
		if liveNum == 0 {
			toNewNum = cp.minIdleNum
		} else if toNewNum+liveNum > cp.maxLiveNum {
			toNewNum = cp.maxLiveNum - liveNum
		}
	}
	if toNewNum < 1 {
		return
	}

	if !cp.toNewNum.CompareAndSwap(0, int32(toNewNum)) {
		return
	}
	for i := 0; i < toNewNum; i++ {
		if err := cp.connector.Connect(cp.addr, &ConnectPoolConn{cp: cp}, 1000); err != nil {
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
func (cp *ConnectPool) onNewConn(fd int, ch ConnectPoolHandler) {
	if ch.OnOpen(fd, time.Now().UnixMilli()) == false {
		return
	}
	cp.liveNum.Add(1)
	cp.Release(ch)
}
func (cp *ConnectPool) closed() {
	cp.liveNum.Add(-1)
}

// =
type ConnectPoolConn struct {
	Event

	cp *ConnectPool
}

func (cpc *ConnectPoolConn) OnOpen(fd int, now int64) bool {
	cpc.cp.toNewNum.Add(-1)

	netfd.SetKeepAlive(fd, 60, 40, 3)

	connHandler := cpc.cp.newConnectPoolHandlerFunc()
	connHandler.setReactor(cpc.GetReactor())
	connHandler.setPool(cpc.cp)
	cpc.cp.newConnChan <- newConnInPool{fd: fd, ch: connHandler}

	return false
}
func (cpc *ConnectPoolConn) OnConnectFail(err error) {
	cpc.cp.toNewNum.Add(-1)
}
func (cpc *ConnectPoolConn) OnClose(fd int) {
}
