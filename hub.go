package wsx

import (
    "context"
    "encoding/json"
)

type Hub struct {

    router *Router

    conns map[*Conn]struct{}

    add    chan *Conn
    remove chan *Conn

    pool *WorkerPool

    ctx context.Context
}

func NewHub(ctx context.Context, workers int) *Hub {

    h := &Hub{

        router: NewRouter(),

        conns: make(map[*Conn]struct{}),

        add:    make(chan *Conn),
        remove: make(chan *Conn),

        pool: NewWorkerPool(workers),

        ctx: ctx,
    }

    go h.run()

    return h
}

func (h *Hub) run() {

    for {

        select {

        case c := <-h.add:

            h.conns[c] = struct{}{}

        case c := <-h.remove:

            delete(h.conns, c)

        case <-h.ctx.Done():

            return
        }
    }
}

func (h *Hub) dispatch(c *Conn, msg []byte) {

    var env RawEnvelope

    json.Unmarshal(msg, &env)

    handler := h.router.Match(env.Topic)

    if handler == nil {
        return
    }

    ctx := &Context{

        Context: h.ctx,
        Conn:    c,
        Hub:     h,
    }

    h.pool.Submit(func() {

        handler(ctx, env)

    })
}

func (h *Hub) Broadcast(topic string, event string, payload any) {

	for c := range h.conns {
		c.Send(topic, event, payload)
	}
}
