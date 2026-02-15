package wsx

import "testing"

func TestRouterExactAndVersion(t *testing.T) {
	r := NewRouter()
	defaultHit := false
	v2Hit := false

	r.Handle("chat:send", func(*Context, RawEnvelope) error {
		defaultHit = true
		return nil
	})

	r.HandleVersion("chat:send", "v2", func(*Context, RawEnvelope) error {
		v2Hit = true
		return nil
	})

	h := r.MatchVersion("chat:send", "v2")
	if h == nil {
		t.Fatal("expected versioned handler")
	}
	_ = h(nil, RawEnvelope{})
	if !v2Hit {
		t.Fatal("expected v2 handler to execute")
	}

	h = r.MatchVersion("chat:send", "v1")
	if h == nil {
		t.Fatal("expected fallback handler")
	}
	_ = h(nil, RawEnvelope{})
	if !defaultHit {
		t.Fatal("expected default handler to execute")
	}
}

func TestRouterWildcardLongestPrefix(t *testing.T) {
	r := NewRouter()
	hitA := false
	hitB := false

	r.Handle("chat:*", func(*Context, RawEnvelope) error {
		hitA = true
		return nil
	})
	r.Handle("chat:room:*", func(*Context, RawEnvelope) error {
		hitB = true
		return nil
	})

	h := r.Match("chat:room:123")
	if h == nil {
		t.Fatal("expected wildcard handler")
	}
	_ = h(nil, RawEnvelope{})

	if hitA {
		t.Fatal("expected broader wildcard not to match first")
	}
	if !hitB {
		t.Fatal("expected longest-prefix wildcard handler")
	}
}
