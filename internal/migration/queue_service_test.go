package migration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestQueueService_GetAll(t *testing.T) {
	tests := []struct {
		name          string
		repoGetAll    migration.QueueEntries
		repoGetAllErr error

		assertErr      require.ErrorAssertionFunc
		wantQueueItems migration.QueueEntries
	}{
		{
			name: "success - no batches",

			assertErr: require.NoError,
		},
		{
			name: "success - with batches",
			repoGetAll: migration.QueueEntries{
				{BatchName: "one"},
				{BatchName: "two"},
				{BatchName: "three"},
				{BatchName: "four"},
				{BatchName: "five"},
			},

			assertErr: require.NoError,
			wantQueueItems: migration.QueueEntries{
				{BatchName: "one"},
				{BatchName: "two"},
				{BatchName: "three"},
				{BatchName: "four"},
				{BatchName: "five"},
			},
		},
		{
			name:          "error - repo.GetAll",
			repoGetAllErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mock.QueueRepoMock{
				GetAllFunc: func(ctx context.Context) (migration.QueueEntries, error) {
					return tc.repoGetAll, tc.repoGetAllErr
				},
			}

			queueSvc := migration.NewQueueService(repo, nil, nil, nil, nil)

			// Run test
			queueItems, err := queueSvc.GetAll(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantQueueItems, queueItems)
		})
	}
}

func TestQueueService_GetByInstanceID(t *testing.T) {
	tests := []struct {
		name    string
		uuidArg uuid.UUID

		repoGetByInstanceUUID    *migration.QueueEntry
		repoGetByInstanceUUIDErr error

		assertErr      require.ErrorAssertionFunc
		wantQueueEntry *migration.QueueEntry
	}{
		{
			name:                  "success",
			uuidArg:               uuidA,
			repoGetByInstanceUUID: &migration.QueueEntry{InstanceUUID: uuidA},
			wantQueueEntry:        &migration.QueueEntry{InstanceUUID: uuidA},
			assertErr:             require.NoError,
		},
		{
			name:                     "error - instance.GetByID",
			uuidArg:                  uuidA,
			repoGetByInstanceUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mock.QueueRepoMock{
				GetByInstanceUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.QueueEntry, error) {
					return tc.repoGetByInstanceUUID, tc.repoGetByInstanceUUIDErr
				},
			}
			// Setup

			queueSvc := migration.NewQueueService(repo, nil, nil, nil, nil)

			// Run test
			queueEntry, err := queueSvc.GetByInstanceUUID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantQueueEntry, queueEntry)
		})
	}
}

