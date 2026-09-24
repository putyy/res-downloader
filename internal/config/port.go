package config

import (
	"encoding/json"
	"errors"
	"strconv"
)

// SavePortForRestart changes only the listen port. Startup recovery must not
// apply settings hooks or validate unrelated legacy settings before restarting.
func (c *Config) SavePortForRestart(port string) error {
	value, err := strconv.Atoi(port)
	if err != nil || value <= 1024 || value >= 65535 || !decimalDigits(port) {
		return &ValidationError{Field: "Port", Err: errors.New("listen port must be between 1025 and 65534")}
	}
	if c.state == nil || c.storage == nil {
		return errors.New("config storage is unavailable")
	}
	c.state.applyMu.Lock()
	defer c.state.applyMu.Unlock()
	snapshot := c.Snapshot()
	snapshot.Port = port
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if err := c.storage.Store(data); err != nil {
		return err
	}
	c.replace(snapshot)
	return nil
}
