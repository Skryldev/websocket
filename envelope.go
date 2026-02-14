package wsx

import "encoding/json"

type Envelope[T any] struct {
    Topic string `json:"topic"`
    Event string `json:"event"`
    Data  T      `json:"data"`
}

type RawEnvelope struct {
    Topic string          `json:"topic"`
    Event string          `json:"event"`
    Data  json.RawMessage `json:"data"`
}