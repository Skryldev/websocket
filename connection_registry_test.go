package wsx

import "testing"

func fakeConn(id ConnID, meta ConnMeta) *Conn {
	c := &Conn{id: id, meta: meta}
	return c
}

func TestConnRegistryIndexes(t *testing.T) {
	r := NewConnRegistry()

	c1 := fakeConn("c1", ConnMeta{UserID: "u1", Tags: []string{"a", "b"}})
	c2 := fakeConn("c2", ConnMeta{UserID: "u1", Tags: []string{"b"}})
	c3 := fakeConn("c3", ConnMeta{UserID: "u2", Tags: []string{"z"}})

	r.Add(c1)
	r.Add(c2)
	r.Add(c3)

	if got := len(r.ByUser("u1")); got != 2 {
		t.Fatalf("expected 2 connections for u1, got %d", got)
	}

	if got := len(r.ByTag("b")); got != 2 {
		t.Fatalf("expected 2 connections for tag b, got %d", got)
	}

	r.Remove(c2)
	if got := len(r.ByUser("u1")); got != 1 {
		t.Fatalf("expected 1 connection for u1 after remove, got %d", got)
	}
}

func TestConnRegistryReindex(t *testing.T) {
	r := NewConnRegistry()
	c := fakeConn("c1", ConnMeta{UserID: "u1", Tags: []string{"old"}})
	r.Add(c)

	old := c.Meta()
	newMeta := ConnMeta{UserID: "u2", Tags: []string{"new"}}
	c.SetMeta(newMeta)
	r.Reindex(c, old, newMeta)

	if got := len(r.ByUser("u1")); got != 0 {
		t.Fatalf("expected 0 old user conns, got %d", got)
	}
	if got := len(r.ByUser("u2")); got != 1 {
		t.Fatalf("expected 1 new user conn, got %d", got)
	}
	if got := len(r.ByTag("old")); got != 0 {
		t.Fatalf("expected 0 old tag conns, got %d", got)
	}
	if got := len(r.ByTag("new")); got != 1 {
		t.Fatalf("expected 1 new tag conn, got %d", got)
	}
}
