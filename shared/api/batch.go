package api

import (
	"fmt"
	"time"
)

type BatchStatusType int

const (
	BATCHSTATUS_UNKNOWN BatchStatusType = iota
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

	// A free-form string to provide additional information about the status
	// Example: "4 of 5 instances migrated"
	StatusString string `json:"statusString" yaml:"statusString"`

	// The Incus storage pool that this batch should use for creating VMs and mounting ISO images
	// Example: local
	StoragePool string `json:"storagePool" yaml:"storagePool"`

	// A regular expression used to select instances to add to this batch
	// Example: .*
	IncludeRegex string `json:"includeRegex" yaml:"includeRegex"`

	// A regular expression used to exclude instances from this batch
	// Example: Windows
	ExcludeRegex string `json:"excludeRegex" yaml:"excludeRegex"`

	// If specified, don't start the migration before this time
	MigrationWindowStart time.Time `json:"migrationWindowStart" yaml:"migrationWindowStart"`

	// If specified, don't start the migration after this time
	MigrationWindowEnd time.Time `json:"migrationWindowEnd" yaml:"migrationWindowEnd"`

	// Default network to use for instances if not specified by their NIC(s) definition
	DefaultNetwork string `json:"defaultNetwork" yaml:"defaultNetwork"`
}
