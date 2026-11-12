package migration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestIncusAgentPolicyUpdates(t *testing.T) {
	tests := []struct {
		name   string
		status api.MigrationStatusType
		config *api.IncusAgentConfig
		denied bool
	}{
		{name: "background validation started", status: api.MIGRATIONSTATUS_BACKGROUND_IMPORT, config: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{}}, denied: true},
		{name: "guest configuration started", status: api.MIGRATIONSTATUS_POST_IMPORT, config: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{Exec: true}}, denied: true},
		{name: "canceled migration", status: api.MIGRATIONSTATUS_CANCELED, config: &api.IncusAgentConfig{Features: &api.IncusAgentFeatures{}}},
		{name: "equivalent inheritance", status: api.MIGRATIONSTATUS_POST_IMPORT, config: &api.IncusAgentConfig{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := migration.QueueEntry{MigrationStatus: tc.status}
			batch := migration.Batch{Name: "one", Status: api.BATCHSTATUS_RUNNING, Defaults: defaultPlacement, IncludeExpression: "true"}
			batch.Config.BackgroundSyncInterval = api.AsDuration(time.Minute)
			batch.Config.FinalBackgroundSyncLimit = api.AsDuration(time.Minute)
			repo := &mock.BatchRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) { return &batch, nil },
				UpdateFunc:    func(ctx context.Context, name string, in migration.Batch) error { return nil },
			}

			queueSvc := &QueueServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, name string) (migration.QueueEntries, error) {
					return migration.QueueEntries{entry}, nil
				},
			}

			batchSvc := migration.NewBatchService(repo, nil)
			updated := batch
			updated.Config.IncusAgent = tc.config
			err := batchSvc.Update(context.Background(), queueSvc, batch.Name, &updated)
			if tc.denied {
				require.ErrorIs(t, err, migration.ErrOperationNotPermitted)
				require.Empty(t, repo.UpdateCalls())
			} else {
				require.NoError(t, err)
				require.Len(t, repo.UpdateCalls(), 1)
			}

			instance := migration.Instance{
				UUID: uuidA, Source: "one",
				Properties: api.InstanceProperties{Location: "/one", OS: "ubuntu", OSDescription: "Ubuntu 24.04", InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Name: "one"}},
			}

			instanceRepo := &mock.InstanceRepoMock{
				GetByUUIDFunc:           func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) { return &instance, nil },
				GetQueueEntryByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.QueueEntry, error) { return &entry, nil },
				UpdateFunc:              func(ctx context.Context, in migration.Instance) error { return nil },
			}

			instanceSvc := migration.NewInstanceService(instanceRepo)
			err = instanceSvc.UpdateOverride(context.Background(), uuidA, api.InstanceOverride{IncusAgent: tc.config})
			if tc.denied {
				require.ErrorIs(t, err, migration.ErrOperationNotPermitted)
				require.Empty(t, instanceRepo.UpdateCalls())
			} else {
				require.NoError(t, err)
				require.Len(t, instanceRepo.UpdateCalls(), 1)
			}
		})
	}
}
