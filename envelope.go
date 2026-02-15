package wsx

import "encoding/json"

type Envelope[T any] struct {
	ID        string            `json:"id,omitempty"`
	Ref       string            `json:"ref,omitempty"`
	Topic     string            `json:"topic"`
	Event     string            `json:"event"`
	Namespace string            `json:"namespace,omitempty"`
	Version   string            `json:"version,omitempty"`
	Data      T                 `json:"data"`
	Headers   map[string]string `json:"headers,omitempty"`
	Ack       bool              `json:"ack,omitempty"`
}

type RawEnvelope struct {
	ID        string            `json:"id,omitempty"`
	Ref       string            `json:"ref,omitempty"`
	Topic     string            `json:"topic"`
	Event     string            `json:"event"`
	Namespace string            `json:"namespace,omitempty"`
	Version   string            `json:"version,omitempty"`
	Data      json.RawMessage   `json:"data"`
	Headers   map[string]string `json:"headers,omitempty"`
	Ack       bool              `json:"ack,omitempty"`
}
