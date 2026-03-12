package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/endpoint/mock"
	"github.com/FuturFusion/migration-manager/internal/properties"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestQueueAPI_cancel(t *testing.T) {
	cases := []struct {
		name string

		cleanup          bool
		status           api.MigrationStatusType
		importStage      migration.ImportStage
		placementRunning bool

		wantHTTPStatus int
		wantStatus     api.MigrationStatusType
		wantCleanup    bool
		wantPowerOn    bool
	}{
		{
			name:           "success - cancel early queue entry -- no cleanup, placement stopped",
			status:         api.MIGRATIONSTATUS_WAITING,
			wantStatus:     api.MIGRATIONSTATUS_CANCELED,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - cancel late queue entry -- no cleanup, placement stopped",
			status:         api.MIGRATIONSTATUS_WORKER_DONE,
			wantStatus:     api.MIGRATIONSTATUS_CANCELED,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:             "success - cancel early queue entry -- no cleanup, placement running",
			placementRunning: true,
			status:           api.MIGRATIONSTATUS_WAITING,
			wantStatus:       api.MIGRATIONSTATUS_CANCELED,
			wantHTTPStatus:   http.StatusOK,
		},
		{
			name:             "success - cancel late queue entry -- no cleanup, placement running",
			placementRunning: true,
			status:           api.MIGRATIONSTATUS_WORKER_DONE,
			wantStatus:       api.MIGRATIONSTATUS_CANCELED,
			wantPowerOn:      true,
			wantHTTPStatus:   http.StatusOK,
		},
		{
			name:           "success - cancel early queue entry -- cleanup, placement stopped",
			cleanup:        true,
			status:         api.MIGRATIONSTATUS_WAITING,
			wantStatus:     api.MIGRATIONSTATUS_CANCELED,
			wantCleanup:    true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:           "success - cancel late queue entry -- cleanup, placement stopped",
			cleanup:        true,
			status:         api.MIGRATIONSTATUS_WORKER_DONE,
			wantStatus:     api.MIGRATIONSTATUS_CANCELED,
			wantCleanup:    true,
			wantHTTPStatus: http.StatusOK,
		},
		{
			name:             "success - cancel early queue entry -- cleanup, placement running",
			cleanup:          true,
			placementRunning: true,
			status:           api.MIGRATIONSTATUS_WAITING,
			wantStatus:       api.MIGRATIONSTATUS_CANCELED,
			wantCleanup:      true,
			wantHTTPStatus:   http.StatusOK,
		},
		{
			name:             "success - cancel late queue entry -- cleanup, placement running",
			cleanup:          true,
			placementRunning: true,
			status:           api.MIGRATIONSTATUS_WORKER_DONE,
			wantStatus:       api.MIGRATIONSTATUS_CANCELED,
			wantCleanup:      true,
			wantPowerOn:      true,
			wantHTTPStatus:   http.StatusOK,
		},
		{
			name:             "success - cancel during background import -- cleanup, placement running",
			cleanup:          true,
			placementRunning: true,
			status:           api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
			wantStatus:       api.MIGRATIONSTATUS_CANCELED,
			wantCleanup:      true,
			wantHTTPStatus:   http.StatusOK,
		},
		{
			name:             "success - cancel during idle (pre-final sync) -- cleanup, placement running",
			cleanup:          true,
			placementRunning: true,
			status:           api.MIGRATIONSTATUS_IDLE,
			wantStatus:       api.MIGRATIONSTATUS_CANCELED,
			wantCleanup:      true,
			wantHTTPStatus:   http.StatusOK,
		},
		{
			name:             "success - cancel during idle (post-final sync) -- cleanup, placement running",
			cleanup:          true,
			placementRunning: true,
			status:           api.MIGRATIONSTATUS_IDLE,
			importStage:      migration.IMPORTSTAGE_COMPLETE,
			wantStatus:       api.MIGRATIONSTATUS_CANCELED,
			wantCleanup:      true,
			wantPowerOn:      true,
			wantHTTPStatus:   http.StatusOK,
		},
		{
			name:             "success - cancel during final import -- cleanup, placement running",
			cleanup:          true,
			placementRunning: true,
			status:           api.MIGRATIONSTATUS_FINAL_IMPORT,
			wantStatus:       api.MIGRATIONSTATUS_CANCELED,
			wantCleanup:      true,
			wantPowerOn:      true,
			wantHTTPStatus:   http.StatusOK,
		},
		{
			name:           "error - queue entry already finished, no cleanup",
			status:         api.MIGRATIONSTATUS_FINISHED,
			wantStatus:     api.MIGRATIONSTATUS_FINISHED,
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name:           "error - queue entry already finished",
			status:         api.MIGRATIONSTATUS_FINISHED,
			wantStatus:     api.MIGRATIONSTATUS_FINISHED,
			cleanup:        true,
			wantHTTPStatus: http.StatusInternalServerError,
		},
	}

	require.NoError(t, properties.InitDefinitions())

	defaultTargetEndpoint := func(api.Target) (migration.TargetEndpoint, error) {
		return &mock.TargetEndpointMock{
			ConnectFunc:                func(ctx context.Context) error { return nil },
			IsWaitingForOIDCTokensFunc: func() bool { return false },
			DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
				return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
			},
		}, nil
	}

	defaultSourceEndpointFunc := func(api.Source) (migration.SourceEndpoint, error) {
		return &mock.SourceEndpointMock{
			ConnectFunc: func(ctx context.Context) error { return nil },
			DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
				return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
			},
		}, nil
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)

			queueUUID := uuid.New()
			d := daemonSetup(t)
			client, srvURL := startTestDaemon(t, d, []APIEndpoint{queueCancelCmd}, nil)
			path := srvURL + "/1.0/queue/" + queueUUID.String() + "/:cancel"
			if tc.cleanup {
				path = path + "?cleanup=1"
			}

			batch := migration.Batch{
				Name: "b1",
				Defaults: api.BatchDefaults{
					Placement: api.BatchPlacement{Target: "default", TargetProject: "default", StoragePool: "default"},
				},
				Status:            api.BATCHSTATUS_DEFINED,
				IncludeExpression: "true",
				Config: api.BatchConfig{
					BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
					FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
				},
			}

			_, err := d.batch.Create(d.ShutdownCtx, batch)
			require.NoError(t, err)
			batch.Status = api.BATCHSTATUS_RUNNING
			_, err = d.batch.UpdateStatusByName(d.ShutdownCtx, batch.Name, batch.Status, batch.StatusMessage)
			require.NoError(t, err)

			src := migration.Source{Name: "src", SourceType: api.SOURCETYPE_VMWARE, Properties: json.RawMessage(`{"endpoint": "bar", "username":"u", "password":"p"}`), EndpointFunc: defaultSourceEndpointFunc}
			_, err = d.source.Create(d.ShutdownCtx, src)
			require.NoError(t, err)

			tgt := migration.Target{Name: "tgt", TargetType: api.TARGETTYPE_INCUS, Properties: json.RawMessage(`{"endpoint": "bar", "create_limit": 5, "connection_timeout": "30s"}`), EndpointFunc: defaultTargetEndpoint}
			_, err = d.target.Create(d.ShutdownCtx, tgt)
			require.NoError(t, err)

			inst := migration.Instance{
				UUID:                 queueUUID,
				Source:               src.Name,
				SourceType:           src.SourceType,
				LastUpdateFromSource: time.Now(),
				Overrides:            api.InstanceOverride{},
				Properties:           api.InstanceProperties{InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Name: "vm"}, Location: "vm", Running: true},
			}

			_, err = d.instance.Create(t.Context(), inst)
			require.NoError(t, err)

			q := migration.QueueEntry{
				InstanceUUID:    queueUUID,
				BatchName:       "b1",
				MigrationStatus: tc.status,
				SecretToken:     uuid.New(),
				ImportStage:     tc.importStage,
				Placement:       api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}, Running: tc.placementRunning},
			}

			_, err = d.queue.CreateEntry(t.Context(), q)
			require.NoError(t, err)

			var ranCleanup bool
			origTarget := target.NewTarget
			origSource := source.NewVMSource
			defer func() {
				target.NewTarget = origTarget
				source.NewVMSource = origSource
			}()

			target.NewTarget = func(tgt api.Target) (target.Target, error) {
				return &target.TargetMock{
					TimeoutFunc:    func() time.Duration { return time.Second },
					GetNameFunc:    func() string { return tgt.Name },
					ConnectFunc:    func(ctx context.Context) error { return nil },
					SetProjectFunc: func(project string) error { return nil },
					CleanupVMFunc: func(ctx context.Context, name string, requireWorkerVolume bool) error {
						require.True(t, tc.wantCleanup)
						ranCleanup = true
						return nil
					},
				}, nil
			}

			var ranPowerOn bool
			source.NewVMSource = func(s api.Source) (source.Source, error) {
				return &source.SourceMock{
					TimeoutFunc: func() time.Duration { return time.Second },
					GetNameFunc: func() string { return s.Name },
					ConnectFunc: func(ctx context.Context) error { return nil },
					PowerOnVMFunc: func(ctx context.Context, name string) error {
						require.True(t, tc.wantPowerOn)
						ranPowerOn = true
						return nil
					},
				}, nil
			}

			statusCode, _ := probeAPI(t, client, http.MethodPost, path, nil, nil)
			require.Equal(t, tc.wantHTTPStatus, statusCode)
			resultQueue, err := d.queue.GetByInstanceUUID(t.Context(), queueUUID)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resultQueue.MigrationStatus)
			require.Equal(t, tc.wantPowerOn, ranPowerOn)
			require.Equal(t, tc.wantCleanup, ranCleanup)
		})
	}
}
