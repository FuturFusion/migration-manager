package migration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/internal/testing/queue"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var defaultPlacement = api.BatchDefaults{Placement: api.BatchPlacement{Target: "one", TargetProject: "default", StoragePool: "default"}}

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
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoCreateBatch: migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
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
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoCreateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - repo",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Status:            api.BATCHSTATUS_DEFINED,
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
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
					Status: api.BATCHSTATUS_RUNNING,
				},
				migration.Batch{
					ID:     2,
					Name:   "two",
					Status: api.BATCHSTATUS_RUNNING,
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
			batches, err := batchSvc.GetAllByState(context.Background(), api.BATCHSTATUS_RUNNING)

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
		queueSvcGetAllByBatchErr      error

		instanceSvcGetAllByBatch migration.Instances
		queueSvcGetAllByBatch    migration.QueueEntries

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			batch: migration.Batch{
				ID:                1,
				Name:              "new-one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_DEFINED,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_DEFINED,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},

			assertErr: require.NoError,
		},
		{
			name: "success - running batch",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					RerunScriptlets:          true,
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},

			assertErr: require.NoError,
		},
		{
			name: "success - modify matching constraint on running batch with non-committed queue entries",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_RUNNING,
				IncludeExpression: "true",
				Constraints:       []api.BatchConstraint{{Name: "constraint1", IncludeExpression: "true"}},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_RUNNING,
				IncludeExpression: "true",
				Constraints:       []api.BatchConstraint{{Name: "constraint2", IncludeExpression: "true"}},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			queueSvcGetAllByBatch:    migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_WAITING}},
			instanceSvcGetAllByBatch: migration.Instances{{UUID: uuidA}},

			assertErr: require.NoError,
		},
		{
			name: "success - add non-matching constraint on running batch with committed queue entries",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_RUNNING,
				IncludeExpression: "true",
				Constraints:       []api.BatchConstraint{{Name: "constraint1", IncludeExpression: "true"}},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_RUNNING,
				IncludeExpression: "true",
				Constraints:       []api.BatchConstraint{{Name: "constraint1", IncludeExpression: "true"}, {Name: "constraint2", IncludeExpression: "false"}},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			queueSvcGetAllByBatch:    migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_FINAL_IMPORT}},
			instanceSvcGetAllByBatch: migration.Instances{{UUID: uuidA}},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			batch: migration.Batch{
				ID:       -1, // invalid
				Name:     "new-one",
				Defaults: defaultPlacement,
				Status:   api.BATCHSTATUS_DEFINED,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			batch: migration.Batch{
				ID:       1,
				Name:     "", // empty
				Defaults: defaultPlacement,
				Status:   api.BATCHSTATUS_DEFINED,
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
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - running batch - can't change name",
			batch: migration.Batch{
				ID:                1,
				Name:              "new-one",
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name: "error - running batch - can't change placement",
			batch: migration.Batch{
				ID:     1,
				Name:   "one",
				Status: api.BATCHSTATUS_RUNNING,
				Defaults: api.BatchDefaults{
					Placement: api.BatchPlacement{Target: "changed", StoragePool: "default", TargetProject: "default"},
				},
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name: "error - running batch - can't change expression",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
				IncludeExpression: "true and true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
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
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:       1,
				Name:     "one",
				Defaults: defaultPlacement,
				Status:   api.BATCHSTATUS_DEFINED,
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoUpdateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - instanceSvc.GetAllByBatch",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Status:            api.BATCHSTATUS_DEFINED,
				Defaults:          defaultPlacement,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:       1,
				Name:     "one",
				Defaults: defaultPlacement,
				Status:   api.BATCHSTATUS_DEFINED,
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			instanceSvcGetAllByBatchIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - queueSvc.GetAllByBatch",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_RUNNING,
				IncludeExpression: "true",
				Constraints:       []api.BatchConstraint{{Name: "constraint1", IncludeExpression: "true"}},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_RUNNING,
				IncludeExpression: "true",
				Constraints:       []api.BatchConstraint{{Name: "constraint2", IncludeExpression: "true"}},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			queueSvcGetAllByBatchErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - modify matching constraint on running batch",
			batch: migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_RUNNING,
				IncludeExpression: "true",
				Constraints:       []api.BatchConstraint{{Name: "constraint1", IncludeExpression: "true"}},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			repoGetByNameBatch: &migration.Batch{
				ID:                1,
				Name:              "one",
				Defaults:          defaultPlacement,
				Status:            api.BATCHSTATUS_RUNNING,
				IncludeExpression: "true",
				Constraints:       []api.BatchConstraint{{Name: "constraint2", IncludeExpression: "true"}},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			},
			queueSvcGetAllByBatch:    migration.QueueEntries{{InstanceUUID: uuidA, BatchName: "one", MigrationStatus: api.MIGRATIONSTATUS_FINAL_IMPORT}},
			instanceSvcGetAllByBatch: migration.Instances{{UUID: uuidA}},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
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
					return tc.instanceSvcGetAllByBatch, tc.instanceSvcGetAllByBatchIDErr
				},
				GetAllFunc: func(ctx context.Context) (migration.Instances, error) {
					return nil, nil
				},
			}

			queueSvc := &QueueServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.QueueEntries, error) {
					return tc.queueSvcGetAllByBatch, tc.queueSvcGetAllByBatchErr
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc)

			// Run test
			err := batchSvc.Update(context.Background(), queueSvc, tc.batch.Name, &tc.batch)

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
				Status:        api.BATCHSTATUS_RUNNING,
				StatusMessage: string(api.BATCHSTATUS_RUNNING),
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
			batch, err := batchSvc.UpdateStatusByName(context.Background(), tc.nameArg, api.BATCHSTATUS_RUNNING, string(api.BATCHSTATUS_RUNNING))

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
				Status: api.BATCHSTATUS_RUNNING,
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
	asUUID := func(i int) string {
		return fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
	}

	tests := []struct {
		name                 string
		batchName            string
		initBatchState       api.BatchStatusType
		queueEntriesByBatch  map[string][]string
		numMatchingInstances map[bool][]string

		addedQueueEntries []string

		repoGetByNameErr error
		repoUpdateErr    error

		noValidWindows              bool
		instanceSvcGetAllByBatchErr error
		networkSvcGetAllErr         error
		windowSvcGetAllByBatchErr   error
		queueSvcGetAllErr           error
		queueSvcCreateErr           error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:                 "success - all instances match, no existing queue entries",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_DEFINED,
			queueEntriesByBatch:  map[string][]string{},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}},

			addedQueueEntries: []string{asUUID(1), asUUID(2), asUUID(3)},

			assertErr: require.NoError,
		},
		{
			name:                 "success - all instances match, with existing entries from another batch",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_DEFINED,
			queueEntriesByBatch:  map[string][]string{"two": {asUUID(3)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}},

			addedQueueEntries: []string{asUUID(1), asUUID(2)},

			assertErr: require.NoError,
		},
		{
			name:                 "success - some instances match, with existing entries from another batch",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_DEFINED,
			queueEntriesByBatch:  map[string][]string{"two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			addedQueueEntries: []string{asUUID(1), asUUID(3)},

			assertErr: require.NoError,
		},
		{
			name:                 "success - batch stopped, some instances match, with existing entries from another batch, and existing entries in the same batch",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_STOPPED,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			addedQueueEntries: []string{asUUID(3)},

			assertErr: require.NoError,
		},
		{
			name:                 "success - batch errored, some instances match, with existing entries from another batch, and existing entries in the same batch",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			addedQueueEntries: []string{asUUID(3)},

			assertErr: require.NoError,
		},
		{
			name:      "error - empty name",
			batchName: "",

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:             "error - repo.GetByName",
			batchName:        "one",
			repoGetByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:           "error - batch state is not ready to be started",
			batchName:      "one",
			initBatchState: api.BATCHSTATUS_RUNNING,

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:           "error - batch state is already finished",
			batchName:      "one",
			initBatchState: api.BATCHSTATUS_FINISHED,

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:                 "error - no instances available to queue",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1), asUUID(3)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			assertErr: require.Error,
		},
		{
			name:                 "error - repo.Update",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			repoUpdateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                 "error - instanceSvc.GetAllByBatch",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			instanceSvcGetAllByBatchErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                 "error - networkSvc.GetAll",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			networkSvcGetAllErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                 "error - queueSvc.GetAll",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			queueSvcGetAllErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                 "error - queueSvc.Create",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			queueSvcCreateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                 "error - windowSvc.GetAllByBatch",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			windowSvcGetAllByBatchErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                 "error - no valid windows",
			batchName:            "one",
			initBatchState:       api.BATCHSTATUS_ERROR,
			queueEntriesByBatch:  map[string][]string{"one": {asUUID(1)}, "two": {asUUID(2), asUUID(4)}},
			numMatchingInstances: map[bool][]string{true: {asUUID(1), asUUID(2), asUUID(3)}, false: {asUUID(4), asUUID(5)}},

			noValidWindows: true,

			assertErr: require.Error,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)

			newQueueEntries := []string{}
			repo := &mock.BatchRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Batch, error) {
					includeExpr := make([]string, 0, len(tc.numMatchingInstances[true]))
					for _, id := range tc.numMatchingInstances[true] {
						includeExpr = append(includeExpr, "uuid == '"+id+"'")
					}

					return &migration.Batch{
						ID:                1,
						Name:              tc.batchName,
						Status:            tc.initBatchState,
						IncludeExpression: strings.Join(includeExpr, " or "),
						StatusMessage:     string(tc.initBatchState),
						Defaults:          defaultPlacement,
					}, tc.repoGetByNameErr
				},
				UpdateFunc: func(ctx context.Context, name string, b migration.Batch) error {
					if tc.repoUpdateErr != nil {
						newQueueEntries = nil
					}

					return tc.repoUpdateErr
				},
			}

			queueSvc := &QueueServiceMock{
				GetAllFunc: func(ctx context.Context) (migration.QueueEntries, error) {
					entries := migration.QueueEntries{}
					for batch, ids := range tc.queueEntriesByBatch {
						for _, id := range ids {
							entries = append(entries, migration.QueueEntry{BatchName: batch, InstanceUUID: uuid.MustParse(id)})
						}
					}

					return entries, tc.queueSvcGetAllErr
				},
				CreateEntryFunc: func(ctx context.Context, queue migration.QueueEntry) (migration.QueueEntry, error) {
					if tc.queueSvcCreateErr == nil {
						newQueueEntries = append(newQueueEntries, queue.InstanceUUID.String())
					}

					return migration.QueueEntry{}, tc.queueSvcCreateErr
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.Instances, error) {
					instances := migration.Instances{}
					for _, id := range tc.numMatchingInstances[true] {
						instances = append(instances, migration.Instance{UUID: uuid.MustParse(id)})
					}

					return instances, tc.instanceSvcGetAllByBatchErr
				},
			}

			networkSvc := &NetworkServiceMock{
				GetAllFunc: func(ctx context.Context) (migration.Networks, error) {
					return nil, tc.networkSvcGetAllErr
				},
			}

			windowSvc := &WindowServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batchName string) (migration.Windows, error) {
					if tc.noValidWindows {
						now := time.Now().UTC()
						return migration.Windows{{Start: now.Add(-10 * time.Second)}}, tc.windowSvcGetAllByBatchErr
					}

					return nil, tc.windowSvcGetAllByBatchErr
				},
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc)

			// Run test
			_, err := batchSvc.StartBatchByName(context.Background(), tc.batchName, windowSvc, networkSvc, queueSvc)

			// Assert
			tc.assertErr(t, err)

			require.Len(t, newQueueEntries, len(tc.addedQueueEntries))
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
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
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
				Defaults:          defaultPlacement,
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
				Status:            api.BATCHSTATUS_RUNNING,
				Defaults:          defaultPlacement,
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
			_, err := batchSvc.StopBatchByName(context.Background(), tc.nameArg)

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
						OS:     "Ubuntu 22.04",
						Name:   "c",
					},
					Location: "/a/b/c",
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

