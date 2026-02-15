<div dir="rtl">

# WSX

`WSX` یک WebSocket Orchestration Framework برای Go است که برای بار واقعی، اتصال‌های زیاد، و نگه‌داری بلندمدت طراحی شده است.
تمرکز اصلی پروژه: API تمیز، هم‌زمانی امن، مسیر ارتقای واضح از MVP تا production multi-node.

## 1. Title + Tagline
`WSX` به‌جای یک «wrapper ساده» دور WebSocket، یک orchestration layer کامل ارائه می‌دهد: routing، rooms، middleware، delivery semantics، identity و lifecycle management در یک هسته منسجم.

## 2. ✨ Features
### ⚙️ Performance
- پردازش concurrent پیام‌ها با `WorkerPool` و queue جداگانه
- صف خروجی per-connection با drop policy قابل تنظیم
- ارسال non-blocking و کنترل backpressure برای consumerهای کند
- heartbeat + deadline برای تشخیص سریع اتصال‌های ناسالم

### 🏗 Architecture
- `Hub` به‌عنوان orchestrator مرکزی برای پیام، identity، room و lifecycle
- routing دقیق با پشتیبانی از wildcard و version
- مدیریت room با policy، capacity، invite و auto-cleanup
- middleware chain سراسری و route-level

### 🧩 Developer Experience
- API صریح و idiomatic Go
- سازگاری عقب‌رو با متدهای legacy (`Broadcast`, `Join`, `Leave`, ...)
- مدل پیام استاندارد (`Envelope`) با `id/ref/ack/version`
- گزینه‌های قابل ترکیب با pattern `Option`

### 🌐 Scalability
- abstraction برای pub/sub توزیع‌شده با `PubSub` interface
- زیرساخت fanout بین nodeها
- adapter درون‌پردازشی (`MemoryPubSub`) برای dev/test

### 🛡 Reliability & Safety
- graceful shutdown
- ACK/NACK + retry + timeout
- lifecycle hooks (`OnConnect`, `OnDisconnect`)
- interfaces برای auth، access control، validation، metrics و logging

## 3. 🏗 Architecture Overview
### اجزای اصلی
- `Server`: ورودی HTTP/WebSocket، upgrade، تزریق auth metadata
- `Hub`: orchestration مرکزی (routing، rooms، identity، dispatch، delivery)
- `Conn`: lifecycle اتصال، read/write loop، queue و heartbeat
- `Router`: mapping پیام به handler با exact/wildcard/version
- `RoomManager`: عضویت، policy، role، capacity، broadcast room
- `WorkerPool`: اجرای handlerها با کنترل concurrency
- `Middleware`: policyهای cross-cutting مثل auth/log/rate-limit/validation

### جریان کلی پیام
1. کلاینت به endpoint وب‌سوکت وصل می‌شود.
2. `Server` اتصال را upgrade می‌کند و `ConnMeta` را می‌سازد.
3. `Conn.readLoop` پیام را می‌خواند و به `Hub.Dispatch` می‌فرستد.
4. `Hub` پیام را validate/authz کرده و route مناسب را پیدا می‌کند.
5. handler در `WorkerPool` اجرا می‌شود.
6. خروجی handler از طریق `Conn` به صف send وارد می‌شود.
7. `writeLoop` با کنترل backpressure پیام را روی socket می‌نویسد.

## 4. ⚡ Quick Start (Hello World)
### نصب
```bash
go get github.com/Skryldev/websocket
```

### سرور مینیمال

<div dir="ltr">

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	wsx "github.com/Skryldev/websocket"
)

