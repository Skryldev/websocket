<div dir="rtl">

# 🚀 WSX — ماژول WebSocket حرفه‌ای و Production-Grade برای Golang    

`wsx` یک ماژول سبک برای ساخت WebSocket Server در Go است که روی سادگی API و همزمانی امن تمرکز دارد.
این ماژول برای سناریوهای real-time (مثل چت، نوتیفیکیشن لحظه‌ای، event stream داخلی) مناسب است.

---

## 1) 📖 معرفی و قابلیت‌ها
#### WSX یک ماژول WebSocket پیشرفته برای زبان Go است که برای استفاده در محیط‌های production واقعی طراحی شده است.

- ساخت WebSocket Server با API ساده (`NewServer`, `Handle`, `ServeHTTP`)
- مسیریابی پیام بر اساس `topic`
- پشتیبانی از wildcard prefix با `*` (مثل `chat:*`)
- پردازش concurrent پیام‌ها با Worker Pool
- Context اختصاصی برای هر هندلر (`*wsx.Context`)
- قابلیت تعریف Middleware chain

---

## 2) 🧠 معماری داخلی

این ماژول از چند جزء اصلی تشکیل شده است:

- `Server`: نقطه ورود HTTP/WebSocket و مدیریت upgrade
- `Hub`: نگهداری connectionها و dispatch پیام‌ها
- `Router`: پیدا کردن handler بر اساس `topic`/pattern
- `WorkerPool`: اجرای async هندلرها
- `Conn`: مدیریت read/write loop هر اتصال

جریان پیام:

1. کلاینت به endpoint وب‌سوکت متصل می‌شود.
2. `Server` اتصال را upgrade می‌کند.
3. `Conn.readLoop` پیام JSON را می‌خواند.
4. `Hub.dispatch` پیام را parse می‌کند.
5. `Router` handler مناسب را پیدا می‌کند.
6. handler داخل `WorkerPool` اجرا می‌شود.
7. پاسخ از طریق `ctx.Send(...)` به همان اتصال برمی‌گردد.

## 3) 📦 ویژگی‌ها
- پشتیبانی از هزاران اتصال همزمان
- worker pool برای کنترل load
- non-blocking architecture
- backpressure handling
---

## 3) نصب و ایمپورت

در این ریپو:

```bash
go get github.com/askari/gpm/tutorial/websocket
```

---

## 4) ساختار پیام (Envelope)

فرمت پیام‌های ورودی/خروجی:

```json
{
  "topic": "chat:room1",
  "event": "message",
  "data": {
    "text": "hello"
  }
}
```

---

## 🧪 استفاده سریع (Quick Start)

### 1️⃣ ایجاد Server

<div dir="ltr">

```go
ctx := context.Background()
server := wsx.NewServer(ctx, 8) // 8 worker برای پردازش پیام‌ها
```

<div dir="rtl">

### 2️⃣ تعریف Handler

<div dir="ltr">

```go
type ChatMessage struct {
    Text string `json:"text"`
}

server.Handle("chat:*", func(ctx *wsx.Context, msg wsx.RawEnvelope) error {
    var payload ChatMessage
    if err := json.Unmarshal(msg.Data, &payload); err != nil {
        return err
    }

    // پاسخ به همان اتصال
    return ctx.Send(msg.Topic, "message", map[string]any{

    "echo": payload.Text,
    })
})
```

<div dir="rtl">

### 3️⃣ اتصال به HTTP Server

<div dir="ltr">

```go
mux := http.NewServeMux()
mux.Handle("/ws", server)

http.ListenAndServe(":8080", mux)
```
<div dir="rtl">

### 4️⃣ اتصال کلاینت (Browser)
#### 🧩 نحوه ارسال پیام از Client
<div dir="ltr">

```js
const ws = new WebSocket("ws://localhost:8080/ws");

ws.onopen = () => {
  ws.send(JSON.stringify({
    topic: "chat:room1",
    event: "send",
    data: { text: "Salam" }
  }));
};
```

<div dir="rtl">

#### 📥 دریافت پیام
<div dir="ltr">

```js
ws.onmessage = (evt) => {
  console.log("server:", JSON.parse(evt.data));
};
```

<div dir="rtl">

### نمونه کامل قابل اجرا (net/http)

<div dir="ltr">

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"

    wsx "github.com/askari/gpm/tutorial/websocket"
)

type ChatMessage struct {
    Text string `json:"text"`
}

