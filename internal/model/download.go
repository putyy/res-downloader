package model

const (
	DownloadTaskPending     = "pending"
	DownloadTaskResolving   = "resolving"
	DownloadTaskDownloading = "downloading"
	DownloadTaskProcessing  = "processing"
	DownloadTaskPausing     = "pausing"
	DownloadTaskPaused      = "paused"
	DownloadTaskCompleted   = "completed"
	DownloadTaskFailed      = "failed"
	DownloadTaskCancelled   = "cancelled"
	DownloadTaskInterrupted = "interrupted"
)

type DownloadTaskRecord struct {
	ID            string             `json:"id"`
	ResourceID    string             `json:"resourceId"`
	ParentID      string             `json:"parentId,omitempty"`
	Resource      ResourceCandidate  `json:"resource"`
	PluginID      string             `json:"pluginId,omitempty"`
	PluginVersion string             `json:"pluginVersion,omitempty"`
	PluginDigest  string             `json:"pluginDigest,omitempty"`
	Plan          DownloadPlan       `json:"plan,omitempty"`
	Items         []DownloadTaskItem `json:"items,omitempty"`
	State         string             `json:"state"`
	Resumable     bool               `json:"resumable,omitempty"`
	Recording     bool               `json:"recording,omitempty"`
	Step          string             `json:"step,omitempty"`
	Attempts      int                `json:"attempts"`
	Resumes       int                `json:"resumes,omitempty"`
	Downloaded    int64              `json:"downloaded,omitempty"`
	Total         int64              `json:"total,omitempty"`
	CreatedAt     int64              `json:"createdAt"`
	UpdatedAt     int64              `json:"updatedAt"`
	StartedAt     int64              `json:"startedAt,omitempty"`
	FinishedAt    int64              `json:"finishedAt,omitempty"`
	SaveDirectory string             `json:"saveDirectory"`
	TempPath      string             `json:"tempPath,omitempty"`
	OutputPath    string             `json:"outputPath,omitempty"`
	Error         string             `json:"error,omitempty"`
}

// DownloadTaskItem is a durable child-resource snapshot owned by a download
// task. Collection execution must not depend on the capture catalog after the
// task has been created.
type DownloadTaskItem struct {
	Resource   ResourceCandidate `json:"resource"`
	Plan       DownloadPlan      `json:"plan,omitempty"`
	State      string            `json:"state,omitempty"`
	OutputPath string            `json:"outputPath,omitempty"`
}

// DownloadExecution identifies the durable workspace used by one task or one
// collection child. Executors may keep resumable checkpoints below WorkDir.
type DownloadExecution struct {
	TaskID  string `json:"taskId"`
	WorkDir string `json:"workDir"`
}
