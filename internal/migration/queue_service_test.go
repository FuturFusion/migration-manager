package migration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/internal/testing/queue"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestQueueService_GetAll(t *testing.T) {
	tests := []struct {
		name                       string
		batchSvcGetAllBatches      migration.Batches
		batchSvcGetAllErr          error
		instanceSvcGetAllByBatchID []queue.Item[migration.Instances]

		assertErr      require.ErrorAssertionFunc
		wantQueueItems migration.QueueEntries
	}{
		{
			name:                  "success - no batches",
			batchSvcGetAllBatches: nil,

			assertErr: require.NoError,
		},
		{
			name: "success - with batches",
			batchSvcGetAllBatches: migration.Batches{
				{
					ID:     2,
					Name:   "two",
					Status: api.BATCHSTATUS_DEFINED, // this batch is ignored
				},
				{
					ID:     3,
					Name:   "three",
					Status: api.BATCHSTATUS_QUEUED,
				},
				{
					ID:     4,
					Name:   "four",
					Status: api.BATCHSTATUS_RUNNING,
				},
				{
					ID:     5,
					Name:   "five",
					Status: api.BATCHSTATUS_RUNNING,
				},
			},
			instanceSvcGetAllByBatchID: []queue.Item[migration.Instances]{
				// Instances for batch 3
				{
					Value: migration.Instances{
						{
							UUID:                   uuidA,
							Properties:             api.InstanceProperties{Location: "/some/instance/A", Name: "A"},
							MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
							MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
							Batch:                  ptr.To("three"),
						},
					},
				},
				// Instances for batch 4
				{
					Value: migration.Instances{
						{
							UUID:                   uuidB,
							Properties:             api.InstanceProperties{Location: "/some/instance/B", Name: "B"},
							MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
							MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
							Batch:                  ptr.To("four"),
						},
					},
				},
				// Instances for batch 5
				{
					Value: migration.Instances{
						{
							UUID:                   uuidC,
							Properties:             api.InstanceProperties{Location: "/some/instance/C", Name: "C"},
							MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
							MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
							Batch:                  ptr.To("five"),
						},
					},
				},
			},

			assertErr: require.NoError,
			wantQueueItems: migration.QueueEntries{
				{
					InstanceUUID:           uuidA,
					InstanceName:           "A",
					MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
					MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
					BatchName:              "three",
				},
				{
					InstanceUUID:           uuidB,
					InstanceName:           "B",
					MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
					MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
					BatchName:              "four",
				},
				{
					InstanceUUID:           uuidC,
					InstanceName:           "C",
					MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
					MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
					BatchName:              "five",
				},
			},
		},
		{
			name:              "error - batch.GetAll",
			batchSvcGetAllErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - instance.GetAllByBatchID",
			batchSvcGetAllBatches: migration.Batches{
				{
					ID:     1,
					Name:   "one",
					Status: api.BATCHSTATUS_RUNNING,
				},
			},
			instanceSvcGetAllByBatchID: []queue.Item[migration.Instances]{
				{
					Err: boom.Error,
				},
			},

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			batchSvc := &BatchServiceMock{
				GetAllFunc: func(ctx context.Context) (migration.Batches, error) {
					return tc.batchSvcGetAllBatches, tc.batchSvcGetAllErr
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string, withOverrides bool) (migration.Instances, error) {
					return queue.Pop(t, &tc.instanceSvcGetAllByBatchID)
				},
			}

			queueSvc := migration.NewQueueService(batchSvc, instanceSvc, nil)

			// Run test
			queueItems, err := queueSvc.GetAll(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantQueueItems, queueItems)

			// Ensure queues are completely drained.
			require.Empty(t, tc.instanceSvcGetAllByBatchID)
		})
	}
}

