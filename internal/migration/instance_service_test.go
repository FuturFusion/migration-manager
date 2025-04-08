package migration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/ptr"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	uuidA = uuid.MustParse(`432a3dbc-3cf4-4b99-8708-bc6d6e5e867f`)
	uuidB = uuid.MustParse(`7a24aba4-9a90-4132-9429-e0e8a4d3c49f`)
	uuidC = uuid.MustParse(`6b1b8486-dbae-4273-83b0-dfb5ceb085a7`)
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
				UUID:                  uuidA,
				Properties:            api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				LastUpdateFromSource:  now,
				Source:                "one",
				SecretToken:           uuidB,
			},

			assertErr: require.NoError,
			wantInstance: migration.Instance{
				UUID:                  uuidA,
				Properties:            api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				LastUpdateFromSource:  now,
				Source:                "one",
				SecretToken:           uuidB,
			},
		},
		{
			name: "error - missing uuid",
			instance: migration.Instance{
				UUID:                  uuid.Nil,
				Properties:            api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				LastUpdateFromSource:  now,
				Source:                "one",
				SecretToken:           uuidB,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - missing secret token",
			instance: migration.Instance{
				UUID:                  uuidA,
				Properties:            api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				LastUpdateFromSource:  now,
				Source:                "one",
				SecretToken:           uuid.Nil,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid inventory path",
			instance: migration.Instance{
				UUID:                  uuidA,
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				LastUpdateFromSource:  now,
				Source:                "one",
				SecretToken:           uuidB,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - source id",
			instance: migration.Instance{
				UUID:                  uuid.Nil,
				Properties:            api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				LastUpdateFromSource:  now,
				Source:                "",
				SecretToken:           uuidB,
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

			instanceSvc := migration.NewInstanceService(repo, nil)

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

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instances, err := instanceSvc.GetAll(context.Background(), false)

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
			repoGetAllByStateErr: boom.Error,

			assertErr: boom.ErrorIs,
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
			instances, err := instanceSvc.GetAllByState(context.Background(), api.MIGRATIONSTATUS_ASSIGNED_BATCH, false)

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

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instances, err := instanceSvc.GetAllByBatch(context.Background(), "one", false)

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

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instances, err := instanceSvc.GetAllUnassigned(context.Background(), false)

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
			name:    "success - with overrides",
			uuidArg: uuidA,
			repoGetByUUIDInstance: &migration.Instance{
				UUID:       uuidA,
				Properties: api.InstanceProperties{Location: "/inventory/path/A"},
				Overrides:  &migration.InstanceOverride{UUID: uuidA},
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

				GetOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error) {
					return &migration.InstanceOverride{}, nil
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			checkOverrides := tc.repoGetByUUIDInstance != nil && tc.repoGetByUUIDInstance.Overrides != nil
			instance, err := instanceSvc.GetByUUID(context.Background(), tc.uuidArg, checkOverrides)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetByUUIDInstance, instance)
		})
	}
}

func TestInstanceService_Update(t *testing.T) {
	tests := []struct {
		name                  string
		instance              migration.Instance
		repoGetByUUIDInstance migration.Instance
		repoGetByUUIDErr      error
		repoUpdateErr         error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			instance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid UUID",
			instance: migration.Instance{
				UUID:            uuid.Nil, // invalid
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid inventory path",
			instance: migration.Instance{
				UUID:            uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid source",
			instance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "",
				SecretToken:     uuidB,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid migration status",
			instance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: -1,
				Source:          "one",
				SecretToken:     uuidB,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo.GetByUUID",
			instance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
			},
			repoGetByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - already assigned to batch",
			instance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				Batch:           ptr.To("one"),
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name: "error - repo.Update",
			instance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
			},
			repoGetByUUIDInstance: migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
				SecretToken:     uuidB,
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
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			err := instanceSvc.Update(context.Background(), &tc.instance)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestInstanceService_UnassignFromBatch(t *testing.T) {
	tests := []struct {
		name                  string
		uuidArg               uuid.UUID
		repoGetByUUIDInstance migration.Instance
		repoGetByUUIDErr      error
		repoUpdateErr         error

		assertErr    require.ErrorAssertionFunc
		wantInstance migration.Instance
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID:                  uuidA,
				Properties:            api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				Batch:                 ptr.To("one"),
				MigrationStatus:       api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_ASSIGNED_BATCH.String(),
				Source:                "one",
			},

			assertErr: require.NoError,
			wantInstance: migration.Instance{
				UUID:                  uuidA,
				Properties:            api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				Batch:                 nil,
				MigrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(),
				Source:                "one",
			},
		},
		{
			name:             "error - repo.GetByUUID",
			repoGetByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - repo.Update",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID:                  uuidA,
				Properties:            api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				Batch:                 ptr.To("one"),
				MigrationStatus:       api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				MigrationStatusString: api.MIGRATIONSTATUS_ASSIGNED_BATCH.String(),
				Source:                "one",
			},
			repoUpdateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			var gotInstance migration.Instance
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return &tc.repoGetByUUIDInstance, tc.repoGetByUUIDErr
				},
				UpdateFunc: func(ctx context.Context, instance migration.Instance) error {
					if tc.repoUpdateErr != nil {
						return tc.repoUpdateErr
					}

					gotInstance = instance
					return nil
				},
				GetOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error) {
					return nil, nil
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

func TestInstanceService_UpdateStatusByUUID(t *testing.T) {
	tests := []struct {
		name                           string
		uuidArg                        uuid.UUID
		statusArg                      api.MigrationStatusType
		repoUpdateStatusByUUIDInstance *migration.Instance
		repoUpdateStatusByUUIDErr      error

		assertErr    require.ErrorAssertionFunc
		wantInstance *migration.Instance
	}{
		{
			name:      "success",
			uuidArg:   uuidA,
			statusArg: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			repoUpdateStatusByUUIDInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
			},

			assertErr: require.NoError,
			wantInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
				Source:          "one",
			},
		},
		{
			name:      "error - invalid status",
			uuidArg:   uuidA,
			statusArg: -1, // invalid

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name:                      "error - repo",
			repoUpdateStatusByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return tc.repoUpdateStatusByUUIDInstance, tc.repoUpdateStatusByUUIDErr
				},

				UpdateFunc: func(ctx context.Context, i migration.Instance) error {
					return tc.repoUpdateStatusByUUIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			instance, err := instanceSvc.UpdateStatusByUUID(context.Background(), tc.uuidArg, tc.statusArg, "", false)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.wantInstance, instance)
		})
	}
}

