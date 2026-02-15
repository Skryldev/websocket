package main

import (
	"context"
	"encoding/json"
	"log"

	wsx "github.com/Skryldev/websocket"
	ginwsx "github.com/Skryldev/websocket/gin"
	"github.com/gin-gonic/gin"
)

// مدل پیام چت
type ChatMessage struct {
	Room string `json:"room"`
	User string `json:"user"`
	Text string `json:"text"`
}

// مدل join/leave
type JoinMessage struct {
	Room string `json:"room"`
	User string `json:"user"`
}

func main() {

	ctx := context.Background()

	// ایجاد Server با 8 worker
	server := wsx.NewServer(ctx, 8)

	// =========================
	// JOIN ROOM
	// =========================
	server.Handle("chat:join", func(ctx *wsx.Context, msg wsx.RawEnvelope) error {
		var join JoinMessage
		if err := json.Unmarshal(msg.Data, &join); err != nil {
			return err
		}

		log.Printf("[JOIN] %s -> %s", join.User, join.Room)

		// اتصال کاربر به room
		ctx.Hub.Join(join.Room, ctx.Conn)

		// اطلاع سایر اعضای room
		ctx.Hub.BroadcastRoom(join.Room, "chat:"+join.Room, "user_joined", join)

		return nil
	})

	// =========================
	// LEAVE ROOM
	// =========================
	server.Handle("chat:leave", func(ctx *wsx.Context, msg wsx.RawEnvelope) error {
		var leave JoinMessage
		if err := json.Unmarshal(msg.Data, &leave); err != nil {
			return err
		}

		log.Printf("[LEAVE] %s <- %s", leave.User, leave.Room)

		ctx.Hub.Leave(leave.Room, ctx.Conn)

		ctx.Hub.BroadcastRoom(leave.Room, "chat:"+leave.Room, "user_left", leave)

		return nil
	})

	// =========================
	// SEND MESSAGE
	// =========================
	server.Handle("chat:*", func(ctx *wsx.Context, msg wsx.RawEnvelope) error {

		if msg.Event != "message" {
			return nil
		}

		var chat ChatMessage
		if err := json.Unmarshal(msg.Data, &chat); err != nil {
			return err
		}

		log.Printf("[MSG][%s] %s: %s", chat.Room, chat.User, chat.Text)

		ctx.Hub.BroadcastRoom(chat.Room, "chat:"+chat.Room, "message", chat)

		return nil
	})

	// =========================
	// HTTP + WebSocket
	// =========================
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// WebSocket endpoint
	r.GET("/ws", ginwsx.Handler(server))

	log.Println("Server started on :8080")
	r.Run(":8080")
}