func TestInstanceService_GetByInstanceID(t *testing.T) {
	tests := []struct {
		name                       string
		uuidArg                    uuid.UUID
		instanceSvcGetByIDInstance migration.Instance
		instanceSvcGetByIDErr      error
		batchSvcGetByIDBatch       migration.Batch
		batchSvcGetByIDErr         error

		assertErr      require.ErrorAssertionFunc
		wantQueueEntry migration.QueueEntry
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:                   uuidA,
				Properties:             api.InstanceProperties{Location: "/some/instance/A", Name: "A"},
				MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
				Batch:                  ptr.To("one"),
			},
			batchSvcGetByIDBatch: migration.Batch{
				ID:   1,
				Name: "one",
			},

			assertErr: require.NoError,
			wantQueueEntry: migration.QueueEntry{
				InstanceUUID:           uuidA,
				InstanceName:           "A",
				MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
				BatchName:              "one",
			},
		},
		{
			name:                  "error - instance.GetByID",
			uuidArg:               uuidA,
			instanceSvcGetByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - instance not assigned to batch",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:                   uuidA,
				Properties:             api.InstanceProperties{Location: "/some/instance/A", Name: "A"},
				MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
				Batch:                  nil, // not assigned to batch
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrNotFound, a...)
			},
		},
		{
			name:    "error - instance not in migration state",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:                   uuidA,
				Properties:             api.InstanceProperties{Location: "/some/instance/A", Name: "A"},
				MigrationStatus:        api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, // not in migration state
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH),
				Batch:                  ptr.To("one"),
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrNotFound, a...)
			},
		},
		{
			name:    "error - batch.GetByID",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID:                   uuidA,
				Properties:             api.InstanceProperties{Location: "/some/instance/A", Name: "A"},
				MigrationStatus:        api.MIGRATIONSTATUS_CREATING,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_CREATING),
				Batch:                  ptr.To("one"),
			},
			batchSvcGetByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			instanceSvc := &InstanceServiceMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID, withOverrides bool) (*migration.Instance, error) {
					return &tc.instanceSvcGetByIDInstance, tc.instanceSvcGetByIDErr
				},
			}

			batchSvc := &BatchServiceMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return &tc.batchSvcGetByIDBatch, tc.batchSvcGetByIDErr
				},
			}

			queueSvc := migration.NewQueueService(batchSvc, instanceSvc, nil)

			// Run test
			queueEntry, err := queueSvc.GetByInstanceID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantQueueEntry, queueEntry)
		})
	}
}

