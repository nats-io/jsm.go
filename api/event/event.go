package event

import (
	"fmt"
	"strings"
	"time"
)

type NATSEvent struct {
	Type string    `json:"type"`
	ID   string    `json:"id"`
	Time time.Time `json:"timestamp"`
}

func (e NATSEvent) EventType() string {
	return e.Type
}

func (e NATSEvent) EventID() string {
	return e.ID
}

func (e NATSEvent) EventTime() time.Time {
	return e.Time
}

func (e NATSEvent) EventSubject() string {
	parts := strings.Split(e.Type, ".")
	return parts[3]
}

func (e NATSEvent) EventSource() string {
	parts := strings.Split(e.Type, ".")
	return fmt.Sprintf("urn:nats:%s", parts[2])
}
