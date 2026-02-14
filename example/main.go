package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/Skryldev/websocket"
	ginwsx "github.com/Skryldev/websocket/gin"
	"github.com/gin-gonic/gin"
)

type ChatMessage struct {
	Room string `json:"room"`
	User string `json:"user"`
	Text string `json:"text"`
}

type JoinMessage struct {
	Room string `json:"room"`
	User string `json:"user"`
}

func main() {

	ctx := context.Background()

	// ایجاد سرور WSX با 8 worker
	server := wsx.NewServer(ctx, 8)

	// =========================
	// JOIN ROOM
	// =========================

	server.Handle("chat:join", func(ctx *wsx.Context, msg wsx.RawEnvelope) error {

		var join JoinMessage

		if err := json.Unmarshal(msg.Data, &join); err != nil {
			return err
		}

		log.Printf("%s joined %s", join.User, join.Room)

		// اطلاع به کل کاربران
		ctx.Hub.Broadcast("chat:"+join.Room, "user_joined", join)

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

		log.Printf("[%s] %s: %s", chat.Room, chat.User, chat.Text)

		// broadcast به room
		ctx.Hub.Broadcast("chat:"+chat.Room, "message", chat)

		return nil
	})

	// =========================
	// HTTP + WS
	// =========================
    gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/ws", ginwsx.Handler(server))

	log.Println("Server started on :8080")

	r.Run(":8080")
}
