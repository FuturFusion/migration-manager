package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestArtifactPut(t *testing.T) {
	tests := []struct {
		name string

		artifactUUID *string
		artifact     api.ArtifactPost
		artifactPut  api.ArtifactPut

		wantHTTPStatus int
	}{
		{
			name: "success",
			artifact: api.ArtifactPost{
				ArtifactPut: api.ArtifactPut{
					Description:   "desc",
					OS:            api.OSTYPE_WINDOWS,
					Architectures: []string{"x86_64"},
					Versions:      []string{"Server 2022"},
				},
				Type: api.ARTIFACTTYPE_DRIVER,
			},
			artifactPut: api.ArtifactPut{
				Description:   "changed",
				OS:            api.OSTYPE_WINDOWS,
				Architectures: []string{"x86_64", "aarch64"},
				Versions:      []string{"Server 2025"},
			},
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:         "error - empty uuid",
			artifactUUID: new(string),
			artifact: api.ArtifactPost{
				ArtifactPut: api.ArtifactPut{
					Description:   "desc",
					OS:            api.OSTYPE_WINDOWS,
					Architectures: []string{"x86_64"},
					Versions:      []string{"Server 2022"},
				},
				Type: api.ARTIFACTTYPE_DRIVER,
			},
			artifactPut: api.ArtifactPut{
				Description:   "changed",
				OS:            api.OSTYPE_WINDOWS,
				Architectures: []string{"x86_64", "aarch64"},
				Versions:      []string{"Server 2025"},
			},
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name:         "error - invalid uuid",
			artifactUUID: func() *string { s := "not a uuid"; return &s }(),
			artifact: api.ArtifactPost{
				ArtifactPut: api.ArtifactPut{
					Description:   "desc",
					OS:            api.OSTYPE_WINDOWS,
					Architectures: []string{"x86_64"},
					Versions:      []string{"Server 2022"},
				},
				Type: api.ARTIFACTTYPE_DRIVER,
			},
			artifactPut: api.ArtifactPut{
				Description:   "changed",
				OS:            api.OSTYPE_WINDOWS,
				Architectures: []string{"x86_64", "aarch64"},
				Versions:      []string{"Server 2025"},
			},
			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name:         "error - not found",
			artifactUUID: func() *string { id := uuid.New().String(); return &id }(),
			artifact: api.ArtifactPost{
				ArtifactPut: api.ArtifactPut{
					Description:   "desc",
					OS:            api.OSTYPE_WINDOWS,
					Architectures: []string{"x86_64"},
					Versions:      []string{"Server 2022"},
				},
				Type: api.ARTIFACTTYPE_DRIVER,
			},
			artifactPut: api.ArtifactPut{
				Description:   "changed",
				OS:            api.OSTYPE_WINDOWS,
				Architectures: []string{"x86_64", "aarch64"},
				Versions:      []string{"Server 2025"},
			},
			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name: "error - validation failed",
			artifact: api.ArtifactPost{
				ArtifactPut: api.ArtifactPut{
					Description:   "desc",
					OS:            api.OSTYPE_WINDOWS,
					Architectures: []string{"x86_64"},
					Versions:      []string{"Server 2022"},
				},
				Type: api.ARTIFACTTYPE_DRIVER,
			},
			artifactPut: api.ArtifactPut{
				Description:   "changed",
				OS:            api.OSTYPE_LINUX,
				Architectures: []string{"x86_64", "aarch64"},
				Versions:      []string{"Server 2025"},
			},
			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			// Setup
			daemon := daemonSetup(t)
			client, srvURL := startTestDaemon(t, daemon, []APIEndpoint{artifactCmd}, nil)

			a, err := daemon.artifact.Create(t.Context(), migration.Artifact{
				UUID:       uuid.New(),
				Type:       tc.artifact.Type,
				Properties: tc.artifact.ArtifactPut,
			})
			require.NoError(t, err)

			putUUID := a.UUID.String()
			if tc.artifactUUID != nil {
				putUUID = *tc.artifactUUID
			}

			b, err := json.Marshal(tc.artifactPut)
			require.NoError(t, err)

			// Execute test
			statusCode, _ := probeAPI(t, client, http.MethodPut, srvURL+fmt.Sprintf("/1.0/artifacts/%s", putUUID), bytes.NewBuffer(b), nil)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
		})
	}
}
