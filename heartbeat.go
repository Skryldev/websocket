package wsx

import "time"

func (c *Conn) StartHeartbeat(interval time.Duration) {

    go func() {

        ticker := time.NewTicker(interval)

        for range ticker.C {

            c.ws.WriteMessage(9, []byte("ping"))
        }
    }()
}