func TestQueueService_NewWorkerCommandByInstanceUUID(t *testing.T) {
	tests := []struct {
		name    string
		uuidArg uuid.UUID

		repoGetByInstanceUUID    migration.QueueEntry
		repoGetByInstanceUUIDErr error

		repoGetAll    migration.QueueEntries
		repoGetAllErr error

		repoUpdateErr error

		batchSvcGetByName    migration.Batch
		batchSvcGetByNameErr error

		instanceSvcGetByIDInstance migration.Instance
		instanceSvcGetByIDErr      error

		instanceSvcGetQueued    migration.Instances
		instanceSvcGetQueuedErr error

		sourceSvcGetByIDSource migration.Source
		sourceSvcGetByIDErr    error

		targetSvcGetByIDTarget migration.Target
		targetSvcGetByIDErr    error

		batchSvcGetWindows    migration.MigrationWindows
		batchSvcGetWindowsErr error

		sourceImportLimit int
		targetImportLimit int

		assertErr                  require.ErrorAssertionFunc
		wantMigrationStatus        api.MigrationStatusType
		wantMigrationStatusMessage string
		wantWorkerCommand          migration.WorkerCommand
	}{
		{
			name:    "success - no update due to concurrency limit on source",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName: migration.Batch{Target: "one", Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte(`{"import_limit": 1}`),
			},

			sourceImportLimit: 1,

			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_IDLE,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte(`{"import_limit": 1}`)},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for other instances to finish importing",
		},
		{
			name:    "success - no update due to concurrency limit on source and target",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName: migration.Batch{Target: "one", Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte(`{"import_limit": 1}`),
			},

			sourceImportLimit: 1,
			targetImportLimit: 1,

			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte(`{"import_limit": 1}`),
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_IDLE,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte(`{"import_limit": 1}`)},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for other instances to finish importing",
		},
		{
			name:    "success - no update due to concurrency limit on target, but unmet on source",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName: migration.Batch{Target: "one", Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte(`{"import_limit": 1}`),
			},

			targetImportLimit: 1,

			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte(`{"import_limit": 1}`),
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_IDLE,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte(`{"import_limit": 1}`)},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for other instances to finish importing",
		},
		{
			name:    "success - update due to unmet concurrency limits",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName: migration.Batch{Target: "one", Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte(`{"import_limit": 1}`),
			},

			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte(`{"import_limit": 1}`),
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte(`{"import_limit": 1}`)},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "success - without migration window start time",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName: migration.Batch{Target: "one", Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},

			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "success - without migration window start time, with matching constraint",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName:    migration.Batch{Target: "one", Name: "one", Constraints: []migration.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:                  "success - background disk sync",
			uuidArg:               uuidA,
			batchSvcGetByName:     migration.Batch{Target: "one", Name: "one"},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", NeedsDiskImport: true, MigrationStatus: api.MIGRATIONSTATUS_IDLE},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:         "/some/instance/A",
					OS:               "ubuntu",
					OSVersion:        "24.04",
					BackgroundImport: true,
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_IMPORT_DISKS,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source: migration.Source{
					ID:         1,
					Name:       "one",
					SourceType: api.SOURCETYPE_VMWARE,
					Properties: []byte("{}"),
				},
				OS:        "ubuntu",
				OSVersion: "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_BACKGROUND_IMPORT),
		},
		{
			name:                  "success - migration window started",
			uuidArg:               uuidA,
			batchSvcGetByName:     migration.Batch{Target: "one", Name: "one"},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:         "/some/instance/A",
					OS:               "ubuntu",
					OSVersion:        "24.04",
					BackgroundImport: true,
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},
			batchSvcGetWindows: migration.MigrationWindows{{Start: time.Now().Add(-time.Minute)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source: migration.Source{
					ID:         1,
					Name:       "one",
					SourceType: api.SOURCETYPE_VMWARE,
					Properties: []byte("{}"),
				},
				OS:        "ubuntu",
				OSVersion: "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "success - migration window started, with matching constraint",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName:    migration.Batch{Target: "one", Name: "one", Constraints: []migration.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:         "/some/instance/A",
					OS:               "ubuntu",
					OSVersion:        "24.04",
					BackgroundImport: true,
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},
			batchSvcGetWindows: migration.MigrationWindows{{Start: time.Now().Add(-time.Minute)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "success - migration window started, with matching constraint and min migration time",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName:    migration.Batch{Target: "one", Name: "one", Constraints: []migration.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:         "/some/instance/A",
					OS:               "ubuntu",
					OSVersion:        "24.04",
					BackgroundImport: true,
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},
			batchSvcGetWindows: migration.MigrationWindows{{Start: time.Now().Add(-time.Minute), End: time.Now().Add(time.Hour * 2)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "no change - migration window started, with matching constraint and min migration time not met",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName:    migration.Batch{Target: "one", Name: "one", Constraints: []migration.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:         "/some/instance/A",
					OS:               "ubuntu",
					OSVersion:        "24.04",
					BackgroundImport: true,
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},
			batchSvcGetWindows: migration.MigrationWindows{{Start: time.Now().Add(-time.Minute), End: time.Now().Add(time.Hour)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_IDLE,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "no change - migration window started, with matching constraint and concurrent instances exceeded",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}, {InstanceUUID: uuidB, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_FINAL_IMPORT}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},

			batchSvcGetByName:    migration.Batch{Name: "one", Constraints: []migration.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}, {UUID: uuidB}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:         "/some/instance/A",
					OS:               "ubuntu",
					OSVersion:        "24.04",
					BackgroundImport: true,
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte(`{}`),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},
			batchSvcGetWindows: migration.MigrationWindows{{Start: time.Now().Add(-time.Minute), End: time.Now().Add(time.Hour * 2)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_IDLE,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:                  "error - instance.GetByID",
			uuidArg:               uuidA,
			instanceSvcGetByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                  "error - queue is not in idle state",
			uuidArg:               uuidA,
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_BACKGROUND_IMPORT},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:                  "error - source.GetByID",
			uuidArg:               uuidA,
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                  "error - batch.GetByName",
			uuidArg:               uuidA,
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},
			batchSvcGetByNameErr:  boom.Error,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},

			assertErr: boom.ErrorIs,
		},
		{
			name:                  "error - target.GetByName",
			uuidArg:               uuidA,
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},
			batchSvcGetByName:     migration.Batch{Name: "one", Constraints: []migration.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour}}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                  "error - batch.GetEarliestWindow",
			uuidArg:               uuidA,
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE},
			batchSvcGetByName:     migration.Batch{Name: "one", Constraints: []migration.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour}}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},
			batchSvcGetWindowsErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                     "error - repo.GetByInstanceUUID",
			repoGetByInstanceUUIDErr: boom.Error,
			uuidArg:                  uuidA,

			assertErr: boom.ErrorIs,
		},
		{
			name:                  "error - repo.Update",
			uuidArg:               uuidA,
			batchSvcGetByName:     migration.Batch{Name: "one", Constraints: []migration.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour}}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", NeedsDiskImport: true, MigrationStatus: api.MIGRATIONSTATUS_IDLE},
			instanceSvcGetByIDInstance: migration.Instance{
				Source: "one",
				UUID:   uuidA,
				Properties: api.InstanceProperties{
					Location:         "/some/instance/A",
					OS:               "ubuntu",
					OSVersion:        "24.04",
					BackgroundImport: true,
				},
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: []byte("{}"),
			},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},
			repoUpdateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mock.QueueRepoMock{
				GetByInstanceUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.QueueEntry, error) {
					return &tc.repoGetByInstanceUUID, tc.repoGetByInstanceUUIDErr
				},

				UpdateFunc: func(ctx context.Context, entry migration.QueueEntry) error {
					return tc.repoUpdateErr
				},

				GetAllByBatchAndStateFunc: func(ctx context.Context, batch string, statuses ...api.MigrationStatusType) (migration.QueueEntries, error) {
					return tc.repoGetAll, tc.repoGetAllErr
				},
			}
			// Setup
			instanceSvc := &InstanceServiceMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return &tc.instanceSvcGetByIDInstance, tc.instanceSvcGetByIDErr
				},
				GetAllQueuedFunc: func(ctx context.Context, queue migration.QueueEntries) (migration.Instances, error) {
					return tc.instanceSvcGetQueued, tc.instanceSvcGetQueuedErr
				},
			}

			sourceSvc := &SourceServiceMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Source, error) {
					return &tc.sourceSvcGetByIDSource, tc.sourceSvcGetByIDErr
				},
				RecordActiveImportFunc: func(targetName string) {},
				GetCachedImportsFunc:   func(sourceName string) int { return tc.sourceImportLimit },
			}

			batchSvc := &BatchServiceMock{
				GetMigrationWindowsFunc: func(ctx context.Context, batch string) (migration.MigrationWindows, error) {
					return tc.batchSvcGetWindows, tc.batchSvcGetWindowsErr
				},
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return &tc.batchSvcGetByName, tc.batchSvcGetByNameErr
				},
			}

			targetSvc := &TargetServiceMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Target, error) {
					return &tc.targetSvcGetByIDTarget, tc.targetSvcGetByIDErr
				},
				RecordActiveImportFunc: func(targetName string) {},
				GetCachedImportsFunc:   func(targetName string) int { return tc.targetImportLimit },
			}

			queueSvc := migration.NewQueueService(repo, batchSvc, instanceSvc, sourceSvc, targetSvc)

			// Run test
			workerCommand, err := queueSvc.NewWorkerCommandByInstanceUUID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantWorkerCommand, workerCommand)
		})
	}
}

