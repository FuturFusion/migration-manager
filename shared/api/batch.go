package api

import (
	"fmt"
	"time"
)

type BatchStatusType string

const (
	BATCHSTATUS_DEFINED  BatchStatusType = "Defined"
	BATCHSTATUS_QUEUED   BatchStatusType = "Queued"
	BATCHSTATUS_RUNNING  BatchStatusType = "Running"
	BATCHSTATUS_STOPPED  BatchStatusType = "Stopped"
	BATCHSTATUS_FINISHED BatchStatusType = "Finished"
	BATCHSTATUS_ERROR    BatchStatusType = "Error"
)

// Validate ensures the BatchStatusType is valid.
func (b BatchStatusType) Validate() error {
	switch b {
	case BATCHSTATUS_DEFINED:
	case BATCHSTATUS_ERROR:
	case BATCHSTATUS_FINISHED:
	case BATCHSTATUS_QUEUED:
	case BATCHSTATUS_RUNNING:
	case BATCHSTATUS_STOPPED:
	default:
		return fmt.Errorf("%s is not a valid batch status", b)
	}

	return nil
}

// Batch defines a collection of Instances to be migrated, possibly during a specific window of time.
//
// swagger:model
type Batch struct {
	BatchPut

	// The status of this batch
	// Example: BATCHSTATUS_DEFINED
	Status BatchStatusType `json:"status" yaml:"status"`

	// A free-form string to provide additional information about the status
	// Example: "4 of 5 instances migrated"
	StatusMessage string `json:"status_message" yaml:"status_message"`
}

// BatchPut defines the configurable fields of Batch.
//
// swagger:model
type BatchPut struct {
	// A human-friendly name for this batch
	// Example: MyBatch
	Name string `json:"name" yaml:"name"`

	// The destination target name to be used by all instances in this batch
	// Example: Mytarget
	Target string `json:"target" yaml:"target"`

	// The target project to use
	// Example: default
	TargetProject string `json:"target_project" yaml:"target_project"`

	// The Incus storage pool that this batch should use for creating VMs and mounting ISO images
	// Example: local
	StoragePool string `json:"storage_pool" yaml:"storage_pool"`

	// A Expression used to select instances to add to this batch
	// Language reference: https://expr-lang.org/docs/language-definition
	// Example: GetInventoryPath() matches "^foobar/.*"
	IncludeExpression string `json:"include_expression" yaml:"include_expression"`

	// Set of migration window timings.
	MigrationWindows []MigrationWindow `json:"migration_windows" yaml:"migration_windows"`
}

// MigrationWindow defines the scheduling of a batch migration.
type MigrationWindow struct {
	// Start time for finalizing migrations after background import.
	Start time.Time `json:"start" yaml:"start"`

	// End time for finalizing migrations after background import.
	End time.Time `json:"end" yaml:"end"`

	// Lockout time after which the batch can no longer modify the target instance.
	Lockout time.Time `json:"lockout" yaml:"lockout"`
}
}
