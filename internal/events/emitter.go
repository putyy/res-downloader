package events

import (
	"encoding/json"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Emitter struct {
	mu  sync.RWMutex
	app *application.App
}

func New() *Emitter { return &Emitter{} }

func (e *Emitter) SetApplication(app *application.App) {
	e.mu.Lock()
	e.app = app
	e.mu.Unlock()
}

func (e *Emitter) Emit(eventType string, data interface{}) error {
	raw, err := json.Marshal(map[string]interface{}{"type": eventType, "data": data})
	if err != nil {
		return err
	}
	e.mu.RLock()
	app := e.app
	e.mu.RUnlock()
	if app != nil {
		app.Event.Emit("event", string(raw))
	}
	return nil
}