func TestInstanceService_ProcessWorkerUpdate(t *testing.T) {
	tests := []struct {
		name                  string
		uuidArg               uuid.UUID
		workerResponseTypeArg api.WorkerResponseType
		statusStringArg       string

		repoGetByUUIDInstance          *migration.Instance
		repoGetByUUIDErr               error
		repoUpdateStatusByUUIDInstance *migration.Instance
		repoUpdateStatusByUUIDErr      error

		assertErr                 require.ErrorAssertionFunc
		wantMigrationStatus       api.MigrationStatusType
		wantMigrationStatusString string
	}{
		{
			name:                  "success - migration running",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_RUNNING,
			statusStringArg:       "creating",
			repoGetByUUIDInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				Batch:           ptr.To("one"),
				Source:          "one",
			},

			assertErr:                 require.NoError,
			wantMigrationStatus:       api.MIGRATIONSTATUS_CREATING,
			wantMigrationStatusString: "creating",
		},
		{
			name:                  "success - migration success background import",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_SUCCESS,
			statusStringArg:       "done",
			repoGetByUUIDInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
				Batch:           ptr.To("one"),
				Source:          "one",
			},

			assertErr:                 require.NoError,
			wantMigrationStatus:       api.MIGRATIONSTATUS_IDLE,
			wantMigrationStatusString: "Idle",
		},
		{
			name:                  "success - migration success final import",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_SUCCESS,
			statusStringArg:       "done",
			repoGetByUUIDInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_FINAL_IMPORT,
				Batch:           ptr.To("one"),
				Source:          "one",
			},

			assertErr:                 require.NoError,
			wantMigrationStatus:       api.MIGRATIONSTATUS_IMPORT_COMPLETE,
			wantMigrationStatusString: "Import tasks complete",
		},
		{
			name:                  "success - migration failed",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_FAILED,
			statusStringArg:       "boom!",
			repoGetByUUIDInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				Batch:           ptr.To("one"),
				Source:          "one",
			},

			assertErr:                 require.NoError,
			wantMigrationStatus:       api.MIGRATIONSTATUS_ERROR,
			wantMigrationStatusString: "boom!",
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
			name:                  "error - instance not part of batch",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_RUNNING,
			statusStringArg:       "creating",
			repoGetByUUIDInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				Batch:           nil, // not assigned to batch
				Source:          "one",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrNotFound, a...)
			},
		},
		{
			name:                  "error - instance not in migration state",
			uuidArg:               uuidA,
			workerResponseTypeArg: api.WORKERRESPONSE_RUNNING,
			statusStringArg:       "creating",
			repoGetByUUIDInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, // not in migration state
				Batch:           ptr.To("one"),
				Source:          "one",
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
			repoGetByUUIDInstance: &migration.Instance{
				UUID:            uuidA,
				Properties:      api.InstanceProperties{Location: "/inventory/path", Name: "path"},
				MigrationStatus: api.MIGRATIONSTATUS_CREATING,
				Batch:           ptr.To("one"),
				Source:          "one",
			},
			repoUpdateStatusByUUIDErr: boom.Error,

			assertErr:                 boom.ErrorIs,
			wantMigrationStatus:       api.MIGRATIONSTATUS_CREATING,
			wantMigrationStatusString: "creating",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return tc.repoGetByUUIDInstance, tc.repoGetByUUIDErr
				},
				UpdateFunc: func(ctx context.Context, i migration.Instance) error {
					require.Equal(t, tc.wantMigrationStatus, i.MigrationStatus)
					require.Equal(t, tc.wantMigrationStatusString, i.MigrationStatusString)
					return tc.repoUpdateStatusByUUIDErr
				},
				GetOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error) {
					return nil, nil
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			_, err := instanceSvc.ProcessWorkerUpdate(context.Background(), tc.uuidArg, tc.workerResponseTypeArg, tc.statusStringArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestInstanceService_DeleteByUUID(t *testing.T) {
	tests := []struct {
		name                         string
		uuidArg                      uuid.UUID
		repoGetByUUIDInstance        migration.Instance
		repoGetByUUIDErr             error
		repoDeleteOverridesByUUIDErr error
		repoDeleteByUUIDErr          error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID:            uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			},

			assertErr: require.NoError,
		},
		{
			name:    "success - batch ID set, modifiable",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID:            uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
				Batch:           ptr.To("one"),
			},

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
				UUID:            uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_ASSIGNED_BATCH,
				Batch:           ptr.To("one"),
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:    "error - status is migrating",
			uuidArg: uuidA,
			repoGetByUUIDInstance: migration.Instance{
				UUID:            uuidA,
				MigrationStatus: api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:                         "error - repo.DeleteOverridesByUUID",
			uuidArg:                      uuidA,
			repoDeleteOverridesByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                "error - repo.DeleteByUUID",
			uuidArg:             uuidA,
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
				DeleteOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) error {
					return tc.repoDeleteOverridesByUUIDErr
				},
				DeleteByUUIDFunc: func(ctx context.Context, id uuid.UUID) error {
					return tc.repoDeleteByUUIDErr
				},
			}

			targetSvc := migration.NewInstanceService(repo, nil)

			// Run test
			err := targetSvc.DeleteByUUID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestInstanceService_CreateOverrides(t *testing.T) {
	lastUpdate := time.Date(2025, 1, 22, 9, 12, 53, 0, time.UTC)

	tests := []struct {
		name                      string
		overrides                 migration.InstanceOverride
		repoUpdateStatusByUUIDErr error
		repoCreateOverrides       migration.InstanceOverride
		repoCreateOverridesErr    error

		assertErr  require.ErrorAssertionFunc
		wantStatus api.MigrationStatusType
	}{
		{
			name: "success",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: lastUpdate,
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},
			repoCreateOverrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: lastUpdate,
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},

			assertErr: require.NoError,
		},
		{
			name: "success - disable migration",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: lastUpdate,
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: true,
			},
			repoCreateOverrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: lastUpdate,
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: true,
			},

			assertErr:  require.NoError,
			wantStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
		},
		{
			name: "error - invalid id",
			overrides: migration.InstanceOverride{
				UUID:       uuid.Nil, // invalid
				LastUpdate: lastUpdate,
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "success - disable migration",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: lastUpdate,
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: true,
			},
			repoUpdateStatusByUUIDErr: boom.Error,

			assertErr:  boom.ErrorIs,
			wantStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
		},
		{
			name: "error - repo",
			overrides: migration.InstanceOverride{
				UUID:       uuid.Must(uuid.NewRandom()),
				LastUpdate: lastUpdate,
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},
			repoCreateOverridesErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return &migration.Instance{UUID: id}, nil
				},
				UpdateFunc: func(ctx context.Context, i migration.Instance) error {
					require.Equal(t, tc.wantStatus, i.MigrationStatus)
					return tc.repoUpdateStatusByUUIDErr
				},
				CreateOverridesFunc: func(ctx context.Context, in migration.InstanceOverride) (int64, error) {
					return tc.repoCreateOverrides.ID, tc.repoCreateOverridesErr
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

func TestInstanceService_GetOverridesByUUID(t *testing.T) {
	tests := []struct {
		name                            string
		uuidArg                         uuid.UUID
		repoGetOverridesByUUIDOverrides *migration.InstanceOverride
		repoGetOverridesByUUIDErr       error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			uuidArg: uuidA,
			repoGetOverridesByUUIDOverrides: &migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},

			assertErr: require.NoError,
		},
		{
			name:                      "error - repo",
			uuidArg:                   uuidA,
			repoGetOverridesByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetOverridesByUUIDFunc: func(ctx context.Context, uuid uuid.UUID) (*migration.InstanceOverride, error) {
					return tc.repoGetOverridesByUUIDOverrides, tc.repoGetOverridesByUUIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			overrides, err := instanceSvc.GetOverridesByUUID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetOverridesByUUIDOverrides, overrides)
		})
	}
}

