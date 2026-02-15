package wsx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var messageSeq atomic.Uint64

type presenceEntry struct {
	connections int
	lastSeen    time.Time
}

type clusterEnvelope struct {
	NodeID    string            `json:"node_id"`
	Kind      string            `json:"kind"`
	RoomID    RoomID            `json:"room_id,omitempty"`
	UserID    UserID            `json:"user_id,omitempty"`
	MessageID string            `json:"message_id,omitempty"`
	Topic     string            `json:"topic"`
	Event     string            `json:"event"`
	Namespace string            `json:"namespace,omitempty"`
	Version   string            `json:"version,omitempty"`
	Data      json.RawMessage   `json:"data"`
	Headers   map[string]string `json:"headers,omitempty"`
	Except    []ConnID          `json:"except,omitempty"`
}

// Hub مسئول orchestration کل سیستم WebSocket است
// Hub thread-safe است و APIهای legacy را حفظ می‌کند.
type Hub struct {
	ctx    context.Context
	cancel context.CancelFunc

	router   *Router
	registry *ConnRegistry
	rooms    *RoomManager
	pool     *WorkerPool

	queueConfig     QueueConfig
	heartbeatConfig HeartbeatConfig
	maxMessageSize  int64

	logger  Logger
	metrics Metrics
	debug   bool

	authenticator Authenticator
	access        AccessController
	validator     Validator

	middlewaresMu sync.RWMutex
	middlewares   []Middleware

	hooksMu           sync.RWMutex
	onConnectHooks    []OnConnectHook
	onDisconnectHooks []OnDisconnectHook

	pendingAckMu sync.Mutex
	pendingAck   map[string]chan AckPayload

	presenceMu sync.RWMutex
	presence   map[UserID]*presenceEntry
	typing     map[UserID]map[RoomID]time.Time

	inMessages  atomic.Uint64
	outMessages atomic.Uint64
	errorsCount atomic.Uint64
	drops       atomic.Uint64

	shuttingDown atomic.Bool
	shutdownOnce sync.Once

	pubsub         PubSub
	nodeID         string
	subCloser      ioCloser
	clusterChannel string
	clusterOnce    sync.Once
}

// سازنده Hub
func NewHub(ctx context.Context, workers int) *Hub {
	if ctx == nil {
		ctx = context.Background()
	}

	hctx, cancel := context.WithCancel(ctx)

	h := &Hub{
		ctx:             hctx,
		cancel:          cancel,
		router:          NewRouter(),
		registry:        NewConnRegistry(),
		rooms:           NewRoomManager(),
		pool:            NewWorkerPoolWithQueue(workers, 2048),
		queueConfig:     defaultQueueConfig(),
		heartbeatConfig: defaultHeartbeatConfig(),
		maxMessageSize:  defaultHeartbeatConfig().ReadLimit,
		logger:          stdLogger{},
		metrics:         noopMetrics{},
		pendingAck:      make(map[string]chan AckPayload),
		presence:        make(map[UserID]*presenceEntry),
		typing:          make(map[UserID]map[RoomID]time.Time),
		nodeID:          "local",
		clusterChannel:  "wsx.cluster",
	}

	h.Use(RecoverMiddleware(h.logger))

	go func() {
		<-hctx.Done()
		_ = h.GracefulShutdown(context.Background())
	}()

	return h
}

func (h *Hub) startCluster() error {
	if h.pubsub == nil {
		return nil
	}

	var subscribeErr error
	h.clusterOnce.Do(func() {
		closer, err := h.pubsub.Subscribe(h.ctx, h.clusterChannel, h.handleClusterPayload)
		if err != nil {
			subscribeErr = err
			return
		}
		h.subCloser = closer
	})

	return subscribeErr
}

func (h *Hub) handleClusterPayload(payload []byte) {
	var env clusterEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		h.metrics.IncErrors("cluster_unmarshal")
		return
	}
	if env.NodeID == h.nodeID {
		return
	}

	msg := OutboundMessage{
		ID:        env.MessageID,
		Topic:     env.Topic,
		Event:     env.Event,
		Namespace: env.Namespace,
		Version:   env.Version,
		Data:      env.Data,
		Headers:   env.Headers,
	}

	switch env.Kind {
	case "broadcast":
		_, _ = h.broadcastLocal(h.ctx, msg, nil, SendOptions{})
	case "room":
		_, _ = h.emitRoomLocal(h.ctx, env.RoomID, env.Except, msg, SendOptions{})
	case "user":
		_, _ = h.sendToUserLocal(h.ctx, env.UserID, msg, SendOptions{})
	}
}

