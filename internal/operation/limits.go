package operation

import "time"

const (
	MaxInputBytes       = 32 * 1024
	MaxResultBytes      = 60 * 1024
	MaxBatch            = 32
	MaxItems            = 100
	MaxActive           = 256
	MaxPluginActive     = 64
	MaxPageActive       = 32
	MaxConcurrent       = 8
	MaxPluginConcurrent = 2
	MaxHistory          = 10000
	MaxHistoryBytes     = 32 * 1024 * 1024
	MaxResults          = 1000
	MaxResultTotalBytes = 16 * 1024 * 1024
	MaxTextBytes        = 32 * 1024
	MaxTextTotalBytes   = 1024 * 1024
	DefaultTimeout      = 120 * time.Second
	MaxTimeout          = 30 * time.Minute
	AcceptTimeout       = 30 * time.Second
	HeartbeatTimeout    = 90 * time.Second
	QueueTimeout        = 5 * time.Minute
	IdempotencyTTL      = 24 * time.Hour
	HistoryTTL          = 30 * 24 * time.Hour
	ResultTTL           = 24 * time.Hour
	CursorTTL           = 15 * time.Minute
)

type Error struct {
	Code    string `json:"errorCode"`
	Message string `json:"message"`
}

func (e *Error) Error() string           { return e.Message }
func problem(code, message string) error { return &Error{code, message} }
func Terminal(state string) bool {
	return oneOf(state, "succeeded", "failed", "cancelled", "timed_out", "interrupted")
}
