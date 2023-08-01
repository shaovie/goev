package goev

type timerx interface {
	timerfd() int

	schedule(eh EvHandler, delay, interval int64) error

	cancel(eh EvHandler)

	handleExpired(now int64) int64

	size() int
}