func (h *Hub) publishCluster(kind string, roomID RoomID, userID UserID, except []ConnID, msg OutboundMessage) {
	if h.pubsub == nil || h.clusterChannel == "" {
		return
	}

	data, err := json.Marshal(msg.Data)
	if err != nil {
		h.metrics.IncErrors("cluster_data_marshal")
		return
	}

	env := clusterEnvelope{
		NodeID:    h.nodeID,
		Kind:      kind,
		RoomID:    roomID,
		UserID:    userID,
		MessageID: msg.ID,
		Topic:     msg.Topic,
		Event:     msg.Event,
		Namespace: msg.Namespace,
		Version:   msg.Version,
		Data:      data,
		Headers:   msg.Headers,
		Except:    except,
	}

	payload, err := json.Marshal(env)
	if err != nil {
		h.metrics.IncErrors("cluster_marshal")
		return
	}

	if err := h.pubsub.Publish(h.ctx, h.clusterChannel, payload); err != nil {
		h.metrics.IncErrors("cluster_publish")
	}
}

// =========================
// Lifecycle
// =========================

// ثبت connection جدید
func (h *Hub) Register(c *Conn) {
	if c == nil {
		return
	}
	if h.shuttingDown.Load() {
		c.close(DisconnectServerStop)
		return
	}

	h.registry.Add(c)
	h.metrics.IncConnections(1)
	h.updatePresenceOnConnect(c.Meta().UserID)

	h.hooksMu.RLock()
	hooks := append([]OnConnectHook(nil), h.onConnectHooks...)
	h.hooksMu.RUnlock()

	for _, hook := range hooks {
		hook := hook
		meta := c.Meta()
		if err := hook(h.ctx, c, meta); err != nil {
			h.logger.Error(formatLog("onConnect hook failed", "conn_id", c.ID(), "err", err.Error()))
			h.unregister(c, DisconnectPolicyViolate)
			c.close(DisconnectPolicyViolate)
			return
		}
	}
}

// حذف connection
func (h *Hub) Unregister(c *Conn) {
	h.unregister(c, DisconnectNormal)
}

func (h *Hub) unregister(c *Conn, reason DisconnectReason) {
	if c == nil {
		return
	}
	if !c.unregistered.CompareAndSwap(false, true) {
		return
	}

	h.registry.Remove(c)
	h.rooms.RemoveConn(c)
	h.metrics.IncConnections(-1)
	h.updatePresenceOnDisconnect(c.Meta().UserID)

	h.hooksMu.RLock()
	hooks := append([]OnDisconnectHook(nil), h.onDisconnectHooks...)
	h.hooksMu.RUnlock()

	for _, hook := range hooks {
		hook(h.ctx, c, reason)
	}
}

// =========================
// Lifecycle hooks
// =========================

func (h *Hub) OnConnect(hook OnConnectHook) {
	if hook == nil {
		return
	}
	h.hooksMu.Lock()
	defer h.hooksMu.Unlock()
	h.onConnectHooks = append(h.onConnectHooks, hook)
}

func (h *Hub) OnDisconnect(hook OnDisconnectHook) {
	if hook == nil {
		return
	}
	h.hooksMu.Lock()
	defer h.hooksMu.Unlock()
	h.onDisconnectHooks = append(h.onDisconnectHooks, hook)
}

// =========================
// Metadata / Presence
// =========================

func (h *Hub) SetConnMeta(connID ConnID, patch ConnMeta) error {
	conn, ok := h.registry.ByID(connID)
	if !ok {
		return ErrConnectionClosed
	}

	oldMeta := conn.Meta()
	conn.SetMeta(patch)
	h.registry.Reindex(conn, oldMeta, patch)

	if oldMeta.UserID != patch.UserID {
		h.updatePresenceOnDisconnect(oldMeta.UserID)
		h.updatePresenceOnConnect(patch.UserID)
	}
	return nil
}

func (h *Hub) GetConnMeta(connID ConnID) (ConnMeta, bool) {
	conn, ok := h.registry.ByID(connID)
	if !ok {
		return ConnMeta{}, false
	}
	return conn.Meta(), true
}

