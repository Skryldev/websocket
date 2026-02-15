package wsx

import "context"

type Context struct {
	context.Context
	Conn     *Conn
	Hub      *Hub
	Incoming RawEnvelope
}

func (c *Context) Send(topic string, event string, payload any) error {
	return c.Conn.Send(topic, event, payload)
}

func (c *Context) SendMsg(msg OutboundMessage, opts SendOptions) (DeliveryReport, error) {
	return c.Hub.sendToConn(c.Context, c.Conn, msg, opts)
}

func (c *Context) UserID() UserID {
	meta := c.Conn.Meta()
	return meta.UserID
}

func (c *Context) Meta() ConnMeta {
	return c.Conn.Meta()
}
