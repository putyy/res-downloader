package config

import (
	"encoding/json"
	"errors"
)

const (
	DefaultWindowWidth  = 1280
	DefaultWindowHeight = 800
	MinWindowWidth      = 960
	MinWindowHeight     = 600
)

func (c *Config) WindowSize() (int, int) {
	snapshot := c.Snapshot()
	if snapshot.WindowWidth < MinWindowWidth || snapshot.WindowHeight < MinWindowHeight {
		return DefaultWindowWidth, DefaultWindowHeight
	}
	return snapshot.WindowWidth, snapshot.WindowHeight
}

// SaveWindowSize serializes with settings updates without reapplying unrelated
// proxy, download or interception settings.
func (c *Config) SaveWindowSize(width, height int) error {
	if width < MinWindowWidth || height < MinWindowHeight {
		return nil
	}
	if c.state == nil || c.storage == nil {
		return errors.New("config storage is unavailable")
	}
	c.state.applyMu.Lock()
	defer c.state.applyMu.Unlock()
	snapshot := c.Snapshot()
	if snapshot.WindowWidth == width && snapshot.WindowHeight == height {
		return nil
	}
	snapshot.WindowWidth, snapshot.WindowHeight = width, height
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