func TestInstanceService_NewWorkerCommandByInstanceUUID(t *testing.T) {
	tests := []struct {
		name                           string
		uuidArg                        uuid.UUID
		instanceSvcGetByIDInstance     migration.Instance
		instanceSvcGetByIDErr          error
		sourceSvcGetByIDSource         migration.Source
		sourceSvcGetByIDErr            error
		batchSvcGetByIDBatch           migration.Batch
		batchSvcGetByIDErr             error
		instanceSvcUpdateStatusByIDErr error

		assertErr                  require.ErrorAssertionFunc
		wantMigrationStatus        api.MigrationStatusType
		wantMigrationStatusMessage string
		wantWorkerCommand          migration.WorkerCommand
	}{
		{
			name:    "success - without migration window start time",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID: uuidA,
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
					Disks:     []api.InstancePropertiesDisk{{}},
				},
				MigrationStatus:        api.MIGRATIONSTATUS_IDLE,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_IDLE),
				Batch:                  ptr.To("one"),
				NeedsDiskImport:        true,
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
			},
			batchSvcGetByIDBatch: migration.Batch{
				ID:   2,
				Name: "two",
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source: migration.Source{
					ID:         1,
					Name:       "one",
					SourceType: api.SOURCETYPE_VMWARE,
				},
				OS:        "ubuntu",
				OSVersion: "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
		{
			name:    "success - background disk sync",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID: uuidA,
				Properties: api.InstanceProperties{
					Location:         "/some/instance/A",
					Name:             "A",
					OS:               "ubuntu",
					OSVersion:        "24.04",
					BackgroundImport: true,
					Disks:            []api.InstancePropertiesDisk{{}},
				},
				MigrationStatus:        api.MIGRATIONSTATUS_IDLE,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_IDLE),
				Batch:                  ptr.To("one"),
				NeedsDiskImport:        true,
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
			},
			batchSvcGetByIDBatch: migration.Batch{
				ID:   2,
				Name: "two",
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
				},
				OS:        "ubuntu",
				OSVersion: "24.04",
			},
			wantMigrationStatus:        api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_BACKGROUND_IMPORT),
		},
		{
			name:    "success - migration window started",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID: uuidA,
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
					Disks:     []api.InstancePropertiesDisk{{}},
				},
				MigrationStatus:        api.MIGRATIONSTATUS_IDLE,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_IDLE),
				Batch:                  ptr.To("one"),
				NeedsDiskImport:        true,
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
			},
			batchSvcGetByIDBatch: migration.Batch{
				ID:                   2,
				Name:                 "two",
				MigrationWindowStart: time.Now().Add(-1 * time.Minute),
			},

			assertErr: require.NoError,
			wantWorkerCommand: migration.WorkerCommand{
				Command:    api.WORKERCOMMAND_FINALIZE_IMPORT,
				Location:   "/some/instance/A",
				SourceType: api.SOURCETYPE_VMWARE,
				Source: migration.Source{
					ID:         1,
					Name:       "one",
					SourceType: api.SOURCETYPE_VMWARE,
				},
				OS:        "ubuntu",
				OSVersion: "24.04",
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
			name:    "error - not assigned to batch",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID: uuidA,
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
					Disks:     []api.InstancePropertiesDisk{{}},
				},
				MigrationStatus:        api.MIGRATIONSTATUS_IDLE,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_IDLE),
				Batch:                  nil, // not assigned to batch
				NeedsDiskImport:        true,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrNotFound, a...)
			},
		},
		{
			name:    "error - instance is not in idle state",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID: uuidA,
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
					Disks:     []api.InstancePropertiesDisk{{}},
				},
				MigrationStatus:        api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_ASSIGNED_BATCH),
				Batch:                  ptr.To("one"),
				NeedsDiskImport:        true,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:    "error - source.GetByID",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID: uuidA,
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
					Disks:     []api.InstancePropertiesDisk{{}},
				},
				MigrationStatus:        api.MIGRATIONSTATUS_IDLE,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_IDLE),
				Batch:                  ptr.To("one"),
				NeedsDiskImport:        true,
			},
			sourceSvcGetByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - batch.GetByID",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID: uuidA,
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
					Disks:     []api.InstancePropertiesDisk{{}},
				},
				MigrationStatus:        api.MIGRATIONSTATUS_IDLE,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_IDLE),
				Batch:                  ptr.To("one"),
				NeedsDiskImport:        true,
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
			},
			batchSvcGetByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - instance.UpdateStatusByID",
			uuidArg: uuidA,
			instanceSvcGetByIDInstance: migration.Instance{
				UUID: uuidA,
				Properties: api.InstanceProperties{
					Location:  "/some/instance/A",
					Name:      "A",
					OS:        "ubuntu",
					OSVersion: "24.04",
					Disks:     []api.InstancePropertiesDisk{{}},
				},
				MigrationStatus:        api.MIGRATIONSTATUS_IDLE,
				MigrationStatusMessage: string(api.MIGRATIONSTATUS_IDLE),
				Batch:                  ptr.To("one"),
				NeedsDiskImport:        true,
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "one",
				SourceType: api.SOURCETYPE_VMWARE,
			},
			batchSvcGetByIDBatch: migration.Batch{
				ID:   2,
				Name: "two",
			},
			instanceSvcUpdateStatusByIDErr: boom.Error,

			assertErr:                  boom.ErrorIs,
			wantMigrationStatus:        api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantMigrationStatusMessage: string(api.MIGRATIONSTATUS_FINAL_IMPORT),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			instanceSvc := &InstanceServiceMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID, withOverrides bool) (*migration.Instance, error) {
					return &tc.instanceSvcGetByIDInstance, tc.instanceSvcGetByIDErr
				},
				UpdateStatusByUUIDFunc: func(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool, workerUpdate bool) (*migration.Instance, error) {
					require.Equal(t, tc.wantMigrationStatus, status)
					require.Equal(t, tc.wantMigrationStatusMessage, statusString)
					return nil, tc.instanceSvcUpdateStatusByIDErr
				},
			}

			sourceSvc := &SourceServiceMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Source, error) {
					return &tc.sourceSvcGetByIDSource, tc.sourceSvcGetByIDErr
				},
			}

			batchSvc := &BatchServiceMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return &tc.batchSvcGetByIDBatch, tc.batchSvcGetByIDErr
				},
			}

			queueSvc := migration.NewQueueService(batchSvc, instanceSvc, sourceSvc)

			// Run test
			workerCommand, err := queueSvc.NewWorkerCommandByInstanceUUID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantWorkerCommand, workerCommand)
		})
	}
}
