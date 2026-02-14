package wsx

import "strings"

type Router struct {
    handlers map[string]HandlerFunc
}

func NewRouter() *Router {

    return &Router{
        handlers: make(map[string]HandlerFunc),
    }
}

func (r *Router) Handle(topic string, h HandlerFunc) {

    r.handlers[topic] = h
}

func (r *Router) Match(topic string) HandlerFunc {

    if h, ok := r.handlers[topic]; ok {
        return h
    }

    for pattern, handler := range r.handlers {

        if strings.HasSuffix(pattern, "*") {

            prefix := strings.TrimSuffix(pattern, "*")

            if strings.HasPrefix(topic, prefix) {
                return handler
            }
        }
    }

    return nil
}
