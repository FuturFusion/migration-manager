package api

import (
	"time"

	"github.com/google/uuid"
)

// StatsOverview provides a high-level summary of the state of migration manager.
//
// swagger:model
type StatsOverview struct {
	// Summary of sources
	Sources SourcesOverview `json:"sources" yaml:"sources"`

	// Summary of batches
	Batches BatchesOverview `json:"batches" yaml:"batches"`
}

// SourcesOverview provides a high-level summary of all sources.
//
// swagger:model
type SourcesOverview struct {
	// Number of source instances known to migration manager
	// Example: 42
	TotalInstances int `json:"total_instances" yaml:"total_instances"`

	// Number of instances whose properties were fully read from their source, plus any already migrated
	// Example: 30
	ImportedInstances int `json:"imported_instances" yaml:"imported_instances"`

	// Number of instances that have finished migrating
	// Example: 10
	MigratedInstances int `json:"migrated_instances" yaml:"migrated_instances"`

	// Number of instances that cannot be migrated as currently configured, excluding those already migrated
	// Example: 5
	IneligibleInstances int `json:"ineligible_instances" yaml:"ineligible_instances"`

	// Breakdown of ineligible instances by the reason they cannot be migrated
	IneligibleReasons []ReasonInstances `json:"ineligible_reasons" yaml:"ineligible_reasons"`

	// Ineligible instances whose restriction is waived by a batch they are assigned to
	OverriddenInstances []OverriddenInstance `json:"overridden_instances" yaml:"overridden_instances"`

	// Instances not fully synced because their source was unavailable
	BlockedImports []InstanceRef `json:"blocked_imports" yaml:"blocked_imports"`

	// Instances not fully synced because their source reported an import failure
	FailedImports []InstanceRef `json:"failed_imports" yaml:"failed_imports"`

	// Instances not fully synced because their source only reported some of their properties
	PartialImports []InstanceRef `json:"partial_imports" yaml:"partial_imports"`

	// Per-source statistics, one entry per defined source
	Items []SourceSummary `json:"items" yaml:"items"`
}

// BatchesOverview summarizes all batches, counting each instance once so totals aren't the sum across Items.
//
// swagger:model
type BatchesOverview struct {
	// Number of instances assigned to a batch
	// Example: 12
	TotalInstances int `json:"total_instances" yaml:"total_instances"`

	// Number of instances assigned to a batch that have finished migrating
	// Example: 5
	MigratedInstances int `json:"migrated_instances" yaml:"migrated_instances"`

	// Number of queued instances whose migration is currently blocked
	// Example: 2
	BlockedInstances int `json:"blocked_instances" yaml:"blocked_instances"`

	// Breakdown of blocked instances by the reason their migration is blocked
	BlockedReasons []ReasonInstances `json:"blocked_reasons" yaml:"blocked_reasons"`

	// Total disk size, in bytes, across all instances assigned to a batch
	// Example: 21474836480
	TotalDiskSize int64 `json:"total_disk_size" yaml:"total_disk_size"`

	// Total disk size, in bytes, that has been transferred to the target
	// Example: 10737418240
	MigratedDiskSize int64 `json:"migrated_disk_size" yaml:"migrated_disk_size"`

	// Per-batch statistics, one entry per defined batch
	Items []BatchSummary `json:"items" yaml:"items"`
}

// BatchSummary provides a high-level summary of a single batch.
//
// swagger:model
type BatchSummary struct {
	// A human-friendly name for the batch
	// Example: MyBatch
	Name string `json:"name" yaml:"name"`

	// The status of the batch
	// Example: Running
	Status BatchStatusType `json:"status" yaml:"status"`

	// Number of instances assigned to the batch
	// Example: 12
	TotalInstances int `json:"total_instances" yaml:"total_instances"`

	// Number of instances in the batch that have finished migrating
	// Example: 5
	MigratedInstances int `json:"migrated_instances" yaml:"migrated_instances"`

	// Number of instances in the batch whose migration is currently blocked
	// Example: 2
	BlockedInstances int `json:"blocked_instances" yaml:"blocked_instances"`

	// Breakdown of blocked instances in the batch by the reason their migration is blocked
	BlockedReasons []ReasonInstances `json:"blocked_reasons" yaml:"blocked_reasons"`

	// Total disk size, in bytes, across all instances in the batch
	// Example: 21474836480
	TotalDiskSize int64 `json:"total_disk_size" yaml:"total_disk_size"`

	// Total disk size, in bytes, transferred to the target, counted once background import completes
	// Example: 10737418240
	MigratedDiskSize int64 `json:"migrated_disk_size" yaml:"migrated_disk_size"`

	// The next upcoming (or currently active) migration window for the batch, nil if none is scheduled
	NextWindow *BatchWindow `json:"next_window,omitempty" yaml:"next_window,omitempty"`
}

