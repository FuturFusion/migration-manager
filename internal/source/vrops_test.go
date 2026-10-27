package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	incusTLS "github.com/lxc/incus/v7/shared/tls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestVROpsMonitoring(t *testing.T) {
	goodID, failedID, ambiguousID := uuid.New(), uuid.New(), uuid.New()
	authCalls := 0
	resourcePages := 0
	retried := false
	memoryData := "0"
	memoryTimestamp := int64(1700000002000)
	filesystemCapacity := "256"
	statsField := "stat-list"
	mux := http.NewServeMux()
	mux.HandleFunc("POST /suite-api/api/auth/token/acquire", func(w http.ResponseWriter, r *http.Request) {
		var credentials map[string]string
		err := json.NewDecoder(r.Body).Decode(&credentials)
		assert.NoError(t, err)
		assert.Equal(t, "LOCAL", credentials["authSource"])
		assert.Equal(t, "secret", credentials["password"])
		authCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"token": fmt.Sprintf("token-%d", authCalls), "validity": time.Now().Add(time.Hour).UnixMilli()})
	})
	mux.HandleFunc("GET /suite-api/api/resources", func(w http.ResponseWriter, r *http.Request) {
		resourcePages++
		assert.Equal(t, "vRealizeOpsToken token-1", r.Header.Get("Authorization"))
		assert.Equal(t, "VirtualMachine", r.URL.Query().Get("resourceKind"))
		assert.Equal(t, "VMWARE", r.URL.Query().Get("adapterKind"))
		resource := func(id string, vmID uuid.UUID) map[string]any {
			return map[string]any{
				"identifier": id, "resourceHealth": "GREEN",
				"resourceKey": map[string]any{
					"name": "same-name", "adapterKindKey": "VMWARE", "resourceKindKey": "VirtualMachine",
					"resourceIdentifiers": []any{map[string]any{"identifierType": map[string]string{"name": "VMEntityInstanceUUID"}, "value": vmID.String()}},
				},
			}
		}

		if r.URL.Query().Get("page") == "1" {
			_ = json.NewEncoder(w).Encode(map[string]any{"pageInfo": map[string]int{"totalCount": 5, "pageSize": 2}, "resourceList": []any{
				resource("duplicate-1", ambiguousID), resource("duplicate-2", ambiguousID), resource("unrelated", uuid.New()),
			}})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"pageInfo": map[string]int{"totalCount": 5, "pageSize": 2}, "resourceList": []any{
			resource("good", goodID), resource("failed", failedID),
		}})
	})
	mux.HandleFunc("GET /suite-api/api/resources/{id}/stats/latest", func(w http.ResponseWriter, r *http.Request) {
		assert.ElementsMatch(t, []string{
			"cpu|usage_average", "mem|usage_average", "disk|read_average", "disk|write_average",
			"guestfilesystem|usage_total", "guestfilesystem|capacity_total",
			"storage|usage_average",
			"storage|commandsAveraged_average", "storage|totalLatency_average",
			"net|received_average", "net|transmitted_average",
		}, r.URL.Query()["statKey"])
		assert.Equal(t, "1", r.URL.Query().Get("maxSamples"))
		if r.PathValue("id") == "failed" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		assert.Equal(t, "good", r.PathValue("id"))
		if !retried {
			retried = true
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		assert.Equal(t, "vRealizeOpsToken token-2", r.Header.Get("Authorization"))
		_, _ = fmt.Fprintf(w, `{"values":[{"resourceId":"good","%s":{"stat":[
			{"statKey":{"key":"cpu|usage_average"},"data":[12.5,99],"timestamps":[1700000002000,1700000001000]},
			{"statKey":{"key":"mem|usage_average"},"data":[%s],"timestamps":[%d]},
			{"statKey":{"key":"disk|read_average"},"data":[1024,1],"timestamps":[1700000002000,1700000001000]},
			{"statKey":{"key":"disk|write_average"},"data":[0],"timestamps":[1700000002000]},
			{"statKey":{"key":"guestfilesystem|usage_total"},"data":[120.5],"timestamps":[1700000002000]},
			{"statKey":{"key":"guestfilesystem|capacity_total"},"data":[%s],"timestamps":[1700000002000]},
			{"statKey":{"key":"storage|commandsAveraged_average"},"data":[85],"timestamps":[1700000002000]},
			{"statKey":{"key":"storage|usage_average"},"data":[4096],"timestamps":[1700000002000]},
			{"statKey":{"key":"storage|totalLatency_average"},"data":[2.5],"timestamps":[1700000002000]},
			{"statKey":{"key":"net|received_average"},"data":[64],"timestamps":[1700000002000]},
			{"statKey":{"key":"net|transmitted_average"},"data":[0],"timestamps":[1700000002000]},
			{"statKey":{"key":"unrequested"},"data":[42],"timestamps":[1700000002000]}
		]}}]}`, statsField, memoryData, memoryTimestamp, filesystemCapacity)
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()
	props, err := json.Marshal(api.VROpsProperties{
		Endpoint: server.URL, Username: "admin", Password: "secret",
		ServerCertificate:                   server.Certificate().Raw,
		TrustedServerCertificateFingerprint: incusTLS.CertFingerprint(server.Certificate()),
	})
	require.NoError(t, err)
	client, err := NewInternalVROpsSourceFrom(api.Source{SourceType: api.SOURCETYPE_VROPS, SourcePut: api.SourcePut{Name: "ops", Properties: props}})
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.Connect(t.Context()))
	instances := make(migration.Instances, 0, 4)
	for _, id := range []uuid.UUID{goodID, failedID, ambiguousID, uuid.New()} {
		instances = append(instances, migration.Instance{UUID: id, SourceType: api.SOURCETYPE_VMWARE, Properties: api.InstanceProperties{InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Name: "same-name"}}})
	}

	results, err := client.CollectMonitoring(t.Context(), instances)
	require.ErrorContains(t, err, "multiple vROps resources")
	require.ErrorContains(t, err, "503")
	require.Len(t, results, 2)
	expectedMetrics := map[string]float64{
		"cpu|usage_average": 12.5, "mem|usage_average": 0, "disk|read_average": 1024, "disk|write_average": 0,
		"guestfilesystem|usage_total": 120.5, "guestfilesystem|capacity_total": 256,
		"storage|usage_average":            4096,
		"storage|commandsAveraged_average": 85, "storage|totalLatency_average": 2.5,
		"net|received_average": 64, "net|transmitted_average": 0,
	}

	require.Equal(t, expectedMetrics, results[goodID].Metrics)
	for key := range expectedMetrics {
		require.Equal(t, time.UnixMilli(1700000002000).UTC(), results[goodID].SampleTimes[key])
	}

	require.Equal(t, "ops", results[goodID].Source)
	require.False(t, results[goodID].CollectedAt.IsZero())
	require.NotEmpty(t, results[failedID].Error)
	require.True(t, results[failedID].CollectedAt.IsZero())
	require.Equal(t, 2, authCalls)
	require.Equal(t, 2, resourcePages)

	memoryData = "null"
	metrics, _, err := client.getLatestStats(t.Context(), "good")
	require.NoError(t, err)
	require.NotContains(t, metrics, "mem|usage_average")

	memoryData = "1"
	memoryTimestamp = 1<<63 - 1
	metrics, sampleTimes, err := client.getLatestStats(t.Context(), "good")
	require.NoError(t, err)
	require.NotContains(t, metrics, "mem|usage_average")
	_, err = json.Marshal(api.InstanceMonitoring{Metrics: metrics, SampleTimes: sampleTimes})
	require.NoError(t, err)

	statsField = "stats"
	memoryData = "0"
	memoryTimestamp = 1700000002000
	metrics, _, err = client.getLatestStats(t.Context(), "good")
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, metrics)

	filesystemCapacity = "-1"
	metrics, sampleTimes, err = client.getLatestStats(t.Context(), "good")
	require.NoError(t, err)
	require.NotContains(t, metrics, "guestfilesystem|capacity_total")
	require.NotContains(t, sampleTimes, "guestfilesystem|capacity_total")
	require.Contains(t, metrics, "cpu|usage_average")
}

