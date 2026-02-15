package wsx

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolTrySubmit(t *testing.T) {
	p := NewWorkerPoolWithQueue(1, 2)
	defer p.Shutdown()

	var ran atomic.Int32

	if err := p.TrySubmit(func() {
		time.Sleep(50 * time.Millisecond)
		ran.Add(1)
	}); err != nil {
		t.Fatalf("unexpected submit error: %v", err)
	}

	if err := p.TrySubmit(func() {
		ran.Add(1)
	}); err != nil {
		t.Fatalf("unexpected submit error: %v", err)
	}

	if err := p.TrySubmit(func() {}); err == nil {
		t.Fatal("expected pool full error")
	}

	time.Sleep(120 * time.Millisecond)
	if got := ran.Load(); got < 2 {
		t.Fatalf("expected at least 2 jobs to run, got %d", got)
	}
}
