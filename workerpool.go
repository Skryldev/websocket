package wsx

import (
	"sync"
	"sync/atomic"
)

type job func()

type WorkerPool struct {
	queue  chan job
	wg     sync.WaitGroup
	closed atomic.Bool
}

func NewWorkerPool(size int) *WorkerPool {
	return NewWorkerPoolWithQueue(size, 1024)
}

func NewWorkerPoolWithQueue(size int, queueSize int) *WorkerPool {
	if size <= 0 {
		size = 1
	}
	if queueSize <= 0 {
		queueSize = 1024
	}

	p := &WorkerPool{
		queue: make(chan job, queueSize),
	}

	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for j := range p.queue {
				j()
			}
		}()
	}

	return p
}

func (p *WorkerPool) Submit(j job) {
	_ = p.TrySubmit(j)
}

func (p *WorkerPool) TrySubmit(j job) error {
	if j == nil {
		return nil
	}
	if p.closed.Load() {
		return ErrPoolClosed
	}

	select {
	case p.queue <- j:
		return nil
	default:
		return ErrPoolFull
	}
}

func (p *WorkerPool) Shutdown() {
	if p.closed.CompareAndSwap(false, true) {
		close(p.queue)
	}
	p.wg.Wait()
}