// BatchWindow provides a high-level summary of a batch's upcoming migration window.
//
// swagger:model
type BatchWindow struct {
	// A human-friendly name for the migration window
	// Example: MyWindow
	Name string `json:"name" yaml:"name"`

	// Time in UTC that the migration window starts
	Start time.Time `json:"start" yaml:"start"`

	// Time in UTC that the migration window ends
	End time.Time `json:"end" yaml:"end"`
}

// SourceSummary provides a high-level summary of a single source.
//
// swagger:model
type SourceSummary struct {
	// A human-friendly name for the source
	// Example: MySource
	Name string `json:"name" yaml:"name"`

	// The connectivity status of the source
	// Example: OK
	ConnectivityStatus ExternalConnectivityStatus `json:"connectivity_status" yaml:"connectivity_status"`

	// Number of instances known to migration manager from this source
	// Example: 20
	TotalInstances int `json:"total_instances" yaml:"total_instances"`

	// Number of instances from this source whose properties were fully read, plus any already migrated
	// Example: 15
	ImportedInstances int `json:"imported_instances" yaml:"imported_instances"`

	// Number of instances from this source that have finished migrating
	// Example: 5
	MigratedInstances int `json:"migrated_instances" yaml:"migrated_instances"`

	// Number of instances from this source that cannot be migrated as currently configured
	// Example: 2
	IneligibleInstances int `json:"ineligible_instances" yaml:"ineligible_instances"`

	// Breakdown of ineligible instances from this source by the reason they cannot be migrated
	IneligibleReasons []ReasonInstances `json:"ineligible_reasons" yaml:"ineligible_reasons"`

	// Ineligible instances from this source whose restriction is waived by a batch they are assigned to
	OverriddenInstances []OverriddenInstance `json:"overridden_instances" yaml:"overridden_instances"`

	// Instances from this source not fully synced because the source was unavailable
	BlockedImports []InstanceRef `json:"blocked_imports" yaml:"blocked_imports"`

	// Instances from this source not fully synced because it reported an import failure
	FailedImports []InstanceRef `json:"failed_imports" yaml:"failed_imports"`

	// Instances from this source not fully synced because only some of their properties were reported
	PartialImports []InstanceRef `json:"partial_imports" yaml:"partial_imports"`
}

// ReasonInstances groups the instances that share a reason for being ineligible or blocked.
//
// swagger:model
type ReasonInstances struct {
	// The reason the instances are ineligible or blocked
	// Example: Unknown OS
	Reason string `json:"reason" yaml:"reason"`

	// The instances affected by this reason
	Instances []InstanceRef `json:"instances" yaml:"instances"`
}

// InstanceRef identifies an instance well enough to display and link to it.
//
// swagger:model
type InstanceRef struct {
	// The UUID of the instance
	// Example: 2ed700e6-45ca-4b1a-a1c3-0b0a4dd7b5d5
	UUID uuid.UUID `json:"uuid" yaml:"uuid"`

	// The inventory path of the instance on its source
	// Example: /SDDC-Datacenter/vm/MyVM
	Location string `json:"location" yaml:"location"`
}

// OverriddenInstance is an ineligible instance whose restriction is waived by one of its batches.
//
// swagger:model
type OverriddenInstance struct {
	InstanceRef `yaml:",inline"`

	// The name of the batch whose overrides waive the restriction
	// Example: batch1
	Batch string `json:"batch" yaml:"batch"`
}
