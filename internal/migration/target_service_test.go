package migration_test

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	endpointMock "github.com/FuturFusion/migration-manager/internal/migration/endpoint/mock"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestTargetService_Create(t *testing.T) {
	tests := []struct {
		name             string
		target           migration.Target
		repoCreateTarget migration.Target
		repoCreateErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			target: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert", "connectivity_status": 1}`),
			},
			repoCreateTarget: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert", "connectivity_status": 1}`),
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			target: migration.Target{
				ID:         -1, // invalid
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			target: migration.Target{
				ID:         1,
				Name:       "", // empty
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid endpoint url",
			target: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": ":|\\", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo",
			target: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},
			repoCreateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.TargetRepoMock{
				CreateFunc: func(ctx context.Context, in migration.Target) (int64, error) {
					return tc.repoCreateTarget.ID, tc.repoCreateErr
				},
			}

			endpointFunc := func(t api.Target) (migration.TargetEndpoint, error) {
				return &endpointMock.TargetEndpointMock{
					ConnectFunc: func(ctx context.Context) error {
						return nil
					},
					DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
						return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
					},
					IsWaitingForOIDCTokensFunc: func() bool {
						return false
					},
				}, nil
			}

			targetSvc := migration.NewTargetService(repo)
			tc.target.EndpointFunc = endpointFunc

			// Run test
			target, err := targetSvc.Create(context.Background(), tc.target)

			// Assert
			tc.assertErr(t, err)

			if tc.assertErr == nil {
				require.NotNil(t, tc.repoCreateTarget.EndpointFunc)
				require.NotNil(t, target.EndpointFunc)
			}

			target.EndpointFunc = nil
			tc.repoCreateTarget.EndpointFunc = nil

			require.Equal(t, tc.repoCreateTarget, target)
		})
	}
}

func TestTargetService_GetAll(t *testing.T) {
	tests := []struct {
		name              string
		repoGetAllTargets migration.Targets
		repoGetAllErr     error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllTargets: migration.Targets{
				migration.Target{
					ID:   1,
					Name: "one",
				},
				migration.Target{
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
			repo := &mock.TargetRepoMock{
				GetAllFunc: func(ctx context.Context) (migration.Targets, error) {
					return tc.repoGetAllTargets, tc.repoGetAllErr
				},
			}

			targetSvc := migration.NewTargetService(repo)

			// Run test
			targets, err := targetSvc.GetAll(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, targets, tc.count)
		})
	}
}

func TestTargetService_GetAllNames(t *testing.T) {
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
				"targetA", "targetB",
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
			repo := &mock.TargetRepoMock{
				GetAllNamesFunc: func(ctx context.Context) ([]string, error) {
					return tc.repoGetAllNames, tc.repoGetAllErr
				},
			}

			targetSvc := migration.NewTargetService(repo)

			// Run test
			inventoryNames, err := targetSvc.GetAllNames(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, inventoryNames, tc.count)
		})
	}
}

func TestTargetService_GetByName(t *testing.T) {
	tests := []struct {
		name                string
		nameArg             string
		repoGetByNameTarget *migration.Target
		repoGetByNameErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",
			repoGetByNameTarget: &migration.Target{
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
			repo := &mock.TargetRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Target, error) {
					return tc.repoGetByNameTarget, tc.repoGetByNameErr
				},
			}

			targetSvc := migration.NewTargetService(repo)

			// Run test
			target, err := targetSvc.GetByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetByNameTarget, target)
		})
	}
}

func TestTargetService_UpdateByID(t *testing.T) {
	tests := []struct {
		name          string
		target        migration.Target
		repoUpdateErr error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			target: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			target: migration.Target{
				ID:         -1, // invalid
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			target: migration.Target{
				ID:         1,
				Name:       "", // empty
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - invalid endpoint url",
			target: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": ":|\\", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo",
			target: migration.Target{
				ID:         1,
				Name:       "one",
				TargetType: api.TARGETTYPE_INCUS,
				Properties: json.RawMessage(`{"endpoint": "endpoint.url", "tls_client_key": "key", "tls_client_cert": "cert"}`),
			},
			repoUpdateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.TargetRepoMock{
				UpdateFunc: func(ctx context.Context, in migration.Target) error {
					return tc.repoUpdateErr
				},
			}

			endpointFunc := func(t api.Target) (migration.TargetEndpoint, error) {
				return &endpointMock.TargetEndpointMock{
					ConnectFunc: func(ctx context.Context) error {
						return nil
					},
					DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
						return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
					},
					IsWaitingForOIDCTokensFunc: func() bool {
						return false
					},
				}, nil
			}

			targetSvc := migration.NewTargetService(repo)
			tc.target.EndpointFunc = endpointFunc

			// Run test
			err := targetSvc.Update(context.Background(), tc.target)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestTargetService_DeleteByName(t *testing.T) {
	tests := []struct {
		name                string
		nameArg             string
		repoDeleteByNameErr error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",

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
			name:                "error - repo",
			nameArg:             "one",
			repoDeleteByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.TargetRepoMock{
				DeleteByNameFunc: func(ctx context.Context, name string) error {
					return tc.repoDeleteByNameErr
				},
			}

			targetSvc := migration.NewTargetService(repo)

			// Run test
			err := targetSvc.DeleteByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}
