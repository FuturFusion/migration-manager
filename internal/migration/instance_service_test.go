package migration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/internal/testing/queue"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	uuidA = uuid.MustParse(`432a3dbc-3cf4-4b99-8708-bc6d6e5e867f`)
	uuidB = uuid.MustParse(`7a24aba4-9a90-4132-9429-e0e8a4d3c49f`)
)

func TestInstanceService_Create(t *testing.T) {
	now := time.Date(2025, 1, 22, 9, 12, 53, 0, time.UTC)

	tests := []struct {
		name             string
		instance         migration.Instance
		randomUUIDValues []queue.Item[uuid.UUID]
		repoCreateErr    error

		assertErr    require.ErrorAssertionFunc
		wantInstance migration.Instance
	}{
		{
			name: "success",
			instance: migration.Instance{
				InventoryPath: "/inventory/path",
				SourceID:      1,
			},
			randomUUIDValues: []queue.Item[uuid.UUID]{
				{Value: uuidA, Err: nil},
				{Value: uuidB, Err: nil},
			},

			assertErr: require.NoError,
			wantInstance: migration.Instance{
				UUID:                  uuidA,
				InventoryPath:         "/inventory/path",
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				LastUpdateFromSource:  now,
				SourceID:              1,
				SecretToken:           uuidB,
			},
		},
		{
			name: "error - random uuid 1",
			instance: migration.Instance{
				InventoryPath: "",
			},
			randomUUIDValues: []queue.Item[uuid.UUID]{
				{Value: uuid.Nil, Err: errors.New("boom!")},
			},

			assertErr: require.Error,
		},
		{
			name: "error - random uuid 2",
			instance: migration.Instance{
				InventoryPath: "",
			},
			randomUUIDValues: []queue.Item[uuid.UUID]{
				{Value: uuidA},
				{Value: uuid.Nil, Err: errors.New("boom!")},
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid inventory path",
			instance: migration.Instance{
				InventoryPath: "",
			},
			randomUUIDValues: []queue.Item[uuid.UUID]{
				{Value: uuidA},
				{Value: uuidB},
			},

			assertErr: require.Error,
		},
		{
			name: "error - repo",
			instance: migration.Instance{
				InventoryPath: "/inventory/path",
				SourceID:      1,
			},
			randomUUIDValues: []queue.Item[uuid.UUID]{
				{Value: uuidA},
				{Value: uuidB},
			},
			repoCreateErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				CreateFunc: func(ctx context.Context, in migration.Instance) (migration.Instance, error) {
					if tc.repoCreateErr != nil {
						return migration.Instance{}, tc.repoCreateErr
					}

					return in, tc.repoCreateErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil,
				migration.WithNow(func() time.Time { return now }),
				migration.WithRandomUUID(func() (uuid.UUID, error) { return queue.Pop(t, &tc.randomUUIDValues) }),
			)

			// Run test
			instance, err := instanceSvc.Create(context.Background(), tc.instance)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantInstance, instance)

			// Ensure queues are completely drained.
			require.Empty(t, tc.randomUUIDValues)
		})
	}
}