func TestVROpsCollectionTimeout(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/token/acquire") {
			_, _ = fmt.Fprint(w, `{"token":"token"}`)
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	props, err := json.Marshal(api.VROpsProperties{
		Endpoint: server.URL, Username: "admin", Password: "secret",
		ServerCertificate:                   server.Certificate().Raw,
		TrustedServerCertificateFingerprint: incusTLS.CertFingerprint(server.Certificate()),
		ConnectionTimeout:                   api.AsDuration(time.Second), SyncTimeout: api.AsDuration(20 * time.Millisecond),
	})
	require.NoError(t, err)
	client, err := NewInternalVROpsSourceFrom(api.Source{SourceType: api.SOURCETYPE_VROPS, SourcePut: api.SourcePut{Properties: props}})
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.Connect(t.Context()))
	_, err = client.CollectMonitoring(t.Context(), migration.Instances{{UUID: uuid.New(), SourceType: api.SOURCETYPE_VMWARE}})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestVROpsConnection(t *testing.T) {
	for _, tc := range []struct {
		name             string
		trustCertificate bool
		status           int
		expected         api.ExternalConnectivityStatus
	}{
		{"trusted certificate", true, http.StatusOK, api.EXTERNALCONNECTIVITYSTATUS_OK},
		{"untrusted certificate", false, http.StatusOK, api.EXTERNALCONNECTIVITYSTATUS_TLS_ERROR},
		{"invalid credentials", true, http.StatusUnauthorized, api.EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, `{"token":"token"}`)
			}))
			defer server.Close()
			props := api.VROpsProperties{Endpoint: server.URL, Username: "admin", Password: "secret"}
			if tc.trustCertificate {
				props.ServerCertificate = server.Certificate().Raw
				props.TrustedServerCertificateFingerprint = incusTLS.CertFingerprint(server.Certificate())
			}

			raw, err := json.Marshal(props)
			require.NoError(t, err)
			client, err := NewInternalVROpsSourceFrom(api.Source{SourceType: api.SOURCETYPE_VROPS, SourcePut: api.SourcePut{Properties: raw}})
			require.NoError(t, err)
			defer client.Close()
			require.Equal(t, tc.expected, api.MapExternalConnectivityStatusToStatus(client.Connect(t.Context())))
		})
	}
}