func main() {
    ctx := context.Background()
    server := wsx.NewServer(ctx, 8)

    server.Handle("chat:*", func(ctx *wsx.Context, msg wsx.RawEnvelope) error {
        var chat ChatMessage
        if err := json.Unmarshal(msg.Data, &chat); err != nil {
            return err
        }
        return ctx.Send(msg.Topic, "message", chat)
    })

    mux := http.NewServeMux()
    mux.Handle("/ws", server)

    log.Println("ws server on :8080")
    if err := http.ListenAndServe(":8080", mux); err != nil {
        log.Fatal(err)
    }
}
```
<div dir="rtl">

---
## 🌐 استفاده با Gin

این پکیج یک adaptor ساده برای Gin دارد:

<div dir="ltr">

```go
r := gin.Default()
r.GET("/ws", wsx.Handler(server))
r.Run(":8080")
```
<div dir="rtl">

نمونه کامل:

<div dir="ltr">

```go
package main

import (
    "context"
    "encoding/json"

    "github.com/gin-gonic/gin"
    wsx "github.com/askari/gpm/tutorial/websocket"
)

type ChatMessage struct {
    Text string `json:"text"`
}

func main() {
    server := wsx.NewServer(context.Background(), 8)

    server.Handle("chat:*", func(ctx *wsx.Context, msg wsx.RawEnvelope) error {
        var chat ChatMessage
        if err := json.Unmarshal(msg.Data, &chat); err != nil {
            return err
        }
        return ctx.Send(msg.Topic, "message", chat)
    })

    r := gin.Default()
    r.GET("/ws", wsx.Handler(server))
    _ = r.Run(":8080")
}
```
<div dir="rtl">

---

## 📡 استفاده از Middleware

##### نوع middleware:

<div dir="ltr">

```go
type Middleware func(next HandlerFunc) HandlerFunc
```
<div dir="rtl">

##### ترکیب middlewareها:

<div dir="ltr">

```go
chain := wsx.Chain(loggingMW, authMW)
handler := chain(func(ctx *wsx.Context, msg wsx.RawEnvelope) error {
return ctx.Send(msg.Topic, "ok", map[string]any{"status": "done"})
})

server.Handle("chat:*", handler)
```
<div dir="rtl">

##### نمونه middleware:

<div dir="ltr">

```go
func loggingMW(next wsx.HandlerFunc) wsx.HandlerFunc {
    return func(ctx *wsx.Context, msg wsx.RawEnvelope) error {
        log.Printf("topic=%s event=%s", msg.Topic, msg.Event)
        return next(ctx, msg)
    }
}

server.Use(LoggerMiddleware)
```
<div dir="rtl">

---
## 🧯 خطاها و مدیریت خطا

خطاهای تعریف‌شده:

- `wsx.ErrConnectionClosed`
- `wsx.ErrHandlerNotFound`
- `wsx.ErrRateLimited`

نکته مهم: در پیاده‌سازی فعلی، مقدار `error` برگشتی از handler در `Hub.dispatch` استفاده نمی‌شود (فقط فراخوانی می‌شود). اگر نیاز به سیاست خطای مرکزی دارید، در middleware یا کد `dispatch` آن را مدیریت کنید.

---

## API Reference

### Server

- `wsx.NewServer(ctx context.Context, workers int) *Server`
- `(*Server).Handle(topic string, handler HandlerFunc)`
- `(*Server).ServeHTTP(w http.ResponseWriter, r *http.Request)`

### Router

- `wsx.NewRouter() *Router`
- `(*Router).Handle(topic string, h HandlerFunc)`
- `(*Router).Match(topic string) HandlerFunc`

### Connection / Context

- `(*Conn).Send(topic, event string, payload any) error`
- `(*Conn).StartHeartbeat(interval time.Duration)`
- `(*Context).Send(topic, event string, payload any) error`

### WorkerPool

- `wsx.NewWorkerPool(size int) *WorkerPool`
- `(*WorkerPool).Submit(j func())`
- `(*WorkerPool).Shutdown()`

### Middleware

- `type HandlerFunc func(ctx *Context, msg RawEnvelope) error`
- `type Middleware func(next HandlerFunc) HandlerFunc`
- `wsx.Chain(m ...Middleware) Middleware`

---

## 📈 نکات Production

- `CheckOrigin` در `Server` فعلا روی `true` است؛ حتما برای production محدودش کنید.
- `json.Unmarshal` و `json.Marshal` در حال حاضر بدون مدیریت کامل خطا/validation استفاده می‌شوند؛ برای data contract سخت‌گیرانه‌تر، validation اضافه کنید.
- در `writeLoop` خطای `WriteMessage` هندل نمی‌شود؛ برای پایداری بیشتر، مدیریت close/retry را اضافه کنید.
- `Hub` هنگام `ctx.Done()` فقط loop را متوقف می‌کند؛ برای shutdown کامل، بستن connectionها و `WorkerPool` را هم پیاده‌سازی کنید.
- اگر queue پر شود (`WorkerPool.queue`)، ارسال job بلاک می‌شود؛ متناسب با throughput اندازه queue/workers را تنظیم کنید.
