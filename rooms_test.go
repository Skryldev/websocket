package wsx

import "testing"

func TestRoomJoinCapacity(t *testing.T) {
	rm := NewRoomManager()
	rm.CreateRoom("r1", RoomOptions{Policy: RoomPolicyPublic, Capacity: 1, AutoCleanup: true})

	c1 := fakeConn("c1", ConnMeta{UserID: "u1"})
	c2 := fakeConn("c2", ConnMeta{UserID: "u2"})

	if err := rm.JoinWithOptions("r1", c1, JoinOptions{}); err != nil {
		t.Fatalf("unexpected join err for c1: %v", err)
	}
	if err := rm.JoinWithOptions("r1", c2, JoinOptions{}); err == nil {
		t.Fatal("expected capacity error for c2")
	}
}

func TestRoomInviteOnly(t *testing.T) {
	rm := NewRoomManager()
	rm.CreateRoom("r2", RoomOptions{Policy: RoomPolicyInviteOnly, AutoCleanup: true})

	c := fakeConn("c1", ConnMeta{UserID: "u1"})
	if err := rm.JoinWithOptions("r2", c, JoinOptions{}); err == nil {
		t.Fatal("expected join forbidden without invite")
	}

	if err := rm.Invite("r2", "u1"); err != nil {
		t.Fatalf("unexpected invite error: %v", err)
	}
	if err := rm.JoinWithOptions("r2", c, JoinOptions{}); err != nil {
		t.Fatalf("expected join success with invite, got %v", err)
	}
}

func TestRoomAutoCleanup(t *testing.T) {
	rm := NewRoomManager()
	rm.CreateRoom("r3", RoomOptions{Policy: RoomPolicyPublic, AutoCleanup: true})

	c := fakeConn("c1", ConnMeta{UserID: "u1"})
	if err := rm.JoinWithOptions("r3", c, JoinOptions{}); err != nil {
		t.Fatalf("unexpected join err: %v", err)
	}

	rm.Leave("r3", c)
	if got := rm.RoomCount(); got != 0 {
		t.Fatalf("expected room cleanup, got room count %d", got)
	}
}
