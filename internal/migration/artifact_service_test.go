package migration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestArtifact_Create(t *testing.T) {
	cases := []struct {
		name      string
		artifact  migration.Artifact
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success - vmware sdk",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeSDK,
				Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE},
			},
			assertErr: require.NoError,
		},
		{
			name: "success - windows",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeOSImage,
				Properties: api.ArtifactProperties{OS: api.OSTYPE_WINDOWS, Architectures: []string{"x86_64"}},
			},
			assertErr: require.NoError,
		},
		{
			name: "success - fortigate",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeOSImage,
				Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Architectures: []string{"x86_64"}, Versions: []string{"7.4"}},
			},
			assertErr: require.NoError,
		},
		{
			name: "error - vmware sdk with version",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeSDK,
				Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE, Versions: []string{"1.0"}},
			},
			assertErr: require.Error,
		},
		{
			name: "error - vmware sdk with architecture",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeSDK,
				Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE, Architectures: []string{"x86_64"}},
			},
			assertErr: require.Error,
		},
		{
			name: "error - sdk with without source",
			artifact: migration.Artifact{
				UUID: uuid.New(),
				Type: api.ArtifactTypeSDK,
			},
			assertErr: require.Error,
		},
		{
			name: "error - os image without os",
			artifact: migration.Artifact{
				UUID: uuid.New(),
				Type: api.ArtifactTypeOSImage,
			},
			assertErr: require.Error,
		},
		{
			name: "error - fortigate image without version",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeOSImage,
				Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Architectures: []string{"x86_64"}},
			},
			assertErr: require.Error,
		},
		{
			name: "error - fortigate image with invalid version",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeOSImage,
				Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Architectures: []string{"x86_64"}, Versions: []string{"version1"}},
			},
			assertErr: require.Error,
		},
		{
			name: "error - fortigate image without architecture",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeOSImage,
				Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Versions: []string{"1.0"}},
			},
			assertErr: require.Error,
		},
		{
			name: "error - fortigate image with invalid architecture",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeOSImage,
				Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Versions: []string{"1.0"}, Architectures: []string{"fake-arch"}},
			},
			assertErr: require.Error,
		},
		{
			name: "error - virtio-win image with invalid architecture",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeOSImage,
				Properties: api.ArtifactProperties{OS: api.OSTYPE_WINDOWS, Architectures: []string{"fake-arch"}},
			},
			assertErr: require.Error,
		},
		{
			name: "error - virtio-win image with no architecture",
			artifact: migration.Artifact{
				UUID:       uuid.New(),
				Type:       api.ArtifactTypeOSImage,
				Properties: api.ArtifactProperties{OS: api.OSTYPE_WINDOWS},
			},
			assertErr: require.Error,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			repo := &mock.ArtifactRepoMock{
				CreateFunc: func(ctx context.Context, artifact migration.Artifact) (int64, error) {
					return -1, nil
				},
			}

			artifactSvc := migration.NewArtifactService(repo, &sys.OS{})
			_, err := artifactSvc.Create(context.Background(), tc.artifact)
			tc.assertErr(t, err)
		})
	}
}

