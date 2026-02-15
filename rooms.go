package wsx

import (
	"sync"
	"time"
)

type roomMember struct {
	conn     *Conn
	userID   UserID
	role     RoomRole
	joinedAt time.Time
}

type roomState struct {
	id      RoomID
	opts    RoomOptions
	members map[ConnID]*roomMember
	invites map[UserID]struct{}
}

type RoomManager struct {
	mu        sync.RWMutex
	rooms     map[RoomID]*roomState
	connRooms map[ConnID]map[RoomID]struct{}
}

func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:     make(map[RoomID]*roomState),
		connRooms: make(map[ConnID]map[RoomID]struct{}),
	}
}

func (rm *RoomManager) CreateRoom(roomID RoomID, opts RoomOptions) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.getOrCreateLocked(roomID, opts)
}

func (rm *RoomManager) Invite(roomID RoomID, userID UserID) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	room, ok := rm.rooms[roomID]
	if !ok {
		return ErrRoomNotFound
	}
	room.invites[userID] = struct{}{}
	return nil
}

// JoinWithOptions اتصال را با policy های room join می‌کند.
func (rm *RoomManager) JoinWithOptions(roomID RoomID, c *Conn, joinOpts JoinOptions) error {
	if c == nil {
		return ErrConnectionClosed
	}

	meta := c.Meta()

	rm.mu.Lock()
	defer rm.mu.Unlock()

	room := rm.getOrCreateLocked(roomID, RoomOptions{
		Policy:      RoomPolicyPublic,
		AutoCleanup: true,
	})

	if _, exists := room.members[c.ID()]; exists {
		return nil
	}

	if room.opts.Capacity > 0 && len(room.members) >= room.opts.Capacity {
		return ErrRoomCapacity
	}

	if !joinOpts.Force {
		switch room.opts.Policy {
		case RoomPolicyPublic:
			// no-op
		case RoomPolicyPrivate:
			if !hasAnyRole(meta.Roles, room.opts.AllowedRoles) {
				return ErrRoomJoinForbidden
			}
		case RoomPolicyInviteOnly:
			if _, ok := room.invites[meta.UserID]; !ok {
				return ErrRoomJoinForbidden
			}
			delete(room.invites, meta.UserID)
		}
	}

	role := joinOpts.Role
	if role == "" {
		role = RoomRoleMember
	}

	room.members[c.ID()] = &roomMember{
		conn:     c,
		userID:   meta.UserID,
		role:     role,
		joinedAt: time.Now().UTC(),
	}

	if _, ok := rm.connRooms[c.ID()]; !ok {
		rm.connRooms[c.ID()] = make(map[RoomID]struct{})
	}
	rm.connRooms[c.ID()][roomID] = struct{}{}

	return nil
}

// join room
func (rm *RoomManager) Join(room string, c *Conn) {
	_ = rm.JoinWithOptions(RoomID(room), c, JoinOptions{Role: RoomRoleMember})
}

// leave room
func (rm *RoomManager) Leave(room string, c *Conn) {
	if c == nil {
		return
	}
	rm.leaveByID(RoomID(room), c.ID())
}

func (rm *RoomManager) LeaveByConnID(roomID RoomID, connID ConnID) {
	rm.leaveByID(roomID, connID)
}

func (rm *RoomManager) leaveByID(roomID RoomID, connID ConnID) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, ok := rm.rooms[roomID]
	if !ok {
		return
	}

	delete(room.members, connID)
	if connRooms, ok := rm.connRooms[connID]; ok {
		delete(connRooms, roomID)
		if len(connRooms) == 0 {
			delete(rm.connRooms, connID)
		}
	}

	if room.opts.AutoCleanup && len(room.members) == 0 {
		delete(rm.rooms, roomID)
	}
}

// remove connection from ALL rooms
func (rm *RoomManager) RemoveConn(c *Conn) {
	if c == nil {
		return
	}
	rm.RemoveConnID(c.ID())
}

func (rm *RoomManager) RemoveConnID(connID ConnID) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rooms, ok := rm.connRooms[connID]
	if !ok {
		return
	}

	for roomID := range rooms {
		room, ok := rm.rooms[roomID]
		if !ok {
			continue
		}
		delete(room.members, connID)
		if room.opts.AutoCleanup && len(room.members) == 0 {
			delete(rm.rooms, roomID)
		}
	}
	delete(rm.connRooms, connID)
}

func (rm *RoomManager) KickUser(roomID RoomID, userID UserID) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, ok := rm.rooms[roomID]
	if !ok {
		return 0
	}

	removed := 0
	for connID, member := range room.members {
		if member.userID != userID {
			continue
		}
		delete(room.members, connID)
		if connRooms, ok := rm.connRooms[connID]; ok {
			delete(connRooms, roomID)
			if len(connRooms) == 0 {
				delete(rm.connRooms, connID)
			}
		}
		removed++
	}

	if room.opts.AutoCleanup && len(room.members) == 0 {
		delete(rm.rooms, roomID)
	}

	return removed
}

func (rm *RoomManager) Members(roomID RoomID) []*Conn {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	room, ok := rm.rooms[roomID]
	if !ok {
		return nil
	}

	out := make([]*Conn, 0, len(room.members))
	for _, m := range room.members {
		out = append(out, m.conn)
	}
	return out
}

func (rm *RoomManager) ConnIDs(roomID RoomID) []ConnID {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	room, ok := rm.rooms[roomID]
	if !ok {
		return nil
	}

	out := make([]ConnID, 0, len(room.members))
	for connID := range room.members {
		out = append(out, connID)
	}
	return out
}

func (rm *RoomManager) RoomCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.rooms)
}

func (rm *RoomManager) GetRoomOptions(roomID RoomID) (RoomOptions, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	room, ok := rm.rooms[roomID]
	if !ok {
		return RoomOptions{}, false
	}
	return cloneRoomOptions(room.opts), true
}

func (rm *RoomManager) Broadcast(room, topic, event string, payload any) {
	conns := rm.Members(RoomID(room))
	for _, c := range conns {
		_ = c.Send(topic, event, payload)
	}
}

func (rm *RoomManager) getOrCreateLocked(roomID RoomID, opts RoomOptions) *roomState {
	if room, ok := rm.rooms[roomID]; ok {
		return room
	}
	if opts.Policy == "" {
		opts.Policy = RoomPolicyPublic
	}
	room := &roomState{
		id:      roomID,
		opts:    cloneRoomOptions(opts),
		members: make(map[ConnID]*roomMember),
		invites: make(map[UserID]struct{}),
	}
	rm.rooms[roomID] = room
	return room
}

func cloneRoomOptions(opts RoomOptions) RoomOptions {
	out := opts
	if opts.Metadata != nil {
		out.Metadata = make(map[string]string, len(opts.Metadata))
		for k, v := range opts.Metadata {
			out.Metadata[k] = v
		}
	}
	if opts.AllowedRoles != nil {
		out.AllowedRoles = append([]string(nil), opts.AllowedRoles...)
	}
	return out
}

func hasAnyRole(userRoles []string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(userRoles))
	for _, role := range userRoles {
		set[role] = struct{}{}
	}
	for _, role := range allowed {
		if _, ok := set[role]; ok {
			return true
		}
	}
	return false
}