type Ping struct {
	Text string `json:"text"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := wsx.NewServer(
		ctx,
		runtime.NumCPU(),
		wsx.WithCheckOrigin(func(r *http.Request) bool { return true }),
		wsx.WithQueueConfig(wsx.QueueConfig{Size: 512, DropPolicy: wsx.DropOldest}),
		wsx.WithHeartbeat(wsx.HeartbeatConfig{
			Interval:    30 * time.Second,
			PongTimeout: 60 * time.Second,
			WriteWait:   10 * time.Second,
			ReadLimit:   1 << 20,
		}),
	)

	server.Handle("echo:ping", func(c *wsx.Context, msg wsx.RawEnvelope) error {
		var p Ping
		if err := json.Unmarshal(msg.Data, &p); err != nil {
			return err
		}
		return c.Send("echo:pong", "message", map[string]any{
			"echo": p.Text,
			"ts":   time.Now().UTC(),
		})
	})

	mux := http.NewServeMux()
	mux.Handle("/ws", server)

	httpServer := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		log.Println("ws server listening on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	_ = httpServer.Shutdown(shutdownCtx)
}
```
<div dir="rtl">

### کلاینت (Browser)

<div dir="ltr">

```js
const ws = new WebSocket("ws://localhost:8080/ws");

ws.onopen = () => {
  ws.send(JSON.stringify({
    id: "m-1",
    topic: "echo:ping",
    event: "message",
    data: { text: "hello wsx" }
  }));
};

ws.onmessage = (evt) => {
  console.log(JSON.parse(evt.data));
};
```
<div dir="rtl">

## 5. 📡 Message Model
### Envelope
تمام پیام‌های ورودی/خروجی از ساختار یکپارچه استفاده می‌کنند:

<div dir="ltr">

```json
{
  "id": "m-123",
  "ref": "m-122",
  "topic": "chat:room:general",
  "event": "message",
  "namespace": "chat",
  "version": "v1",
  "data": {"text": "hello"},
  "headers": {"trace_id": "abc"},
  "ack": true
}
```
<div dir="rtl">

### فیلدهای کلیدی
- `topic`: مسیر منطقی event (مثل `chat:room:general`)
- `event`: نوع عملیات (مثل `message`, `join`, `leave`, `ack`)
- `data`: payload اصلی
- `id`: شناسه پیام برای traceability/ack
- `ref`: ارجاع به پیام قبلی (برای پاسخ یا ack)
- `ack`: درخواست acknowledge از سمت receiver
- `version`: نسخه قرارداد پیام

## 6. 🔁 Routing & Events
### ثبت handler

<div dir="ltr">

```go
server.Handle("chat:send", handleSend)
server.Handle("chat:*", handleChatWildcard)
server.HandleVersioned("chat:send", "v2", handleSendV2)
```
<div dir="rtl">

### route-level middleware

<div dir="ltr">

```go
server.HandleWith("billing:invoice:create", handleCreateInvoice,
	wsx.AuthRequiredMiddleware(),
)
```
<div dir="rtl">

### naming best practices
- از namespaceهای domain-based استفاده کنید: `chat:*`, `system:*`, `game:*`
- eventها را عملیاتی نگه دارید: `join`, `leave`, `message`, `typing`, `ack`
- topic را stable نگه دارید و تغییرات breaking را با `version` مدیریت کنید

## 7. 👥 Rooms (Group Messaging)
### سناریوی join/leave/broadcast

<div dir="ltr">

```go
type JoinReq struct {
	Room string `json:"room"`
}

type ChatReq struct {
	Room string `json:"room"`
	Text string `json:"text"`
}

hub := server.Hub()
_ = hub.CreateRoom(context.Background(), wsx.RoomID("chat:general"), wsx.RoomOptions{
	Policy:      wsx.RoomPolicyPublic,
	Capacity:    2000,
	AutoCleanup: true,
})

server.Handle("chat:join", func(c *wsx.Context, msg wsx.RawEnvelope) error {
	var req JoinReq
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return err
	}
	return c.Hub.JoinWithOptions(c.Context, c.Conn.ID(), wsx.RoomID(req.Room), wsx.JoinOptions{
		Role: wsx.RoomRoleMember,
	})
})

server.Handle("chat:leave", func(c *wsx.Context, msg wsx.RawEnvelope) error {
	var req JoinReq
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return err
	}
	c.Hub.Leave(req.Room, c.Conn)
	return nil
})

server.Handle("chat:message", func(c *wsx.Context, msg wsx.RawEnvelope) error {
	var req ChatReq
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return err
	}
	c.Hub.BroadcastRoom(req.Room, "chat:"+req.Room, "message", map[string]any{
		"from": c.UserID(),
		"text": req.Text,
	})
	return nil
})
```
<div dir="rtl">

## 8. 🔐 Private Messaging (PvP)
### user registry + send-to-user

<div dir="ltr">

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	wsx "github.com/Skryldev/websocket"
)

