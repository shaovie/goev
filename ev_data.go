package goev

import (
	"sync"
)

type evData struct {
	fd     int
	events uint32
	eh     EvHandler
}

type evDataMap struct {
	arrSize int
	arr     []evData // 如果针对fd, 这里应该可以不用atomic, 直接保存value

	// sync.Map is not suitable for use in evpoll as it is write-only, without read support
	sMap   map[int]*evData
	mapMtx sync.Mutex
}

func newEvDataMap(arrSize int) *evDataMap {
	if arrSize < 1 {
		panic("evFdMaxSize < 1")
	}
	mapPreSize := arrSize / 9 // 1/10
	if mapPreSize < 1 {
		mapPreSize = 128
	}
	amu := &evDataMap{
		arrSize: arrSize,
		arr:     make([]evData, arrSize),
		sMap:    make(map[int]*evData, mapPreSize),
	}
	return amu
}

func (dm *evDataMap) newOne(i int) *evData {
	if i < dm.arrSize {
		p := &(dm.arr[i])
		if p.fd > 0 { // fd MUST > 0
			panic("fd release fail!")
		}
		return p
	}
	return &evData{}
}

func (dm *evDataMap) load(i int) *evData {
	if i < dm.arrSize {
		p := &(dm.arr[i])
		if p.fd < 1 {
			return nil
		}
		return p
	}
	dm.mapMtx.Lock()
	if v, ok := dm.sMap[i]; ok {
		dm.mapMtx.Unlock()
		return v
	}
	dm.mapMtx.Unlock()
	return nil
}

func (dm *evDataMap) store(i int, v *evData) {
	if i < dm.arrSize {
		return
	}
	dm.mapMtx.Lock()
	dm.sMap[i] = v
	dm.mapMtx.Unlock()
}

func (dm *evDataMap) del(i int) {
	if i < dm.arrSize {
		p := &(dm.arr[i])
		p.fd = -1
		return
	}
	dm.mapMtx.Lock()
	delete(dm.sMap, i)
	dm.mapMtx.Unlock()
}
