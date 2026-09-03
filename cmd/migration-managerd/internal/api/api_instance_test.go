package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/endpoint/mock"
	"github.com/FuturFusion/migration-manager/internal/properties"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestInstanceDumpPost(t *testing.T) {
	require.NoError(t, properties.InitDefinitions())

	origSource := source.NewVMSource
	defer func() {
		source.NewVMSource = origSource
	}()

	uuid1 := uuid.New()
	uuid2 := uuid.New()

	cases := []struct {
		name           string
		uuid           string
		sourceType     api.SourceType
		setupMock      func() source.Source
		wantHTTPStatus int
		wantDumpVMName string
	}{
		{
			name:           "instance not found",
			uuid:           uuid.New().String(),
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "non-vmware source",
			uuid:           uuid2.String(),
			sourceType:     api.SOURCETYPE_NSX_T,
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "source connectivity not ok",
			uuid:           uuid1.String(),
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "vm not found on source",
			uuid: uuid1.String(),
			setupMock: func() source.Source {
				return &source.SourceMock{
					TimeoutFunc: func() time.Duration { return time.Second },
					ConnectFunc: func(ctx context.Context) error { return nil },
					DumpVMFunc: func(ctx context.Context, id uuid.UUID) (vm source.RawVMwareVM, err error) {
						return vm, fmt.Errorf("not found")
					},
				}
			},
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "success",
			uuid: uuid1.String(),
			setupMock: func() source.Source {
				return &source.SourceMock{
					TimeoutFunc: func() time.Duration { return time.Second },
					ConnectFunc: func(ctx context.Context) error { return nil },
					DumpVMFunc: func(ctx context.Context, id uuid.UUID) (vm source.RawVMwareVM, err error) {
						vm.Name = "sample-vm"
						return vm, nil
					},
				}
			},
			wantHTTPStatus: http.StatusOK,
			wantDumpVMName: "sample-vm",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source.NewVMSource = func(s api.Source) (source.Source, error) {
				if tc.setupMock == nil {
					return nil, fmt.Errorf("no mock configured")
				}

				return tc.setupMock(), nil
			}

			d := daemonSetup(t)

			sourceType := tc.sourceType
			if sourceType == "" {
				sourceType = api.SOURCETYPE_VMWARE
			}

			defaultSourceEndpointFunc := func(api.Source) (migration.SourceEndpoint, error) {
				return &mock.SourceEndpointMock{
					ConnectFunc: func(ctx context.Context) error { return nil },
					DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
						return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
					},
				}, nil
			}

			// source
			src := migration.Source{
				Name:         "src",
				SourceType:   sourceType,
				Properties:   json.RawMessage([]byte(`{"endpoint":"bar","username":"u","password":"p"}`)),
				EndpointFunc: defaultSourceEndpointFunc,
			}

			_, err := d.source.Create(t.Context(), src)
			require.NoError(t, err)

			parsedUUID, err := uuid.Parse(tc.uuid)
			require.NoError(t, err)

			// instance
			inst := migration.Instance{
				UUID: parsedUUID,
				Properties: api.InstanceProperties{
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
						Name: "vm1",
					},
					UUID:     parsedUUID,
					Location: "/path/to/vm1",
				},
				Source:     src.Name,
				SourceType: sourceType,
			}

			_, err = d.instance.Create(t.Context(), inst)
			require.NoError(t, err)

			// api call
			client, baseURL := startTestDaemon(t, d, []APIEndpoint{instanceDumpCmd}, nil)
			path := fmt.Sprintf("%s/1.0/instances/%s/:dump", baseURL, tc.uuid)
			statusCode, body := probeAPI(t, client, http.MethodPost, path, nil, nil)

			require.Equal(t, tc.wantHTTPStatus, statusCode, "body: %s", body)

			// name is under metadata
			var resp struct {
				Metadata source.RawVMwareVM `json:"metadata"`
			}

			require.NoError(t, json.Unmarshal([]byte(body), &resp))
			require.Equal(t, tc.wantDumpVMName, resp.Metadata.Name, body)
		})
	}
}
