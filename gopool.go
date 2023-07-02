// Refer to https://www.zhihu.com/question/486002075/answer/2823943072
package goev

type GoPool struct {
    noCopy
	sem  chan struct{}
	work chan func()
}

// Fixed number of goroutines, reusable. M:N model
//
// M: the number of reusable goroutines,
// N: the capacity for asynchronous task processing.
func NewGoPool(sizeM, preSpawn, queueN int) *GoPool {
	if preSpawn <= 0 && queueN > 0 {
		panic("GoPool: dead queue")
	}
	if preSpawn > sizeM {
		preSpawn = sizeM
	}
	p := &GoPool{
		sem:  make(chan struct{}, sizeM),
		work: make(chan func(), queueN),
	}
	for i := 0; i < preSpawn; i++ { // pre spawn
		p.sem <- struct{}{}
		go p.worker(func() {})
	}
	return p
}
func (p *GoPool) Go(task func()) {
	select {
	case p.work <- task:
	case p.sem <- struct{}{}:
		go p.worker(task)
	}
}
func (p *GoPool) worker(task func()) {
	defer func() { <-p.sem }()

	for {
		task()
		task = <-p.work
	}
}