func TestInstanceService_GetAll(t *testing.T) {
	tests := []struct {
		name                string
		repoGetAllInstances migration.Instances
		repoGetAllErr       error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllInstances: migration.Instances{
				migration.Instance{
					UUID:          uuidA,
					InventoryPath: "/inventory/path/A",
				},
				migration.Instance{
					UUID:          uuidB,
					InventoryPath: "/inventory/path/B",
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:          "error - repo",
			repoGetAllErr: errors.New("boom!"),

			assertErr: require.Error,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetAllFunc: func(ctx context.Context) (migration.Instances, error) {
					return tc.repoGetAllInstances, tc.repoGetAllErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instances, err := instanceSvc.GetAll(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, instances, tc.count)
		})
	}
}

func TestInstanceService_GetAllByState(t *testing.T) {
	tests := []struct {
		name                       string
		repoGetAllByStateInstances migration.Instances
		repoGetAllByStateErr       error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllByStateInstances: migration.Instances{
				migration.Instance{
					UUID:          uuidA,
					InventoryPath: "/inventory/path/A",
				},
				migration.Instance{
					UUID:          uuidB,
					InventoryPath: "/inventory/path/B",
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:                 "error - repo",
			repoGetAllByStateErr: errors.New("boom!"),

			assertErr: require.Error,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetAllByStateFunc: func(ctx context.Context, state api.MigrationStatusType) (migration.Instances, error) {
					return tc.repoGetAllByStateInstances, tc.repoGetAllByStateErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instances, err := instanceSvc.GetAllByState(context.Background(), api.MIGRATIONSTATUS_ASSIGNED_BATCH)

			// Assert
			tc.assertErr(t, err)
			require.Len(t, instances, tc.count)
		})
	}
}

func TestInstanceService_GetAllByBatchID(t *testing.T) {
	tests := []struct {
		name                         string
		repoGetAllByBatchIDInstances migration.Instances
		repoGetAllByBatchIDErr       error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllByBatchIDInstances: migration.Instances{
				migration.Instance{
					UUID:          uuidA,
					InventoryPath: "/inventory/path/A",
				},
				migration.Instance{
					UUID:          uuidB,
					InventoryPath: "/inventory/path/B",
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:                   "error - repo",
			repoGetAllByBatchIDErr: errors.New("boom!"),

			assertErr: require.Error,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetAllByBatchIDFunc: func(ctx context.Context, batchID int) (migration.Instances, error) {
					return tc.repoGetAllByBatchIDInstances, tc.repoGetAllByBatchIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instances, err := instanceSvc.GetAllByBatchID(context.Background(), 1)

			// Assert
			tc.assertErr(t, err)
			require.Len(t, instances, tc.count)
		})
	}
}

func TestInstanceService_GetAllInventoryPaths(t *testing.T) {
	tests := []struct {
		name            string
		repoGetAllUUIDs []uuid.UUID
		repoGetAllErr   error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllUUIDs: []uuid.UUID{
				uuidA, uuidB,
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:          "error - repo",
			repoGetAllErr: errors.New("boom!"),

			assertErr: require.Error,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetAllUUIDsFunc: func(ctx context.Context) ([]uuid.UUID, error) {
					return tc.repoGetAllUUIDs, tc.repoGetAllErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			inventoryNames, err := instanceSvc.GetAllUUIDs(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, inventoryNames, tc.count)
		})
	}
}

func TestInstanceService_GetAllUnassigned(t *testing.T) {
	tests := []struct {
		name                          string
		repoGetAllUnassignedInstances migration.Instances
		repoGetAllUnassignedErr       error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllUnassignedInstances: migration.Instances{
				migration.Instance{
					UUID:          uuidA,
					InventoryPath: "/inventory/path/A",
				},
				migration.Instance{
					UUID:          uuidB,
					InventoryPath: "/inventory/path/B",
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:                    "error - repo",
			repoGetAllUnassignedErr: errors.New("boom!"),

			assertErr: require.Error,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetAllUnassignedFunc: func(ctx context.Context) (migration.Instances, error) {
					return tc.repoGetAllUnassignedInstances, tc.repoGetAllUnassignedErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instances, err := instanceSvc.GetAllUnassigned(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, instances, tc.count)
		})
	}
}

func TestInstanceService_GetByID(t *testing.T) {
	tests := []struct {
		name                string
		uuidArg             uuid.UUID
		repoGetByIDInstance migration.Instance
		repoGetByIDErr      error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:          uuidA,
				InventoryPath: "/inventory/path/A",
			},

			assertErr: require.NoError,
		},
		{
			name:           "error - repo",
			uuidArg:        uuidA,
			repoGetByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (migration.Instance, error) {
					return tc.repoGetByIDInstance, tc.repoGetByIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instance, err := instanceSvc.GetByID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetByIDInstance, instance)
		})
	}
}

func TestInstanceService_GetByIDWithDetails(t *testing.T) {
	tests := []struct {
		name                          string
		uuidArg                       uuid.UUID
		repoGetByIDInstance           migration.Instance
		repoGetByIDErr                error
		repoGetOverridesByIDOverrides migration.Overrides
		repoGetOverridesByIDErr       error
		sourceSvcGetByIDSource        migration.Source
		sourceSvcGetByIDErr           error

		assertErr               require.ErrorAssertionFunc
		wantInstanceWithDetails migration.InstanceWithDetails
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:          uuidA,
				InventoryPath: "/inventory/path/A",
				SourceID:      1,
			},
			repoGetOverridesByIDOverrides: migration.Overrides{
				UUID:          uuidA,
				NumberCPUs:    2,
				MemoryInBytes: 4 * 1024 * 1024 * 1024,
			},
			sourceSvcGetByIDSource: migration.Source{
				ID:         1,
				Name:       "source name",
				SourceType: api.SOURCETYPE_VMWARE,
			},

			assertErr: require.NoError,
			wantInstanceWithDetails: migration.InstanceWithDetails{
				Name:          "A",
				InventoryPath: "/inventory/path/A",
				Source: migration.Source{
					Name:       "source name",
					SourceType: api.SOURCETYPE_VMWARE,
				},
				Overrides: migration.Overrides{
					UUID:          uuidA,
					NumberCPUs:    2,
					MemoryInBytes: 4 * 1024 * 1024 * 1024,
				},
			},
		},
		{
			name:           "error - repo.GetByID",
			uuidArg:        uuidA,
			repoGetByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name:    "error - repo.GetOverridesByID",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:          uuidA,
				InventoryPath: "/inventory/path/A",
				SourceID:      1,
			},
			repoGetOverridesByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name:    "error - sourceSvc.GetByID",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:          uuidA,
				InventoryPath: "/inventory/path/A",
				SourceID:      1,
			},
			repoGetOverridesByIDOverrides: migration.Overrides{
				UUID:          uuidA,
				NumberCPUs:    2,
				MemoryInBytes: 4 * 1024 * 1024 * 1024,
			},
			sourceSvcGetByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (migration.Instance, error) {
					return tc.repoGetByIDInstance, tc.repoGetByIDErr
				},
				GetOverridesByIDFunc: func(ctx context.Context, id uuid.UUID) (migration.Overrides, error) {
					return tc.repoGetOverridesByIDOverrides, tc.repoGetOverridesByIDErr
				},
			}

			sourceSvc := &SourceServiceMock{
				GetByIDFunc: func(ctx context.Context, id int) (migration.Source, error) {
					return tc.sourceSvcGetByIDSource, tc.sourceSvcGetByIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, sourceSvc)

			// Run test
			instanceWithDetails, err := instanceSvc.GetByIDWithDetails(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantInstanceWithDetails, instanceWithDetails)
		})
	}
}