func (h *Hub) FindConnsByTag(tag string) []ConnID {
	return h.registry.ConnIDsByTag(tag)
}

func (h *Hub) UserConnections(userID UserID) []ConnID {
	conns := h.registry.ByUser(userID)
	out := make([]ConnID, 0, len(conns))
	for _, c := range conns {
		out = append(out, c.ID())
	}
	return out
}

func (h *Hub) Presence(userID UserID) (PresenceState, bool) {
	h.presenceMu.RLock()
	defer h.presenceMu.RUnlock()

	entry, ok := h.presence[userID]
	if !ok {
		return PresenceState{}, false
	}

	state := PresenceState{
		Online:          entry.connections > 0,
		Connections:     entry.connections,
		LastSeen:        entry.lastSeen,
		TypingByRoom:    make(map[RoomID]bool),
		TypingExpiresAt: make(map[RoomID]time.Time),
	}

	if rooms, ok := h.typing[userID]; ok {
		now := time.Now().UTC()
		for roomID, expires := range rooms {
			if expires.After(now) {
				state.TypingByRoom[roomID] = true
				state.TypingExpiresAt[roomID] = expires
			}
		}
	}

	return state, true
}

func (h *Hub) SetTyping(userID UserID, roomID RoomID, typing bool, ttl time.Duration) error {
	if userID == "" || roomID == "" {
		return ErrInvalidPayload
	}
	if ttl <= 0 {
		ttl = 5 * time.Second
	}

	h.presenceMu.Lock()
	defer h.presenceMu.Unlock()

	if _, ok := h.typing[userID]; !ok {
		h.typing[userID] = make(map[RoomID]time.Time)
	}

	if !typing {
		delete(h.typing[userID], roomID)
		if len(h.typing[userID]) == 0 {
			delete(h.typing, userID)
		}
		return nil
	}

	h.typing[userID][roomID] = time.Now().UTC().Add(ttl)
	return nil
}

func (h *Hub) updatePresenceOnConnect(userID UserID) {
	if userID == "" {
		return
	}
	h.presenceMu.Lock()
	defer h.presenceMu.Unlock()
	entry, ok := h.presence[userID]
	if !ok {
		entry = &presenceEntry{}
		h.presence[userID] = entry
	}
	entry.connections++
	entry.lastSeen = time.Now().UTC()
}

func (h *Hub) updatePresenceOnDisconnect(userID UserID) {
	if userID == "" {
		return
	}
	h.presenceMu.Lock()
	defer h.presenceMu.Unlock()
	entry, ok := h.presence[userID]
	if !ok {
		return
	}
	if entry.connections > 0 {
		entry.connections--
	}
	entry.lastSeen = time.Now().UTC()
}

// =========================
// Middleware / Routing
// =========================

func (h *Hub) Use(mw ...Middleware) {
	h.middlewaresMu.Lock()
	defer h.middlewaresMu.Unlock()
	h.middlewares = append(h.middlewares, mw...)
}

// ثبت handler
func (h *Hub) Handle(pattern string, handler HandlerFunc) {
	h.HandleWith(pattern, handler)
}

func (h *Hub) HandleWith(pattern string, handler HandlerFunc, mw ...Middleware) {
	h.HandleVersioned(pattern, "", handler, mw...)
}

func (h *Hub) HandleVersioned(pattern string, version string, handler HandlerFunc, mw ...Middleware) {
	wrapped := h.wrapHandler(handler, mw...)
	h.router.HandleVersion(pattern, version, wrapped)
}

func (h *Hub) wrapHandler(handler HandlerFunc, routeMW ...Middleware) HandlerFunc {
	if handler == nil {
		return nil
	}

	h.middlewaresMu.RLock()
	global := append([]Middleware(nil), h.middlewares...)
	h.middlewaresMu.RUnlock()

	chain := append(global, routeMW...)
	if len(chain) == 0 {
		return handler
	}
	return Chain(chain...)(handler)
}

