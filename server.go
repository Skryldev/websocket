package wsx

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

type Server struct {
	hub *Hub

	upgrader websocket.Upgrader
}

func NewServer(ctx context.Context, workers int, opts ...Option) *Server {
	s := &Server{
		hub: NewHub(ctx, workers),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
				// برای backward compatibility
				return true
			},
		},
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(s)
	}

	if s.hub.pubsub != nil {
		if err := s.hub.startCluster(); err != nil {
			s.hub.logger.Error(formatLog("cluster subscribe failed", "err", err.Error()))
		}
	}

	if s.hub.maxMessageSize <= 0 {
		s.hub.maxMessageSize = defaultHeartbeatConfig().ReadLimit
	}

	return s
}

func (s *Server) Hub() *Hub {
	return s.hub
}

func (s *Server) Use(mw ...Middleware) {
	s.hub.Use(mw...)
}

func (s *Server) Handle(topic string, handler HandlerFunc) {
	s.hub.Handle(topic, handler)
}

func (s *Server) HandleWith(topic string, handler HandlerFunc, mw ...Middleware) {
	s.hub.HandleWith(topic, handler, mw...)
}

func (s *Server) HandleVersioned(topic string, version string, handler HandlerFunc, mw ...Middleware) {
	s.hub.HandleVersioned(topic, version, handler, mw...)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.hub.GracefulShutdown(ctx)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.hub.shuttingDown.Load() {
		http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		return
	}

	meta := ConnMeta{
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
		Attributes: map[string]string{
			"path":   r.URL.Path,
			"method": r.Method,
		},
	}

	if s.hub.authenticator != nil {
		authMeta, err := s.hub.authenticator.Authenticate(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		meta = mergeMeta(meta, authMeta)
	}

	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	_ = newConn(ws, s.hub, meta)
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func mergeMeta(base ConnMeta, incoming ConnMeta) ConnMeta {
	out := cloneConnMeta(base)
	if incoming.UserID != "" {
		out.UserID = incoming.UserID
	}
	if incoming.DeviceID != "" {
		out.DeviceID = incoming.DeviceID
	}
	if len(incoming.Roles) > 0 {
		out.Roles = append([]string(nil), incoming.Roles...)
	}
	if len(incoming.Tags) > 0 {
		out.Tags = append([]string(nil), incoming.Tags...)
	}
	if out.Attributes == nil {
		out.Attributes = map[string]string{}
	}
	for k, v := range incoming.Attributes {
		out.Attributes[k] = v
	}
	if incoming.IP != "" {
		out.IP = incoming.IP
	}
	if incoming.UserAgent != "" {
		out.UserAgent = incoming.UserAgent
	}
	return out
}
