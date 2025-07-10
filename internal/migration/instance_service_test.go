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

var (
	uuidA = uuid.MustParse(`432a3dbc-3cf4-4b99-8708-bc6d6e5e867f`)
	uuidB = uuid.MustParse(`7a24aba4-9a90-4132-9429-e0e8a4d3c49f`)
)

func TestInstanceService_Create(t *testing.T) {
	now := time.Date(2025, 1, 22, 9, 12, 53, 0, time.UTC)

	tests := []struct {
		name          string
		instance      migration.Instance
		repoCreateErr error

		assertErr    require.ErrorAssertionFunc
		wantInstance migration.Instance
	}{
		{
			name: "success",
			instance: migration.Instance{
				UUID:                 uuidA,
				Properties:           api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				LastUpdateFromSource: now,
				Source:               "one",
			},

			assertErr: require.NoError,
			wantInstance: migration.Instance{
				UUID:                 uuidA,
				Properties:           api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				LastUpdateFromSource: now,
				Source:               "one",
			},
		},
		{
			name: "error - missing uuid",
			instance: migration.Instance{
				UUID:                 uuid.Nil,
				Properties:           api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				LastUpdateFromSource: now,
				Source:               "one",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid inventory path",
			instance: migration.Instance{
				UUID: uuidA,

				LastUpdateFromSource: now,
				Source:               "one",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - source id",
			instance: migration.Instance{
				UUID:       uuid.Nil,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path"},

				LastUpdateFromSource: now,
				Source:               "",
			},

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				CreateFunc: func(ctx context.Context, in migration.Instance) (int64, error) {
					if tc.repoCreateErr != nil {
						return -1, tc.repoCreateErr
					}

					return in.ID, tc.repoCreateErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo)

			// Run test
			instance, err := instanceSvc.Create(context.Background(), tc.instance)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantInstance, instance)
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
					UUID:       uuidA,
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
				migration.Instance{
					UUID:       uuidB,
					Properties: api.InstanceProperties{Location: "/inventory/path/B"},
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
			repo := &mock.InstanceRepoMock{
				GetAllFunc: func(ctx context.Context) (migration.Instances, error) {
					return tc.repoGetAllInstances, tc.repoGetAllErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo)

			// Run test
			instances, err := instanceSvc.GetAll(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, instances, tc.count)
		})
	}
}

func TestInstanceService_GetAllByBatch(t *testing.T) {
	tests := []struct {
		name                       string
		repoGetAllByBatchInstances migration.Instances
		repoGetAllByBatchErr       error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllByBatchInstances: migration.Instances{
				migration.Instance{
					UUID:       uuidA,
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
				migration.Instance{
					UUID:       uuidB,
					Properties: api.InstanceProperties{Location: "/inventory/path/B"},
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:                 "error - repo",
			repoGetAllByBatchErr: boom.Error,

			assertErr: boom.ErrorIs,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.Instances, error) {
					return tc.repoGetAllByBatchInstances, tc.repoGetAllByBatchErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo)

			// Run test
			instances, err := instanceSvc.GetAllByBatch(context.Background(), "one")

			// Assert
			tc.assertErr(t, err)
			require.Len(t, instances, tc.count)
		})
	}
}

func TestInstanceService_GetAllLocations(t *testing.T) {
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
			repoGetAllErr: boom.Error,

			assertErr: boom.ErrorIs,
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

			instanceSvc := migration.NewInstanceService(repo)

			// Run test
			inventoryNames, err := instanceSvc.GetAllUUIDs(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, inventoryNames, tc.count)
		})
	}
}

func TestInstanceService_GetAllAssigned(t *testing.T) {
	tests := []struct {
		name                        string
		repoGetAllAssignedInstances migration.Instances
		repoGetAllAssignedErr       error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllAssignedInstances: migration.Instances{
				migration.Instance{
					UUID:       uuidA,
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
				migration.Instance{
					UUID:       uuidB,
					Properties: api.InstanceProperties{Location: "/inventory/path/B"},
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:                  "error - repo",
			repoGetAllAssignedErr: boom.Error,

			assertErr: boom.ErrorIs,
			count:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetAllAssignedFunc: func(ctx context.Context) (migration.Instances, error) {
					return tc.repoGetAllAssignedInstances, tc.repoGetAllAssignedErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo)

			// Run test
			instances, err := instanceSvc.GetAllAssigned(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, instances, tc.count)
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
					UUID:       uuidA,
					Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				},
				migration.Instance{
					UUID:       uuidB,
					Properties: api.InstanceProperties{Location: "/inventory/path/B"},
				},
			},

			assertErr: require.NoError,
			count:     2,
		},
		{
			name:                    "error - repo",
			repoGetAllUnassignedErr: boom.Error,

			assertErr: boom.ErrorIs,
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

			instanceSvc := migration.NewInstanceService(repo)

			// Run test
			instances, err := instanceSvc.GetAllUnassigned(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, instances, tc.count)
		})
	}
}

func TestInstanceService_GetByUUID(t *testing.T) {
	tests := []struct {
		name                  string
		uuidArg               uuid.UUID
		repoGetByUUIDInstance *migration.Instance
		repoGetByUUIDErr      error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetByUUIDInstance: &migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path/A"},
			},

			assertErr: require.NoError,
		},
		{
			name:             "error - repo",
			uuidArg:          uuidA,
			repoGetByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return tc.repoGetByUUIDInstance, tc.repoGetByUUIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo)

			// Run test
			instance, err := instanceSvc.GetByUUID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetByUUIDInstance, instance)
		})
	}
}