// dispatch پیام به handler مناسب
func (h *Hub) Dispatch(c *Conn, msg []byte) {
	if h.shuttingDown.Load() {
		return
	}

	var env RawEnvelope
	if err := json.Unmarshal(msg, &env); err != nil {
		h.errorsCount.Add(1)
		h.metrics.IncErrors("invalid_json")
		return
	}

	h.inMessages.Add(1)
	h.metrics.IncMessagesIn(env.Topic)

	if env.Event == "ack" {
		h.handleAck(c, env)
		return
	}

	meta := c.Meta()
	if h.access != nil && !h.access.CanPublish(h.ctx, meta, env.Topic, env.Event) {
		h.errorsCount.Add(1)
		h.metrics.IncErrors("forbidden_publish")
		_ = c.sendEnvelope(Envelope[any]{
			Topic: env.Topic,
			Event: "error",
			Data:  map[string]any{"code": "forbidden"},
		})
		return
	}

	if h.validator != nil {
		if err := h.validator.Validate(env.Topic, env.Event, env.Data); err != nil {
			h.errorsCount.Add(1)
			h.metrics.IncErrors("validation")
			_ = c.sendEnvelope(Envelope[any]{
				Ref:   env.ID,
				Topic: env.Topic,
				Event: "error",
				Data:  map[string]any{"code": "validation", "error": err.Error()},
			})
			return
		}
	}

	handler := h.router.MatchVersion(env.Topic, env.Version)
	if handler == nil {
		h.errorsCount.Add(1)
		h.metrics.IncErrors("handler_not_found")
		if env.Ack {
			_ = c.sendEnvelope(Envelope[AckPayload]{
				Ref:   env.ID,
				Topic: env.Topic,
				Event: "ack",
				Data: AckPayload{
					MessageID: env.ID,
					Status:    AckStatusNACK,
					Error:     ErrHandlerNotFound.Error(),
				},
			})
		}
		return
	}

	ctx := &Context{
		Context:  h.ctx,
		Conn:     c,
		Hub:      h,
		Incoming: env,
	}

	err := h.pool.TrySubmit(func() {
		start := time.Now()
		err := handler(ctx, env)
		h.metrics.ObserveHandlerLatency(env.Topic, time.Since(start))
		if err != nil {
			h.errorsCount.Add(1)
			h.metrics.IncErrors("handler")
			h.logger.Error(formatLog("handler failed", "topic", env.Topic, "event", env.Event, "err", err.Error()))
		}

		if env.Ack {
			ack := AckPayload{
				MessageID: env.ID,
				Status:    AckStatusOK,
			}
			if err != nil {
				ack.Status = AckStatusNACK
				ack.Error = err.Error()
			}
			_ = c.sendEnvelope(Envelope[AckPayload]{
				Ref:   env.ID,
				Topic: env.Topic,
				Event: "ack",
				Data:  ack,
			})
		}
	})

	if err != nil {
		h.errorsCount.Add(1)
		h.metrics.IncErrors("pool")
		_ = c.sendEnvelope(Envelope[any]{
			Ref:   env.ID,
			Topic: env.Topic,
			Event: "error",
			Data:  map[string]any{"code": "overloaded"},
		})
	}
}

// =========================
// Rooms API (Facade)
// =========================

// Join کردن connection به room
func (h *Hub) Join(room string, c *Conn) {
	h.rooms.Join(room, c)
}

func (h *Hub) CreateRoom(ctx context.Context, roomID RoomID, opts RoomOptions) error {
	_ = ctx
	h.rooms.CreateRoom(roomID, opts)
	return nil
}

func (h *Hub) JoinWithOptions(ctx context.Context, connID ConnID, roomID RoomID, opts JoinOptions) error {
	_ = ctx
	conn, ok := h.registry.ByID(connID)
	if !ok {
		return ErrConnectionClosed
	}

	meta := conn.Meta()
	if h.access != nil && !h.access.CanSubscribe(h.ctx, meta, string(roomID)) {
		return ErrForbidden
	}

	return h.rooms.JoinWithOptions(roomID, conn, opts)
}

// Leave کردن connection از room
func (h *Hub) Leave(room string, c *Conn) {
	h.rooms.Leave(room, c)
}

// حذف کامل connection از همه room ها
func (h *Hub) LeaveAll(c *Conn) {
	h.rooms.RemoveConn(c)
}

func (h *Hub) KickUserFromRoom(ctx context.Context, userID UserID, roomID RoomID, reason string) (int, error) {
	_ = ctx
	_ = reason
	removed := h.rooms.KickUser(roomID, userID)
	return removed, nil
}

// =========================
// Messaging
// =========================

