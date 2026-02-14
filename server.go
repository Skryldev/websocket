package wsx

import (
    "context"
    "github.com/gorilla/websocket"
    "net/http"
)

type Server struct {

    hub *Hub

    upgrader websocket.Upgrader
}

func NewServer(ctx context.Context, workers int) *Server {

    return &Server{

        hub: NewHub(ctx, workers),

        upgrader: websocket.Upgrader{

            CheckOrigin: func(r *http.Request) bool {
                return true
            },
        },
    }
}

func (s *Server) Handle(topic string, handler HandlerFunc) {

    s.hub.router.Handle(topic, handler)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

    ws, err := s.upgrader.Upgrade(w, r, nil)

    if err != nil {
        return
    }

    conn := newConn(ws, s.hub)

    s.hub.add <- conn
}
