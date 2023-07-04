package goev

type timer interface {

    schedule(eh EvHandler, delay, interval int64) error

    // Don't call EvHandler.OnClose
    cancel(eh EvHandler)

    handleExpired(now int64) int64
}
