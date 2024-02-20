package events

import (
	"log"
	"time"
)

type Event struct {
	Timestamp time.Time
	Message   string
}

type Emitter struct{}

func (e *Emitter) Emit(event *Event) {
	log.Printf("Emitted: %v", event)
}
