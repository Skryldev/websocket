package wsx

import "context"

type Context struct {
    context.Context
    Conn *Conn
    Hub  *Hub
}

func (c *Context) Send(topic string, event string, payload any) error {
    return c.Conn.Send(topic, event, payload)
}
