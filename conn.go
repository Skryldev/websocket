package wsx

import (
    "encoding/json"
    "github.com/gorilla/websocket"
    "sync"
)

type Conn struct {

    ws   *websocket.Conn
    send chan []byte

    hub *Hub

    mu sync.Mutex

}

func newConn(ws *websocket.Conn, hub *Hub) *Conn {

    c := &Conn{
        ws:   ws,
        hub:  hub,
        send: make(chan []byte, 256),
    }

    go c.writeLoop()

    go c.readLoop()

    return c
}

func (c *Conn) readLoop() {

    for {

        _, msg, err := c.ws.ReadMessage()

        if err != nil {
            c.hub.remove <- c
            return
        }

        c.hub.dispatch(c, msg)
    }
}

func (c *Conn) writeLoop() {

    for msg := range c.send {

        c.ws.WriteMessage(websocket.TextMessage, msg)
    }
}

func (c *Conn) Send(topic string, event string, payload any) error {

    env := Envelope[any]{
        Topic: topic,
        Event: event,
        Data:  payload,
    }

    b, _ := json.Marshal(env)

    c.send <- b

    return nil
}