func TestArtifact_HasRequiredArtifactsForInstance(t *testing.T) {
	cases := []struct {
		name      string
		artifacts []api.Artifact
		instance  migration.Instance
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "success - linux from unknown source (no artifact)",
			assertErr: require.NoError,
		},
		{
			name:      "success - linux from vmware (sdk)",
			assertErr: require.NoError,
			artifacts: []api.Artifact{{
				ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeSDK, Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE}},
				HasContent:   true,
			}},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE},
		},
		{
			name:      "success - window from vmware (sdk,virtio-win)",
			assertErr: require.NoError,
			artifacts: []api.Artifact{
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeSDK, Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE}},
					HasContent:   true,
				},
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeOSImage, Properties: api.ArtifactProperties{OS: api.OSTYPE_WINDOWS, Architectures: []string{"x86_64"}}},
					HasContent:   true,
				},
			},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{OS: "Windows", Architecture: "x86_64"}},
		},
		{
			name:      "success - fortigate from vmware (sdk,kvm-img)",
			assertErr: require.NoError,
			artifacts: []api.Artifact{
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeSDK, Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE}},
					HasContent:   true,
				},
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeOSImage, Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Architectures: []string{"x86_64"}}},
					HasContent:   true,
				},
			},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{
				InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Description: "FortiGate"},
				Architecture:                   "x86_64",
			}},
		},
		{
			name:      "error - fortigate from vmware (missing sdk)",
			assertErr: require.Error,
			artifacts: []api.Artifact{
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeOSImage, Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Architectures: []string{"x86_64"}}},
					HasContent:   true,
				},
			},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{
				InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Description: "FortiGate"},
				Architecture:                   "x86_64",
			}},
		},
		{
			name:      "error - fortigate from vmware (kvm-img architecture doesnt match)",
			assertErr: require.Error,
			artifacts: []api.Artifact{
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeSDK, Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE}},
					HasContent:   true,
				},
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeOSImage, Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Architectures: []string{"x86_64"}}},
					HasContent:   true,
				},
			},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{
				InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Description: "FortiGate"},
				Architecture:                   "aarch64",
			}},
		},
		{
			name:      "error - fortigate from vmware (empty matching artifacts)",
			assertErr: require.Error,
			artifacts: []api.Artifact{
				{ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeSDK, Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE}}},
				{ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeOSImage, Properties: api.ArtifactProperties{OS: api.OSTYPE_FORTIGATE, Architectures: []string{"x86_64"}}}},
			},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{
				InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Description: "FortiGate"},
				Architecture:                   "x86_64",
			}},
		},
		{
			name:      "error - windows from vmware (missing sdk)",
			assertErr: require.Error,
			artifacts: []api.Artifact{
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeOSImage, Properties: api.ArtifactProperties{OS: api.OSTYPE_WINDOWS, Architectures: []string{"x86_64"}}},
					HasContent:   true,
				},
			},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{OS: "Windows", Architecture: "x86_64"}},
		},
		{
			name:      "error - windows from vmware (virtio-win architecture doesnt match)",
			assertErr: require.Error,
			artifacts: []api.Artifact{
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeSDK, Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE}},
					HasContent:   true,
				},
				{
					ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeOSImage, Properties: api.ArtifactProperties{OS: api.OSTYPE_WINDOWS, Architectures: []string{"x86_64"}}},
					HasContent:   true,
				},
			},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{OS: "Windows", Architecture: "aarch64"}},
		},
		{
			name:      "error - windows from vmware (no artifacts)",
			assertErr: require.Error,
			artifacts: []api.Artifact{},
			instance:  migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{OS: "Windows", Architecture: "x86_64"}},
		},
		{
			name:      "error - windows from vmware (empty matching artifacts)",
			assertErr: require.Error,
			artifacts: []api.Artifact{
				{ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeSDK, Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE}}},
				{ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeOSImage, Properties: api.ArtifactProperties{OS: api.OSTYPE_WINDOWS, Architectures: []string{"x86_64"}}}},
			},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{OS: "Windows", Architecture: "x86_64"}},
		},
		{
			name:      "error - linux from vmware (no sdk)",
			assertErr: require.Error,
			artifacts: []api.Artifact{{}},
			instance:  migration.Instance{SourceType: api.SOURCETYPE_VMWARE},
		},
		{
			name:      "error - linux from vmware (empty matching artifacts)",
			assertErr: require.Error,
			artifacts: []api.Artifact{{
				ArtifactPost: api.ArtifactPost{Type: api.ArtifactTypeSDK, Properties: api.ArtifactProperties{SourceType: api.SOURCETYPE_VMWARE}},
			}},
			instance: migration.Instance{SourceType: api.SOURCETYPE_VMWARE},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			repo := &mock.ArtifactRepoMock{}

			dir := t.TempDir()
			artifactSvc := migration.NewArtifactService(repo, &sys.OS{ArtifactDir: dir})
			artifacts := migration.Artifacts{}
			for _, a := range tc.artifacts {
				art := migration.Artifact{
					UUID:       uuid.New(),
					Type:       a.Type,
					Properties: a.Properties,
				}

				artifacts = append(artifacts, art)
				if a.HasContent {
					require.NoError(t, os.WriteFile(filepath.Join(dir, art.UUID.String()+".tar.gz"), nil, 0o644))
				}
			}

			tc.assertErr(t, artifactSvc.HasRequiredArtifactsForInstance(artifacts, tc.instance))
		})
	}
}
