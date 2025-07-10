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
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.Instances, error) {
					return nil, tc.instanceSvcGetAllByBatchIDErr
				},
				GetAllFunc: func(ctx context.Context) (migration.Instances, error) {
					return nil, nil
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc)

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

			batchSvc := migration.NewBatchService(repo, nil)

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

			batchSvc := migration.NewBatchService(repo, nil)

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

			batchSvc := migration.NewBatchService(repo, nil)

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

			batchSvc := migration.NewBatchService(repo, nil)

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
				Name:              "new-one",
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
				Name:   "new-one",
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
				Name:              "new-one",
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
				Name:              "new-one",
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
				UpdateFunc: func(ctx context.Context, name string, in migration.Batch) error {
					return tc.repoUpdateErr
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.Instances, error) {
					return nil, tc.instanceSvcGetAllByBatchIDErr
				},
				GetAllFunc: func(ctx context.Context) (migration.Instances, error) {
					return nil, nil
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc)

			// Run test
			err := batchSvc.Update(context.Background(), tc.batch.Name, &tc.batch)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestBatchService_UpdateInstancesAssignedToBatch(t *testing.T) {
	tests := []struct {
		name                                string
		batch                               migration.Batch
		instanceSvcGetAllByBatchIDInstances migration.Instances
		instanceSvcGetAllByBatchIDErr       error
		instanceSvcGetAll                   migration.Instances
		instanceSvcGetAllErr                error
		instanceSvcUnassignFromBatch        []queue.Item[struct{}]
		instanceSvcAssignBatch              []queue.Item[migration.Instance]

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
				IncludeExpression: `location matches "^/inventory/path/A"`,
				Status:            api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					// Matching instance, will get updated.
					UUID:       uuid.Must(uuid.NewRandom()),
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
				{
					// Not matching instance, will be unassigned from batch
					UUID:       uuid.Must(uuid.NewRandom()),
					Properties: api.InstanceProperties{Location: "/inventory/path/B"},
				},
				{
					// Instance in state "user disabled", will be skipped
					UUID:       uuid.Must(uuid.NewRandom()),
					Properties: api.InstanceProperties{Location: "/inventory/path/A user disabled"},
					Overrides:  api.InstanceOverride{DisableMigration: true},
				},
			},
			instanceSvcGetAll: migration.Instances{
				{
					// Matching instance, will get updated.
					UUID:       uuid.Must(uuid.NewRandom()),
					Properties: api.InstanceProperties{Location: "/inventory/path/A unassigned"},
				},
				{
					// Not matching instance, will stay unassigned from batch
					UUID:       uuid.Must(uuid.NewRandom()),
					Properties: api.InstanceProperties{Location: "/inventory/path/B unassigned"},
				},
				{
					// Instance in state "user disabled", will be skipped
					UUID:       uuid.Must(uuid.NewRandom()),
					Properties: api.InstanceProperties{Location: "/inventory/path/A unassigned user disabled"},
					Overrides:  api.InstanceOverride{DisableMigration: true},
				},
			},
			instanceSvcUnassignFromBatch: []queue.Item[struct{}]{{}},
			instanceSvcAssignBatch:       []queue.Item[migration.Instance]{{}, {}},

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
			name: "error - svcSource.GetByName",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `location matches "^/inventory/path/A`, // invalid expression, missing " at the end
				Status:            api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
			},

			assertErr: require.Error,
		},
		{
			name: "error - batch.InstanceMatchesCriteria",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `location matches "^/inventory/path/A`, // invalid expression, missing " at the end
				Status:            api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "error - instance.UnassignFromBatch",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `location matches "^/inventory/path/A"`,
				Status:            api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchIDInstances: migration.Instances{
				{
					Properties: api.InstanceProperties{Location: "/inventory/path/B"},
				},
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
			name: "error - svcSource.GetByName for unassigned",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `location matches "^/inventory/path/A`, // invalid expression, missing " at the end
				Status:            api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAll: migration.Instances{
				{
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "error - batch.InstanceMatchesCriteria for unassigned",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `location matches "^/inventory/path/A`, // invalid expression, missing " at the end
				Status:            api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAll: migration.Instances{
				{
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
			},

			assertErr: require.Error,
		},
		{
			name: "error - instanceSvc.UpdateByID for unassigned",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				IncludeExpression: `location matches "^/inventory/path/A"`,
				Status:            api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAll: migration.Instances{
				{
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
			},
			instanceSvcAssignBatch: []queue.Item[migration.Instance]{
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
			repo := &mock.BatchRepoMock{
				UnassignBatchFunc: func(ctx context.Context, batchName string, instanceUUID uuid.UUID) error {
					_, err := queue.Pop(t, &tc.instanceSvcUnassignFromBatch)
					return err
				},
				AssignBatchFunc: func(ctx context.Context, batchName string, instanceUUID uuid.UUID) error {
					_, err := queue.Pop(t, &tc.instanceSvcAssignBatch)
					return err
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.Instances, error) {
					return tc.instanceSvcGetAllByBatchIDInstances, tc.instanceSvcGetAllByBatchIDErr
				},
				GetAllFunc: func(ctx context.Context) (migration.Instances, error) {
					return tc.instanceSvcGetAll, tc.instanceSvcGetAllErr
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc)

			// Run test
			err := batchSvc.UpdateInstancesAssignedToBatch(context.Background(), tc.batch)

			// Assert
			tc.assertErr(t, err)

			// Ensure queues are completely drained.
			require.Empty(t, tc.instanceSvcUnassignFromBatch)
			require.Empty(t, tc.instanceSvcAssignBatch)
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
				UpdateFunc: func(ctx context.Context, name string, b migration.Batch) error {
					return tc.repoUpdateStatusByNameErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil)

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
		repoUnassignWindowsErr            error

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
					UUID: uuidA,
				},
				{
					UUID: uuidB,
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
			name:    "error - batch unassignment",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			instanceSvcGetAllByBatchInstances: migration.Instances{
				{
					UUID: uuidA,
				},
			},
			instanceSvcUnassignFromBatchErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - repo.UnassignMigrationWindows",
			nameArg: "one",
			repoGetByNameBatch: &migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_DEFINED,
			},
			repoUnassignWindowsErr: boom.Error,

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
				UnassignBatchFunc: func(ctx context.Context, batchName string, instanceUUID uuid.UUID) error {
					return tc.instanceSvcUnassignFromBatchErr
				},

				UnassignMigrationWindowsFunc: func(ctx context.Context, batch string) error {
					return tc.repoUnassignWindowsErr
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.Instances, error) {
					return tc.instanceSvcGetAllByBatchInstances, tc.instanceSvcGetAllByBatchErr
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc)

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
				UpdateFunc: func(ctx context.Context, name string, b migration.Batch) error {
					return tc.repoUpdateStatusByIDErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil)

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
				UpdateFunc: func(ctx context.Context, name string, b migration.Batch) error {
					return tc.repoUpdateStatusByIDErr
				},
			}

			batchSvc := migration.NewBatchService(repo, nil)

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
			expression: `location == "/a/b/c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Inventory path regex match",
			expression: `location matches "^/a/[^/]+/c*"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "Name exact match",
			expression: `name == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "boolean or expression",
			expression: `location matches "^/e/f/.*" || name == "c"`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "boolean and expression",
			expression: `location == "/a/b/c" && tpm`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "exclude regex",
			expression: `!(location matches "^/a/e/.*$")`,

			assertErr:  require.NoError,
			wantResult: true,
		},
		{
			name:       "custom path_base function exact match",
			expression: `path_base(location) == "c"`,

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
			expression: `path_dir(location) == "/a/b"`,

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
			expression: `source in ["vcenter01", "vcenter02", "vcenter03"] && (location startsWith "/a/b" || location startsWith "/e/f") && cpus <= 4 && memory <= 1024*1024*1024*8 && len(disks) == 1 && !disks[0].shared && os in ["Ubuntu 22.04", "Ubuntu 24.04"]`,

			assertErr:  require.NoError,
			wantResult: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			currentBatch := migration.Batch{
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

				Source:     "vcenter01",
				SourceType: api.SOURCETYPE_VMWARE,
			}

			res, err := instance.MatchesCriteria(currentBatch.IncludeExpression)
			tc.assertErr(t, err)

			require.Equal(t, tc.wantResult, res)
		})
	}
}