type QueryAuth struct{}

func (QueryAuth) Authenticate(r *http.Request) (wsx.ConnMeta, error) {
	uid := r.URL.Query().Get("uid")
	if uid == "" {
		return wsx.ConnMeta{}, errors.New("uid is required")
	}
	return wsx.ConnMeta{UserID: wsx.UserID(uid), Tags: []string{"web"}}, nil
}

type PMReq struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

func main() {
	server := wsx.NewServer(context.Background(), 8, wsx.WithAuthenticator(QueryAuth{}))

	server.Handle("chat:pm", func(c *wsx.Context, msg wsx.RawEnvelope) error {
		var req PMReq
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return err
		}

		_, err := c.Hub.SendToUser(c.Context, wsx.UserID(req.To), wsx.OutboundMessage{
			Topic: "chat:private",
			Event: "message",
			Data: map[string]any{
				"from": c.UserID(),
				"text": req.Text,
			},
		}, wsx.SendOptions{
			RequireAck: true,
			AckTimeout: 3 * time.Second,
			RetryMax:   1,
		})

		return err
	})
}
```

<div dir="rtl">

نکته: هر `UserID` می‌تواند چند connection هم‌زمان داشته باشد (multi-device)، بنابراین `SendToUser` به تمام اتصال‌های فعال همان کاربر fanout می‌کند.

## 9. ⚙️ Hub API (Orchestration Layer)
### Identity / Presence / Lifecycle

<div dir="ltr">

```go
func (h *Hub) OnConnect(hook OnConnectHook)
func (h *Hub) OnDisconnect(hook OnDisconnectHook)
func (h *Hub) SetConnMeta(connID ConnID, patch ConnMeta) error
func (h *Hub) GetConnMeta(connID ConnID) (ConnMeta, bool)
func (h *Hub) FindConnsByTag(tag string) []ConnID
func (h *Hub) UserConnections(userID UserID) []ConnID
func (h *Hub) Presence(userID UserID) (PresenceState, bool)
func (h *Hub) SetTyping(userID UserID, roomID RoomID, typing bool, ttl time.Duration) error
```
<div dir="rtl">

### Routing / Middleware

<div dir="ltr">

```go
func (h *Hub) Use(mw ...Middleware)
func (h *Hub) Handle(pattern string, handler HandlerFunc)
func (h *Hub) HandleWith(pattern string, handler HandlerFunc, mw ...Middleware)
func (h *Hub) HandleVersioned(pattern string, version string, handler HandlerFunc, mw ...Middleware)
```
<div dir="rtl">

### Rooms

<div dir="ltr">

```go
func (h *Hub) CreateRoom(ctx context.Context, roomID RoomID, opts RoomOptions) error
func (h *Hub) Join(room string, c *Conn)
func (h *Hub) JoinWithOptions(ctx context.Context, connID ConnID, roomID RoomID, opts JoinOptions) error
func (h *Hub) Leave(room string, c *Conn)
func (h *Hub) LeaveAll(c *Conn)
func (h *Hub) KickUserFromRoom(ctx context.Context, userID UserID, roomID RoomID, reason string) (int, error)
```
<div dir="rtl">

### Messaging

<div dir="ltr">

```go
func (h *Hub) Broadcast(topic string, event string, payload any)
func (h *Hub) BroadcastRoom(room string, topic string, event string, payload any)
func (h *Hub) SendToConn(c *Conn, topic string, event string, payload any)
func (h *Hub) SendToUser(ctx context.Context, userID UserID, msg OutboundMessage, opts SendOptions) (BatchDeliveryReport, error)
func (h *Hub) SendToUsers(ctx context.Context, userIDs []UserID, msg OutboundMessage, opts SendOptions) (BatchDeliveryReport, error)
func (h *Hub) BroadcastWithFilter(ctx context.Context, msg OutboundMessage, filter ConnFilter, opts SendOptions) (int, error)
func (h *Hub) EmitToRoomExcept(ctx context.Context, roomID RoomID, exceptConnIDs []ConnID, msg OutboundMessage, opts SendOptions) (int, error)
func (h *Hub) EmitWithAck(ctx context.Context, target Target, msg OutboundMessage, opts AckOptions) (AckResult, error)
```
<div dir="rtl">

### Operations

<div dir="ltr">

```go
func (h *Hub) DisconnectUser(ctx context.Context, userID UserID, reason DisconnectReason) (int, error)
func (h *Hub) DisconnectConn(ctx context.Context, connID ConnID, reason DisconnectReason) error
func (h *Hub) GracefulShutdown(ctx context.Context) error
func (h *Hub) Stats() HubStats
```

<div dir="rtl">

## 10. 🧩 Middleware System
### تعریف middleware سفارشی

<div dir="ltr">

```go
func AuditMiddleware(next wsx.HandlerFunc) wsx.HandlerFunc {
	return func(ctx *wsx.Context, msg wsx.RawEnvelope) error {
		start := time.Now()
		err := next(ctx, msg)
		log.Printf("topic=%s event=%s user=%s latency=%s err=%v",
			msg.Topic, msg.Event, ctx.UserID(), time.Since(start), err)
		return err
	}
}
```
<div dir="rtl">

### استفاده

<div dir="ltr">

```go
limiter := wsx.NewInMemoryTokenBucketLimiter(50, 200)

