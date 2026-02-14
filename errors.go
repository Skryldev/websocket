package wsx

import "errors"

var (
    ErrConnectionClosed = errors.New("connection closed")
    ErrHandlerNotFound  = errors.New("handler not found")
    ErrRateLimited      = errors.New("rate limited")
)
