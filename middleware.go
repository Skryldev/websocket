package wsx

import (
	"context"
	"fmt"
	"sync"
	"time"
)

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

func RecoverMiddleware(logger Logger) Middleware {
	if logger == nil {
		logger = noopLogger{}
	}
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context, msg RawEnvelope) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error(fmt.Sprintf("panic recovered topic=%s event=%s panic=%v", msg.Topic, msg.Event, r))
					err = ErrInvalidPayload
				}
			}()
			return next(ctx, msg)
		}
	}
}

func LoggingMiddleware(logger Logger) Middleware {
	if logger == nil {
		logger = noopLogger{}
	}
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context, msg RawEnvelope) error {
			start := time.Now()
			err := next(ctx, msg)
			if err != nil {
				logger.Error(formatLog("handler error", "topic", msg.Topic, "event", msg.Event, "err", err.Error(), "latency", time.Since(start)))
			} else {
				logger.Info(formatLog("handler ok", "topic", msg.Topic, "event", msg.Event, "latency", time.Since(start)))
			}
			return err
		}
	}
}

func AuthRequiredMiddleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context, msg RawEnvelope) error {
			meta := ctx.Conn.Meta()
			if meta.UserID == "" {
				return ErrUnauthorized
			}
			return next(ctx, msg)
		}
	}
}

func ValidationMiddleware(v Validator) Middleware {
	if v == nil {
		return func(next HandlerFunc) HandlerFunc { return next }
	}
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context, msg RawEnvelope) error {
			if err := v.Validate(msg.Topic, msg.Event, msg.Data); err != nil {
				return err
			}
			return next(ctx, msg)
		}
	}
}

type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, func(err error))
}

func TracingMiddleware(tracer Tracer) Middleware {
	if tracer == nil {
		return func(next HandlerFunc) HandlerFunc { return next }
	}
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context, msg RawEnvelope) error {
			nctx, end := tracer.Start(ctx.Context, "wsx."+msg.Topic+"."+msg.Event)
			ctx.Context = nctx
			err := next(ctx, msg)
			end(err)
			return err
		}
	}
}

type Limiter interface {
	Allow(key string) bool
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type InMemoryTokenBucketLimiter struct {
	mu         sync.Mutex
	buckets    map[string]tokenBucket
	capacity   float64
	refillRate float64
}

func NewInMemoryTokenBucketLimiter(ratePerSec float64, burst int) *InMemoryTokenBucketLimiter {
	if ratePerSec <= 0 {
		ratePerSec = 10
	}
	if burst <= 0 {
		burst = 20
	}
	return &InMemoryTokenBucketLimiter{
		buckets:    make(map[string]tokenBucket),
		capacity:   float64(burst),
		refillRate: ratePerSec,
	}
}

func (l *InMemoryTokenBucketLimiter) Allow(key string) bool {
	if key == "" {
		key = "anonymous"
	}

	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.buckets[key]
	if b.last.IsZero() {
		b.tokens = l.capacity
		b.last = now
	}

	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.refillRate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}

	if b.tokens < 1 {
		b.last = now
		l.buckets[key] = b
		return false
	}

	b.tokens -= 1
	b.last = now
	l.buckets[key] = b
	return true
}

func RateLimitMiddleware(limiter Limiter, keyFn func(ctx *Context, msg RawEnvelope) string) Middleware {
	if limiter == nil {
		return func(next HandlerFunc) HandlerFunc { return next }
	}
	if keyFn == nil {
		keyFn = func(ctx *Context, _ RawEnvelope) string {
			meta := ctx.Conn.Meta()
			if meta.UserID != "" {
				return "user:" + string(meta.UserID)
			}
			return "ip:" + meta.IP
		}
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context, msg RawEnvelope) error {
			if !limiter.Allow(keyFn(ctx, msg)) {
				return ErrRateLimited
			}
			return next(ctx, msg)
		}
	}
}