server.Use(
	wsx.RecoverMiddleware(nil),
	wsx.RateLimitMiddleware(limiter, nil),
)

server.HandleWith("secure:*", secureHandler,
	wsx.AuthRequiredMiddleware(),
)
```
<div dir="rtl">

### middlewareهای آماده
- `RecoverMiddleware`
- `LoggingMiddleware`
- `AuthRequiredMiddleware`
- `ValidationMiddleware`
- `TracingMiddleware`
- `RateLimitMiddleware`

## 11. 🚀 Performance & Concurrency
- handlerها داخل `WorkerPool` اجرا می‌شوند؛ spikes ترافیک مستقیماً goroutine explosion ایجاد نمی‌کنند.
- صف خروجی هر connection مستقل است؛ یک client کند کل سیستم را block نمی‌کند.
- drop policy قابل تنظیم است:
  - `DropNewest`
  - `DropOldest`
  - `DropAndDisconnect`
- lookupهای user/tag و fanout با index انجام می‌شوند (به‌جای scan کامل همه اتصال‌ها).

### نمونه tuning

<div dir="ltr">

```go
server := wsx.NewServer(context.Background(), 32,
	wsx.WithQueueConfig(wsx.QueueConfig{
		Size:       1024,
		DropPolicy: wsx.DropOldest,
	}),
	wsx.WithHeartbeat(wsx.HeartbeatConfig{
		Interval:    20 * time.Second,
		PongTimeout: 40 * time.Second,
		WriteWait:   5 * time.Second,
		ReadLimit:   2 << 20,
	}),
)
```
<div dir="rtl">

## 12. 🛡 Reliability
- heartbeat داخلی با ping/pong و read/write deadline
- تشخیص اتصال ناسالم و cleanup خودکار room membership
- ACK/NACK + timeout + retry در ارسال پیام‌های حساس
- `GracefulShutdown` برای drain اتصال‌ها و shutdown امن

### الگوی shutdown پیشنهادی

<div dir="ltr">

```go
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

_ = server.Shutdown(shutdownCtx)
_ = httpServer.Shutdown(shutdownCtx)
```
<div dir="rtl">

### reconnect
استراتژی reconnect در WSX سمت کلاینت کنترل می‌شود. توصیه: exponential backoff + jitter و ارسال مجدد token/session در handshake.

## 13. 📊 Observability
### Logging

با `WithLogger` یک logger سازگار تزریق کنید:

<div dir="ltr">

```go
type AppLogger struct{}

func (AppLogger) Info(msg string)  { log.Println("INFO", msg) }
func (AppLogger) Error(msg string) { log.Println("ERROR", msg) }

