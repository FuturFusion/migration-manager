package migration_test

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"
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

			queueSvc := migration.NewQueueService(repo, nil, nil, nil, nil, nil)

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

			queueSvc := migration.NewQueueService(repo, nil, nil, nil, nil, nil)

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

		batchSvcGetWindows    migration.Windows
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

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName: migration.Batch{Defaults: defaultPlacement, Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
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
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for other instances to finish importing",
		},
		{
			name:    "success - no update due to concurrency limit on source and target",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName: migration.Batch{Defaults: defaultPlacement, Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
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
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for other instances to finish importing",
		},
		{
			name:    "success - no update due to concurrency limit on target, but unmet on source",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName: migration.Batch{Defaults: defaultPlacement, Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
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
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for other instances to finish importing",
		},
		{
			name:    "success - update due to unmet concurrency limits",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName: migration.Batch{Defaults: defaultPlacement, Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
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
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "success - without migration window start time",
			uuidArg: uuidA,

			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName: migration.Batch{Defaults: defaultPlacement, Name: "one"},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
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
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "success - without migration window start time, with matching constraint",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName:    migration.Batch{Defaults: defaultPlacement, Name: "one", Constraints: []api.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
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
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:                  "success - background disk sync",
			uuidArg:               uuidA,
			batchSvcGetByName:     migration.Batch{Defaults: defaultPlacement, Name: "one"},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", ImportStage: migration.IMPORTSTAGE_BACKGROUND, MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               true,
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
				OSType:    api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_BACKGROUND_IMPORT),
		},
		{
			name:                  "success - migration window started (perform final import)",
			uuidArg:               uuidA,
			batchSvcGetByName:     migration.Batch{Defaults: defaultPlacement, Name: "one"},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, ImportStage: migration.IMPORTSTAGE_FINAL, Placement: api.Placement{TargetName: "one"}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               true,
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
			batchSvcGetWindows: migration.Windows{{Name: "w1", Start: time.Now().Add(-time.Minute)}},

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
				OSType:    api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:                  "success - migration window started (perform full initial import)",
			uuidArg:               uuidA,
			batchSvcGetByName:     migration.Batch{Defaults: defaultPlacement, Name: "one"},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, ImportStage: migration.IMPORTSTAGE_BACKGROUND, Placement: api.Placement{TargetName: "one"}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               false,
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
			batchSvcGetWindows: migration.Windows{{Name: "w1", Start: time.Now().Add(-time.Minute)}},

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
				OSType:    api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:                  "success - migration window started (perform post import)",
			uuidArg:               uuidA,
			batchSvcGetByName:     migration.Batch{Defaults: defaultPlacement, Name: "one"},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, ImportStage: migration.IMPORTSTAGE_COMPLETE, Placement: api.Placement{TargetName: "one"}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               true,
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
			batchSvcGetWindows: migration.Windows{{Name: "w1", Start: time.Now().Add(-time.Minute)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_POST_IMPORT,
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
				OSType:    api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_POST_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_POST_IMPORT),
		},
		{
			name:                  "success - migration window started (perform post import with full import)",
			uuidArg:               uuidA,
			batchSvcGetByName:     migration.Batch{Defaults: defaultPlacement, Name: "one"},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, ImportStage: migration.IMPORTSTAGE_COMPLETE, Placement: api.Placement{TargetName: "one"}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               false,
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
			batchSvcGetWindows: migration.Windows{{Name: "w1", Start: time.Now().Add(-time.Minute)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_POST_IMPORT,
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
				OSType:    api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_POST_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_POST_IMPORT),
		},
		{
			name:    "success - migration window started, with matching constraint",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName:    migration.Batch{Defaults: defaultPlacement, Name: "one", Constraints: []api.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               true,
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
			batchSvcGetWindows: migration.Windows{{Name: "w1", Start: time.Now().Add(-time.Minute)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "success - migration window started, with matching constraint and min migration time",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName:    migration.Batch{Defaults: defaultPlacement, Name: "one", Constraints: []api.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour.String()}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               true,
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
			batchSvcGetWindows: migration.Windows{{Name: "w1", Start: time.Now().Add(-time.Minute), End: time.Now().Add(time.Hour * 2)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "no change - migration window started, with matching constraint and min migration time not met",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName:    migration.Batch{Defaults: defaultPlacement, Name: "one", Constraints: []api.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour.String()}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               true,
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
			batchSvcGetWindows: migration.Windows{{Name: "w1", Start: time.Now().Add(-time.Minute), End: time.Now().Add(time.Hour)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_IDLE,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
				OSType:     api.OSTYPE_LINUX,
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "no change - migration window started, with matching constraint and concurrent instances exceeded",
			uuidArg: uuidA,

			repoGetAll:            migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE}, {InstanceUUID: uuidB, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_FINAL_IMPORT}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},

			batchSvcGetByName:    migration.Batch{Name: "one", Constraints: []api.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour.String()}}},
			instanceSvcGetQueued: migration.Instances{{UUID: uuidA}, {UUID: uuidB}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               true,
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
			batchSvcGetWindows: migration.Windows{{Name: "w1", Start: time.Now().Add(-time.Minute), End: time.Now().Add(time.Hour * 2)}},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_IDLE,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source:     migration.Source{ID: 1, Name: "one", SourceType: api.SOURCETYPE_VMWARE, Properties: []byte("{}")},
				OS:         "ubuntu",
				OSVersion:  "24.04",
				OSType:     api.OSTYPE_LINUX,
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
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04", Name: "A"},
				},
			},
			sourceSvcGetByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                  "error - batch.GetByName",
			uuidArg:               uuidA,
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},
			targetSvcGetByIDTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: []byte("{}"),
			},
			batchSvcGetByNameErr: boom.Error,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04", Name: "A"},
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
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},
			batchSvcGetByName:     migration.Batch{Name: "one", Constraints: []api.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour.String()}}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04", Name: "A"},
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
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},
			batchSvcGetByName:     migration.Batch{Name: "one", Constraints: []api.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour.String()}}},
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:   uuidA,
				Source: "one",
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04", Name: "A"},
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
			batchSvcGetByName:     migration.Batch{Name: "one", Constraints: []api.BatchConstraint{{IncludeExpression: "true", MaxConcurrentInstances: 1, MinInstanceBootTime: time.Hour.String()}}},
			repoGetByInstanceUUID: migration.QueueEntry{InstanceUUID: uuidA, BatchName: "one", ImportStage: migration.IMPORTSTAGE_BACKGROUND, MigrationStatus: api.MIGRATIONSTATUS_IDLE, Placement: api.Placement{TargetName: "one"}},
			instanceSvcGetByIDInstance: migration.Instance{
				Source: "one",
				UUID:   uuidA,
				Properties: api.InstanceProperties{
					Location:                       "/some/instance/A",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{OS: "ubuntu", OSVersion: "24.04"},
					BackgroundImport:               true,
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

				GetAllFunc: func(ctx context.Context) (migration.QueueEntries, error) {
					return tc.repoGetAll, nil
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

			windowSvc := &WindowServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batchName string) (migration.Windows, error) {
					return tc.batchSvcGetWindows, tc.batchSvcGetWindowsErr
				},
			}

			queueSvc := migration.NewQueueService(repo, batchSvc, instanceSvc, sourceSvc, targetSvc, windowSvc)

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
		wantImportStage            migration.ImportStage
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
				ImportStage:     migration.IMPORTSTAGE_BACKGROUND,
			},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_CREATING,
			wantMigrationStatusMessage: "creating",
			wantImportStage:            migration.IMPORTSTAGE_BACKGROUND,
		},
		{
			name:                  "success - migration success background import",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_SUCCESS,
			statusStringArg:       "done",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
				ImportStage:     migration.IMPORTSTAGE_BACKGROUND,
				BatchName:       "one",
				Placement:       api.Placement{TargetName: "one"},
			},
			instanceSvcGetByUUIDInstance: migration.Instance{UUID: uuidA, Source: "one"},
			batchSvcGetByNameBatch:       migration.Batch{Name: "one", Defaults: defaultPlacement},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for migration window",
			wantImportStage:            migration.IMPORTSTAGE_FINAL,
		},
		{
			name:                  "success - migration success final import (full initial import)",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_SUCCESS,
			statusStringArg:       "done",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_FINAL_IMPORT,
				BatchName:       "one",
				ImportStage:     migration.IMPORTSTAGE_BACKGROUND,
				Placement:       api.Placement{TargetName: "one"},
			},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for worker to begin post-import tasks",
			wantImportStage:            migration.IMPORTSTAGE_COMPLETE,
		},
		{
			name:                  "success - migration success final import (incremental import)",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_SUCCESS,
			statusStringArg:       "done",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_FINAL_IMPORT,
				BatchName:       "one",
				ImportStage:     migration.IMPORTSTAGE_FINAL,
				Placement:       api.Placement{TargetName: "one"},
			},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusMessage: "Waiting for worker to begin post-import tasks",
			wantImportStage:            migration.IMPORTSTAGE_COMPLETE,
		},
		{
			name:                  "success - migration success post import",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_SUCCESS,
			statusStringArg:       "done",
			repoGetByUUIDQueueEntry: &migration.QueueEntry{
				InstanceUUID: uuidA,

				MigrationStatus: api.MIGRATIONSTATUS_POST_IMPORT,
				BatchName:       "one",
				ImportStage:     migration.IMPORTSTAGE_COMPLETE,
				Placement:       api.Placement{TargetName: "one"},
			},

			assertErr:                  require.NoError,
			wantMigrationStatus:        api.MIGRATIONSTATUS_WORKER_DONE,
			wantMigrationStatusMessage: "Starting target instance",
			wantImportStage:            migration.IMPORTSTAGE_COMPLETE,
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
				Placement:       api.Placement{TargetName: "one"},
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
				Placement:       api.Placement{TargetName: "one"},
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
				Placement:       api.Placement{TargetName: "one"},
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
					require.Equal(t, tc.wantImportStage, i.ImportStage)
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

			queueSvc := migration.NewQueueService(repo, batchSvc, instanceSvc, sourceSvc, targetSvc, nil)

			// Run test
			_, err := queueSvc.ProcessWorkerUpdate(context.Background(), tc.uuidArg, tc.workerResponseTypeArg, tc.statusStringArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestQueueService_GetNextWindow(t *testing.T) {
	type window struct {
		s int
		e int
		c int
	}

	toWindows := func(ws []window, t time.Time) migration.Windows {
		windows := make([]migration.Window, len(ws))
		for i, w := range ws {
			windows[i] = migration.Window{
				ID:    int64(i),
				Name:  "w" + strconv.Itoa(i),
				Start: t.Add(time.Duration(w.s) * time.Minute),
				End:   t.Add(time.Duration(w.e) * time.Minute),
				Config: api.MigrationWindowConfig{
					Capacity: w.c,
				},
			}
		}

		return windows
	}

	cases := []struct {
		name        string
		queueEntry  migration.QueueEntry
		constraints []api.BatchConstraint

		matchingInstances    []int // slice where the index represents the constraint index, and the value is the number of matching instances.
		notMatchingInstances []int // slice where the index represents the constraint index, and the value is the number of non-matching instances.
		targetExprValue      int   // corresponds to index-1 of the matching constraint (0 is none).
		windows              []window
		waitingEntries       map[string]int // number of entries already assigned to a particular window name.

		wantWindowIndex int
		assertErr       require.ErrorAssertionFunc
	}{
		{
			name:                 "success - no constraints",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{},
			matchingInstances:    []int{},
			notMatchingInstances: []int{},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - no constraints, matches already started window",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{},
			matchingInstances:    []int{},
			notMatchingInstances: []int{},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 20}, {s: -30, e: 40}},
			wantWindowIndex:      1,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - no constraints, earlier window is at capacity",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{},
			matchingInstances:    []int{},
			notMatchingInstances: []int{},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 20, c: 10}, {s: -30, e: 40, c: 10}},
			wantWindowIndex:      0,
			waitingEntries:       map[string]int{"w0": 5, "w1": 10},
			assertErr:            require.NoError,
		},
		{
			name:                 "success - no constraints, earlier window is at capacity, but already assigned to this instance",
			queueEntry:           migration.QueueEntry{MigrationWindowName: sql.NullString{Valid: true, String: "w1"}},
			constraints:          []api.BatchConstraint{},
			matchingInstances:    []int{},
			notMatchingInstances: []int{},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 20, c: 10}, {s: -30, e: 40, c: 10}},
			wantWindowIndex:      1,
			waitingEntries:       map[string]int{"w0": 5, "w1": 10},
			assertErr:            require.NoError,
		},
		{
			name:                 "success - window already assigned",
			queueEntry:           migration.QueueEntry{MigrationWindowName: sql.NullString{Valid: true, String: "w1"}},
			constraints:          []api.BatchConstraint{},
			matchingInstances:    []int{},
			notMatchingInstances: []int{},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      1,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - window already assigned, but ended",
			queueEntry:           migration.QueueEntry{MigrationWindowName: sql.NullString{Valid: true, String: "w1"}},
			constraints:          []api.BatchConstraint{},
			matchingInstances:    []int{},
			notMatchingInstances: []int{},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 20}, {s: -30, e: -40}, {s: -30, e: 2}},
			wantWindowIndex:      2,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - constraint matches but limit not reached (no other instances)",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MaxConcurrentInstances: 10}},
			matchingInstances:    []int{},
			notMatchingInstances: []int{},
			targetExprValue:      1,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - constraint matches but limit not reached (no other instances)",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MaxConcurrentInstances: 1}},
			matchingInstances:    []int{},
			notMatchingInstances: []int{},
			targetExprValue:      1,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - constraint matches only other instances",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MaxConcurrentInstances: 3}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - constraint matches only other instances, with some not matching",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MaxConcurrentInstances: 3}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{3},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - constraint matches target and other instances, with some not matching",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MaxConcurrentInstances: 4}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{3},
			targetExprValue:      1,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - last constraint considered first - sequential match",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MaxConcurrentInstances: 3}, {IncludeExpression: "cpus == 2", MaxConcurrentInstances: 3}, {IncludeExpression: "cpus == 3", MaxConcurrentInstances: 3}},
			matchingInstances:    []int{3, 3, 2},
			notMatchingInstances: []int{3, 3, 3},
			targetExprValue:      3,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - last constraint considered first",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 2", MaxConcurrentInstances: 1}, {IncludeExpression: "cpus == 2", MaxConcurrentInstances: 3}},
			matchingInstances:    []int{0, 2},
			notMatchingInstances: []int{3, 3},
			targetExprValue:      2,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - matching constraints with unlimited concurrency",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1"}, {IncludeExpression: "cpus == 2"}},
			matchingInstances:    []int{5, 5},
			notMatchingInstances: []int{3, 3},
			targetExprValue:      1,
			windows:              []window{{s: 10, e: 20}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - matches constraint with boot time forcing later window",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MinInstanceBootTime: (time.Minute * 5).String()}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{3},
			targetExprValue:      1,
			windows:              []window{{s: 10, e: 14}, {s: 30, e: 40}},
			wantWindowIndex:      1,
			assertErr:            require.NoError,
		},
		{
			name:                 "success - matches constraint with boot time forcing later window, respects time left",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MinInstanceBootTime: (time.Minute * 5).String()}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{3},
			targetExprValue:      1,
			windows:              []window{{s: 10, e: 14}, {s: -30, e: 6}, {s: -30, e: 7}, {s: 1, e: 8}}, // expects >1 minute buffer.
			wantWindowIndex:      2,                                                                      // picks the earliest valid window.
			assertErr:            require.NoError,
		},
		{
			name:                 "success - non-matching constraint with boot time using earlier window",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MinInstanceBootTime: (time.Minute * 5).String()}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{3},
			targetExprValue:      0,
			windows:              []window{{s: 10, e: 14}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr:            require.NoError,
		},
		{
			name:                 "error - constraint limit reached",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MaxConcurrentInstances: 3}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{3},
			targetExprValue:      1,
			windows:              []window{{s: 10, e: 14}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr: func(tt require.TestingT, err error, i ...any) {
				require.True(t, incusAPI.StatusErrorCheck(err, http.StatusNotFound))
			},
		},
		{
			name:                 "error - constraint limit reached, earlier matches ignored",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 2", MaxConcurrentInstances: 10}, {IncludeExpression: "cpus == 2", MaxConcurrentInstances: 3}},
			matchingInstances:    []int{0, 3},
			notMatchingInstances: []int{3, 3},
			targetExprValue:      2,
			windows:              []window{{s: 10, e: 14}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr: func(tt require.TestingT, err error, i ...any) {
				require.True(t, incusAPI.StatusErrorCheck(err, http.StatusNotFound))
			},
		},
		{
			name:                 "error - no valid window for boot time",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1", MinInstanceBootTime: time.Hour.String()}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{3},
			targetExprValue:      1,
			windows:              []window{{s: 10, e: 14}, {s: 30, e: 40}},
			wantWindowIndex:      0,
			assertErr: func(tt require.TestingT, err error, i ...any) {
				require.True(t, incusAPI.StatusErrorCheck(err, http.StatusNotFound))
			},
		},
		{
			name:                 "error - no valid window with no restrictions",
			queueEntry:           migration.QueueEntry{},
			constraints:          []api.BatchConstraint{{IncludeExpression: "cpus == 1"}},
			matchingInstances:    []int{3},
			notMatchingInstances: []int{3},
			targetExprValue:      1,
			windows:              []window{{s: -10, e: -14}, {s: -30, e: -40}, {s: -35, e: 1}, {s: 10, e: 11}}, // all windows ended, or fail >1 min buffer requirement.
			wantWindowIndex:      0,
			assertErr: func(tt require.TestingT, err error, i ...any) {
				require.True(t, incusAPI.StatusErrorCheck(err, http.StatusNotFound))
			},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)

			// sanity check.
			require.LessOrEqual(t, len(tc.matchingInstances), len(tc.constraints))
			require.LessOrEqual(t, len(tc.notMatchingInstances), len(tc.constraints))
			require.LessOrEqual(t, tc.targetExprValue, len(tc.constraints))

			now := time.Now().UTC()
			windows := toWindows(tc.windows, now)
			tc.queueEntry.InstanceUUID = uuid.New()
			repo := &mock.QueueRepoMock{
				GetAllByBatchAndStateFunc: func(ctx context.Context, batch string, statuses ...api.MigrationStatusType) (migration.QueueEntries, error) {
					entries := []migration.QueueEntry{tc.queueEntry}
					for _, count := range tc.matchingInstances {
						for i := 0; i < count; i++ {
							entries = append(entries, migration.QueueEntry{})
						}
					}

					for _, count := range tc.notMatchingInstances {
						for i := 0; i < count; i++ {
							entries = append(entries, migration.QueueEntry{})
						}
					}

					return entries, nil
				},

				GetAllFunc: func(ctx context.Context) (migration.QueueEntries, error) {
					entries := migration.QueueEntries{}
					for wName, count := range tc.waitingEntries {
						if tc.queueEntry.MigrationWindowName.String == wName {
							entries = append(entries, tc.queueEntry)
							count = count - 1
						}

						for i := 0; i < count; i++ {
							entries = append(entries, migration.QueueEntry{MigrationWindowName: sql.NullString{Valid: true, String: wName}})
						}
					}

					return entries, nil
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllQueuedFunc: func(ctx context.Context, queue migration.QueueEntries) (migration.Instances, error) {
					targetInstance := migration.Instance{
						UUID:       tc.queueEntry.InstanceUUID,
						Properties: api.InstanceProperties{InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{CPUs: int64(tc.targetExprValue)}},
					}

					instances := []migration.Instance{targetInstance}
					for idx, count := range tc.matchingInstances {
						for i := 0; i < count; i++ {
							instances = append(instances, migration.Instance{Properties: api.InstanceProperties{InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{CPUs: int64(idx) + 1}}})
						}
					}

					for _, count := range tc.notMatchingInstances {
						for i := 0; i < count; i++ {
							instances = append(instances, migration.Instance{})
						}
					}

					return instances, nil
				},
			}

			batchSvc := &BatchServiceMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return &migration.Batch{Constraints: tc.constraints}, nil
				},
			}

			windowSvc := &WindowServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batchName string) (migration.Windows, error) {
					return windows, nil
				},
			}

			queueSvc := migration.NewQueueService(repo, batchSvc, instanceSvc, nil, nil, windowSvc)
			w, err := queueSvc.GetNextWindow(context.Background(), tc.queueEntry)
			tc.assertErr(t, err)
			if err == nil {
				require.Equal(t, &windows[tc.wantWindowIndex], w)
			}
		})
	}
}
