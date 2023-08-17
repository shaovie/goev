package goev

import (
	"time"
)

type cacheTime struct {
	IOHandle

	ep *evPoll
}

func newCacheTime(ep *evPoll, period int) *cacheTime {
	ct := &cacheTime{
		ep: ep,
	}
	ep.updateCachedTime(time.Now().UnixMilli())
	ep.scheduleTimer(ct, int64(period), int64(period))
	return ct
}
func (ct *cacheTime) OnTimeout(now int64) bool {
	ct.ep.updateCachedTime(now)
	return true
}