func TestInstanceService_Update(t *testing.T) {
	tests := []struct {
		name                        string
		instance                    migration.Instance
		repoGetByUUIDInstance       migration.Instance
		repoGetByUUIDErr            error
		repoUpdateErr               error
		instanceSvcGetBatchesByUUID migration.Batches

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},

			assertErr: require.NoError,
		},
		{
			name: "success - can edit if in a batch, but already disabled",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
				Overrides:  api.InstanceOverride{DisableMigration: true, Comment: "edited instance"},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
				Overrides:  api.InstanceOverride{DisableMigration: true},

				Source: "one",
			},
			instanceSvcGetBatchesByUUID: migration.Batches{{}},

			assertErr: require.NoError,
		},
		{
			name: "success - can edit and enable if in a running batch, but already disabled",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
				Overrides:  api.InstanceOverride{Comment: "edited instance"},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
				Overrides:  api.InstanceOverride{DisableMigration: true},

				Source: "one",
			},
			instanceSvcGetBatchesByUUID: migration.Batches{{Status: api.BATCHSTATUS_RUNNING}},

			assertErr: require.NoError,
		},
		{
			name: "success - can only disable if in non-running batches",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
				Overrides:  api.InstanceOverride{DisableMigration: true},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			instanceSvcGetBatchesByUUID: migration.Batches{{Status: api.BATCHSTATUS_DEFINED}},

			assertErr: require.NoError,
		},
		{
			name: "error - cannot edit if in a running batch",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
				Overrides:  api.InstanceOverride{Comment: "edited instance"},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			instanceSvcGetBatchesByUUID: migration.Batches{{Status: api.BATCHSTATUS_RUNNING}},

			assertErr: require.Error,
		},
		{
			name: "error - cannot edit if in a non-running batch",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
				Overrides:  api.InstanceOverride{Comment: "edited instance"},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			instanceSvcGetBatchesByUUID: migration.Batches{{Status: api.BATCHSTATUS_DEFINED}},

			assertErr: require.Error,
		},
		{
			name: "error - invalid UUID",
			instance: migration.Instance{
				UUID:       uuid.Nil, // invalid
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid inventory path",
			instance: migration.Instance{
				UUID: uuidA,

				Source: "one",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid source",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo.GetByUUID",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			repoGetByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - already assigned to batch",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			instanceSvcGetBatchesByUUID: migration.Batches{
				{},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},

		{
			name: "error - can't disable, already assigned to running batch",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
				Overrides:  api.InstanceOverride{DisableMigration: true},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			instanceSvcGetBatchesByUUID: migration.Batches{
				{Status: api.BATCHSTATUS_QUEUED},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name: "error - repo.Update",
			instance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},

				Source: "one",
			},
			repoUpdateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return &tc.repoGetByUUIDInstance, tc.repoGetByUUIDErr
				},
				UpdateFunc: func(ctx context.Context, in migration.Instance) error {
					if tc.repoUpdateErr != nil {
						return tc.repoUpdateErr
					}

					return tc.repoUpdateErr
				},

				GetBatchesByUUIDFunc: func(ctx context.Context, instanceUUID uuid.UUID) (migration.Batches, error) {
					return tc.instanceSvcGetBatchesByUUID, nil
				},
			}

			instanceSvc := migration.NewInstanceService(repo)

			// Run test
			err := instanceSvc.Update(context.Background(), &tc.instance)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestInstanceService_DeleteByUUID(t *testing.T) {
	tests := []struct {
		name                        string
		uuidArg                     uuid.UUID
		repoGetByUUIDInstance       migration.Instance
		repoGetByUUIDErr            error
		repoDeleteByUUIDErr         error
		instanceSvcGetBatchesByUUID migration.Batches

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID: uuidA,
			},

			assertErr: require.NoError,
		},
		{
			name:    "success - batch ID set, modifiable (instance is disabled)",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID:      uuidA,
				Overrides: api.InstanceOverride{DisableMigration: true},
			},
			instanceSvcGetBatchesByUUID: migration.Batches{{}},

			assertErr: require.NoError,
		},
		{
			name:             "error - repo.GetByUUID",
			uuidArg:          uuidA,
			repoGetByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - batch ID set, not modifiable",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path", Name: "path", OS: "os", OSVersion: "os_version", BackgroundImport: true},
			},
			instanceSvcGetBatchesByUUID: migration.Batches{{}},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:    "error - repo.DeleteByUUID",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID: uuidA,
			},
			repoDeleteByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return &tc.repoGetByUUIDInstance, tc.repoGetByUUIDErr
				},
				DeleteByUUIDFunc: func(ctx context.Context, id uuid.UUID) error {
					return tc.repoDeleteByUUIDErr
				},
				GetBatchesByUUIDFunc: func(ctx context.Context, instanceUUID uuid.UUID) (migration.Batches, error) {
					return tc.instanceSvcGetBatchesByUUID, nil
				},
			}

			targetSvc := migration.NewInstanceService(repo)

			// Run test
			err := targetSvc.DeleteByUUID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}