func TestInstanceService_UpdateByID(t *testing.T) {
	tests := []struct {
		name                string
		instance            migration.Instance
		repoGetByIDInstance migration.Instance
		repoGetByIDErr      error
		repoUpdateByIDErr   error

		assertErr    require.ErrorAssertionFunc
		wantInstance migration.Instance
	}{
		{
			name: "success",
			instance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},
			repoGetByIDInstance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},

			assertErr: require.NoError,
			wantInstance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},
		},
		{
			name: "error - invalid UUID",
			instance: migration.Instance{
				UUID:            uuid.Nil, // invalid
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid inventory path",
			instance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid source",
			instance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        -1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid migration status",
			instance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: -1,
			},

			assertErr: require.Error,
		},
		{
			name: "error - repo.GetByID",
			instance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},
			repoGetByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name: "error - already assigned to batch",
			instance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},
			repoGetByIDInstance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				BatchID:         ptr.To(1),
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},

			assertErr: require.Error,
		},
		{
			name: "error - repo.UpdateByID",
			instance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},
			repoGetByIDInstance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},
			repoUpdateByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (migration.Instance, error) {
					return tc.repoGetByIDInstance, tc.repoGetByIDErr
				},
				UpdateByIDFunc: func(ctx context.Context, in migration.Instance) (migration.Instance, error) {
					if tc.repoUpdateByIDErr != nil {
						return migration.Instance{}, tc.repoUpdateByIDErr
					}

					return in, tc.repoUpdateByIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instance, err := instanceSvc.UpdateByID(context.Background(), tc.instance)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantInstance, instance)
		})
	}
}

