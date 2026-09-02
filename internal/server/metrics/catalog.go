package metrics

const (
	gauge   = "gauge"
	counter = "counter"
)

// The metrics reported by the daemon.
var (
	Artifacts                 = Metric{name: "migration_manager_artifacts", kind: gauge, help: "The number of defined artifacts."}
	Batches                   = Metric{name: "migration_manager_batches", kind: gauge, help: "The number of defined batches."}
	BatchInstances            = Metric{name: "migration_manager_batch_instances", kind: gauge, help: "The number of instances assigned to a batch."}
	BatchMigratedInstances    = Metric{name: "migration_manager_batch_migrated_instances", kind: gauge, help: "The number of instances of a batch that have been migrated."}
	BatchBlockedInstances     = Metric{name: "migration_manager_batch_blocked_instances", kind: gauge, help: "The number of instances of a batch that are blocked from migrating."}
	BatchDiskBytes            = Metric{name: "migration_manager_batch_disk_bytes", kind: gauge, help: "The disk size in bytes of the instances assigned to a batch."}
	BatchMigratedDiskBytes    = Metric{name: "migration_manager_batch_migrated_disk_bytes", kind: gauge, help: "The disk size in bytes of a batch that has been transferred to the target."}
	Instances                 = Metric{name: "migration_manager_instances", kind: gauge, help: "The number of instances known to the daemon."}
	Networks                  = Metric{name: "migration_manager_networks", kind: gauge, help: "The number of networks known to the daemon."}
	QueueEntries              = Metric{name: "migration_manager_queue_entries", kind: gauge, help: "The number of instances currently in a migration queue."}
	Sources                   = Metric{name: "migration_manager_sources", kind: gauge, help: "The number of configured sources."}
	SourceInstances           = Metric{name: "migration_manager_source_instances", kind: gauge, help: "The number of instances reported by a source."}
	SourceImportedInstances   = Metric{name: "migration_manager_source_imported_instances", kind: gauge, help: "The number of instances of a source that were fully imported by the last sync."}
	SourceMigratedInstances   = Metric{name: "migration_manager_source_migrated_instances", kind: gauge, help: "The number of instances of a source that have been migrated."}
	SourceIneligibleInstances = Metric{name: "migration_manager_source_ineligible_instances", kind: gauge, help: "The number of instances of a source that aren't eligible for migration."}
	SourceImportIssues        = Metric{name: "migration_manager_source_import_issues", kind: gauge, help: "The number of instances of a source affected by an import issue."}
	Targets                   = Metric{name: "migration_manager_targets", kind: gauge, help: "The number of configured targets."}
	Warnings                  = Metric{name: "migration_manager_warnings", kind: gauge, help: "The number of recorded warnings."}
	Uptime                    = Metric{name: "migration_manager_uptime_seconds", kind: gauge, help: "The daemon uptime in seconds."}
	GoGoroutines              = Metric{name: "migration_manager_go_goroutines", kind: gauge, help: "The number of goroutines that currently exist."}
	GoAllocBytes              = Metric{name: "migration_manager_go_alloc_bytes", kind: gauge, help: "The number of bytes allocated and still in use."}
	GoAllocBytesTotal         = Metric{name: "migration_manager_go_alloc_bytes_total", kind: counter, help: "The total number of bytes allocated, even if freed."}
	GoHeapObjects             = Metric{name: "migration_manager_go_heap_objects", kind: gauge, help: "The number of allocated objects."}
	GoSysBytes                = Metric{name: "migration_manager_go_sys_bytes", kind: gauge, help: "The number of bytes obtained from the system."}
)
