package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"strconv"
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

func TestBatchAPI_reset(t *testing.T) {
	// instance names keyed by running state.
	type vms map[bool][]string

	// list of vm names of queue entries keyed by migration state.
	type queue map[api.MigrationStatusType][]string

	cases := []struct {
		name  string
		force bool

		status api.BatchStatusType

		vms            vms
		queue          queue
		wantHTTPStatus int

		wantStatus       api.BatchStatusType
		cleanupVMs       []string
		powerOnVMs       []string
		wantQueueEntries int
	}{
		{
			name:   "success - clear all queue entries with force flag",
			status: api.BATCHSTATUS_RUNNING,
			force:  true,
			vms: vms{true: func() (v []string) {
				for i := range 12 {
					v = append(v, "vm"+strconv.Itoa(i+1))
				}

				return v
			}()},

			queue: queue{
				api.MIGRATIONSTATUS_BLOCKED:           {"vm1"},
				api.MIGRATIONSTATUS_WAITING:           {"vm2"},
				api.MIGRATIONSTATUS_CREATING:          {"vm3"},
				api.MIGRATIONSTATUS_BACKGROUND_IMPORT: {"vm4"},
				api.MIGRATIONSTATUS_IDLE:              {"vm5"},
				api.MIGRATIONSTATUS_FINAL_IMPORT:      {"vm6"},
				api.MIGRATIONSTATUS_POST_IMPORT:       {"vm7"},
				api.MIGRATIONSTATUS_WORKER_DONE:       {"vm8"},
				api.MIGRATIONSTATUS_ERROR:             {"vm9"},
				api.MIGRATIONSTATUS_CANCELED:          {"vm10"},
				api.MIGRATIONSTATUS_CONFLICT:          {"vm11"},
				api.MIGRATIONSTATUS_FINISHED:          {"vm12"}, // Should not be cleaned up.
			},

			wantHTTPStatus: http.StatusOK,
			wantStatus:     api.BATCHSTATUS_DEFINED,
			cleanupVMs: func() (v []string) {
				for i := range 11 {
					v = append(v, "vm"+strconv.Itoa(i+1))
				}

				return v
			}(),

			powerOnVMs: func() (v []string) {
				for i := range 11 {
					v = append(v, "vm"+strconv.Itoa(i+1))
				}

				return v
			}(),

			wantQueueEntries: 0,
		},
		{
			name:   "success - clear all queue entries with force flag, but only power on half",
			status: api.BATCHSTATUS_RUNNING,
			force:  true,
			vms: vms{
				false: func() (v []string) {
					for i := range 6 {
						v = append(v, "vm"+strconv.Itoa(i+1))
					}

					return v
				}(),

				true: func() (v []string) {
					for i := range 6 {
						v = append(v, "vm"+strconv.Itoa(i+1+6))
					}

					return v
				}(),
			},

			queue: queue{
				api.MIGRATIONSTATUS_BLOCKED:           {"vm1"},
				api.MIGRATIONSTATUS_WAITING:           {"vm2"},
				api.MIGRATIONSTATUS_CREATING:          {"vm3"},
				api.MIGRATIONSTATUS_BACKGROUND_IMPORT: {"vm4"},
				api.MIGRATIONSTATUS_IDLE:              {"vm5"},
				api.MIGRATIONSTATUS_FINAL_IMPORT:      {"vm6"},
				api.MIGRATIONSTATUS_POST_IMPORT:       {"vm7"},
				api.MIGRATIONSTATUS_WORKER_DONE:       {"vm8"},
				api.MIGRATIONSTATUS_ERROR:             {"vm9"},
				api.MIGRATIONSTATUS_CANCELED:          {"vm10"},
				api.MIGRATIONSTATUS_CONFLICT:          {"vm11"},
				api.MIGRATIONSTATUS_FINISHED:          {"vm12"}, // Should not be cleaned up.
			},

			wantHTTPStatus: http.StatusOK,
			wantStatus:     api.BATCHSTATUS_DEFINED,
			cleanupVMs: func() (v []string) {
				for i := range 11 {
					v = append(v, "vm"+strconv.Itoa(i+1))
				}

				return v
			}(),

			powerOnVMs: func() (v []string) {
				for i := range 6 {
					v = append(v, "vm"+strconv.Itoa(i+1+6))
				}

				return v
			}(),

			wantQueueEntries: 0,
		},
		{
			name:   "success - clear queue entries without force flag",
			status: api.BATCHSTATUS_RUNNING,
			vms: vms{true: func() (v []string) {
				for i := range 8 {
					v = append(v, "vm"+strconv.Itoa(i+1))
				}

				return v
			}()},

			queue: queue{
				api.MIGRATIONSTATUS_BLOCKED:           {"vm1"},
				api.MIGRATIONSTATUS_WAITING:           {"vm2"},
				api.MIGRATIONSTATUS_CREATING:          {"vm3"},
				api.MIGRATIONSTATUS_BACKGROUND_IMPORT: {"vm4"},
				api.MIGRATIONSTATUS_IDLE:              {"vm5"},
				api.MIGRATIONSTATUS_ERROR:             {"vm6"},
				api.MIGRATIONSTATUS_CANCELED:          {"vm7"},
				api.MIGRATIONSTATUS_CONFLICT:          {"vm8"},
			},

			wantHTTPStatus: http.StatusOK,
			wantStatus:     api.BATCHSTATUS_DEFINED,
			cleanupVMs: func() (v []string) {
				for i := range 8 {
					v = append(v, "vm"+strconv.Itoa(i+1))
				}

				return v
			}(),

			powerOnVMs: func() (v []string) {
				for i := range 8 {
					v = append(v, "vm"+strconv.Itoa(i+1))
				}

				return v
			}(),

			wantQueueEntries: 0,
		},
		{
			name:   "error - late-stage queue entries without force flag",
			status: api.BATCHSTATUS_RUNNING,
			vms: vms{true: func() (v []string) {
				for i := range 9 {
					v = append(v, "vm"+strconv.Itoa(i+1))
				}

				return v
			}()},

			queue: queue{
				api.MIGRATIONSTATUS_BLOCKED:           {"vm1"},
				api.MIGRATIONSTATUS_WAITING:           {"vm2"},
				api.MIGRATIONSTATUS_CREATING:          {"vm3"},
				api.MIGRATIONSTATUS_BACKGROUND_IMPORT: {"vm4"},
				api.MIGRATIONSTATUS_IDLE:              {"vm5"},
				api.MIGRATIONSTATUS_ERROR:             {"vm6"},
				api.MIGRATIONSTATUS_CANCELED:          {"vm7"},
				api.MIGRATIONSTATUS_CONFLICT:          {"vm8"},
				api.MIGRATIONSTATUS_FINAL_IMPORT:      {"vm9"},
			},

			wantHTTPStatus:   http.StatusBadRequest,
			wantStatus:       api.BATCHSTATUS_RUNNING,
			cleanupVMs:       []string{},
			powerOnVMs:       []string{},
			wantQueueEntries: 9,
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

			d := daemonSetup(t)
			client, srvURL := startTestDaemon(t, d, []APIEndpoint{batchResetCmd}, nil)
			path := srvURL + "/1.0/batches/b1/:reset"
			if tc.force {
				path = path + "?force=1"
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

			uuidsByName := map[string]uuid.UUID{}
			runningByName := map[string]bool{}
			for running, vms := range tc.vms {
				for _, vm := range vms {
					require.Equal(t, uuid.Nil, uuidsByName[vm])
					inst := migration.Instance{
						UUID:                 uuid.New(),
						Source:               src.Name,
						SourceType:           src.SourceType,
						LastUpdateFromSource: time.Now(),
						Overrides:            api.InstanceOverride{},
						Properties:           api.InstanceProperties{InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Name: vm}, Location: vm, Running: running},
					}

					_, err = d.instance.Create(t.Context(), inst)
					require.NoError(t, err)
					uuidsByName[vm] = inst.UUID
					runningByName[vm] = running
				}
			}

			statesByName := map[string]api.MigrationStatusType{}
			for state, vms := range tc.queue {
				for _, vm := range vms {
					q := migration.QueueEntry{
						InstanceUUID:    uuidsByName[vm],
						BatchName:       "b1",
						MigrationStatus: state,
						SecretToken:     uuid.New(),
						Placement:       api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}, Running: runningByName[vm]},
					}

					_, err = d.queue.CreateEntry(t.Context(), q)
					require.NoError(t, err)
					statesByName[vm] = q.MigrationStatus
				}
			}

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
						status, ok := statesByName[name]
						require.True(t, ok)
						require.NotEqual(t, api.MIGRATIONSTATUS_FINISHED, status)
						require.Contains(t, tc.cleanupVMs, name)
						return nil
					},
				}, nil
			}

			source.NewVMSource = func(s api.Source) (source.Source, error) {
				return &source.SourceMock{
					TimeoutFunc: func() time.Duration { return time.Second },
					GetNameFunc: func() string { return s.Name },
					ConnectFunc: func(ctx context.Context) error { return nil },
					PowerOnVMFunc: func(ctx context.Context, name string) error {
						status, ok := statesByName[name]
						require.True(t, ok)
						require.NotEqual(t, api.MIGRATIONSTATUS_FINISHED, status)
						require.Contains(t, tc.powerOnVMs, name)
						return nil
					},
				}, nil
			}

			statusCode, _ := probeAPI(t, client, http.MethodPost, path, nil, nil)
			require.Equal(t, tc.wantHTTPStatus, statusCode)
			resultBatch, err := d.batch.GetByName(t.Context(), "b1")
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resultBatch.Status)

			resultQueue, err := d.queue.GetAllByBatch(t.Context(), "b1")
			require.NoError(t, err)
			require.Len(t, resultQueue, tc.wantQueueEntries)
		})
	}
}
