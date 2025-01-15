package migration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
)

func TestTargetService_Create(t *testing.T) {
	tests := []struct {
		name             string
		target           migration.Target
		repoUpsertTarget migration.Target
		repoUpsertErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			target: migration.Target{
				ID:            1,
				Name:          "one",
				Endpoint:      "endpoint.url",
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},
			repoUpsertTarget: migration.Target{
				ID:            1,
				Name:          "one",
				Endpoint:      "endpoint.url",
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			target: migration.Target{
				ID:            -1, // invalid
				Name:          "one",
				Endpoint:      "endpoint.url",
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},

			assertErr: require.Error,
		},
		{
			name: "error - name empty",
			target: migration.Target{
				ID:            1,
				Name:          "", // empty
				Endpoint:      "endpoint.url",
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid endpoint url",
			target: migration.Target{
				ID:            1,
				Name:          "one",
				Endpoint:      ":|\\", // invalid
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.TargetRepoMock{
				CreateFunc: func(ctx context.Context, in migration.Target) (migration.Target, error) {
					return tc.repoUpsertTarget, tc.repoUpsertErr
				},
			}

			targetSvc := migration.NewTargetService(repo)

			// Run test
			target, err := targetSvc.Create(context.Background(), tc.target)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoUpsertTarget, target)
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
			repoGetAllErr: errors.New("boom!"),

			assertErr: require.Error,
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

func TestTargetService_GetByName(t *testing.T) {
	tests := []struct {
		name                string
		nameArg             string
		repoGetByNameTarget migration.Target
		repoGetByNameErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",
			repoGetByNameTarget: migration.Target{
				ID:   1,
				Name: "one",
			},

			assertErr: require.NoError,
		},
		{
			name:    "error - name argument empty string",
			nameArg: "",

			assertErr: require.Error,
		},
		{
			name:             "error - repo",
			nameArg:          "one",
			repoGetByNameErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.TargetRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (migration.Target, error) {
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

func TestTargetService_UpdateByName(t *testing.T) {
	tests := []struct {
		name             string
		target           migration.Target
		repoUpsertTarget migration.Target
		repoUpsertErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			target: migration.Target{
				ID:            1,
				Name:          "one",
				Endpoint:      "endpoint.url",
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},
			repoUpsertTarget: migration.Target{
				ID:            1,
				Name:          "one",
				Endpoint:      "endpoint.url",
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			target: migration.Target{
				ID:            -1, // invalid
				Name:          "one",
				Endpoint:      "endpoint.url",
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},

			assertErr: require.Error,
		},
		{
			name: "error - name empty",
			target: migration.Target{
				ID:            1,
				Name:          "", // empty
				Endpoint:      "endpoint.url",
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid endpoint url",
			target: migration.Target{
				ID:            1,
				Name:          "one",
				Endpoint:      ":|\\", // invalid
				TLSClientKey:  "key",
				TLSClientCert: "cert",
				Insecure:      false,
				IncusProject:  "project",
			},

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.TargetRepoMock{
				UpdateByNameFunc: func(ctx context.Context, in migration.Target) (migration.Target, error) {
					return tc.repoUpsertTarget, tc.repoUpsertErr
				},
			}

			targetSvc := migration.NewTargetService(repo)

			// Run test
			target, err := targetSvc.UpdateByName(context.Background(), tc.target)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoUpsertTarget, target)
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

			assertErr: require.Error,
		},
		{
			name:                "error - repo",
			nameArg:             "one",
			repoDeleteByNameErr: errors.New("boom!"),

			assertErr: require.Error,
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
