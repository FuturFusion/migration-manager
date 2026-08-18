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

func TestStatsAPI_get(t *testing.T) {
	d := daemonSetup(t)
	client, srvURL := startTestDaemon(t, d, []APIEndpoint{statsCmd}, nil)

	u := uuidCache{}

	defaultSourceEndpointFunc := func(api.Source) (migration.SourceEndpoint, error) {
		return &mock.SourceEndpointMock{
			ConnectFunc: func(ctx context.Context) error { return nil },
			DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
				return api.EXTERNALCONNECTIVITYSTATUS_OK, nil
			},
		}, nil
	}

	for _, name := range []string{"src1", "src2"} {
		_, err := d.source.Create(t.Context(), migration.Source{
			Name:         name,
			SourceType:   api.SOURCETYPE_VMWARE,
			Properties:   json.RawMessage(`{"endpoint": "bar", "username":"u", "password":"p"}`),
			EndpointFunc: defaultSourceEndpointFunc,
		})
		require.NoError(t, err)
	}

	// src1 reports two warnings at once, so its unsynced instances appear under both.
	syncWarnings := migration.Warnings{
		migration.NewSyncWarning(api.SourceUnavailable, "src1", `status: "Cannot connect"`),
		migration.NewSyncWarning(api.InstanceIncomplete, "src1", "vm-f has incomplete properties"),
		migration.NewSyncWarning(api.InstanceImportFailed, "src2", "Failed to fetch instances"),
	}

	for _, w := range syncWarnings {
		_, err := d.warning.Emit(t.Context(), w)
		require.NoError(t, err)
	}

	// Instance with a single 1 GiB disk, imported but not yet migrated.
	instA := u.newTestInstance("vm-a", map[int]bool{0: true}, map[int]string{}, api.OSTYPE_LINUX, false)
	instA.Source = "src1"
	_, err := d.instance.Create(t.Context(), instA)
	require.NoError(t, err)

	// Instance with two 1 GiB disks and no IPv4, fully migrated, so it no longer counts as ineligible.
	instB := u.newTestInstance("vm-b", map[int]bool{0: true, 1: true}, map[int]string{0: ""}, api.OSTYPE_LINUX, false)
	instB.Source = "src1"
	_, err = d.instance.Create(t.Context(), instB)
	require.NoError(t, err)

	// Manually disabled instance, blocked from migrating.
	instD := u.newTestInstance("vm-d", map[int]bool{}, map[int]string{}, api.OSTYPE_LINUX, false)
	instD.Source = "src1"
	instD.Overrides.DisableMigration = true
	_, err = d.instance.Create(t.Context(), instD)
	require.NoError(t, err)

	// Instance with no known IP address, blocked from migrating.
	instE := u.newTestInstance("vm-e", map[int]bool{}, map[int]string{0: ""}, api.OSTYPE_LINUX, false)
	instE.Source = "src2"
	_, err = d.instance.Create(t.Context(), instE)
	require.NoError(t, err)

	// Batch created after vm-a, vm-b, vm-d and vm-e already exist, so they all get auto-assigned.
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

	_, err = d.batch.Create(t.Context(), batch)
	require.NoError(t, err)

	// Instance with five 1 GiB disks, from a different source, created after the batch so it stays unassigned.
	instC := u.newTestInstance("vm-c", map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true}, map[int]string{}, api.OSTYPE_LINUX, false)
	instC.Source = "src2"
	_, err = d.instance.Create(t.Context(), instC)
	require.NoError(t, err)

	// vm-a: background import has finished, but the final sync hasn't, so it should count as imported but not migrated.
	_, err = d.queue.CreateEntry(t.Context(), migration.QueueEntry{
		InstanceUUID:    instA.UUID,
		BatchName:       "b1",
		MigrationStatus: api.MIGRATIONSTATUS_IDLE,
		ImportStage:     migration.IMPORTSTAGE_FINAL,
		SecretToken:     uuid.New(),
		Placement:       api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}},
	})
	require.NoError(t, err)

	// vm-b: fully migrated.
	_, err = d.queue.CreateEntry(t.Context(), migration.QueueEntry{
		InstanceUUID:    instB.UUID,
		BatchName:       "b1",
		MigrationStatus: api.MIGRATIONSTATUS_FINISHED,
		ImportStage:     migration.IMPORTSTAGE_COMPLETE,
		SecretToken:     uuid.New(),
		Placement:       api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}},
	})
	require.NoError(t, err)

	// vm-c is left without a queue entry, and unassigned to any batch.

	// vm-d is blocked by its instance restrictions.
	_, err = d.queue.CreateEntry(t.Context(), migration.QueueEntry{
		InstanceUUID:           instD.UUID,
		BatchName:              "b1",
		MigrationStatus:        api.MIGRATIONSTATUS_BLOCKED,
		MigrationStatusMessage: "Instance is disabled",
		ImportStage:            migration.IMPORTSTAGE_BACKGROUND,
		SecretToken:            uuid.New(),
		Placement:              api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}},
	})
	require.NoError(t, err)

	// vm-e also fails its restrictions, but was blocked on placement, so the recorded reason wins.
	_, err = d.queue.CreateEntry(t.Context(), migration.QueueEntry{
		InstanceUUID:           instE.UUID,
		BatchName:              "b1",
		MigrationStatus:        api.MIGRATIONSTATUS_BLOCKED,
		MigrationStatusMessage: `Cannot place instance: Target network "incusbr0" has no free addresses`,
		ImportStage:            migration.IMPORTSTAGE_BACKGROUND,
		SecretToken:            uuid.New(),
		Placement:              api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}},
	})
	require.NoError(t, err)

	_, err = d.batch.UpdateStatusByName(t.Context(), "b1", api.BATCHSTATUS_RUNNING, "")
	require.NoError(t, err)

	// vm-f and vm-g both lack an IPv4 address, and are assigned to a batch that waives that restriction.
	instF := u.newTestInstance("vm-f", map[int]bool{0: true}, map[int]string{0: ""}, api.OSTYPE_LINUX, false)
	instF.Source = "src1"
	_, err = d.instance.Create(t.Context(), instF)
	require.NoError(t, err)

	instG := u.newTestInstance("vm-g", map[int]bool{}, map[int]string{0: ""}, api.OSTYPE_LINUX, false)
	instG.Source = "src2"
	_, err = d.instance.Create(t.Context(), instG)
	require.NoError(t, err)

	batch2 := migration.Batch{
		Name: "b2",
		Defaults: api.BatchDefaults{
			Placement: api.BatchPlacement{Target: "default", TargetProject: "default", StoragePool: "default"},
		},
		Status:            api.BATCHSTATUS_DEFINED,
		IncludeExpression: `location matches "vm-f" or location matches "vm-g"`,
		Config: api.BatchConfig{
			BackgroundSyncInterval:   api.AsDuration(10 * time.Minute),
			FinalBackgroundSyncLimit: api.AsDuration(10 * time.Minute),
			RestrictionOverrides:     api.InstanceRestrictionOverride{AllowNoIPv4: true},
		},
	}

	_, err = d.batch.Create(t.Context(), batch2)
	require.NoError(t, err)

	_, err = d.batch.UpdateStatusByName(t.Context(), "b2", api.BATCHSTATUS_RUNNING, "")
	require.NoError(t, err)

	// vm-f is blocked on placement rather than a restriction. Its batch allows migrating without an
	// IPv4 address, so the placement failure must be reported instead of the missing address.
	_, err = d.queue.CreateEntry(t.Context(), migration.QueueEntry{
		InstanceUUID:           instF.UUID,
		BatchName:              "b2",
		MigrationStatus:        api.MIGRATIONSTATUS_BLOCKED,
		MigrationStatusMessage: `Cannot place instance: Target network "incusbr0" does not contain IP "10.103.1.120" in subnet "10.6.139.1/24"`,
		ImportStage:            migration.IMPORTSTAGE_BACKGROUND,
		SecretToken:            uuid.New(),
		Placement:              api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}},
	})
	require.NoError(t, err)

	// vm-g is progressing under the waived restriction, so it counts as overridden rather than blocked.
	_, err = d.queue.CreateEntry(t.Context(), migration.QueueEntry{
		InstanceUUID:           instG.UUID,
		BatchName:              "b2",
		MigrationStatus:        api.MIGRATIONSTATUS_IDLE,
		MigrationStatusMessage: "Waiting for migration window",
		ImportStage:            migration.IMPORTSTAGE_BACKGROUND,
		SecretToken:            uuid.New(),
		Placement:              api.Placement{TargetName: "tgt", TargetProject: "default", StoragePools: map[string]string{"root": "default"}, Networks: map[string]api.NetworkPlacement{}},
	})
	require.NoError(t, err)

	// vm-f is also assigned to b3, so it must only be counted once towards the batch totals.
	batch3 := batch2
	batch3.Name = "b3"
	_, err = d.batch.Create(t.Context(), batch3)
	require.NoError(t, err)

	statusCode, body := probeAPI(t, client, http.MethodGet, srvURL+"/1.0/stats", nil, nil)
	require.Equal(t, http.StatusOK, statusCode, "body: %s", body)

	var resp struct {
		Metadata api.StatsOverview `json:"metadata"`
	}

	require.NoError(t, json.Unmarshal([]byte(body), &resp))
	stats := resp.Metadata

	const gib = 1024 * 1024 * 1024

	ref := func(inst migration.Instance) api.InstanceRef {
		return api.InstanceRef{UUID: inst.UUID, Location: inst.Properties.Location}
	}

	require.Equal(t, 7, stats.Sources.TotalInstances)

	// vm-f is on src1, which reports both warnings, so it is listed under each.
	require.Equal(t, []api.InstanceRef{ref(instF)}, stats.Sources.BlockedImports)
	require.Equal(t, []api.InstanceRef{ref(instF)}, stats.Sources.PartialImports)
	require.Equal(t, []api.InstanceRef{ref(instE), ref(instG)}, stats.Sources.FailedImports)

	// vm-e, vm-f and vm-g are each unsynced, but vm-f only drops out of the imported count once.
	require.Equal(t, 4, stats.Sources.ImportedInstances)
	require.Equal(t, 1, stats.Sources.MigratedInstances)

	// vm-b also fails the IPv4 restriction, but has migrated, so it drops out of the ineligible count.
	require.Equal(t, 4, stats.Sources.IneligibleInstances)
	require.Equal(t, []api.ReasonInstances{
		{Reason: "Unknown IP address", Instances: []api.InstanceRef{ref(instE), ref(instF), ref(instG)}},
		{Reason: "Manually disabled", Instances: []api.InstanceRef{ref(instD)}},
	}, stats.Sources.IneligibleReasons)

	// vm-f and vm-g are ineligible, but their batch waives the restriction.
	require.Equal(t, []api.OverriddenInstance{
		{InstanceRef: ref(instF), Batch: "b2"},
		{InstanceRef: ref(instG), Batch: "b2"},
	}, stats.Sources.OverriddenInstances)

	require.Len(t, stats.Sources.Items, 2)
	require.Equal(t, api.SourceSummary{
		Name:                "src1",
		ConnectivityStatus:  api.EXTERNALCONNECTIVITYSTATUS_OK,
		TotalInstances:      4,
		ImportedInstances:   3,
		MigratedInstances:   1,
		IneligibleInstances: 2, IneligibleReasons: []api.ReasonInstances{
			{Reason: "Manually disabled", Instances: []api.InstanceRef{ref(instD)}},
			{Reason: "Unknown IP address", Instances: []api.InstanceRef{ref(instF)}},
		},
		OverriddenInstances: []api.OverriddenInstance{{InstanceRef: ref(instF), Batch: "b2"}},
		BlockedImports:      []api.InstanceRef{ref(instF)},
		FailedImports:       []api.InstanceRef{},
		PartialImports:      []api.InstanceRef{ref(instF)},
	}, stats.Sources.Items[0])
	require.Equal(t, api.SourceSummary{
		Name:                "src2",
		ConnectivityStatus:  api.EXTERNALCONNECTIVITYSTATUS_OK,
		TotalInstances:      3,
		ImportedInstances:   1,
		MigratedInstances:   0,
		IneligibleInstances: 2,
		IneligibleReasons: []api.ReasonInstances{
			{Reason: "Unknown IP address", Instances: []api.InstanceRef{ref(instE), ref(instG)}},
		},
		OverriddenInstances: []api.OverriddenInstance{{InstanceRef: ref(instG), Batch: "b2"}},
		BlockedImports:      []api.InstanceRef{},
		FailedImports:       []api.InstanceRef{ref(instE), ref(instG)},
		PartialImports:      []api.InstanceRef{},
	}, stats.Sources.Items[1])

	// vm-f and vm-g are in both b2 and b3, but only count once towards the totals.
	require.Equal(t, 6, stats.Batches.TotalInstances)
	require.Equal(t, 1, stats.Batches.MigratedInstances)
	require.Equal(t, int64(4*gib), stats.Batches.TotalDiskSize)
	require.Equal(t, int64(3*gib), stats.Batches.MigratedDiskSize)
	require.Equal(t, 3, stats.Batches.BlockedInstances)
	require.Equal(t, []api.ReasonInstances{
		{Reason: "Cannot place instance", Instances: []api.InstanceRef{ref(instE), ref(instF)}},
		{Reason: "Manually disabled", Instances: []api.InstanceRef{ref(instD)}},
	}, stats.Batches.BlockedReasons)

	// vm-f is waived too, but it's blocked on placement.
	require.Len(t, stats.Batches.Items, 3)
	require.Equal(t, api.BatchSummary{
		Name:              "b1",
		Status:            api.BATCHSTATUS_RUNNING,
		TotalInstances:    4,
		MigratedInstances: 1,
		TotalDiskSize:     3 * gib,
		MigratedDiskSize:  3 * gib,
		BlockedInstances:  2,
		BlockedReasons: []api.ReasonInstances{
			{Reason: "Cannot place instance", Instances: []api.InstanceRef{ref(instE)}},
			{Reason: "Manually disabled", Instances: []api.InstanceRef{ref(instD)}},
		},
	}, stats.Batches.Items[0])
	require.Equal(t, api.BatchSummary{
		Name:             "b2",
		Status:           api.BATCHSTATUS_RUNNING,
		TotalInstances:   2,
		TotalDiskSize:    gib,
		BlockedInstances: 1,
		BlockedReasons: []api.ReasonInstances{
			{Reason: "Cannot place instance", Instances: []api.InstanceRef{ref(instF)}},
		},
	}, stats.Batches.Items[1])
	require.Equal(t, api.BatchSummary{
		Name:           "b3",
		Status:         api.BATCHSTATUS_DEFINED,
		TotalInstances: 2,
		TotalDiskSize:  gib,
		BlockedReasons: []api.ReasonInstances{},
	}, stats.Batches.Items[2])
}
