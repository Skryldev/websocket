package wsx

import "sync"

// ConnRegistry مسئول مدیریت تمام connection های Hub است
// این رجیستری ایندکس user/tag را هم نگه می‌دارد تا fanout سریع باشد.
type ConnRegistry struct {
	mu     sync.RWMutex
	byID   map[ConnID]*Conn
	byUser map[UserID]map[ConnID]*Conn
	byTag  map[string]map[ConnID]*Conn
}

func NewConnRegistry() *ConnRegistry {
	return &ConnRegistry{
		byID:   make(map[ConnID]*Conn),
		byUser: make(map[UserID]map[ConnID]*Conn),
		byTag:  make(map[string]map[ConnID]*Conn),
	}
}

// اضافه کردن connection به registry
func (r *ConnRegistry) Add(c *Conn) {
	if c == nil {
		return
	}

	id := c.ID()
	meta := c.Meta()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.byID[id] = c
	r.addIndexesLocked(c, meta)
}

// حذف connection از registry
func (r *ConnRegistry) Remove(c *Conn) {
	if c == nil {
		return
	}

	id := c.ID()
	meta := c.Meta()

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.byID, id)
	r.removeIndexesLocked(id, meta)
}

func (r *ConnRegistry) Reindex(c *Conn, oldMeta ConnMeta, newMeta ConnMeta) {
	if c == nil {
		return
	}
	id := c.ID()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.removeIndexesLocked(id, oldMeta)
	r.addIndexesLocked(c, newMeta)
}

func (r *ConnRegistry) ByID(id ConnID) (*Conn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.byID[id]
	return c, ok
}

func (r *ConnRegistry) ByUser(userID UserID) []*Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userConns, ok := r.byUser[userID]
	if !ok {
		return nil
	}

	out := make([]*Conn, 0, len(userConns))
	for _, c := range userConns {
		out = append(out, c)
	}
	return out
}

func (r *ConnRegistry) ByTag(tag string) []*Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tagConns, ok := r.byTag[tag]
	if !ok {
		return nil
	}

	out := make([]*Conn, 0, len(tagConns))
	for _, c := range tagConns {
		out = append(out, c)
	}
	return out
}

func (r *ConnRegistry) ConnIDsByTag(tag string) []ConnID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tagConns, ok := r.byTag[tag]
	if !ok {
		return nil
	}

	out := make([]ConnID, 0, len(tagConns))
	for id := range tagConns {
		out = append(out, id)
	}
	return out
}

// دریافت همه connection ها (کپی شده برای ایمنی)
func (r *ConnRegistry) All() []*Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conns := make([]*Conn, 0, len(r.byID))
	for _, c := range r.byID {
		conns = append(conns, c)
	}
	return conns
}

// اجرا روی تمام connection ها (safer)
func (r *ConnRegistry) ForEach(fn func(c *Conn)) {
	for _, c := range r.All() {
		fn(c)
	}
}

func (r *ConnRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

func (r *ConnRegistry) UserCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byUser)
}

func (r *ConnRegistry) addIndexesLocked(c *Conn, meta ConnMeta) {
	id := c.ID()

	if meta.UserID != "" {
		if _, ok := r.byUser[meta.UserID]; !ok {
			r.byUser[meta.UserID] = make(map[ConnID]*Conn)
		}
		r.byUser[meta.UserID][id] = c
	}

	for _, tag := range uniqueStrings(meta.Tags) {
		if tag == "" {
			continue
		}
		if _, ok := r.byTag[tag]; !ok {
			r.byTag[tag] = make(map[ConnID]*Conn)
		}
		r.byTag[tag][id] = c
	}
}

func (r *ConnRegistry) removeIndexesLocked(connID ConnID, meta ConnMeta) {
	if meta.UserID != "" {
		if userConns, ok := r.byUser[meta.UserID]; ok {
			delete(userConns, connID)
			if len(userConns) == 0 {
				delete(r.byUser, meta.UserID)
			}
		}
	}

	for _, tag := range uniqueStrings(meta.Tags) {
		if tagConns, ok := r.byTag[tag]; ok {
			delete(tagConns, connID)
			if len(tagConns) == 0 {
				delete(r.byTag, tag)
			}
		}
	}
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