func TestInstanceService_UnassignFromBatch(t *testing.T) {
	tests := []struct {
		name                string
		uuidArg             uuid.UUID
		repoGetByIDInstance migration.Instance
		repoGetByIDErr      error
		repoUpdateByIDErr   error

		assertErr    require.ErrorAssertionFunc
		wantInstance migration.Instance
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:                  uuidA,
				InventoryPath:         "/inventory/path",
				SourceID:              1,
				BatchID:               ptr.To(1),
				TargetID:              ptr.To(1),
				MigrationStatus:       api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_ASSIGNED_BATCH.String(),
			},

			assertErr: require.NoError,
			wantInstance: migration.Instance{
				UUID:                  uuidA,
				InventoryPath:         "/inventory/path",
				SourceID:              1,
				TargetID:              nil,
				BatchID:               nil,
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
			},
		},
		{
			name:           "error - repo.GetByID",
			repoGetByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name:    "error - repo.UpdateByID",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:                  uuidA,
				InventoryPath:         "/inventory/path",
				SourceID:              1,
				BatchID:               ptr.To(1),
				TargetID:              ptr.To(1),
				MigrationStatus:       api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_ASSIGNED_BATCH.String(),
			},
			repoUpdateByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			var gotInstance migration.Instance
			repo := &mock.InstanceRepoMock{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (migration.Instance, error) {
					return tc.repoGetByIDInstance, tc.repoGetByIDErr
				},
				UpdateByIDFunc: func(ctx context.Context, instance migration.Instance) (migration.Instance, error) {
					if tc.repoUpdateByIDErr != nil {
						return migration.Instance{}, tc.repoUpdateByIDErr
					}

					gotInstance = instance
					return migration.Instance{}, nil
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			err := instanceSvc.UnassignFromBatch(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantInstance, gotInstance)
		})
	}
}

func TestInstanceService_UpdateStatusByID(t *testing.T) {
	tests := []struct {
		name                         string
		uuidArg                      uuid.UUID
		statusArg                    api.MigrationStatusType
		repoUpdateStatusByIDInstance migration.Instance
		repoUpdateStatusByIDErr      error

		assertErr    require.ErrorAssertionFunc
		wantInstance migration.Instance
	}{
		{
			name:      "success",
			uuidArg:   uuidA,
			statusArg: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			repoUpdateStatusByIDInstance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},

			assertErr: require.NoError,
			wantInstance: migration.Instance{
				UUID:            uuidA,
				InventoryPath:   "/inventory/path",
				SourceID:        1,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},
		},
		{
			name:      "error - invalid status",
			uuidArg:   uuidA,
			statusArg: -1, // invalid

			assertErr: require.Error,
		},
		{
			name:                    "error - repo",
			repoUpdateStatusByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				UpdateStatusByIDFunc: func(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (migration.Instance, error) {
					return tc.repoUpdateStatusByIDInstance, tc.repoUpdateStatusByIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instance, err := instanceSvc.UpdateStatusByID(context.Background(), tc.uuidArg, tc.statusArg, "", false)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantInstance, instance)
		})
	}
}

