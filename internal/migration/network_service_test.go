package migration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
)

func TestNetworkService_Create(t *testing.T) {
	tests := []struct {
		name              string
		network           migration.Network
		repoCreateNetwork migration.Network
		repoCreateErr     error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			network: migration.Network{
				ID:     1,
				Name:   "one",
				Config: map[string]string{},
			},
			repoCreateNetwork: migration.Network{
				ID:     1,
				Name:   "one",
				Config: map[string]string{},
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			network: migration.Network{
				ID:     -1, // invalid
				Name:   "one",
				Config: map[string]string{},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			network: migration.Network{
				ID:     1,
				Name:   "", // empty
				Config: map[string]string{},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo",
			network: migration.Network{
				ID:     1,
				Name:   "one",
				Config: map[string]string{},
			},
			repoCreateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.NetworkRepoMock{
				CreateFunc: func(ctx context.Context, in migration.Network) (int64, error) {
					return tc.repoCreateNetwork.ID, tc.repoCreateErr
				},
			}

			networkSvc := migration.NewNetworkService(repo)

			// Run test
			network, err := networkSvc.Create(context.Background(), tc.network)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoCreateNetwork, network)
		})
	}
}

func TestNetworkService_GetAll(t *testing.T) {
	tests := []struct {
		name               string
		repoGetAllNetworks migration.Networks
		repoGetAllErr      error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllNetworks: migration.Networks{
				migration.Network{
					ID:   1,
					Name: "one",
				},
				migration.Network{
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
			repo := &mock.NetworkRepoMock{
				GetAllFunc: func(ctx context.Context) (migration.Networks, error) {
					return tc.repoGetAllNetworks, tc.repoGetAllErr
				},
			}

			networkSvc := migration.NewNetworkService(repo)

			// Run test
			networks, err := networkSvc.GetAll(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, networks, tc.count)
		})
	}
}

func TestNetworkService_GetAllNames(t *testing.T) {
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
				"networkA", "networkB",
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
			repo := &mock.NetworkRepoMock{
				GetAllNamesFunc: func(ctx context.Context) ([]string, error) {
					return tc.repoGetAllNames, tc.repoGetAllErr
				},
			}

			networkSvc := migration.NewNetworkService(repo)

			// Run test
			inventoryNames, err := networkSvc.GetAllNames(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, inventoryNames, tc.count)
		})
	}
}

func TestNetworkService_GetByName(t *testing.T) {
	tests := []struct {
		name                 string
		nameArg              string
		repoGetByNameNetwork *migration.Network
		repoGetByNameErr     error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",
			repoGetByNameNetwork: &migration.Network{
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
			repo := &mock.NetworkRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (*migration.Network, error) {
					return tc.repoGetByNameNetwork, tc.repoGetByNameErr
				},
			}

			networkSvc := migration.NewNetworkService(repo)

			// Run test
			network, err := networkSvc.GetByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetByNameNetwork, network)
		})
	}
}

func TestNetworkService_UpdateByID(t *testing.T) {
	tests := []struct {
		name          string
		network       migration.Network
		repoUpdateErr error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			network: migration.Network{
				ID:     1,
				Name:   "one",
				Config: map[string]string{},
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			network: migration.Network{
				ID:     -1, // invalid
				Name:   "one",
				Config: map[string]string{},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			network: migration.Network{
				ID:     1,
				Name:   "", // empty
				Config: map[string]string{},
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo",
			network: migration.Network{
				ID:     1,
				Name:   "one",
				Config: map[string]string{},
			},
			repoUpdateErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.NetworkRepoMock{
				UpdateFunc: func(ctx context.Context, in migration.Network) error {
					return tc.repoUpdateErr
				},
			}

			networkSvc := migration.NewNetworkService(repo)

			// Run test
			err := networkSvc.Update(context.Background(), &tc.network)

			// Assert
			tc.assertErr(t, err)
		})
	}
}

func TestNetworkService_DeleteByName(t *testing.T) {
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
			repo := &mock.NetworkRepoMock{
				DeleteByNameFunc: func(ctx context.Context, name string) error {
					return tc.repoDeleteByNameErr
				},
			}

			networkSvc := migration.NewNetworkService(repo)

			// Run test
			err := networkSvc.DeleteByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}
