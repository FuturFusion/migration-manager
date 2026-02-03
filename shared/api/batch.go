package api

import (
	"fmt"
	"time"
)

type BatchStatusType string

const (
	BATCHSTATUS_DEFINED  BatchStatusType = "Defined"
	BATCHSTATUS_RUNNING  BatchStatusType = "Running"
	BATCHSTATUS_STOPPED  BatchStatusType = "Stopped"
	BATCHSTATUS_FINISHED BatchStatusType = "Finished"
	BATCHSTATUS_ERROR    BatchStatusType = "Error"
)

const (
	DefaultTarget        = "default"
	DefaultTargetProject = "default"
	DefaultStoragePool   = "default"
)

// Validate ensures the BatchStatusType is valid.
func (b BatchStatusType) Validate() error {
	switch b {
	case BATCHSTATUS_DEFINED:
	case BATCHSTATUS_ERROR:
	case BATCHSTATUS_FINISHED:
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
	BatchPut `yaml:",inline"`

	// The status of this batch
	// Example: BATCHSTATUS_DEFINED
	Status BatchStatusType `json:"status" yaml:"status"`

	// A free-form string to provide additional information about the status
	// Example: "4 of 5 instances migrated"
	StatusMessage string `json:"status_message" yaml:"status_message"`

	// Time in UTC when the batch was started.
	// Example: 2025-01-01 01:00:00
	StartDate time.Time `json:"start_date" yaml:"start_date"`
}

// BatchPut defines the configurable fields of Batch.
//
// swagger:model
type BatchPut struct {
	// A human-friendly name for this batch
	// Example: MyBatch
	Name string `json:"name" yaml:"name"`

	// A Expression used to select instances to add to this batch
	// Language reference: https://expr-lang.org/docs/language-definition
	// Example: location matches "^foobar/.*"
	IncludeExpression string `json:"include_expression" yaml:"include_expression"`

	// Set of migration window timings.
	MigrationWindows []MigrationWindow `json:"migration_windows" yaml:"migration_windows"`

	// Set of constraints to apply to the batch. For each instance, the last constraint in the list that matches will be applied.
	Constraints []BatchConstraint `json:"constraints" yaml:"constraints"`

	// Default configurations for the batch.
	Defaults BatchDefaults `json:"defaults" yaml:"defaults"`

	// Additional configuration for the batch.
	Config BatchConfig `json:"config" yaml:"config"`
}

type BatchDefaults struct {
	// Default target placement for instances. Can be overridden with the placement scriptlet.
	Placement BatchPlacement `json:"placement" yaml:"placement"`

	// Network configuration to use during migration of the instance. If unspecified, the default network configuration in the default profile will be used.
	MigrationNetwork []MigrationNetworkPlacement `json:"migration_network" yaml:"migration_network"`
}

type MigrationNetworkPlacement struct {
	NetworkPlacement `yaml:",inline"`

	// Target that the network is available in.
	Target string `json:"target" yaml:"target"`

	// Target project that the network is available in.
	TargetProject string `json:"target_project" yaml:"target_project"`
}

type BatchPlacement struct {
	// The destination target name to be used by all instances in this batch
	// Example: Mytarget
	Target string `json:"target" yaml:"target"`

	// The target project to use
	// Example: default
	TargetProject string `json:"target_project" yaml:"target_project"`

	// The Incus storage pool that this batch should use for creating VMs and mounting ISO images
	// Example: local
	StoragePool string `json:"storage_pool" yaml:"storage_pool"`
}

type BatchConfig struct {
	// Whether to re-run scriptlets if a migration restarts
	RerunScriptlets bool `json:"rerun_scriptlets" yaml:"rerun_scriptlets"`

	// The placement scriptlet used to determine the target for queued instances.
	// Example: starlark scriptlet
	PlacementScriptlet string `json:"placement_scriptlet" yaml:"placement_scriptlet"`

	// PostMigrationRetries is the maximum number of times post-migration steps will be retried upon errors.
	// Example: 5
	PostMigrationRetries int `json:"post_migration_retries" yaml:"post_migration_retries"`

	// Overrides to allow migrating instances that are otherwise restricted.
	RestrictionOverrides InstanceRestrictionOverride `json:"instance_restriction_overrides" yaml:"instance_restriction_overrides"`

	// Interval over which background sync will be rerun until the migration window has begun.
	BackgroundSyncInterval Duration `json:"background_sync_interval" yaml:"background_sync_interval"`

	// The minimum amount of time before the migration window begins that background sync can be re-attempted.
	FinalBackgroundSyncLimit Duration `json:"final_background_sync_limit" yaml:"final_background_sync_limit"`
}

// BatchConstraint is a constraint to be applied to a batch to determine which instances can be migrated.
type BatchConstraint struct {
	// Name of the constraint.
	Name string `json:"name" yaml:"name"`

	// Description of the constraint.
	Description string `json:"description" yaml:"description"`

	// Expression used to select instances for the constraint.
	IncludeExpression string `json:"include_expression" yaml:"include_expression"`

	// Maximum amount of matched instances that can concurrently migrate, before moving to the next migration window.
	MaxConcurrentInstances int `json:"max_concurrent_instances" yaml:"max_concurrent_instances"`

	// Minimum amount of time required for an instance to boot after initial disk import. Migration window duration must be at least this much.
	MinInstanceBootTime Duration `json:"min_instance_boot_time" yaml:"min_instance_boot_time"`
}

type InstanceRestrictionOverride struct {
	// Allow migration of instances with unknown OSes.
	AllowUnknownOS bool `json:"allow_unknown_os" yaml:"allow_unknown_os"`

	// Allow migration of instances with no IPv4 addresses.
	AllowNoIPv4 bool `json:"allow_no_ipv4" yaml:"allow_no_ipv4"`

	// Allow migration of instances with no background import support.
	AllowNoBackgroundImport bool `json:"allow_no_background_import" yaml:"allow_no_background_import"`
}
