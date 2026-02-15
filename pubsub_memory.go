package wsx

import (
	"context"
	"sync"
	"sync/atomic"
)

type memorySubscription struct {
	bus     *MemoryPubSub
	channel string
	id      uint64
}

func (s *memorySubscription) Close() error {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()
	if channelSubs, ok := s.bus.subs[s.channel]; ok {
		delete(channelSubs, s.id)
		if len(channelSubs) == 0 {
			delete(s.bus.subs, s.channel)
		}
	}
	return nil
}

// MemoryPubSub یک pub/sub سبک درون-پردازشی برای توسعه/تست است.
// در multi-node واقعی باید جایگزین Redis/NATS شود.
type MemoryPubSub struct {
	mu   sync.RWMutex
	subs map[string]map[uint64]func([]byte)
	seq  atomic.Uint64
}

func NewMemoryPubSub() *MemoryPubSub {
	return &MemoryPubSub{
		subs: make(map[string]map[uint64]func([]byte)),
	}
}

func (b *MemoryPubSub) Publish(ctx context.Context, channel string, payload []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	b.mu.RLock()
	handlers := b.subs[channel]
	copies := make([]func([]byte), 0, len(handlers))
	for _, h := range handlers {
		copies = append(copies, h)
	}
	b.mu.RUnlock()

	for _, h := range copies {
		h(payload)
	}

	return nil
}

func (b *MemoryPubSub) Subscribe(ctx context.Context, channel string, handler func([]byte)) (ioCloser, error) {
	if handler == nil {
		return noopCloser{}, nil
	}

	id := b.seq.Add(1)

	b.mu.Lock()
	if _, ok := b.subs[channel]; !ok {
		b.subs[channel] = make(map[uint64]func([]byte))
	}
	b.subs[channel][id] = handler
	b.mu.Unlock()

	sub := &memorySubscription{bus: b, channel: channel, id: id}
	go func() {
		<-ctx.Done()
		_ = sub.Close()
	}()

	return sub, nil
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }
