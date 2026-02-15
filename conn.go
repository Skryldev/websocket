package wsx

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var connSeq atomic.Uint64

// Conn نماینده یک WebSocket connection است
type Conn struct {
	id   ConnID
	ws   *websocket.Conn
	hub  *Hub
	send chan []byte

	sendMu    sync.Mutex
	metaMu    sync.RWMutex
	meta      ConnMeta
	createdAt time.Time

	closed       atomic.Bool
	unregistered atomic.Bool
	closeOnce    sync.Once
	done         chan struct{}

	drops atomic.Uint64
}

// ایجاد یک Conn جدید
func newConn(ws *websocket.Conn, hub *Hub, meta ConnMeta) *Conn {
	queueSize := defaultQueueConfig().Size
	if hub != nil && hub.queueConfig.Size > 0 {
		queueSize = hub.queueConfig.Size
	}

	c := &Conn{
		id:        nextConnID(),
		ws:        ws,
		hub:       hub,
		send:      make(chan []byte, queueSize),
		meta:      cloneConnMeta(meta),
		createdAt: time.Now().UTC(),
		done:      make(chan struct{}),
	}

	if c.meta.Attributes == nil {
		c.meta.Attributes = make(map[string]string)
	}
	c.meta.Attributes["conn_id"] = string(c.id)

	if hub != nil {
		hub.Register(c)
	}

	go c.writeLoop()
	go c.readLoop()

	return c
}

func nextConnID() ConnID {
	seq := connSeq.Add(1)
	return ConnID(fmt.Sprintf("c-%d-%d", time.Now().UnixNano(), seq))
}

func (c *Conn) ID() ConnID {
	return c.id
}

func (c *Conn) CreatedAt() time.Time {
	return c.createdAt
}

func (c *Conn) Meta() ConnMeta {
	c.metaMu.RLock()
	defer c.metaMu.RUnlock()
	return cloneConnMeta(c.meta)
}

func (c *Conn) SetMeta(meta ConnMeta) {
	c.metaMu.Lock()
	defer c.metaMu.Unlock()
	c.meta = cloneConnMeta(meta)
	if c.meta.Attributes == nil {
		c.meta.Attributes = make(map[string]string)
	}
	c.meta.Attributes["conn_id"] = string(c.id)
}

func (c *Conn) isClosed() bool {
	if c.closed.Load() {
		return true
	}
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// =========================
// Read Loop
// =========================
func (c *Conn) readLoop() {
	if c.hub != nil {
		cfg := c.hub.heartbeatConfig
		readLimit := cfg.ReadLimit
		if c.hub.maxMessageSize > 0 {
			readLimit = c.hub.maxMessageSize
		}
		if readLimit > 0 {
			c.ws.SetReadLimit(readLimit)
		}
		if cfg.PongTimeout > 0 {
			_ = c.ws.SetReadDeadline(time.Now().Add(cfg.PongTimeout))
			c.ws.SetPongHandler(func(string) error {
				_ = c.ws.SetReadDeadline(time.Now().Add(cfg.PongTimeout))
				return nil
			})
		}
	}

	reason := DisconnectNormal
	defer func() {
		if c.hub != nil {
			c.hub.unregister(c, reason)
		}
		c.close(reason)
	}()

	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			reason = DisconnectReadError
			return
		}

		if c.hub != nil {
			c.hub.Dispatch(c, msg)
		}
	}
}

// =========================
// Write Loop
// =========================
func (c *Conn) writeLoop() {
	interval := time.Duration(0)
	if c.hub != nil {
		interval = c.hub.heartbeatConfig.Interval
	}

	var ticker *time.Ticker
	if interval > 0 {
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
	}

	reason := DisconnectNormal
	defer func() {
		if c.hub != nil {
			c.hub.unregister(c, reason)
		}
		c.close(reason)
	}()

	for {
		select {
		case <-c.done:
			return

		case payload := <-c.send:
			if err := c.writeFrame(websocket.TextMessage, payload); err != nil {
				reason = DisconnectWriteError
				return
			}

		case <-tickerChan(ticker):
			if err := c.writeFrame(websocket.PingMessage, []byte("ping")); err != nil {
				reason = DisconnectWriteError
				return
			}
		}
	}
}

func tickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func (c *Conn) writeFrame(frameType int, payload []byte) error {
	if c.isClosed() {
		return ErrConnectionClosed
	}

	writeWait := defaultHeartbeatConfig().WriteWait
	if c.hub != nil && c.hub.heartbeatConfig.WriteWait > 0 {
		writeWait = c.hub.heartbeatConfig.WriteWait
	}

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))

	if frameType == websocket.PingMessage || frameType == websocket.PongMessage || frameType == websocket.CloseMessage {
		return c.ws.WriteControl(frameType, payload, time.Now().Add(writeWait))
	}
	return c.ws.WriteMessage(frameType, payload)
}

// =========================
// ارسال پیام
// =========================
func (c *Conn) Send(topic string, event string, payload any) error {
	env := Envelope[any]{
		Topic: topic,
		Event: event,
		Data:  payload,
	}
	return c.sendEnvelope(env)
}

func (c *Conn) sendEnvelope(env any) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return c.enqueue(b)
}

func (c *Conn) enqueue(payload []byte) error {
	if c.isClosed() {
		return ErrConnectionClosed
	}

	select {
	case c.send <- payload:
		if c.hub != nil {
			c.hub.metrics.IncMessagesOut("")
		}
		return nil
	default:
	}

	policy := defaultQueueConfig().DropPolicy
	if c.hub != nil && c.hub.queueConfig.DropPolicy != "" {
		policy = c.hub.queueConfig.DropPolicy
	}

	switch policy {
	case DropOldest:
		select {
		case <-c.send:
		default:
		}
		select {
		case c.send <- payload:
			c.drops.Add(1)
			if c.hub != nil {
				c.hub.metrics.IncDropped("drop_oldest")
			}
			return nil
		default:
			c.drops.Add(1)
			if c.hub != nil {
				c.hub.metrics.IncDropped("drop_oldest_failed")
			}
			return ErrSendQueueFull
		}

	case DropAndDisconnect:
		c.drops.Add(1)
		if c.hub != nil {
			c.hub.metrics.IncDropped("drop_disconnect")
			c.hub.unregister(c, DisconnectSlowConsumer)
		}
		c.close(DisconnectSlowConsumer)
		return ErrSlowConsumer

	case DropNewest:
		fallthrough
	default:
		c.drops.Add(1)
		if c.hub != nil {
			c.hub.metrics.IncDropped("drop_newest")
		}
		return ErrSendQueueFull
	}
}

// =========================
// Close connection
// =========================
func (c *Conn) close(reason DisconnectReason) {
	c.closeOnce.Do(func() {
		close(c.done)

		writeWait := defaultHeartbeatConfig().WriteWait
		if c.hub != nil && c.hub.heartbeatConfig.WriteWait > 0 {
			writeWait = c.hub.heartbeatConfig.WriteWait
		}

		c.sendMu.Lock()
		_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
		_ = c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, string(reason)), time.Now().Add(writeWait))
		_ = c.ws.Close()
		c.sendMu.Unlock()

		c.closed.Store(true)
	})
}

// =========================
// Optional helper: context-aware close
// =========================
func (c *Conn) WithContext(ctx context.Context) *Conn {
	go func() {
		<-ctx.Done()
		if c.hub != nil {
			c.hub.unregister(c, DisconnectServerStop)
		}
		c.close(DisconnectServerStop)
	}()
	return c
}

func cloneConnMeta(meta ConnMeta) ConnMeta {
	out := meta
	if meta.Roles != nil {
		out.Roles = append([]string(nil), meta.Roles...)
	}
	if meta.Tags != nil {
		out.Tags = append([]string(nil), meta.Tags...)
	}
	if meta.Attributes != nil {
		out.Attributes = make(map[string]string, len(meta.Attributes))
		for k, v := range meta.Attributes {
			out.Attributes[k] = v
		}
	}
	return out
}
