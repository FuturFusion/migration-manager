package api

import (
	"fmt"
	"time"
)

type BatchStatusType int
const (
	BATCHSTATUS_UNKNOWN = iota
	BATCHSTATUS_DEFINED
	BATCHSTATUS_READY
	BATCHSTATUS_QUEUED
	BATCHSTATUS_RUNNING
	BATCHSTATUS_STOPPED
	BATCHSTATUS_FINISHED
	BATCHSTATUS_ERROR
)

// Implement the stringer interface.
func (b BatchStatusType) String() string {
	switch b {
	case BATCHSTATUS_UNKNOWN:
		return "Unknown"
	case BATCHSTATUS_DEFINED:
		return "Defined"
	case BATCHSTATUS_READY:
		return "Ready"
	case BATCHSTATUS_QUEUED:
		return "Queued"
	case BATCHSTATUS_RUNNING:
		return "Running"
	case BATCHSTATUS_STOPPED:
		return "Stopped"
	case BATCHSTATUS_FINISHED:
		return "Finished"
	case BATCHSTATUS_ERROR:
		return "Error"
	default:
		return fmt.Sprintf("BatchStatusType(%d)", b)
	}
}

// Batch defines a collection of Instances to be migrated, possibly during a specific window of time.
//
// swagger:model
type Batch struct {
	// A human-friendly name for this batch
	// Example: MyBatch
	Name string `json:"name" yaml:"name"`

	// An opaque integer identifier for the batch
	// Example: 123
	DatabaseID int `json:"databaseID" yaml:"databaseID"`

	// The status of this batch
	// Example: BATCHSTATUS_DEFINED
	Status BatchStatusType `json:"status" yaml:"status"`

	IncludeRegex string `json:"includeRegex" yaml:"includeRegex"`

	ExcludeRegex string `json:"excludeRegex" yaml:"excludeRegex"`

	MigrationWindowStart time.Time `json:"migrationWindowStart" yaml:"migrationWindowStart"`

	MigrationWindowEnd time.Time `json:"migrationWindowEnd" yaml:"migrationWindowEnd"`
}