func TestBatchService_DeterminePlacement(t *testing.T) {
	type strMap map[string]string
	type netMap map[string]api.NetworkPlacement
	netProps := []byte(`{"vlan_id": 1}`)
	cases := []struct {
		name      string
		scriptlet string
		instance  api.InstanceProperties
		networks  migration.Networks

		batchCreateAssertErr require.ErrorAssertionFunc
		placementAssertErr   require.ErrorAssertionFunc
		placement            api.Placement
	}{
		{
			name: "success - no scriptlet, no pools or networks",

			placement:            api.Placement{TargetName: "default", TargetProject: "default", StoragePools: strMap{}, Networks: netMap{}},
			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.NoError,
		},
		{
			name:     "success - no scriptlet, with supported disk",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}}},
			networks: migration.Networks{},

			placement:            api.Placement{TargetName: "default", TargetProject: "default", StoragePools: strMap{"disk1": "default"}, Networks: netMap{}},
			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.NoError,
		},
		{
			name:     "success - no scriptlet, with supported disk and unsupported disk",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}}, // disk2 is unsupported.
			networks: migration.Networks{},

			placement:            api.Placement{TargetName: "default", TargetProject: "default", StoragePools: strMap{"disk1": "default"}, Networks: netMap{}},
			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.NoError,
		},
		{
			name:     "success - no scriptlet, with supported disk and network",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1", HardwareAddress: "/path/to/netname"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname"}},

			placement:            api.Placement{TargetName: "default", TargetProject: "default", StoragePools: strMap{"disk1": "default"}, Networks: netMap{"/path/to/netname": api.NetworkPlacement{Network: "netname", NICType: api.INCUSNICTYPE_MANAGED}}},
			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.NoError,
		},
		{
			name:     "success - with scriptlet",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1", HardwareAddress: "/path/to/netname1"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname1"}},

			scriptlet: `
def placement(instance, batch):
			set_target("tgt1")
			set_project("project1")
			set_pool("disk1", "pool1")
			set_network("/path/to/netname1", "net1", "managed", "")
			`,

			placement:            api.Placement{TargetName: "tgt1", TargetProject: "project1", StoragePools: strMap{"disk1": "pool1"}, Networks: netMap{"/path/to/netname1": api.NetworkPlacement{Network: "net1", NICType: api.INCUSNICTYPE_MANAGED}}},
			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.NoError,
		},
		{
			name: "success - with scriptlet modifying only some networks/pools",
			instance: api.InstanceProperties{
				Disks: []api.InstancePropertiesDisk{
					{Name: "disk1", Supported: true},
					{Name: "disk2", Supported: true},
				},
				NICs: []api.InstancePropertiesNIC{
					{SourceSpecificID: "srcnet1", HardwareAddress: "/path/to/netname1"},
					{SourceSpecificID: "srcnet2", HardwareAddress: "/path/to/netname2"},
					{SourceSpecificID: "srcnet3", HardwareAddress: "/path/to/netname3"},
					{SourceSpecificID: "srcnet4", HardwareAddress: "/path/to/netname4"},
				},
			},
			networks: migration.Networks{
				{SourceSpecificID: "srcnet1", Location: "/path/to/netname1", Type: api.NETWORKTYPE_VMWARE_DISTRIBUTED, Properties: netProps},
				{SourceSpecificID: "srcnet2", Location: "/path/to/netname2", Type: api.NETWORKTYPE_VMWARE_DISTRIBUTED, Properties: netProps},
				{SourceSpecificID: "srcnet3", Location: "/path/to/netname3"},
				{SourceSpecificID: "srcnet4", Location: "/path/to/netname4", Type: api.NETWORKTYPE_VMWARE_DISTRIBUTED, Properties: netProps},
			},

			scriptlet: `
def placement(instance, batch):
			set_target("tgt1")
			set_project("project1")
			set_pool("disk1", "pool1")
			set_network("/path/to/netname1", "net1",    "bridged", "3")
			set_network("/path/to/netname2", "br0",     "bridged", "3,2,1")
			set_network("/path/to/netname4", "br0",     "bridged", "1")
			`,

			placement: api.Placement{
				TargetName:    "tgt1",
				TargetProject: "project1",
				StoragePools:  strMap{"disk1": "pool1", "disk2": "default"},
				Networks: netMap{
					"/path/to/netname1": api.NetworkPlacement{Network: "net1", NICType: api.INCUSNICTYPE_BRIDGED, VlanID: "3"},
					"/path/to/netname2": api.NetworkPlacement{Network: "br0", NICType: api.INCUSNICTYPE_BRIDGED, VlanID: "3,2,1"},
					"/path/to/netname3": api.NetworkPlacement{Network: "netname3", NICType: api.INCUSNICTYPE_MANAGED},
					"/path/to/netname4": api.NetworkPlacement{Network: "br0", NICType: api.INCUSNICTYPE_BRIDGED, VlanID: "1"},
				},
			},
			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.NoError,
		},
		{
			name: "success - with dynamic pool assignment",
			instance: api.InstanceProperties{
				Disks: []api.InstancePropertiesDisk{
					{Name: "disk1", Supported: true},
					{Name: "disk2", Supported: true},
				},
				NICs: []api.InstancePropertiesNIC{
					{SourceSpecificID: "srcnet1", HardwareAddress: "/path/to/netname1"},
					{SourceSpecificID: "srcnet2", HardwareAddress: "/path/to/netname2"},
				},
			},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname1"}, {SourceSpecificID: "srcnet2", Location: "/path/to/netname2"}},

			scriptlet: `
def placement(instance, batch):
			set_target("tgt1")
			set_project("project1")
			for disk in instance.disks:
			  if disk.supported:
			    set_pool(disk.name, "pool1")
			set_network("/path/to/netname1", "net1", "managed", "")
			`,

			placement: api.Placement{
				TargetName:    "tgt1",
				TargetProject: "project1",
				StoragePools:  strMap{"disk1": "pool1", "disk2": "pool1"},
				Networks: netMap{
					"/path/to/netname1": api.NetworkPlacement{Network: "net1", NICType: api.INCUSNICTYPE_MANAGED},
					"/path/to/netname2": api.NetworkPlacement{Network: "netname2", NICType: api.INCUSNICTYPE_MANAGED},
				},
			},
			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.NoError,
		},
		{
			name:     "error - scriptlet syntax",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1"}},

			scriptlet: `
def some_other_func(some_other_field):
			set_target("test")
			`,

			batchCreateAssertErr: require.Error,
			placementAssertErr:   require.NoError,
		},
		{
			name:     "error - set target pool for unknown disk",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1"}},

			scriptlet: `
def placement(instance, batch):
			set_pool("some_disk", "pool1")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
		{
			name:     "error - set target pool for unsupported disk",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1"}},

			scriptlet: `
def placement(instance, batch):
			set_pool("disk2", "pool1")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
		{
			name:     "error - set target network for source instance network with no source",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}, NICs: []api.InstancePropertiesNIC{{HardwareAddress: "srcnet1"}}},
			networks: migration.Networks{}, // No associated network object for the instance's network.

			scriptlet: `
def placement(instance, batch):
			set_network("srcnet1", "net1")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
		{
			name:     "error - set target network for source network not assigned to instance",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}, NICs: []api.InstancePropertiesNIC{}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname2"}},

			scriptlet: `