func TestInstanceService_DeleteByID(t *testing.T) {
	tests := []struct {
		name                       string
		uuidArg                    uuid.UUID
		repoGetByIDInstance        migration.Instance
		repoGetByIDErr             error
		repoDeleteOverridesByIDErr error
		repoDeleteByIDErr          error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:            uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},

			assertErr: require.NoError,
		},
		{
			name:           "error - repo.GetByID",
			uuidArg:        uuidA,
			repoGetByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name:    "error - batch ID set",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:            uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				BatchID:         ptr.To(1),
			},

			assertErr: require.Error,
		},
		{
			name:    "error - status is migrating",
			uuidArg: uuidA,
			repoGetByIDInstance: migration.Instance{
				UUID:            uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
			},

			assertErr: require.Error,
		},
		{
			name:                       "error - repo.DeleteOverridesByID",
			uuidArg:                    uuidA,
			repoDeleteOverridesByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name:              "error - repo.DeleteByID",
			uuidArg:           uuidA,
			repoDeleteByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (migration.Instance, error) {
					return tc.repoGetByIDInstance, tc.repoGetByIDErr
				},
				DeleteOverridesByIDFunc: func(ctx context.Context, id uuid.UUID) error {
					return tc.repoDeleteOverridesByIDErr
				},
				DeleteByIDFunc: func(ctx context.Context, id uuid.UUID) error {
					return tc.repoDeleteByIDErr
				},
			}

			targetSvc := migration.NewInstanceService(repo, nil)

			// Run test
			err := targetSvc.DeleteByID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestInstanceService_CreateOverrides(t *testing.T) {
	lastUpdate := time.Date(2025, 1, 22, 9, 12, 53, 0, time.UTC)

	tests := []struct {
		name                    string
		overrides               migration.Overrides
		repoUpdateStatusByIDErr error
		repoCreateOverrides     migration.Overrides
		repoCreateOverridesErr  error

		assertErr  require.ErrorAssertionFunc
		wantStatus api.MigrationStatusType
	}{
		{
			name: "success",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       lastUpdate,
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoCreateOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       lastUpdate,
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},

			assertErr: require.NoError,
		},
		{
			name: "success - disable migration",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       lastUpdate,
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: true,
			},
			repoCreateOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       lastUpdate,
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: true,
			},

			assertErr:  require.NoError,
			wantStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
		},
		{
			name: "error - invalid id",
			overrides: migration.Overrides{
				UUID:             uuid.Nil, // invalid
				LastUpdate:       lastUpdate,
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},

			assertErr: require.Error,
		},
		{
			name: "success - disable migration",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       lastUpdate,
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: true,
			},
			repoUpdateStatusByIDErr: errors.New("boom!"),

			assertErr:  require.Error,
			wantStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
		},
		{
			name: "error - repo",
			overrides: migration.Overrides{
				UUID:             uuid.Must(uuid.NewRandom()),
				LastUpdate:       lastUpdate,
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoCreateOverridesErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				UpdateStatusByIDFunc: func(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (migration.Instance, error) {
					require.Equal(t, tc.wantStatus, status)
					return migration.Instance{}, tc.repoUpdateStatusByIDErr
				},
				CreateOverridesFunc: func(ctx context.Context, in migration.Overrides) (migration.Overrides, error) {
					return tc.repoCreateOverrides, tc.repoCreateOverridesErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			overrides, err := instanceSvc.CreateOverrides(context.Background(), tc.overrides)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoCreateOverrides, overrides)
		})
	}
}

func TestInstanceService_GetOverridesByID(t *testing.T) {
	tests := []struct {
		name                          string
		uuidArg                       uuid.UUID
		repoGetOverridesByIDOverrides migration.Overrides
		repoGetOverridesByIDErr       error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},

			assertErr: require.NoError,
		},
		{
			name:                    "error - repo",
			uuidArg:                 uuidA,
			repoGetOverridesByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetOverridesByIDFunc: func(ctx context.Context, uuid uuid.UUID) (migration.Overrides, error) {
					return tc.repoGetOverridesByIDOverrides, tc.repoGetOverridesByIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			overrides, err := instanceSvc.GetOverridesByID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetOverridesByIDOverrides, overrides)
		})
	}
}

