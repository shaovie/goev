package goev

type timer interface {

    schedule(eh EvHandler, delay, interval int64) error

    handleExpired(now int64) int64

    size() int
}

type timerItem struct {
    noCopy

    expiredAt int64
    interval  int64

    eh EvHandler
}

//= timer item
// opt: sync.Pool
func newTimerItem(expiredAt, interval int64, eh EvHandler) *timerItem {
    return &timerItem{
        expiredAt: expiredAt,
        interval: interval,
        eh: eh,
    }
}
