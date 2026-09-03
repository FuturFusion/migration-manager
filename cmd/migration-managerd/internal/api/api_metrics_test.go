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
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestMetricsAPI_get(t *testing.T) {
	d := daemonSetup(t)
	client, srvURL := startTestDaemon(t, d, []APIEndpoint{metricsCmd}, nil)

	u := uuidCache{}

	_, err := d.source.Create(t.Context(), migration.Source{
		Name:       "src",
		SourceType: api.SOURCETYPE_VMWARE,
		Properties: json.RawMessage(`{"endpoint": "bar", "username":"u", "password":"p"}`),
		EndpointFunc: func(api.Source) (migration.SourceEndpoint, error) {
			return &mock.SourceEndpointMock{
				ConnectFunc: func(ctx context.Context) error { return nil },
				DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
					return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
				},
			}, nil
		},
	})
	require.NoError(t, err)

	// Instance with a single 1 GiB disk, fully migrated.
	instA := u.newTestInstance("vm-a", map[int]bool{0: true}, map[int]string{}, api.OSTYPE_LINUX, false)
	_, err = d.instance.Create(t.Context(), instA)
	require.NoError(t, err)

	// Manually disabled instance, blocked from migrating.
	instB := u.newTestInstance("vm-b", map[int]bool{0: true}, map[int]string{}, api.OSTYPE_LINUX, false)
	instB.Overrides.DisableMigration = true
	_, err = d.instance.Create(t.Context(), instB)
	require.NoError(t, err)

	_, err = d.batch.Create(t.Context(), migration.Batch{
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
	})
	require.NoError(t, err)

	_, err = d.queue.CreateEntry(t.Context(), migration.QueueEntry{
		InstanceUUID:    instA.UUID,
		BatchName:       "b1",
		MigrationStatus: api.MIGRATIONSTATUS_FINISHED,
		ImportStage:     migration.IMPORTSTAGE_COMPLETE,
		SecretToken:     uuid.New(),
		Placement:       api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}},
	})
	require.NoError(t, err)

	_, err = d.queue.CreateEntry(t.Context(), migration.QueueEntry{
		InstanceUUID:           instB.UUID,
		BatchName:              "b1",
		MigrationStatus:        api.MIGRATIONSTATUS_BLOCKED,
		MigrationStatusMessage: "Instance is disabled",
		ImportStage:            migration.IMPORTSTAGE_BACKGROUND,
		SecretToken:            uuid.New(),
		Placement:              api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}},
	})
	require.NoError(t, err)

	statusCode, body := probeAPI(t, client, http.MethodGet, srvURL+"/1.0/metrics", nil, nil)
	require.Equal(t, http.StatusOK, statusCode, "body: %s", body)

	expected := []string{
		`migration_manager_batches{status="Defined"} 1`,
		`migration_manager_batch_instances{batch="b1",status="Defined"} 2`,
		`migration_manager_batch_migrated_instances{batch="b1",status="Defined"} 1`,
		`migration_manager_batch_blocked_instances{batch="b1",reason="Manually disabled",status="Defined"} 1`,
		`migration_manager_batch_disk_bytes{batch="b1",status="Defined"} 2.147483648e+09`,
		`migration_manager_batch_migrated_disk_bytes{batch="b1",status="Defined"} 1.073741824e+09`,
		`migration_manager_instances 2`,
		`migration_manager_queue_entries{batch="b1",status="Blocked"} 1`,
		`migration_manager_queue_entries{batch="b1",status="Finished"} 1`,
		`migration_manager_sources{type="vmware"} 1`,
		`migration_manager_source_instances{source="src",type="vmware"} 2`,
		`migration_manager_source_imported_instances{source="src"} 2`,
		`migration_manager_source_migrated_instances{source="src"} 1`,
		`migration_manager_source_ineligible_instances{reason="Manually disabled",source="src"} 1`,
		`migration_manager_source_import_issues{issue="blocked",source="src"} 0`,
		"# EOF",
	}

	for _, line := range expected {
		require.Contains(t, body, line+"\n")
	}
}
