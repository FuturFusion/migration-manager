package migration_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestSourceService_Create(t *testing.T) {
	tests := []struct {
		name             string
		source           migration.Source
		repoCreateSource migration.Source
		repoCreateErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success - common",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},
			repoCreateSource: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},

			assertErr: require.NoError,
		},
		{
			name: "success - VMware",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": "endpoint.url",
  "username": "user",
  "password": "pass"
}
`),
			},
			repoCreateSource: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": "endpoint.url",
  "username": "user",
  "password": "pass"
}
`),
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			source: migration.Source{
				ID:         -1, // invalid
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - name empty",
			source: migration.Source{
				ID:         1,
				Name:       "", // empty
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid source type",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SourceType(-1), // invalid
				Properties: json.RawMessage(`{}`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - properties nil",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: nil, // nil
			},

			assertErr: require.Error,
		},
		{
			name: "error - common properties invalid json",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{`), // invalid json
			},

			assertErr: require.Error,
		},
		{
			name: "error - VMware properties invalid json",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{`), // invalid json
			},

			assertErr: require.Error,
		},
		{
			name: "error - VMware invalid endpoint",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": ":|\\",
  "username": "user",
  "password": "pass"
}
`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - VMware empty username",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": "enpoint.url",
  "username": "",
  "password": "pass"
}
`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - VMware empty password",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": "enpoint.url",
  "username": "user",
  "password": ""
}
`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - repo",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},
			repoCreateErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.SourceRepoMock{
				CreateFunc: func(ctx context.Context, in migration.Source) (migration.Source, error) {
					return tc.repoCreateSource, tc.repoCreateErr
				},
			}

			sourceSvc := migration.NewSourceService(repo)

			// Run test
			source, err := sourceSvc.Create(context.Background(), tc.source)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoCreateSource, source)
		})
	}
}

func TestSourceService_GetAll(t *testing.T) {
	tests := []struct {
		name              string
		repoGetAllSources migration.Sources
		repoGetAllErr     error

		assertErr require.ErrorAssertionFunc
		count     int
	}{
		{
			name: "success",
			repoGetAllSources: migration.Sources{
				migration.Source{
					ID:   1,
					Name: "one",
				},
				migration.Source{
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
			repo := &mock.SourceRepoMock{
				GetAllFunc: func(ctx context.Context) (migration.Sources, error) {
					return tc.repoGetAllSources, tc.repoGetAllErr
				},
			}

			sourceSvc := migration.NewSourceService(repo)

			// Run test
			sources, err := sourceSvc.GetAll(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, sources, tc.count)
		})
	}
}

func TestSourceService_GetAllNames(t *testing.T) {
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
				"sourceA", "sourceB",
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
			repo := &mock.SourceRepoMock{
				GetAllNamesFunc: func(ctx context.Context) ([]string, error) {
					return tc.repoGetAllNames, tc.repoGetAllErr
				},
			}

			sourceSvc := migration.NewSourceService(repo)

			// Run test
			inventoryNames, err := sourceSvc.GetAllNames(context.Background())

			// Assert
			tc.assertErr(t, err)
			require.Len(t, inventoryNames, tc.count)
		})
	}
}

func TestSourceService_GetByID(t *testing.T) {
	tests := []struct {
		name              string
		repoGetByIDSource migration.Source
		repoGetByIDErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			repoGetByIDSource: migration.Source{
				ID:   1,
				Name: "one",
			},

			assertErr: require.NoError,
		},
		{
			name:           "error - repo",
			repoGetByIDErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.SourceRepoMock{
				GetByIDFunc: func(ctx context.Context, id int) (migration.Source, error) {
					return tc.repoGetByIDSource, tc.repoGetByIDErr
				},
			}

			sourceSvc := migration.NewSourceService(repo)

			// Run test
			source, err := sourceSvc.GetByID(context.Background(), 1)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetByIDSource, source)
		})
	}
}

func TestSourceService_GetByName(t *testing.T) {
	tests := []struct {
		name                string
		nameArg             string
		repoGetByNameSource migration.Source
		repoGetByNameErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			nameArg: "one",
			repoGetByNameSource: migration.Source{
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
			repo := &mock.SourceRepoMock{
				GetByNameFunc: func(ctx context.Context, name string) (migration.Source, error) {
					return tc.repoGetByNameSource, tc.repoGetByNameErr
				},
			}

			sourceSvc := migration.NewSourceService(repo)

			// Run test
			source, err := sourceSvc.GetByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoGetByNameSource, source)
		})
	}
}

func TestSourceService_UpdateByName(t *testing.T) {
	tests := []struct {
		name             string
		source           migration.Source
		repoUpdateSource migration.Source
		repoUpdateErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success - common",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},
			repoUpdateSource: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},

			assertErr: require.NoError,
		},
		{
			name: "success - VMware",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": "endpoint.url",
  "username": "user",
  "password": "pass"
}
`),
			},
			repoUpdateSource: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": "endpoint.url",
  "username": "user",
  "password": "pass"
}
`),
			},

			assertErr: require.NoError,
		},
		{
			name: "error - invalid id",
			source: migration.Source{
				ID:         -1, // invalid
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - name empty",
			source: migration.Source{
				ID:         1,
				Name:       "", // empty
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - invalid source type",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SourceType(-1), // invalid
				Properties: json.RawMessage(`{}`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - properties nil",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: nil, // nil
			},

			assertErr: require.Error,
		},
		{
			name: "error - common properties invalid json",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{`), // invalid json
			},

			assertErr: require.Error,
		},
		{
			name: "error - VMware properties invalid json",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{`), // invalid json
			},

			assertErr: require.Error,
		},
		{
			name: "error - VMware invalid endpoint",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": ":|\\",
  "username": "user",
  "password": "pass"
}
`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - VMware empty username",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": "enpoint.url",
  "username": "",
  "password": "pass"
}
`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - VMware empty password",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_VMWARE,
				Properties: json.RawMessage(`{
  "endpoint": "enpoint.url",
  "username": "user",
  "password": ""
}
`),
			},

			assertErr: require.Error,
		},
		{
			name: "error - repo",
			source: migration.Source{
				ID:         1,
				Name:       "one",
				Insecure:   false,
				SourceType: api.SOURCETYPE_COMMON,
				Properties: json.RawMessage(`{}`),
			},
			repoUpdateErr: errors.New("boom!"),

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			repo := &mock.SourceRepoMock{
				UpdateByNameFunc: func(ctx context.Context, in migration.Source) (migration.Source, error) {
					return tc.repoUpdateSource, tc.repoUpdateErr
				},
			}

			sourceSvc := migration.NewSourceService(repo)

			// Run test
			source, err := sourceSvc.UpdateByName(context.Background(), tc.source)

			// Assert
			tc.assertErr(t, err)
			require.Equal(t, tc.repoUpdateSource, source)
		})
	}
}

func TestSourceService_DeleteByName(t *testing.T) {
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
			repo := &mock.SourceRepoMock{
				DeleteByNameFunc: func(ctx context.Context, name string) error {
					return tc.repoDeleteByNameErr
				},
			}

			sourceSvc := migration.NewSourceService(repo)

			// Run test
			err := sourceSvc.DeleteByName(context.Background(), tc.nameArg)

			// Assert
			tc.assertErr(t, err)
		})
	}
}