server := wsx.NewServer(context.Background(), 8, wsx.WithLogger(AppLogger{}))
```
<div dir="rtl">

### Metrics
با `WithMetrics` هر backend دلخواه (Prometheus/OpenTelemetry metrics) را وصل کنید:

<div dir="ltr">

```go
type Metrics struct{}

func (Metrics) IncConnections(delta int)                         {}
func (Metrics) IncMessagesIn(topic string)                       {}
func (Metrics) IncMessagesOut(topic string)                      {}
func (Metrics) IncErrors(kind string)                            {}
func (Metrics) IncDropped(reason string)                         {}
func (Metrics) ObserveHandlerLatency(topic string, d time.Duration) {}
```
<div dir="rtl">

### Tracing
`TracingMiddleware` یک `Tracer` abstraction دریافت می‌کند و می‌تواند به OpenTelemetry bridge شود.

## 14. 🌐 Scaling (Multi-node)
WSX از طریق `PubSub` interface برای fanout بین nodeها آماده است.

### فعال‌سازی cluster bus

<div dir="ltr">

```go
bus := wsx.NewMemoryPubSub() // مناسب dev/test (نه production multi-node)

s1 := wsx.NewServer(context.Background(), 8,
	wsx.WithPubSub(bus, "node-a"),
	wsx.WithClusterChannel("wsx.cluster.chat"),
)

s2 := wsx.NewServer(context.Background(), 8,
	wsx.WithPubSub(bus, "node-b"),
	wsx.WithClusterChannel("wsx.cluster.chat"),
)
```
<div dir="rtl">

برای production، `PubSub` را با Redis/NATS/Kafka adapter خودتان پیاده‌سازی کنید.

## 15. 🔒 Security Best Practices
- Origin را محدود کنید (`WithCheckOrigin`) و از `return true` در production اجتناب کنید.
- Authentication را در handshake enforce کنید (`WithAuthenticator`).
- برای publish/subscribe از policy مرکزی استفاده کنید (`WithAccessController`).
- payload را validate کنید (`WithValidator` یا `ValidationMiddleware`).
- rate limiting per-user/per-IP را فعال کنید (`RateLimitMiddleware`).
- سقف اندازه پیام را تنظیم کنید (`WithMaxMessageSize` یا `HeartbeatConfig.ReadLimit`).

## 16. 📁 Project Structure
| Path | مسئولیت |
|---|---|
| `server.go` | HTTP entrypoint، upgrade، wiring |
| `hub.go` | orchestration مرکزی (routing, delivery, presence, shutdown) |
| `conn.go` | lifecycle اتصال، read/write loop، queue/backpressure |
| `router.go` | route matching (exact/wildcard/version) |
| `rooms.go` | room state، policy، membership |
| `connection_registry.go` | indexهای اتصال، user، tag |
| `workerpool.go` | اجرای concurrent handlerها |
| `middleware.go` | middlewareها و limiter |
| `options.go` | option-based configuration |
| `types.go` | type contracts و interfaces |
| `envelope.go` | مدل پیام استاندارد |
| `pubsub_memory.go` | pub/sub درون‌پردازشی برای dev/test |
| `gin/gin_adaptor.go` | adaptor برای Gin |
| `example/` | نمونه‌های قابل اجرا |

## 17. 🧪 Testing
### اجرای تست‌ها
```bash
go test ./...
```

### اجرای race detector
```bash
go test -race ./...
```

### نکات testability
- وابستگی‌ها با interface طراحی شده‌اند (`Authenticator`, `Validator`, `AccessController`, `Metrics`, `PubSub`).
- برای تست integration می‌توانید از `NewMemoryPubSub` استفاده کنید.
- منطق policy قابل تست واحد است (registry/router/rooms/workerpool).

## 18. 📌 Roadmap
- Redis Pub/Sub adapter رسمی برای multi-node production
- delivery ordering guarantee با کلید ترتیبی (`OrderingKey`) در سطح اجرایی
- ابزار benchmark/load-test داخلی
- نمونه‌های production برای OTel + Prometheus + structured logging
- کانفیگ پویا برای room policy و ACL
