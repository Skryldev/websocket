package wsx

import "errors"

var (
	ErrConnectionClosed  = errors.New("connection closed")
	ErrHandlerNotFound   = errors.New("handler not found")
	ErrRateLimited       = errors.New("rate limited")
	ErrPoolClosed        = errors.New("worker pool closed")
	ErrPoolFull          = errors.New("worker pool queue full")
	ErrSendQueueFull     = errors.New("send queue full")
	ErrSlowConsumer      = errors.New("slow consumer")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidPayload    = errors.New("invalid payload")
	ErrRoomNotFound      = errors.New("room not found")
	ErrRoomCapacity      = errors.New("room capacity reached")
	ErrRoomJoinForbidden = errors.New("room join forbidden")
	ErrAckTimeout        = errors.New("ack timeout")
	ErrHubShuttingDown   = errors.New("hub is shutting down")
)