func TestQueueService_ProcessWorkerUpdate(t *testing.T) {
	tests := []struct {
		name                  string
		uuidArg               uuid.UUID
		workerResponseTypeArg api.WorkerResponseType
		statusStringArg       string

		repoGetByUUIDQueueEntry          *migration.QueueEntry
		repoGetByUUIDErr                 error
		repoUpdateStatusByUUIDQueueEntry *migration.QueueEntry
		repoUpdateStatusByUUIDErr        error

		instanceSvcGetByUUIDInstance migration.Instance
		instanceSvcGetByUUIDErr      error

		batchSvcGetByNameBatch migration.Batch
		batchSvcGetByNameErr   error

		assertErr                  require.ErrorAssertionFunc
		wantMigrationStatus        api.MigrationStatusType
		wantMigrationStatusMessage string
	}{
		{
			name:                  "success - migration running",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_RUNNING,
			statusStringArg:       "creating",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID:    uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				BatchName:       "one",
			},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_CREATING,
			wantMigrationStatusMessage: "creating",
		},
		{
			name:                  "success - migration success background import",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_SUCCESS,
			statusStringArg:       "done",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
				BatchName:       "one",
			},
			instanceSvcGetByUUIDInstance: migration.Instance{UUID: uuidA, Source: "one"},
			batchSvcGetByNameBatch:       migration.Batch{Name: "one", Target: "one"},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for migration window",
		},
		{
			name:                  "success - migration success final import",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_SUCCESS,
			statusStringArg:       "done",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_FINAL_IMPORT,
				BatchName:       "one",
			},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_IMPORT_COMPLETE,
			wantMigrationStatusMessage: "Starting target instance",
		},
		{
			name:                  "success - migration failed",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_FAILED,
			statusStringArg:       "boom!",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				BatchName:       "one",
			},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_ERROR,
			wantMigrationStatusMessage: "boom!",
		},
		{
			name:                  "error - GetByUUID",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_RUNNING,
			statusStringArg:       "creating",
			repoGetByUUIDErr:      boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                  "error - instance not in migration state",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_RUNNING,
			statusStringArg:       "creating",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_FINISHED, // not in migration state
				BatchName:       "one",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrNotFound, a...)
			},
		},
		{
			name:                  "error - UpdateStatusByUUID",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_RUNNING,
			statusStringArg:       "creating",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				BatchName:       "one",
			},
			repoUpdateStatusByUUIDErr: boom.Error,

			assertErr:                  boom.ErrorIs,
			wantMigrationStatus:        api.MIGRATIONSTATUS_CREATING,
			wantMigrationStatusMessage: "creating",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.QueueRepoMock{
				GetByInstanceUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.QueueEntry, error) {
					return tc.repoGetByUUIDQueueEntry, tc.repoGetByUUIDErr
				},
				UpdateFunc: func(ctx context.Context, i migration.QueueEntry) error {
					require.Equal(t, tc.wantMigrationStatus, i.MigrationStatus)
					require.Equal(t, tc.wantMigrationStatusMessage, i.MigrationStatusMessage)
					return tc.repoUpdateStatusByUUIDErr
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return &tc.instanceSvcGetByUUIDInstance, tc.instanceSvcGetByUUIDErr
				},
			}

			batchSvc := &BatchServiceMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return &tc.batchSvcGetByNameBatch, tc.batchSvcGetByNameErr
				},
			}

			sourceSvc := &SourceServiceMock{
				RemoveActiveImportFunc: func(sourceName string) {},
			}

			targetSvc := &TargetServiceMock{
				RemoveActiveImportFunc: func(targetName string) {},
			}

			queueSvc := migration.NewQueueService(repo, batchSvc, instanceSvc, sourceSvc, targetSvc)

			// Run test
			_, err := queueSvc.ProcessWorkerUpdate(context.Background(), tc.uuidArg, tc.workerResponseTypeArg, tc.statusStringArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}
