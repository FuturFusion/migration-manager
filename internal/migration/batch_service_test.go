package migration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/internal/testing/queue"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestBatchService_Create(t *testing.T) {
	tests := []struct {
		name                          string
		batch                         migration.Batch
		repoCreateBatch               migration.Batch
		repoCreateErr                 error
		instanceSvcGetAllByBatchIDErr error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			batch: migration.Batch{
				ID:     1,
				Name:   "one",
				Target: "one", IncludeExpression: "true",
				Status: api.BATCHSTATUS_DEFINED,
			},
			repoCreateBatch: migration.Batch{
				ID:                1,
				Name:              "one",
				Target:            "one",
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			batch: migration.Batch{
				ID:                -1, // invalid
				Name:              "one",
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			batch: migration.Batch{
				ID:                1,
				Name:              "", // empty
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - target invalid",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Target:            "", // invalid
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - state invalid",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            "", // invalid
				IncludeExpression: "true",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - state invalid",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: "", // invalid
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Target:            "one",
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
			},
			repoCreateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - repo",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Target:            "one",
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				CreateFunc: func(ctx context.Context, in migration.Batch) (int64, error) {
					return tc.repoCreateBatch.ID, tc.repoCreateErr
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string, withOverrides bool) (migration.Instances, error) {
					return nil, tc.instanceSvcGetAllByBatchIDErr
				},
				GetAllUnassignedFunc: func(ctx context.Context, withOverrides bool) (migration.Instances, error) {
					return nil, nil
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc, nil)

			// Run test
			batch, err := batchSvc.Create(context.Background(), tc.batch)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoCreateBatch, batch)
		})
	}
}

func TestBatchService_GetAll(t *testing.T) {
	tests := []struct {
		name              string
		repoGetAllBatches migration.Batches
		repoGetAllErr     error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllBatches: migration.Batches{
				migration.Batch{
					ID:   1,
					Name: "one",
				},
				migration.Batch{
					ID:   2,
					Name: "two",
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:          "error - repo",
			repoGetAllErr: boom.Error,

			assertErr: boom.ErrorIs,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetAllFunc: func(ctx context.Context) (migration.Batches, error) {
					return tc.repoGetAllBatches, tc.repoGetAllErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil, nil)

			// Run test
			batches, err := batchSvc.GetAll(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, batches, tc.count)
		})
	}
}

func TestBatchService_GetAllByState(t *testing.T) {
	tests := []struct {
		name                     string
		repoGetAllByStateBatches migration.Batches
		repoGetAllByStateErr     error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllByStateBatches: migration.Batches{
				migration.Batch{
					ID:     1,
					Name:   "one",
					Status: api.BATCHSTATUS_QUEUED,
				},
				migration.Batch{
					ID:     2,
					Name:   "two",
					Status: api.BATCHSTATUS_QUEUED,
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:                 "error - repo",
			repoGetAllByStateErr: boom.Error,

			assertErr: boom.ErrorIs,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetAllByStateFunc: func(ctx context.Context, status api.BatchStatusType) (migration.Batches, error) {
					return tc.repoGetAllByStateBatches, tc.repoGetAllByStateErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil, nil)

			// Run test
			batches, err := batchSvc.GetAllByState(context.Background(), api.BATCHSTATUS_QUEUED)

			// Assert
			tc.assertErr(t, err)
			require.Len(t, batches, tc.count)
		})
	}
}

func TestBatchService_GetAllNames(t *testing.T) {
	tests := []struct {
		name            string
		repoGetAllNames []string
		repoGetAllErr   error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllNames: []string{
				"batchA", "batchB",
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:          "error - repo",
			repoGetAllErr: boom.Error,

			assertErr: boom.ErrorIs,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetAllNamesFunc: func(ctx context.Context) ([]string, error) {
					return tc.repoGetAllNames, tc.repoGetAllErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil, nil)

			// Run test
			inventoryNames, err := batchSvc.GetAllNames(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, inventoryNames, tc.count)
		})
	}
}

