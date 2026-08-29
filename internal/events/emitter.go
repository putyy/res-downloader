package events

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Emitter struct {
	mu  sync.RWMutex
	ctx context.Context
}

func New() *Emitter { return &Emitter{} }

func (e *Emitter) SetContext(ctx context.Context) {
	e.mu.Lock()
	e.ctx = ctx
	e.mu.Unlock()
}

func (e *Emitter) Emit(eventType string, data interface{}) error {
	raw, err := json.Marshal(map[string]interface{}{"type": eventType, "data": data})
	if err != nil {
		return err
	}
	e.mu.RLock()
	ctx := e.ctx
	e.mu.RUnlock()
	if ctx != nil {
		runtime.EventsEmit(ctx, "event", string(raw))
	}
	return nil
}
