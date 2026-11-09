package api

import "time"

// WorkerLogChunkSize limits memory use for uploads.
const WorkerLogChunkSize = 64 * 1024

const (
	// WorkerLogDefaultLines is the number of recent log lines shown by default.
	WorkerLogDefaultLines = 200
	// WorkerLogMaxLines limits the number of lines requested in a log preview.
	WorkerLogMaxLines = 10000
	// WorkerLogPreviewSize limits the number of bytes read for a log preview.
	WorkerLogPreviewSize = 1024 * 1024
)

// WorkerLogBatch contains log bytes starting at an offset within a worker session.
type WorkerLogBatch struct {
	Session string `json:"session"`
	Offset  int64  `json:"offset"`
	Data    []byte `json:"data"`
}

// WorkerLogSession describes logs received during one VM boot.
//
// swagger:model
type WorkerLogSession struct {
	Session  string    `json:"session"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// WorkerLogContent contains a bounded preview of a worker log.
//
// swagger:model
type WorkerLogContent struct {
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
}
