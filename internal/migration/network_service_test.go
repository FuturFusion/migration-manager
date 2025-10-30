package migration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/shared/api"
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
				ID:         1,
				SourceSpecificID: "one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
				Source:     "src",
				Location:   "/path/to/one",
			},
			repoCreateNetwork: migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Source:     "src",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
				Location:   "/path/to/one",
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			network: migration.Network{
				ID:         -1, // invalid
				SourceSpecificID: "one",
				Source:     "src",
				Location:   "/path/to/one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "", // empty
				Location:   "/path/to/one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
				Source:     "src",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - location empty",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Location:   "", // empty
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
				Source:     "src",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - type empty",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Location:   "/path/to/one",
				Source:     "src",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - source empty",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Location:   "/path/to/one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Location:   "/path/to/one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
				Source:     "src",
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
					ID:         1,
					SourceSpecificID: "one",
				},
				migration.Network{
					ID:         2,
					SourceSpecificID: "two",
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

func TestNetworkService_GetByName(t *testing.T) {
	tests := []struct {
		name                 string
		nameArg              string
		sourceArg            string
		repoGetByNameNetwork *migration.Network
		repoGetByNameErr     error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "success",
			nameArg:   "one",
			sourceArg: "src",
			repoGetByNameNetwork: &migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Location:   "/path/to/one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
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
			name:      "error - source argument empty string",
			nameArg:   "one",
			sourceArg: "",

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:             "error - repo",
			nameArg:          "one",
			sourceArg:        "src",
			repoGetByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.NetworkRepoMock{
				GetByNameAndSourceFunc: func(ctx context.Context, name string, src string) (*migration.Network, error) {
					return tc.repoGetByNameNetwork, tc.repoGetByNameErr
				},
			}

			networkSvc := migration.NewNetworkService(repo)

			// Run test
			network, err := networkSvc.GetByNameAndSource(context.Background(), tc.nameArg, tc.sourceArg)

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
				ID:         1,
				SourceSpecificID: "one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
				Location:   "/path/to/one",
				Source:     "src",
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			network: migration.Network{
				ID:         -1, // invalid
				SourceSpecificID: "one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
				Location:   "/path/to/one",
				Source:     "src",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - name empty",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "", // empty
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
				Location:   "/path/to/one",
				Source:     "src",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - type empty",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Location:   "/path/to/one",
				Source:     "src",
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - source empty",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Location:   "/path/to/one",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
			},

			assertErr: func(tt require.TestingT, err error, a ...any) {
				var verr migration.ErrValidation
				require.ErrorAs(tt, err, &verr, a...)
			},
		},
		{
			name: "error - repo",
			network: migration.Network{
				ID:         1,
				SourceSpecificID: "one",
				Location:   "/path/to/one",
				Source:     "src",
				Type:       api.NETWORKTYPE_VMWARE_STANDARD,
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
		sourceArg           string
		repoDeleteByNameErr error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "success",
			nameArg:   "one",
			sourceArg: "src",

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
			name:      "error - source argument empty string",
			nameArg:   "one",
			sourceArg: "",

			assertErr: func(tt require.TestingT, err error, a ...any) {
				require.ErrorIs(tt, err, migration.ErrOperationNotPermitted, a...)
			},
		},
		{
			name:                "error - repo",
			nameArg:             "one",
			sourceArg:           "src",
			repoDeleteByNameErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.NetworkRepoMock{
				DeleteByNameAndSourceFunc: func(ctx context.Context, name string, src string) error {
					return tc.repoDeleteByNameErr
				},
			}

			networkSvc := migration.NewNetworkService(repo)

			// Run test
			err := networkSvc.DeleteByNameAndSource(context.Background(), tc.nameArg, tc.sourceArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}