// ارسال به همه connection ها
func (h *Hub) Broadcast(topic string, event string, payload any) {
	msg := OutboundMessage{Topic: topic, Event: event, Data: payload}
	_, _ = h.broadcastLocal(h.ctx, msg, nil, SendOptions{})
	h.publishCluster("broadcast", "", "", nil, msg)
}

// ارسال به یک room خاص
func (h *Hub) BroadcastRoom(room string, topic string, event string, payload any) {
	msg := OutboundMessage{Topic: topic, Event: event, Data: payload}
	_, _ = h.emitRoomLocal(h.ctx, RoomID(room), nil, msg, SendOptions{})
	h.publishCluster("room", RoomID(room), "", nil, msg)
}

// ارسال فقط به یک connection
func (h *Hub) SendToConn(c *Conn, topic string, event string, payload any) {
	if c == nil {
		return
	}
	_, _ = h.sendToConn(h.ctx, c, OutboundMessage{Topic: topic, Event: event, Data: payload}, SendOptions{})
}

func (h *Hub) SendToUser(ctx context.Context, userID UserID, msg OutboundMessage, opts SendOptions) (BatchDeliveryReport, error) {
	report, err := h.sendToUserLocal(ctx, userID, msg, opts)
	if err == nil {
		h.publishCluster("user", "", userID, nil, msg)
	}
	return report, err
}

func (h *Hub) SendToUsers(ctx context.Context, userIDs []UserID, msg OutboundMessage, opts SendOptions) (BatchDeliveryReport, error) {
	connMap := make(map[ConnID]*Conn)
	for _, userID := range userIDs {
		for _, c := range h.registry.ByUser(userID) {
			connMap[c.ID()] = c
		}
	}

	conns := make([]*Conn, 0, len(connMap))
	for _, c := range connMap {
		conns = append(conns, c)
	}

	if len(conns) == 0 {
		return BatchDeliveryReport{}, ErrConnectionClosed
	}
	report, err := h.sendToConnList(ctx, conns, msg, opts)
	if err == nil {
		for _, userID := range userIDs {
			h.publishCluster("user", "", userID, nil, msg)
		}
	}
	return report, err
}

func (h *Hub) BroadcastWithFilter(ctx context.Context, msg OutboundMessage, filter ConnFilter, opts SendOptions) (int, error) {
	return h.broadcastLocal(ctx, msg, filter, opts)
}

func (h *Hub) EmitToRoomExcept(ctx context.Context, roomID RoomID, exceptConnIDs []ConnID, msg OutboundMessage, opts SendOptions) (int, error) {
	sent, err := h.emitRoomLocal(ctx, roomID, exceptConnIDs, msg, opts)
	if err == nil {
		h.publishCluster("room", roomID, "", exceptConnIDs, msg)
	}
	return sent, err
}

func (h *Hub) emitRoomLocal(ctx context.Context, roomID RoomID, exceptConnIDs []ConnID, msg OutboundMessage, opts SendOptions) (int, error) {
	excluded := make(map[ConnID]struct{}, len(exceptConnIDs))
	for _, id := range exceptConnIDs {
		excluded[id] = struct{}{}
	}

	members := h.rooms.Members(roomID)
	sent := 0
	for _, c := range members {
		if _, skip := excluded[c.ID()]; skip {
			continue
		}
		if _, err := h.sendToConn(ctx, c, msg, opts); err == nil {
			sent++
		}
	}
	return sent, nil
}

func (h *Hub) broadcastLocal(ctx context.Context, msg OutboundMessage, filter ConnFilter, opts SendOptions) (int, error) {
	conns := h.registry.All()
	sent := 0
	for _, c := range conns {
		if filter != nil && !filter(c.Meta()) {
			continue
		}
		if _, err := h.sendToConn(ctx, c, msg, opts); err == nil {
			sent++
		}
	}
	return sent, nil
}

func (h *Hub) sendToUserLocal(ctx context.Context, userID UserID, msg OutboundMessage, opts SendOptions) (BatchDeliveryReport, error) {
	conns := h.registry.ByUser(userID)
	if len(conns) == 0 {
		return BatchDeliveryReport{}, ErrConnectionClosed
	}
	return h.sendToConnList(ctx, conns, msg, opts)
}

