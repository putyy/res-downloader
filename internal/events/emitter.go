package events

import (
	"encoding/json"
	"sync"
)

// EmitFunc delivers a named event to the frontend.
type EmitFunc func(name string, data interface{})

type Emitter struct {
	mu   sync.RWMutex
	emit EmitFunc
}

// New returns an emitter without a transport. Events emitted before SetEmitter
// is called are discarded.
func New() *Emitter { return &Emitter{} }

// SetEmitter installs the transport used to deliver events. Until it is set,
// events are dropped so startup can publish state before the native
// application exists.
func (e *Emitter) SetEmitter(emit EmitFunc) {
	e.mu.Lock()
	e.emit = emit
	e.mu.Unlock()
}

// Emit encodes {"type", "data"} as JSON and hands it to the transport under the
// single "event" channel. It returns the marshalling error; a missing transport
// is not an error.
func (e *Emitter) Emit(eventType string, data interface{}) error {
	raw, err := json.Marshal(map[string]interface{}{"type": eventType, "data": data})
	if err != nil {
		return err
	}
	e.mu.RLock()
	emit := e.emit
	e.mu.RUnlock()
	if emit != nil {
		emit("event", string(raw))
	}
	return nil
}