func TestInstanceService_UpdateOverridesByUUID(t *testing.T) {
	tests := []struct {
		name                            string
		overrides                       migration.InstanceOverride
		repoGetOverridesByUUIDOverrides migration.InstanceOverride
		repoGetOverridesByUUIDErr       error
		repoUpdateStatusByUUIDErr       error
		repoUpdateOverridesByUUIDErr    error

		assertErr  require.ErrorAssertionFunc
		wantStatus api.MigrationStatusType
	}{
		{
			name: "success",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},
			repoGetOverridesByUUIDOverrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "old comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},

			assertErr: require.NoError,
		},
		{
			name: "success - new disable migration",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: true,
			},
			repoGetOverridesByUUIDOverrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},

			assertErr:  require.NoError,
			wantStatus: api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
		},
		{
			name: "success - new enable migration",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},
			repoGetOverridesByUUIDOverrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "old comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: true,
			},

			assertErr:  require.NoError,
			wantStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		},
		{
			name: "error - invalid id",
			overrides: migration.InstanceOverride{
				UUID:       uuid.Nil, // invalid
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo.GetOverrideByUUID",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},
			repoGetOverridesByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name: "error - repo.UpdateStatusByUUID",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},
			repoGetOverridesByUUIDOverrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: true,
			},
			repoUpdateStatusByUUIDErr: boom.Error,

			assertErr:  boom.ErrorIs,
			wantStatus: api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
		},
		{
			name: "error - repo.UpdateOverrideByUUID",
			overrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},
			repoGetOverridesByUUIDOverrides: migration.InstanceOverride{
				UUID:       uuidA,
				LastUpdate: time.Now().UTC(),
				Comment:    "comment",
				Properties: api.InstancePropertiesConfigurable{
					CPUs:   4,
					Memory: 8 * 1024 * 1024 * 1024,
				},
				DisableMigration: false,
			},
			repoUpdateOverridesByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return &migration.Instance{UUID: id}, nil
				},
				GetOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error) {
					return &tc.repoGetOverridesByUUIDOverrides, tc.repoGetOverridesByUUIDErr
				},
				UpdateFunc: func(ctx context.Context, i migration.Instance) error {
					require.Equal(t, tc.wantStatus, i.MigrationStatus)
					return tc.repoUpdateStatusByUUIDErr
				},
				UpdateOverridesFunc: func(ctx context.Context, in migration.InstanceOverride) error {
					return tc.repoUpdateOverridesByUUIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			err := instanceSvc.UpdateOverrides(context.Background(), &tc.overrides)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestInstanceService_DeleteOverridesByUUID(t *testing.T) {
	tests := []struct {
		name                            string
		uuidArg                         uuid.UUID
		repoGetOverridesByUUIDOverrides migration.InstanceOverride
		repoGetOverridesByUUIDErr       error
		repoUpdateStatusByUUIDErr       error
		repoDeleteOverridesByUUIDErr    error

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
			repoGetOverridesByUUIDOverrides: migration.InstanceOverride{
				DisableMigration: true,
			},

			assertErr: require.NoError,
		},
		{
			name:                      "error - repo.GetOverridesByUUID",
			uuidArg:                   uuidA,
			repoGetOverridesByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:    "error - repo.UpdateStatusByUUID",
			uuidArg: uuidA,
			repoGetOverridesByUUIDOverrides: migration.InstanceOverride{
				DisableMigration: true,
			},
			repoUpdateStatusByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                         "error - repo",
			uuidArg:                      uuidA,
			repoDeleteOverridesByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.InstanceRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Instance, error) {
					return &migration.Instance{UUID: id}, nil
				},
				GetOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.InstanceOverride, error) {
					return &tc.repoGetOverridesByUUIDOverrides, tc.repoGetOverridesByUUIDErr
				},
				UpdateFunc: func(ctx context.Context, i migration.Instance) error {
					return tc.repoUpdateStatusByUUIDErr
				},
				DeleteOverridesByUUIDFunc: func(ctx context.Context, id uuid.UUID) error {
					return tc.repoDeleteOverridesByUUIDErr
				},
			}

			instanceSvc := migration.NewInstanceService(repo, nil)

			// Run test
			err := instanceSvc.DeleteOverridesByUUID(context.Background(), tc.uuidArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}
