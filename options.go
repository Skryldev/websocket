package wsx

import (
	"net/http"
	"time"
)

type Option func(*Server)

func defaultQueueConfig() QueueConfig {
	return QueueConfig{
		Size:                  256,
		DropPolicy:            DropNewest,
		SlowConsumerThreshold: 0.10,
	}
}

func defaultHeartbeatConfig() HeartbeatConfig {
	return HeartbeatConfig{
		Interval:    30 * time.Second,
		PongTimeout: 60 * time.Second,
		WriteWait:   10 * time.Second,
		ReadLimit:   1024 * 1024,
	}
}

func WithCheckOrigin(check func(r *http.Request) bool) Option {
	return func(s *Server) {
		if check != nil {
			s.upgrader.CheckOrigin = check
		}
	}
}

func WithBufferSizes(read, write int) Option {
	return func(s *Server) {
		if read > 0 {
			s.upgrader.ReadBufferSize = read
		}
		if write > 0 {
			s.upgrader.WriteBufferSize = write
		}
	}
}

func WithAuthenticator(a Authenticator) Option {
	return func(s *Server) {
		s.hub.authenticator = a
	}
}

func WithAccessController(ac AccessController) Option {
	return func(s *Server) {
		s.hub.access = ac
	}
}

func WithValidator(v Validator) Option {
	return func(s *Server) {
		s.hub.validator = v
	}
}

func WithLogger(l Logger) Option {
	return func(s *Server) {
		if l != nil {
			s.hub.logger = l
		}
	}
}

func WithMetrics(m Metrics) Option {
	return func(s *Server) {
		if m != nil {
			s.hub.metrics = m
		}
	}
}

func WithDebug(enabled bool) Option {
	return func(s *Server) {
		s.hub.debug = enabled
	}
}

func WithQueueConfig(cfg QueueConfig) Option {
	return func(s *Server) {
		if cfg.Size <= 0 {
			cfg.Size = defaultQueueConfig().Size
		}
		if cfg.DropPolicy == "" {
			cfg.DropPolicy = defaultQueueConfig().DropPolicy
		}
		s.hub.queueConfig = cfg
	}
}

func WithHeartbeat(cfg HeartbeatConfig) Option {
	return func(s *Server) {
		if cfg.Interval <= 0 {
			cfg.Interval = defaultHeartbeatConfig().Interval
		}
		if cfg.PongTimeout <= 0 {
			cfg.PongTimeout = defaultHeartbeatConfig().PongTimeout
		}
		if cfg.WriteWait <= 0 {
			cfg.WriteWait = defaultHeartbeatConfig().WriteWait
		}
		if cfg.ReadLimit <= 0 {
			cfg.ReadLimit = defaultHeartbeatConfig().ReadLimit
		}
		s.hub.heartbeatConfig = cfg
	}
}

func WithMaxMessageSize(size int64) Option {
	return func(s *Server) {
		if size > 0 {
			s.hub.maxMessageSize = size
		}
	}
}

func WithMiddleware(mw ...Middleware) Option {
	return func(s *Server) {
		s.hub.Use(mw...)
	}
}

func WithPubSub(bus PubSub, nodeID string) Option {
	return func(s *Server) {
		s.hub.pubsub = bus
		if nodeID != "" {
			s.hub.nodeID = nodeID
		}
	}
}

func WithClusterChannel(channel string) Option {
	return func(s *Server) {
		if channel != "" {
			s.hub.clusterChannel = channel
		}
	}
}
