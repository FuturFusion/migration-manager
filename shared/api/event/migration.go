package event

import (
	"encoding/json"

	"github.com/FuturFusion/migration-manager/shared/api"
)

const (
	// MigrationCreated (instance was created as part of an ongoing migration).
	MigrationCreated api.LifecycleAction = "migration-created"

	// migration-sync-started (instance started a pre-migration run).
	MigrationSyncStarted api.LifecycleAction = "migration-sync-started"

	// migration-sync-completed (instance completed a pre-migration run).
	MigrationSyncCompleted api.LifecycleAction = "migration-sync-completed"

	// migration-final-started (final migration has started, source instance is offline).
	MigrationFinalStarted api.LifecycleAction = "migration-final-started"

	// migration-final-completed (final migration has completed).
	MigrationFinalCompleted api.LifecycleAction = "migration-final-completed"
)

type MigrationDetails struct {
	api.QueueEntry `yaml:",inline"`

	Instance api.InstanceFilterable `json:"instance"`
}

func NewMigrationEvent(action api.LifecycleAction, instance api.Instance, queueEntry api.QueueEntry) api.EventLifecycle {
	b, _ := json.Marshal(MigrationDetails{
		QueueEntry: queueEntry,
		Instance:   instance.ToFilterable(),
	})

	entities := []string{
		QueueEntryURI(queueEntry.InstanceUUID),
		InstanceURI(instance.Properties.UUID),
		BatchURI(queueEntry.BatchName),
		SourceURI(instance.Source),
		TargetURI(queueEntry.Placement.TargetName),
	}

	for _, nic := range instance.Properties.NICs {
		entities = append(entities, NetworkURI(nic.UUID))
	}

	return api.EventLifecycle{
		Action:   string(action),
		Entities: entities,
		Metadata: b,
	}
}
