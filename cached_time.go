package goev

import (
	"time"
)

type cachedTime struct{
	IOHandle

    ep *evPoll
}
func newCachedTime(ep *evPoll, period int) *cachedTime {
    ct := &cachedTime{
        ep: ep,
    }
    ep.updateCachedTime(time.Now().UnixMilli())
    ep.scheduleTimer(ct, int64(period), int64(period))
    return ct
}
func (ct *cachedTime) OnTimeout(now int64) bool {
    ct.ep.updateCachedTime(now)
	return true
}