func TestBatchService_GetByName(t *testing.T) {
	tests := []struct {
		name               string
		nameArg            string
		repoGetByNameBatch *migration.Batch
		repoGetByNameErr   error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:   1,
				Name: "one",
			},

			assertErr: require.NoError,
		},
		{
			name:    "error - name argument empty string",
			nameArg: "",

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:             "error - repo",
			nameArg:          "one",
			repoGetByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return tc.repoGetByNameBatch, tc.repoGetByNameErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil, nil)

			// Run test
			batch, err := batchSvc.GetByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetByNameBatch, batch)
		})
	}
}

func TestBatchService_UpdateByID(t *testing.T) {
	tests := []struct {
		name                          string
		batch                         migration.Batch
		repoGetByNameBatch            *migration.Batch
		repoGetByNameErr              error
		repoUpdateErr                 error
		instanceSvcGetAllByBatchIDErr error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			batch: migration.Batch{
				ID:                1,
				Name:              "new one",
				Target:            "one",
				Status:            api.BATCHSTATUS_DEFINED,
				IncludeExpression: "true",
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Target:            "one",
				Status:            api.BATCHSTATUS_DEFINED,
				IncludeExpression: "true",
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			batch: migration.Batch{
				ID:     -1, // invalid
				Name:   "new one",
				Target: "one",
				Status: api.BATCHSTATUS_DEFINED,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			batch: migration.Batch{
				ID:     1,
				Name:   "", // empty
				Target: "one",
				Status: api.BATCHSTATUS_DEFINED,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo.GetByID",
			batch: migration.Batch{
				ID:                1,
				Name:              "new one",
				Status:            api.BATCHSTATUS_DEFINED,
				Target:            "one",
				IncludeExpression: "true",
			},
			repoGetByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - status can not be modified",
			batch: migration.Batch{
				ID:                1,
				Name:              "new one",
				Status:            api.BATCHSTATUS_RUNNING,
				Target:            "one",
				IncludeExpression: "true",
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_RUNNING,
				Target:            "one",
				IncludeExpression: "true",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name: "error - repo.UpdateByID",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_DEFINED,
				Target:            "one",
				IncludeExpression: "true",
			},
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Target: "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			repoUpdateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - repo.UpdateByID",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_DEFINED,
				Target:            "one",
				IncludeExpression: "true",
			},
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Target: "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return tc.repoGetByNameBatch, tc.repoGetByNameErr
				},
				UpdateFunc: func(ctx context.Context, in migration.Batch) error {
					return tc.repoUpdateErr
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string, withOverrides bool) (migration.Instances, error) {
					return nil, tc.instanceSvcGetAllByBatchIDErr
				},
				GetAllUnassignedFunc: func(ctx context.Context, withOverrides bool) (migration.Instances, error) {
					return nil, nil
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc, nil)

			// Run test
			err := batchSvc.Update(context.Background(), &tc.batch)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestBatchService_UpdateInstancesAssignedToBatch(t *testing.T) {
	tests := []struct {
		name                                 string
		batch                                migration.Batch
		instanceSvcGetAllByBatchIDInstances  migration.Instances
		instanceSvcGetAllByBatchIDErr        error
		instanceSvcGetAllUnassignedInstances migration.Instances
		instanceSvcGetAllUnassignedErr       error
		instanceSvcGetByUUID                 []queue.Item[migration.Instance]
		sourceSvcGetByName                   []queue.Item[migration.Source]
		instanceSvcUnassignFromBatch         []queue.Item[struct{}]
		instanceSvcUpdateByID                []queue.Item[migration.Instance]

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success - empty batch, no unassigned instances",
			batch: migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},

			assertErr: require.NoError,
		},
		{
			name: "success - batch with all sort of instances",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A"`,
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					// Matching instance, will get updated.
					Properties:      api.InstanceProperties{Location: "/inventory/path/A"},
					MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				},
				{
					// Not matching instance, will be unassigned from batch
					Properties:      api.InstanceProperties{Location: "/inventory/path/B"},
					MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				},
				{
					// Instance in state "user disabled", will be skipped
					Properties:      api.InstanceProperties{Location: "/inventory/path/A user disabled"},
					MigrationStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
				},
				{
					// Instance is already migrating, will be skipped
					Properties:      api.InstanceProperties{Location: "/inventory/path/A already migrating"},
					MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				},
			},
			instanceSvcGetAllUnassignedInstances: migration.Instances{
				{
					// Matching instance, will get updated.
					Properties:      api.InstanceProperties{Location: "/inventory/path/A unassigned"},
					MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				},
				{
					// Matching instance, will get updated.
					Properties:      api.InstanceProperties{Location: "/inventory/path/A unassigned idle state"},
					MigrationStatus: api.MIGRATIONSTATUS_IDLE,
				},
				{
					// Not matching instance, will stay unassigned from batch
					Properties:      api.InstanceProperties{Location: "/inventory/path/B unassigned"},
					MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				},
				{
					// Instance in state "user disabled", will be skipped
					Properties:      api.InstanceProperties{Location: "/inventory/path/A unassigned user disabled"},
					MigrationStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
				},
			},
			sourceSvcGetByName: []queue.Item[migration.Source]{
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "A",
							Location: "/inventory/path/A",
						},
					},
				},
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "B",
							Location: "/inventory/path/B",
						},
					},
				},
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "A unassigned",
							Location: "/inventory/path/A unassigned",
						},
					},
				},
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "A unassigned idle state",
							Location: "/inventory/path/A unassigned idle state",
						},
					},
				},
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "B unassigned",
							Location: "/inventory/path/B unassigned",
						},
					},
				},
			},
			instanceSvcUnassignFromBatch: []queue.Item[struct{}]{
				{},
			},
			instanceSvcUpdateByID: []queue.Item[migration.Instance]{
				{
					Value: migration.Instance{},
				},
			},

			assertErr: require.NoError,
		},
		{
			name: "error - instanceSvc.GetAllByBatchID",
			batch: migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - svcInstance.GetByUUID",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A"`,
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					Properties:      api.InstanceProperties{Location: "/inventory/path/A"},
					MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Err: boom.Error,
				},
			},

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - svcSource.GetByName",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A`, // invalid expression, missing " at the end
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					Properties:      api.InstanceProperties{Location: "/inventory/path/A"},
					MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "A",
							Location: "/inventory/path/A",
						},
					},
				},
			},
			sourceSvcGetByName: []queue.Item[migration.Source]{
				{Err: boom.Error},
			},

			assertErr: require.Error,
		},
		{
			name: "error - batch.InstanceMatchesCriteria",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A`, // invalid expression, missing " at the end
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					Properties:      api.InstanceProperties{Location: "/inventory/path/A"},
					MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "A",
							Location: "/inventory/path/A",
						},
					},
				},
			},
			sourceSvcGetByName: []queue.Item[migration.Source]{
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
			},

			assertErr: require.Error,
		},
		{
			name: "error - instance.UnassignFromBatch",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A"`,
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					Properties:      api.InstanceProperties{Location: "/inventory/path/B"},
					MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "B",
							Location: "/inventory/path/B",
						},
					},
				},
			},
			sourceSvcGetByName: []queue.Item[migration.Source]{
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
			},
			instanceSvcUnassignFromBatch: []queue.Item[struct{}]{
				{
					Err: boom.Error,
				},
			},

			assertErr: boom.ErrorIs,
		},
		// Unassigned instances error cases.
		{
			name: "error - instanceSvc.GetAllUnassigned",
			batch: migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllUnassignedErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - svcInstance.GetByUUID for unassigned",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A"`,
			},
			instanceSvcGetAllUnassignedInstances: migration.Instances{
				{
					Properties:      api.InstanceProperties{Location: "/inventory/path/A"},
					MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Err: boom.Error,
				},
			},

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - svcSource.GetByName for unassigned",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A`, // invalid expression, missing " at the end
			},
			instanceSvcGetAllUnassignedInstances: migration.Instances{
				{
					Properties:      api.InstanceProperties{Location: "/inventory/path/A"},
					MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "A",
							Location: "/inventory/path/A",
						},
					},
				},
			},
			sourceSvcGetByName: []queue.Item[migration.Source]{
				{Err: boom.Error},
			},

			assertErr: require.Error,
		},
		{
			name: "error - batch.InstanceMatchesCriteria for unassigned",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A`, // invalid expression, missing " at the end
			},
			instanceSvcGetAllUnassignedInstances: migration.Instances{
				{
					Properties:      api.InstanceProperties{Location: "/inventory/path/A"},
					MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "A",
							Location: "/inventory/path/A",
						},
					},
				},
			},
			sourceSvcGetByName: []queue.Item[migration.Source]{
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
			},

			assertErr: require.Error,
		},
		{
			name: "error - instanceSvc.UpdateByID for unassigned",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `Location matches "^/inventory/path/A"`,
			},
			instanceSvcGetAllUnassignedInstances: migration.Instances{
				{
					Properties:      api.InstanceProperties{Location: "/inventory/path/A"},
					MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				},
			},
			instanceSvcGetByUUID: []queue.Item[migration.Instance]{
				{
					Value: migration.Instance{
						Properties: api.InstanceProperties{
							Name:     "A",
							Location: "/inventory/path/A",
						},
					},
				},
			},
			sourceSvcGetByName: []queue.Item[migration.Source]{
				{Value: migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE}},
			},
			instanceSvcUpdateByID: []queue.Item[migration.Instance]{
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
			repo := &mock.BatchRepoMock{}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string, withOverrides bool) (migration.Instances, error) {
					return tc.instanceSvcGetAllByBatchIDInstances, tc.instanceSvcGetAllByBatchIDErr
				},
				GetAllUnassignedFunc: func(ctx context.Context, withOverrides bool) (migration.Instances, error) {
					return tc.instanceSvcGetAllUnassignedInstances, tc.instanceSvcGetAllUnassignedErr
				},
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID, withOverrides bool) (*migration.Instance, error) {
					i, err := queue.Pop(t, &tc.instanceSvcGetByUUID)
					if err != nil {
						return nil, err
					}

					return &i, nil
				},
				UnassignFromBatchFunc: func(ctx context.Context, id uuid.UUID) error {
					_, err := queue.Pop(t, &tc.instanceSvcUnassignFromBatch)
					return err
				},
				UpdateFunc: func(ctx context.Context, instance *migration.Instance) error {
					_, err := queue.Pop(t, &tc.instanceSvcUpdateByID)
					return err
				},
			}

			sourceSvc := &SourceServiceMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Source, error) {
					s, err := queue.Pop(t, &tc.sourceSvcGetByName)
					if err != nil {
						return nil, err
					}

					return &s, nil
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc, sourceSvc)

			// Run test
			err := batchSvc.UpdateInstancesAssignedToBatch(context.Background(), tc.batch)

			// Assert
			tc.assertErr(t, err)

			// Ensure queues are completely drained.
			require.Empty(t, tc.instanceSvcGetByUUID)
			require.Empty(t, tc.instanceSvcUnassignFromBatch)
			require.Empty(t, tc.instanceSvcUpdateByID)
		})
	}
}

func TestBatchService_UpdateStatusByName(t *testing.T) {
	tests := []struct {
		name                        string
		nameArg                     string
		repoUpdateStatusByNameBatch *migration.Batch
		repoUpdateStatusByNameErr   error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",
			repoUpdateStatusByNameBatch: &migration.Batch{
				ID:            1,
				Name:          "one",
				Status:        api.BATCHSTATUS_QUEUED,
				StatusMessage: string(api.BATCHSTATUS_QUEUED),
			},

			assertErr: require.NoError,
		},
		{
			name:                      "error - repo",
			nameArg:                   "one",
			repoUpdateStatusByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return tc.repoUpdateStatusByNameBatch, tc.repoUpdateStatusByNameErr
				},
				UpdateFunc: func(ctx context.Context, b migration.Batch) error {
					return tc.repoUpdateStatusByNameErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil, nil)

			// Run test
			batch, err := batchSvc.UpdateStatusByName(context.Background(), tc.nameArg, api.BATCHSTATUS_QUEUED, string(api.BATCHSTATUS_QUEUED))

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoUpdateStatusByNameBatch, batch)
		})
	}
}

func TestBatchService_DeleteByName(t *testing.T) {
	tests := []struct {
		name                              string
		nameArg                           string
		repoGetByNameBatch                *migration.Batch
		repoGetByNameErr                  error
		instanceSvcGetAllByBatchInstances migration.Instances
		instanceSvcGetAllByBatchErr       error
		instanceSvcUnassignFromBatchErr   error
		repoDeleteByNameErr               error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success - batch without instances",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchInstances: nil,

			assertErr: require.NoError,
		},
		{
			name:    "success - batch without migrating instances",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchInstances: migration.Instances{
				{
					UUID:            uuidA,
					MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				},
				{
					UUID:            uuidB,
					MigrationStatus: api.MIGRATIONSTATUS_ERROR,
				},
			},

			assertErr: require.NoError,
		},
		{
			name:    "error - name argument empty string",
			nameArg: "",

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:             "error - get batch",
			nameArg:          "one",
			repoGetByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - batch without instances",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_QUEUED,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:    "error - instance migrating",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - instance migrating",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchInstances: migration.Instances{
				{
					UUID:            uuidB,
					MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:    "error - batch unassignment",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchInstances: migration.Instances{
				{
					UUID:            uuidA,
					MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				},
			},
			instanceSvcUnassignFromBatchErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - repo.DeleteByName",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			repoDeleteByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return tc.repoGetByNameBatch, tc.repoGetByNameErr
				},
				DeleteByNameFunc: func(ctx context.Context, name string) error {
					return tc.repoDeleteByNameErr
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string, withOverrides bool) (migration.Instances, error) {
					return tc.instanceSvcGetAllByBatchInstances, tc.instanceSvcGetAllByBatchErr
				},
				UnassignFromBatchFunc: func(ctx context.Context, id uuid.UUID) error {
					return tc.instanceSvcUnassignFromBatchErr
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc, nil)

			// Run test
			err := batchSvc.DeleteByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestBatchService_StartBatchByName(t *testing.T) {
	tests := []struct {
		name                    string
		nameArg                 string
		repoGetByNameBatch      migration.Batch
		repoGetByNameErr        error
		repoUpdateStatusByIDErr error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",
			repoGetByNameBatch: migration.Batch{
				ID:                1,
				Name:              "one",
				Target:            "one",
				Status:            api.BATCHSTATUS_DEFINED,
				IncludeExpression: "true",
			},

			assertErr: require.NoError,
		},
		{
			name:    "error - empty name",
			nameArg: "",

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:             "error - repo.GetByName",
			nameArg:          "one",
			repoGetByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - batch state is not ready to be started",
			nameArg: "one",
			repoGetByNameBatch: migration.Batch{
				ID:                1,
				Name:              "one",
				Target:            "one",
				Status:            api.BATCHSTATUS_QUEUED,
				IncludeExpression: "true",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:    "error - batch state is not ready to be started",
			nameArg: "one",
			repoGetByNameBatch: migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_DEFINED,
				Target:            "one",
				IncludeExpression: "true",
			},
			repoUpdateStatusByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return &tc.repoGetByNameBatch, tc.repoGetByNameErr
				},
				UpdateFunc: func(ctx context.Context, b migration.Batch) error {
					return tc.repoUpdateStatusByIDErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil, nil)

			// Run test
			err := batchSvc.StartBatchByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestBatchService_StopBatchByName(t *testing.T) {
	tests := []struct {
		name                    string
		nameArg                 string
		repoGetByNameBatch      *migration.Batch
		repoGetByNameErr        error
		repoUpdateStatusByIDErr error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_QUEUED,
				Target:            "one",
				IncludeExpression: "true",
			},

			assertErr: require.NoError,
		},
		{
			name:    "error - empty name",
			nameArg: "",

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:             "error - repo.GetByName",
			nameArg:          "one",
			repoGetByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - batch state is not ready to be started",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_DEFINED,
				Target:            "one",
				IncludeExpression: "true",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:    "error - batch state is not ready to be started",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_QUEUED,
				Target:            "one",
				IncludeExpression: "true",
			},
			repoUpdateStatusByIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.BatchRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					return tc.repoGetByNameBatch, tc.repoGetByNameErr
				},
				UpdateFunc: func(ctx context.Context, b migration.Batch) error {
					return tc.repoUpdateStatusByIDErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil, nil)

			// Run test
			err := batchSvc.StopBatchByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestInternalBatch_InstanceMatchesCriteria(t *testing.T) {
	tests := []struct {
		name       string
		expression string

		assertErr  require.ErrorAssertionFunc
		wantResult bool
	}{
		{
			name:       "Always true",
			expression: `true`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Always false",
			expression: `false`,

			assertErr:  require.NoError,
			wantResult: false,
		},
		{
			name:       "Inventory path exact match",
			expression: `Location == "/a/b/c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Inventory path regex match",
			expression: `Location matches "^/a/[^/]+/c*"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Name exact match",
			expression: `Name == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "boolean or expression",
			expression: `Location matches "^/e/f/.*" || Name == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "boolean and expression",
			expression: `Location == "/a/b/c" && TPM`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "exclude regex",
			expression: `!(Location matches "^/a/e/.*$")`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "custom path_base function exact match",
			expression: `path_base(Location) == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "custom path_base function without arguments",
			expression: `path_base() == "c"`,

			assertErr:  require.Error,
			wantResult: false,
		},
		{
			name:       "custom path_base function with argument of wrong type",
			expression: `path_base(123) == "c"`,

			assertErr:  require.Error,
			wantResult: false,
		},
		{
			name:       "custom path_dir function exact match",
			expression: `path_dir(Location) == "/a/b"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "custom path_dir function without arguments",
			expression: `path_dir() == "c"`,

			assertErr:  require.Error,
			wantResult: false,
		},
		{
			name:       "custom path_dir function with argument of wrong type",
			expression: `path_dir(123) == "c"`,

			assertErr:  require.Error,
			wantResult: false,
		},
		{
			name:       "expression not returning a boolean",
			expression: `"string"`,

			assertErr:  require.Error,
			wantResult: false,
		},
		{
			name:       "complex expression",
			expression: `Source in ["vcenter01", "vcenter02", "vcenter03"] && (Location startsWith "/a/b" || Location startsWith "/e/f") && CPUs <= 4 && Memory <= 1024*1024*1024*8 && len(Disks) == 1 && !Disks[0].Shared && OS in ["Ubuntu 22.04", "Ubuntu 24.04"]`,

			assertErr:  require.NoError,
			wantResult: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			currentBatch := migration.Batch{
				Name:              "test batch",
				IncludeExpression: tc.expression,
			}

			instance := migration.Instance{
				Properties: api.InstanceProperties{
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
						CPUs:   2,
						Memory: 1024 * 1024 * 1024 * 4,
					},
					Name:     "c",
					Location: "/a/b/c",
					OS:       "Ubuntu 22.04",
					Disks: []api.InstancePropertiesDisk{
						{Name: "disk"},
					},
					TPM: true,
				},

				Source: "vcenter01",
			}

			source := migration.Source{
				Name:       "vcenter01",
				SourceType: api.SOURCETYPE_VMWARE,
			}

			res, err := currentBatch.InstanceMatchesCriteria(instance, source)
			tc.assertErr(t, err)

			require.Equal(t, tc.wantResult, res)
		})
	}
}
