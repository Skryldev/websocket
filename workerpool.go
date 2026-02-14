package wsx

import "sync"

type job func()

type WorkerPool struct {

    queue chan job
    wg    sync.WaitGroup
}

func NewWorkerPool(size int) *WorkerPool {

    p := &WorkerPool{
        queue: make(chan job, 1024),
    }

    for i := 0; i < size; i++ {

        go func() {

            for j := range p.queue {
                j()
            }

        }()
    }

    return p
}

func (p *WorkerPool) Submit(j job) {

    p.queue <- j
}

func (p *WorkerPool) Shutdown() {

    close(p.queue)

    p.wg.Wait()
}