func (h *Hub) EmitWithAck(ctx context.Context, target Target, msg OutboundMessage, opts AckOptions) (AckResult, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	if msg.ID == "" {
		msg.ID = h.nextMessageID()
	}
	msg.RequireAck = true

	conns := h.resolveTarget(target)
	if len(conns) == 0 {
		return AckResult{}, ErrConnectionClosed
	}

	res := AckResult{Expected: len(conns)}

	for _, c := range conns {
		_, err := h.sendToConn(ctx, c, msg, SendOptions{
			RequireAck: true,
			AckTimeout: opts.Timeout,
		})
		if err != nil {
			if errors.Is(err, ErrAckTimeout) {
				res.TimedOut = true
				res.Errors = append(res.Errors, err.Error())
				continue
			}
			if errors.Is(err, ErrForbidden) {
				res.Nacked++
				res.Errors = append(res.Errors, err.Error())
				continue
			}
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		res.Received++
	}

	if opts.RequireAll && res.Received != res.Expected {
		return res, ErrAckTimeout
	}
	if res.Received == 0 && len(res.Errors) > 0 {
		return res, ErrAckTimeout
	}
	return res, nil
}

func (h *Hub) sendToConnList(ctx context.Context, conns []*Conn, msg OutboundMessage, opts SendOptions) (BatchDeliveryReport, error) {
	report := BatchDeliveryReport{Total: len(conns)}
	if len(conns) == 0 {
		return report, nil
	}

	for _, c := range conns {
		dr, err := h.sendToConn(ctx, c, msg, opts)
		report.Reports = append(report.Reports, dr)
		if err != nil {
			report.Failed++
			continue
		}
		switch dr.Status {
		case DeliveryOK:
			report.Success++
		case DeliveryDropped:
			report.Dropped++
		default:
			report.Failed++
		}
	}

	if report.Success == 0 && report.Total > 0 {
		return report, ErrConnectionClosed
	}
	return report, nil
}

func (h *Hub) sendToConn(ctx context.Context, c *Conn, msg OutboundMessage, opts SendOptions) (DeliveryReport, error) {
	if c == nil {
		return DeliveryReport{Status: DeliveryFailed}, ErrConnectionClosed
	}
	if h.shuttingDown.Load() {
		return DeliveryReport{Target: c.ID(), Status: DeliveryFailed, Error: ErrHubShuttingDown.Error()}, ErrHubShuttingDown
	}

	ackRequired := msg.RequireAck || opts.RequireAck
	if msg.ID == "" {
		msg.ID = h.nextMessageID()
	}

	env := Envelope[any]{
		ID:        msg.ID,
		Topic:     msg.Topic,
		Event:     msg.Event,
		Namespace: msg.Namespace,
		Version:   msg.Version,
		Data:      msg.Data,
		Headers:   msg.Headers,
		Ack:       ackRequired,
	}

	retryMax := opts.RetryMax
	if retryMax < 0 {
		retryMax = 0
	}
	backoff := opts.RetryBackoff
	if backoff <= 0 {
		backoff = 50 * time.Millisecond
	}

	var lastErr error
	for attempt := 0; attempt <= retryMax; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff * time.Duration(attempt)):
			case <-ctx.Done():
				return DeliveryReport{Target: c.ID(), Status: DeliveryFailed, Error: ctx.Err().Error()}, ctx.Err()
			}
		}

		var ackCh chan AckPayload
		ackKey := h.pendingAckKey(msg.ID, c.ID())
		if ackRequired {
			ackCh = h.registerAckWaiter(ackKey)
		}

		err := c.sendEnvelope(env)
		if err != nil {
			lastErr = err
			if ackRequired {
				h.unregisterAckWaiter(ackKey)
			}
			if errors.Is(err, ErrSendQueueFull) {
				h.drops.Add(1)
			}
			continue
		}

		h.outMessages.Add(1)
		h.metrics.IncMessagesOut(msg.Topic)

		if !ackRequired {
			return DeliveryReport{Target: c.ID(), Status: DeliveryOK}, nil
		}

		timeout := opts.AckTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}

		select {
		case ack := <-ackCh:
			h.unregisterAckWaiter(ackKey)
			if ack.Status == AckStatusNACK {
				lastErr = errors.New(ack.Error)
				continue
			}
			return DeliveryReport{Target: c.ID(), Status: DeliveryOK}, nil

		case <-time.After(timeout):
			h.unregisterAckWaiter(ackKey)
			lastErr = ErrAckTimeout
			continue

		case <-ctx.Done():
			h.unregisterAckWaiter(ackKey)
			return DeliveryReport{Target: c.ID(), Status: DeliveryFailed, Error: ctx.Err().Error()}, ctx.Err()
		}
	}

	if lastErr == nil {
		lastErr = ErrConnectionClosed
	}

	status := DeliveryFailed
	if errors.Is(lastErr, ErrSendQueueFull) {
		status = DeliveryDropped
	}

	return DeliveryReport{Target: c.ID(), Status: status, Error: lastErr.Error()}, lastErr
}