def placement(instance, batch):
			set_network("/path/to/netname2", "net1")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
		{
			name:     "error - set target vlan ID for unknown network",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1", HardwareAddress: "/path/to/netname1"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname1"}},

			scriptlet: `
def placement(instance, batch):
			set_network("/path/to/netname2", "netname2", "bridged", "3")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
		{
			name:     "error - set target vlan ID for source network not assigned to instance",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}, NICs: []api.InstancePropertiesNIC{}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname1"}},

			scriptlet: `
def placement(instance, batch):
			set_network("/path/to/netname1", "netname1", "bridged", "3")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
		{
			name:     "error - set target vlan ID 0",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1", HardwareAddress: "/path/to/netname1"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname1"}},

			scriptlet: `
def placement(instance, batch):
			set_network("/path/to/netname1", "netname1", "bridged", "0")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
		{
			name:     "error - set target vlan ID list with 0s",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1", HardwareAddress: "/path/to/netname1"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname1"}},

			scriptlet: `
def placement(instance, batch):
			set_network("/path/to/netname1", "netname1", "bridged", "3,0,1")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
		{
			name:     "error - set target vlan ID invalid syntax",
			instance: api.InstanceProperties{Disks: []api.InstancePropertiesDisk{{Name: "disk1", Supported: true}, {Name: "disk2"}}, NICs: []api.InstancePropertiesNIC{{SourceSpecificID: "srcnet1", HardwareAddress: "/path/to/netname1"}}},
			networks: migration.Networks{{SourceSpecificID: "srcnet1", Location: "/path/to/netname1"}},

			scriptlet: `
def placement(instance, batch):
			set_network("/path/to/netname1", "netname1", "bridged", "3 0 1")
			`,

			batchCreateAssertErr: require.NoError,
			placementAssertErr:   require.Error,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			ctx := context.Background()
			repo := &mock.BatchRepoMock{
				CreateFunc: func(ctx context.Context, batch migration.Batch) (int64, error) {
					return 1, nil
				},
			}

			instanceSvc := &InstanceServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.Instances, error) { return nil, nil },
				GetAllFunc:        func(ctx context.Context) (migration.Instances, error) { return nil, nil },
			}

			batchSvc := migration.NewBatchService(repo, instanceSvc)
			batch, err := batchSvc.Create(ctx, migration.Batch{
				Name:              "testbatch",
				Status:            api.BATCHSTATUS_DEFINED,
				IncludeExpression: "true",
				Defaults: api.BatchDefaults{
					Placement: api.BatchPlacement{
						Target:        "default",
						TargetProject: "default",
						StoragePool:   "default",
					},
				},
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
					PlacementScriptlet:       tc.scriptlet,
				},
			})
			tc.batchCreateAssertErr(t, err)

			if err == nil {
				placement, err := batchSvc.DeterminePlacement(ctx, migration.Instance{Properties: tc.instance}, tc.networks, batch, migration.Windows{})
				tc.placementAssertErr(t, err)

				if err == nil {
					require.Equal(t, tc.placement, *placement)
				}
			}
		})
	}
}
