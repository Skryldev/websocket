package wsx

type Middleware func(next HandlerFunc) HandlerFunc

type HandlerFunc func(ctx *Context, msg RawEnvelope) error

func Chain(m ...Middleware) Middleware {

    return func(next HandlerFunc) HandlerFunc {

        for i := len(m) - 1; i >= 0; i-- {
            next = m[i](next)
        }

        return next
    }
}
