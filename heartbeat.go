package wsx

import (
	"time"

	"github.com/gorilla/websocket"
)

// StartHeartbeat برای سازگاری عقب‌رو باقی مانده است.
// heartbeat اصلی به‌صورت پیش‌فرض در writeLoop اجرا می‌شود.
func (c *Conn) StartHeartbeat(interval time.Duration) {
	if interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = c.writeFrame(websocket.PingMessage, []byte("ping"))
			case <-c.done:
				return
			}
		}
	}()
}