func TestInstanceService_UpdateOverridesByID(t *testing.T) {
	tests := []struct {
		name                             string
		overrides                        migration.Overrides
		repoGetOverridesByIDOverrides    migration.Overrides
		repoGetOverridesByIDErr          error
		repoUpdateStatusByIDErr          error
		repoUpdateOverridesByIDOverrides migration.Overrides
		repoUpdateOverridesByIDErr       error

		assertErr  require.ErrorAssertionFunc
		wantStatus api.MigrationStatusType
	}{
		{
			name: "success",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoGetOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "old comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoUpdateOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},

			assertErr: require.NoError,
		},
		{
			name: "success - new disable migration",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: true,
			},
			repoGetOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoUpdateOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: true,
			},

			assertErr:  require.NoError,
			wantStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
		},
		{
			name: "success - new enable migration",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoGetOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "old comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: true,
			},
			repoUpdateOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},

			assertErr:  require.NoError,
			wantStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		},
		{
			name: "error - invalid id",
			overrides: migration.Overrides{
				UUID:             uuid.Nil, // invalid
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},

			assertErr: require.Error,
		},
		{
			name: "error - repo.GetOverrideByID",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoGetOverridesByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name: "error - repo.UpdateStatusByID",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoGetOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: true,
			},
			repoUpdateStatusByIDErr: errors.New("boom!"),

			assertErr:  require.Error,
			wantStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		},
		{
			name: "error - repo.UpdateOverrideByID",
			overrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoGetOverridesByIDOverrides: migration.Overrides{
				UUID:             uuidA,
				LastUpdate:       time.Now().UTC(),
				Comment:          "comment",
				NumberCPUs:       4,
				MemoryInBytes:    8 * 1024 * 1024 * 1024,
				DisableMigration: false,
			},
			repoUpdateOverridesByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetOverridesByIDFunc: func(ctx context.Context, id uuid.UUID) (migration.Overrides, error) {
					return tc.repoGetOverridesByIDOverrides, tc.repoGetOverridesByIDErr
				},
				UpdateStatusByIDFunc: func(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (migration.Instance, error) {
					require.Equal(t, tc.wantStatus, status)
					return migration.Instance{}, tc.repoUpdateStatusByIDErr
				},
				UpdateOverridesByIDFunc: func(ctx context.Context, in migration.Overrides) (migration.Overrides, error) {
					return tc.repoUpdateOverridesByIDOverrides, tc.repoUpdateOverridesByIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			overrides, err := instanceSvc.UpdateOverridesByID(context.Background(), tc.overrides)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoUpdateOverridesByIDOverrides, overrides)
		})
	}
}

func TestInstanceService_DeleteOverridesByID(t *testing.T) {
	tests := []struct {
		name                          string
		uuidArg                       uuid.UUID
		repoGetOverridesByIDOverrides migration.Overrides
		repoGetOverridesByIDErr       error
		repoUpdateStatusByIDErr       error
		repoDeleteOverridesByIDErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			uuidArg: uuidA,

			assertErr: require.NoError,
		},
		{
			name:    "success - with disabled",
			uuidArg: uuidA,
			repoGetOverridesByIDOverrides: migration.Overrides{
				DisableMigration: true,
			},

			assertErr: require.NoError,
		},
		{
			name:                    "error - repo.GetOverridesByID",
			uuidArg:                 uuidA,
			repoGetOverridesByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name:    "error - repo.UpdateStatusByID",
			uuidArg: uuidA,
			repoGetOverridesByIDOverrides: migration.Overrides{
				DisableMigration: true,
			},
			repoUpdateStatusByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
		{
			name:                       "error - repo",
			uuidArg:                    uuidA,
			repoDeleteOverridesByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetOverridesByIDFunc: func(ctx context.Context, id uuid.UUID) (migration.Overrides, error) {
					return tc.repoGetOverridesByIDOverrides, tc.repoGetOverridesByIDErr
				},
				UpdateStatusByIDFunc: func(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (migration.Instance, error) {
					return migration.Instance{}, tc.repoUpdateStatusByIDErr
				},
				DeleteOverridesByIDFunc: func(ctx context.Context, id uuid.UUID) error {
					return tc.repoDeleteOverridesByIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			err := instanceSvc.DeleteOverridesByID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}
