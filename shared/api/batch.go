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
	// A human-friendly name for this batch
	// Example: MyBatch
	Name string `json:"name" yaml:"name"`

	// The destination target name to be used by all instances in this batch
	// Example: Mytarget
	Target string `json:"target" yaml:"target"`

	// The target project to use
	// Example: default
	TargetProject string `json:"target_project" yaml:"target_project"`

	// The status of this batch
	// Example: BATCHSTATUS_DEFINED
	Status BatchStatusType `json:"status" yaml:"status"`

	// A free-form string to provide additional information about the status
	// Example: "4 of 5 instances migrated"
	StatusMessage string `json:"status_message" yaml:"status_message"`

	// The Incus storage pool that this batch should use for creating VMs and mounting ISO images
	// Example: local
	StoragePool string `json:"storage_pool" yaml:"storage_pool"`

	// A Expression used to select instances to add to this batch
	// Language reference: https://expr-lang.org/docs/language-definition
	// Example: GetInventoryPath() matches "^foobar/.*"
	IncludeExpression string `json:"include_expression" yaml:"include_expression"`

	// If specified, don't start the migration before this time
	MigrationWindowStart time.Time `json:"migration_window_start" yaml:"migration_window_start"`

	// If specified, don't start the migration after this time
	MigrationWindowEnd time.Time `json:"migration_window_end" yaml:"migration_window_end"`
}