func (h *Hub) resolveTarget(target Target) []*Conn {
	switch {
	case target.ConnID != "":
		if c, ok := h.registry.ByID(target.ConnID); ok {
			return []*Conn{c}
		}
		return nil

	case target.UserID != "":
		return h.registry.ByUser(target.UserID)

	case target.RoomID != "":
		return h.rooms.Members(target.RoomID)

	default:
		return nil
	}
}

func (h *Hub) pendingAckKey(messageID string, connID ConnID) string {
	return messageID + ":" + string(connID)
}

func (h *Hub) registerAckWaiter(key string) chan AckPayload {
	ch := make(chan AckPayload, 1)
	h.pendingAckMu.Lock()
	h.pendingAck[key] = ch
	h.pendingAckMu.Unlock()
	return ch
}

func (h *Hub) unregisterAckWaiter(key string) {
	h.pendingAckMu.Lock()
	delete(h.pendingAck, key)
	h.pendingAckMu.Unlock()
}

func (h *Hub) handleAck(c *Conn, env RawEnvelope) {
	var payload AckPayload
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			h.metrics.IncErrors("invalid_ack")
			return
		}
	}

	messageID := payload.MessageID
	if messageID == "" {
		messageID = env.Ref
	}
	if messageID == "" {
		return
	}

	key := h.pendingAckKey(messageID, c.ID())
	h.pendingAckMu.Lock()
	ch, ok := h.pendingAck[key]
	h.pendingAckMu.Unlock()
	if !ok {
		return
	}

	if payload.Status == "" {
		payload.Status = AckStatusOK
	}

	select {
	case ch <- payload:
	default:
	}
}

func (h *Hub) nextMessageID() string {
	seq := messageSeq.Add(1)
	return fmt.Sprintf("m-%d-%d", time.Now().UnixNano(), seq)
}

// =========================
// User Disconnect
// =========================

func (h *Hub) DisconnectUser(ctx context.Context, userID UserID, reason DisconnectReason) (int, error) {
	conns := h.registry.ByUser(userID)
	if len(conns) == 0 {
		return 0, nil
	}

	closed := 0
	for _, c := range conns {
		select {
		case <-ctx.Done():
			return closed, ctx.Err()
		default:
		}
		h.unregister(c, reason)
		c.close(reason)
		closed++
	}
	return closed, nil
}

func (h *Hub) DisconnectConn(ctx context.Context, connID ConnID, reason DisconnectReason) error {
	conn, ok := h.registry.ByID(connID)
	if !ok {
		return ErrConnectionClosed
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	h.unregister(conn, reason)
	conn.close(reason)
	return nil
}

// =========================
// Graceful shutdown
// =========================

func (h *Hub) GracefulShutdown(ctx context.Context) error {
	var shutdownErr error
	h.shutdownOnce.Do(func() {
		h.shuttingDown.Store(true)
		h.cancel()

		if h.subCloser != nil {
			_ = h.subCloser.Close()
		}

		conns := h.registry.All()
		for _, c := range conns {
			h.unregister(c, DisconnectServerStop)
			c.close(DisconnectServerStop)
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				if h.registry.Count() == 0 {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()

		select {
		case <-done:
		case <-ctx.Done():
			shutdownErr = ctx.Err()
		}

		h.pool.Shutdown()
	})

	return shutdownErr
}

func (h *Hub) Stats() HubStats {
	return HubStats{
		Connections: h.registry.Count(),
		Users:       h.registry.UserCount(),
		Rooms:       h.rooms.RoomCount(),
		InMessages:  h.inMessages.Load(),
		OutMessages: h.outMessages.Load(),
		Errors:      h.errorsCount.Load(),
		Drops:       h.drops.Load(),
	}
}
