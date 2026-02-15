package wsx

import (
	"fmt"
	"log"
	"time"
)

type Logger interface {
	Info(msg string)
	Error(msg string)
}

type noopLogger struct{}

func (noopLogger) Info(string)  {}
func (noopLogger) Error(string) {}

type stdLogger struct{}

func (stdLogger) Info(msg string) {
	log.Println("[wsx][INFO]", msg)
}

func (stdLogger) Error(msg string) {
	log.Println("[wsx][ERROR]", msg)
}

func formatLog(msg string, kv ...any) string {
	if len(kv) == 0 {
		return msg
	}
	return fmt.Sprintf("%s %v", msg, kv)
}

type noopMetrics struct{}

func (noopMetrics) IncConnections(int)                          {}
func (noopMetrics) IncMessagesIn(string)                        {}
func (noopMetrics) IncMessagesOut(string)                       {}
func (noopMetrics) IncErrors(string)                            {}
func (noopMetrics) IncDropped(string)                           {}
func (noopMetrics) ObserveHandlerLatency(string, time.Duration) {}
