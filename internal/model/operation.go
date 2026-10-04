package model

import "encoding/json"

// OperationDefinition describes a bounded call into an already connected page.
// Schema is a strict subset; unknown validation keywords are rejected at load.
type OperationDefinition struct {
	Name             string                   `json:"name" yaml:"name"`
	Description      string                   `json:"description,omitempty" yaml:"description,omitempty"`
	Locales          map[string]PluginLocale  `json:"locales,omitempty" yaml:"locales,omitempty"`
	Category         string                   `json:"category" yaml:"category"`
	PageScript       string                   `json:"pageScript" yaml:"pageScript"`
	PageMatch        []PluginPageScriptMatch  `json:"pageMatch,omitempty" yaml:"pageMatch,omitempty"`
	RequiresLogin    bool                     `json:"requiresLogin,omitempty" yaml:"requiresLogin,omitempty"`
	InputSchema      map[string]interface{}   `json:"inputSchema" yaml:"inputSchema"`
	OutputSchema     map[string]interface{}   `json:"outputSchema" yaml:"outputSchema"`
	Examples         []map[string]interface{} `json:"examples,omitempty" yaml:"examples,omitempty"`
	Effects          []string                 `json:"effects" yaml:"effects"`
	TimeoutSeconds   int                      `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
	Cancellable      bool                     `json:"cancellable,omitempty" yaml:"cancellable,omitempty"`
	SafeRetry        bool                     `json:"safeRetry,omitempty" yaml:"safeRetry,omitempty"`
	AllowReload      bool                     `json:"allowReload,omitempty" yaml:"allowReload,omitempty"`
	Automation       bool                     `json:"automation,omitempty" yaml:"automation,omitempty"`
	ResultTTLSeconds int                      `json:"resultTTLSeconds,omitempty" yaml:"resultTTLSeconds,omitempty"`
	PersistResult    bool                     `json:"persistResult,omitempty" yaml:"persistResult,omitempty"`
}

type OperationInfo struct {
	RuntimeID         string              `json:"-"`
	PluginID          string              `json:"pluginId"`
	PluginVersion     string              `json:"pluginVersion"`
	OperationID       string              `json:"operationId"`
	Definition        OperationDefinition `json:"definition"`
	AutomationEnabled bool                `json:"automationEnabled"`
	Available         bool                `json:"available"`
}

type OperationSession struct {
	PluginID      string   `json:"pluginId"`
	ScriptID      string   `json:"scriptId"`
	PageSessionID string   `json:"pageSessionId"`
	Title         string   `json:"title"`
	PageURL       string   `json:"pageUrl"`
	Connected     bool     `json:"connected"`
	Ready         bool     `json:"ready"`
	Login         string   `json:"login"`
	Revision      string   `json:"revision"`
	Operations    []string `json:"operations"`
}

type OperationRequest struct {
	PluginID       string                 `json:"pluginId"`
	OperationID    string                 `json:"operationId"`
	PageSessionID  string                 `json:"pageSessionId,omitempty"`
	Input          map[string]interface{} `json:"input"`
	Cursor         string                 `json:"cursor,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
	IdempotencyKey string                 `json:"idempotencyKey,omitempty"`
	RetryOf        string                 `json:"retryOf,omitempty"`
	ResourceID     string                 `json:"resourceId,omitempty"`
	ActionID       string                 `json:"actionId,omitempty"`
}

type OperationExecution struct {
	ExecutionID      string          `json:"executionId"`
	BatchID          string          `json:"batchId,omitempty"`
	Index            int             `json:"index"`
	PluginID         string          `json:"pluginId"`
	PluginVersion    string          `json:"pluginVersion"`
	OperationID      string          `json:"operationId"`
	PageSessionID    string          `json:"pageSessionId"`
	Revision         string          `json:"revision"`
	ReloadCount      int             `json:"reloadCount,omitempty"`
	Source           string          `json:"source"`
	State            string          `json:"state"`
	Certainty        string          `json:"certainty"`
	ErrorCode        string          `json:"errorCode,omitempty"`
	Progress         *float64        `json:"progress,omitempty"`
	CancelRequested  bool            `json:"cancelRequested"`
	CancelConfirmed  bool            `json:"cancelConfirmed"`
	InputSummary     interface{}     `json:"inputSummary,omitempty"`
	ResultStatus     string          `json:"resultStatus"`
	Result           json.RawMessage `json:"result,omitempty"`
	ResultProjection bool            `json:"resultProjection"`
	ResultExpiresAt  int64           `json:"resultExpiresAt,omitempty"`
	CreatedAt        int64           `json:"createdAt"`
	UpdatedAt        int64           `json:"updatedAt"`
	StartedAt        int64           `json:"startedAt,omitempty"`
	AcceptedAt       int64           `json:"acceptedAt,omitempty"`
	Deadline         int64           `json:"deadline,omitempty"`
	RetryOf          string          `json:"retryOf,omitempty"`
	ResourceID       string          `json:"resourceId,omitempty"`
	ResourceIDs      []string        `json:"resourceIds"`
	DownloadTaskIDs  []string        `json:"downloadTaskIds"`
	ArtifactIDs      []string        `json:"artifactIds"`
}

// OperationReloadIdentity is host-only state, never serialized into a ticket.
type OperationReloadIdentity struct {
	PluginID, ScriptID, PageURL, Login, Context, RuntimeID string
}

type OperationReloadTicket struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}

type OperationBatch struct {
	BatchID string               `json:"batchId"`
	Items   []OperationExecution `json:"items"`
	Counts  map[string]int       `json:"counts"`
	Total   int                  `json:"total"`
	Offset  int                  `json:"offset"`
	HasMore bool                 `json:"hasMore"`
}

type OperationReport struct {
	ExecutionID string          `json:"executionId"`
	Revision    string          `json:"revision"`
	State       string          `json:"state"`
	Progress    *float64        `json:"progress,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	NextCursor  string          `json:"nextCursor,omitempty"`
	HasMore     bool            `json:"hasMore,omitempty"`
	Truncated   bool            `json:"truncated,omitempty"`
	Count       int             `json:"count,omitempty"`
}

type OperationArtifact struct {
	ArtifactID     string `json:"artifactId"`
	ExecutionID    string `json:"executionId"`
	PluginID       string `json:"pluginId"`
	ResourceID     string `json:"resourceId,omitempty"`
	DownloadTaskID string `json:"downloadTaskId,omitempty"`
	MIME           string `json:"mime"`
	Size           int64  `json:"size"`
	Status         string `json:"status"`
	ExpiresAt      int64  `json:"expiresAt,omitempty"`
	Output         string `json:"output,omitempty"`
	CaptureKey     string `json:"-"`
}
