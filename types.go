package wsx

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type ConnID string

type UserID string

type RoomID string

type DropPolicy string

const (
	DropNewest        DropPolicy = "drop_newest"
	DropOldest        DropPolicy = "drop_oldest"
	DropAndDisconnect DropPolicy = "drop_and_disconnect"
)

type DisconnectReason string

const (
	DisconnectNormal        DisconnectReason = "normal"
	DisconnectReadError     DisconnectReason = "read_error"
	DisconnectWriteError    DisconnectReason = "write_error"
	DisconnectSlowConsumer  DisconnectReason = "slow_consumer"
	DisconnectServerStop    DisconnectReason = "server_shutdown"
	DisconnectAuthFailed    DisconnectReason = "auth_failed"
	DisconnectPolicyViolate DisconnectReason = "policy_violation"
)

type RoomPolicy string

const (
	RoomPolicyPublic     RoomPolicy = "public"
	RoomPolicyPrivate    RoomPolicy = "private"
	RoomPolicyInviteOnly RoomPolicy = "invite_only"
)

type RoomRole string

const (
	RoomRoleMember RoomRole = "member"
	RoomRoleAdmin  RoomRole = "admin"
)

type ConnMeta struct {
	UserID     UserID            `json:"user_id,omitempty"`
	DeviceID   string            `json:"device_id,omitempty"`
	Roles      []string          `json:"roles,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	IP         string            `json:"ip,omitempty"`
	UserAgent  string            `json:"user_agent,omitempty"`
}

type PresenceState struct {
	Online          bool                 `json:"online"`
	Connections     int                  `json:"connections"`
	LastSeen        time.Time            `json:"last_seen"`
	TypingByRoom    map[RoomID]bool      `json:"typing_by_room,omitempty"`
	TypingExpiresAt map[RoomID]time.Time `json:"typing_expires_at,omitempty"`
}

type RoomOptions struct {
	Metadata     map[string]string
	Policy       RoomPolicy
	Capacity     int
	AllowedRoles []string
	AutoCleanup  bool
}

type JoinOptions struct {
	Role      RoomRole
	InvitedBy UserID
	Force     bool
}

type OutboundMessage struct {
	ID         string
	Topic      string
	Event      string
	Namespace  string
	Version    string
	Data       any
	Headers    map[string]string
	RequireAck bool
}

type SendOptions struct {
	RequireAck   bool
	AckTimeout   time.Duration
	RetryMax     int
	RetryBackoff time.Duration
	Ordered      bool
	OrderingKey  string
}

type ConnFilter func(meta ConnMeta) bool

type DeliveryStatus string

const (
	DeliveryOK      DeliveryStatus = "ok"
	DeliveryDropped DeliveryStatus = "dropped"
	DeliveryFailed  DeliveryStatus = "failed"
)

type DeliveryReport struct {
	Target ConnID
	Status DeliveryStatus
	Error  string
}

type BatchDeliveryReport struct {
	Total   int
	Success int
	Dropped int
	Failed  int
	Reports []DeliveryReport
}

type AckStatus string

const (
	AckStatusOK   AckStatus = "ok"
	AckStatusNACK AckStatus = "nack"
)

type AckPayload struct {
	MessageID string    `json:"message_id"`
	Status    AckStatus `json:"status"`
	Error     string    `json:"error,omitempty"`
}

type AckOptions struct {
	Timeout    time.Duration
	RequireAll bool
}

type AckResult struct {
	Expected int
	Received int
	Nacked   int
	TimedOut bool
	Errors   []string
}

type Target struct {
	ConnID ConnID
	UserID UserID
	RoomID RoomID
}

type QueueConfig struct {
	Size                  int
	DropPolicy            DropPolicy
	SlowConsumerThreshold float64
}

type HeartbeatConfig struct {
	Interval    time.Duration
	PongTimeout time.Duration
	WriteWait   time.Duration
	ReadLimit   int64
}

type HubStats struct {
	Connections int
	Users       int
	Rooms       int
	InMessages  uint64
	OutMessages uint64
	Errors      uint64
	Drops       uint64
}

type Authenticator interface {
	Authenticate(r *http.Request) (ConnMeta, error)
}

type AccessController interface {
	CanSubscribe(ctx context.Context, meta ConnMeta, topic string) bool
	CanPublish(ctx context.Context, meta ConnMeta, topic string, event string) bool
}

type Validator interface {
	Validate(topic string, event string, data json.RawMessage) error
}

type OnConnectHook func(ctx context.Context, c *Conn, meta ConnMeta) error

type OnDisconnectHook func(ctx context.Context, c *Conn, reason DisconnectReason)

type Metrics interface {
	IncConnections(delta int)
	IncMessagesIn(topic string)
	IncMessagesOut(topic string)
	IncErrors(kind string)
	IncDropped(reason string)
	ObserveHandlerLatency(topic string, d time.Duration)
}

type PubSub interface {
	Publish(ctx context.Context, channel string, payload []byte) error
	Subscribe(ctx context.Context, channel string, handler func([]byte)) (ioCloser, error)
}

type ioCloser interface {
	Close() error
}
