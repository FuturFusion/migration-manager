package api

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestInstanceVROpsAPI(t *testing.T) {
	d := daemonSetup(t)
	client, serverURL := startTestDaemon(t, d, []APIEndpoint{
		sourcesCmd, sourceCmd, sourceSyncCmd, instanceCmd,
		instanceMonitoringHistoryCmd, instanceMonitoringSummaryCmd,
	}, nil)

	// Seed imported VMware inventory with one VM that vROps will not report.
	_, err := d.source.Create(t.Context(), migration.Source{
		Name:       "src",
		SourceType: api.SOURCETYPE_VMWARE,
		Properties: json.RawMessage(`{"endpoint":"https://vcenter","username":"admin","password":"secret","connectivity_status":"OK"}`),
	})
	require.NoError(t, err)

	instances := uuidCache{}
	instance := instances.newTestInstance("monitored", map[int]bool{0: true}, map[int]string{}, api.OSTYPE_LINUX, false)
	instance, err = d.instance.Create(t.Context(), instance)
	require.NoError(t, err)
	unmatched := instances.newTestInstance("unmatched", map[int]bool{0: true}, map[int]string{}, api.OSTYPE_LINUX, false)
	unmatched, err = d.instance.Create(t.Context(), unmatched)
	require.NoError(t, err)

	vrops := newVROpsAPIServer(t, instance.UUID)
	// Trust the test appliance certificate through the normal source configuration.
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: vrops.Certificate().Raw})
	properties, err := json.Marshal(api.VROpsProperties{
		Endpoint:                    vrops.URL,
		Username:                    "admin",
		Password:                    "secret",
		TrustedServerCACertificates: []string{string(certificate)},
	})
	require.NoError(t, err)
	src := api.Source{
		SourcePut:  api.SourcePut{Name: "ops", Properties: properties},
		SourceType: api.SOURCETYPE_VROPS,
	}

	sourceBody, err := json.Marshal(src)
	require.NoError(t, err)
	sourcesURL := serverURL + "/1.0/sources"
	sourceURL := sourcesURL + "/" + src.Name
	status, body := probeAPI(t, client, http.MethodPost, sourcesURL, bytes.NewReader(sourceBody), nil)
	require.Equal(t, http.StatusCreated, status, body)
	require.Contains(t, body, `"ConnectivityStatus":"OK"`)

	instanceURL := serverURL + "/1.0/instances/" + instance.UUID.String()
	monitoringURL := instanceURL + "/monitoring"
	getInstance := func(t *testing.T) api.Instance {
		t.Helper()

		status, body := probeAPI(t, client, http.MethodGet, instanceURL, nil, nil)
		require.Equal(t, http.StatusOK, status, body)
		var result struct {
			Metadata api.Instance `json:"metadata"`
		}

		require.NoError(t, json.Unmarshal([]byte(body), &result))
		return result.Metadata
	}

	// Monitoring is available only after a sync matches the VM to a vROps resource.
	status, body = probeAPI(t, client, http.MethodGet, monitoringURL+"/summary", nil, nil)
	require.Equal(t, http.StatusBadRequest, status, body)

	status, body = probeAPI(t, client, http.MethodPost, sourceURL+"/:sync", nil, nil)
	require.Equal(t, http.StatusOK, status, body)

	monitored := getInstance(t)
	monitoring := monitored.Monitoring
	require.NotNil(t, monitoring)
	require.Equal(t, "ops", monitoring.Source)
	require.Equal(t, "resource", monitoring.ResourceID)
	require.Equal(t, "GREEN", monitoring.Health)
	require.Equal(t, "DATA_RECEIVING", monitoring.Status)
	require.InDelta(t, 42.0, monitoring.Metrics["cpu|usage_average"], 0.001)
	require.Equal(t, instance.Properties.Disks, monitored.Disks)
	require.Equal(t, instance.Properties.NICs, monitored.NICs)
	require.Equal(t, instance.LastUpdateFromSource, monitored.LastUpdateFromSource)

	// Run sequentially because the scenarios share a source and monitoring snapshot.
	t.Run("summaries", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			period   string
			duration time.Duration
		}{
			{"default", "", 10 * time.Minute},
			{"10 minutes", "10m", 10 * time.Minute},
			{"1 hour", "1h", time.Hour},
			{"1 day", "1d", 24 * time.Hour},
			{"1 week", "1w", 7 * 24 * time.Hour},
		} {
			t.Run(tc.name, func(t *testing.T) {
				status, body := probeAPI(t, client, http.MethodGet, monitoringURL+"/summary?period="+tc.period, nil, nil)
				require.Equal(t, http.StatusOK, status, body)
				var result struct {
					Metadata api.InstanceMonitoringSummary `json:"metadata"`
				}

				require.NoError(t, json.Unmarshal([]byte(body), &result))
				require.Equal(t, tc.duration, result.Metadata.End.Sub(result.Metadata.Begin))
				require.Equal(t, result.Metadata.Begin.UnixMilli(), vrops.historyBegin.Load())
				require.Equal(t, result.Metadata.End.UnixMilli(), vrops.historyEnd.Load())

				// Utilization is averaged, while filesystem measurements use the latest sample.
				require.InDelta(t, 40.0, result.Metadata.Metrics["cpu|usage_average"], 0.001)
				require.InDelta(t, 120.0, result.Metadata.Metrics["guestfilesystem|usage_total"], 0.001)
				require.InDelta(t, 256.0, result.Metadata.Metrics["guestfilesystem|capacity_total"], 0.001)

				// A reported zero must remain distinguishable from a missing metric.
				require.Contains(t, result.Metadata.Metrics, "mem|usage_average")
				require.Zero(t, result.Metadata.Metrics["mem|usage_average"])
				require.NotContains(t, result.Metadata.Metrics, "storage|usage_average")
			})
		}
	})

	t.Run("history", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			query    string
			duration time.Duration
		}{
			{"default", "", 30 * time.Minute},
			{"30 minutes", "?period=30m", 30 * time.Minute},
			{"45 minutes", "?period=45m", 45 * time.Minute},
			{"1 hour", "?period=1h", time.Hour},
			{"1 day", "?period=1d", 24 * time.Hour},
			{"1 week", "?period=1w", 7 * 24 * time.Hour},
		} {
			t.Run(tc.name, func(t *testing.T) {
				status, body := probeAPI(t, client, http.MethodGet, monitoringURL+"/history"+tc.query, nil, nil)
				require.Equal(t, http.StatusOK, status, body)
				var history struct {
					Metadata api.InstanceMonitoringHistory `json:"metadata"`
				}

				require.NoError(t, json.Unmarshal([]byte(body), &history))
				require.Equal(t, tc.duration, history.Metadata.End.Sub(history.Metadata.Begin))
				require.Equal(t, history.Metadata.Begin.UnixMilli(), vrops.historyBegin.Load())
				require.Equal(t, history.Metadata.End.UnixMilli(), vrops.historyEnd.Load())
				require.Len(t, history.Metadata.Metrics["cpu|usage_average"], 2)
				require.Len(t, history.Metadata.Metrics["disk|read_average"], 2)
				require.InDelta(t, 256.0, history.Metadata.Metrics["disk|read_average"][1].Value, 0.001)
			})
		}
	})

	t.Run("invalid requests", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			path string
		}{
			{"unsupported period", monitoringURL + "/summary?period=1mo"},
			{"invalid history period", monitoringURL + "/history?period=invalid"},
			{"zero history period", monitoringURL + "/history?period=0"},
			{"negative history period", monitoringURL + "/history?period=-1m"},
			{"history below timestamp resolution", monitoringURL + "/history?period=1ns"},
			{"history above one week", monitoringURL + "/history?period=169h"},
			{"overflowing history period", monitoringURL + "/history?period=999999999999999999h"},
			{"invalid instance UUID", serverURL + "/1.0/instances/invalid/monitoring/summary"},
			{"unknown instance", serverURL + "/1.0/instances/" + uuid.NewString() + "/monitoring/history"},
			{"unmatched instance", serverURL + "/1.0/instances/" + unmatched.UUID.String() + "/monitoring/summary"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				vrops.historyBegin.Store(0)
				status, body := probeAPI(t, client, http.MethodGet, tc.path, nil, nil)
				require.Equal(t, http.StatusBadRequest, status, body)
				require.Zero(t, vrops.historyBegin.Load())
			})
		}
	})

	t.Run("authentication required", func(t *testing.T) {
		clientTransport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		// Remove client authentication while retaining trust in the test daemon.
		transport := clientTransport.Clone()
		transport.TLSClientConfig.Certificates = nil
		t.Cleanup(transport.CloseIdleConnections)
		anonymous := &http.Client{Transport: transport}

		for _, endpoint := range []string{"history", "summary"} {
			status, body := probeAPI(t, anonymous, http.MethodGet, monitoringURL+"/"+endpoint, nil, nil)
			require.Equal(t, http.StatusUnauthorized, status, body)
		}
	})

	for _, tc := range []struct {
		name    string
		failure *atomic.Bool
	}{
		{"vROps authentication fails", &vrops.rejectCredentials},
		{"vROps history unavailable", &vrops.rejectHistory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.failure.Store(true)
			// Restore the appliance even if an assertion stops this scenario early.
			t.Cleanup(func() { tc.failure.Store(false) })

			for _, endpoint := range []string{"history", "summary"} {
				status, body := probeAPI(t, client, http.MethodGet, monitoringURL+"/"+endpoint, nil, nil)
				require.Equal(t, http.StatusServiceUnavailable, status, body)
				require.NotContains(t, body, "secret")
				require.NotContains(t, body, "test-token")
			}
		})
	}

	t.Run("monitoring requests preserve VM data", func(t *testing.T) {
		// On-demand reads and upstream failures must leave the stored snapshot intact.
		saved := getInstance(t)
		require.Equal(t, monitored, saved)
	})

	t.Run("source rename and deletion", func(t *testing.T) {
		renamed := src.SourcePut
		renamed.Name = "renamed-ops"
		sourceBody, err := json.Marshal(renamed)
		require.NoError(t, err)
		status, body := probeAPI(t, client, http.MethodPut, sourceURL, bytes.NewReader(sourceBody), nil)
		require.Equal(t, http.StatusCreated, status, body)

		// A source rename must preserve the VM's monitoring association.
		status, body = probeAPI(t, client, http.MethodGet, monitoringURL+"/summary", nil, nil)
		require.Equal(t, http.StatusOK, status, body)
		saved := getInstance(t)
		require.NotNil(t, saved.Monitoring)
		require.Equal(t, renamed.Name, saved.Monitoring.Source)

		// Removing a monitoring source must keep the imported VM and its inventory.
		status, body = probeAPI(t, client, http.MethodDelete, sourcesURL+"/"+renamed.Name, nil, nil)
		require.Equal(t, http.StatusOK, status, body)
		for _, endpoint := range []string{"history", "summary"} {
			status, body := probeAPI(t, client, http.MethodGet, monitoringURL+"/"+endpoint, nil, nil)
			require.Equal(t, http.StatusBadRequest, status, body)
		}

		expected := monitored
		expected.Monitoring = nil
		require.Equal(t, expected, getInstance(t))
	})
}

type vropsAPIServer struct {
	*httptest.Server
	// Handlers and tests access these controls from different goroutines.
	rejectCredentials atomic.Bool
	rejectHistory     atomic.Bool
	historyBegin      atomic.Int64
	historyEnd        atomic.Int64
}

func newVROpsAPIServer(t *testing.T, instanceUUID uuid.UUID) *vropsAPIServer {
	t.Helper()

	server := &vropsAPIServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /suite-api/api/auth/token/acquire", func(w http.ResponseWriter, r *http.Request) {
		var credentials map[string]string
		err := json.NewDecoder(r.Body).Decode(&credentials)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"username": "admin", "password": "secret", "authSource": "LOCAL"}, credentials)
		if server.rejectCredentials.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_, _ = fmt.Fprint(w, vropsTokenResponse)
	})
	mux.HandleFunc("GET /suite-api/api/resources", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "vRealizeOpsToken test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "VMWARE", r.URL.Query().Get("adapterKind"))
		assert.Equal(t, "VirtualMachine", r.URL.Query().Get("resourceKind"))
		_, _ = fmt.Fprintf(w, vropsResourcesResponse, instanceUUID)
	})
	mux.HandleFunc("GET /suite-api/api/resources/resource/stats/latest", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "vRealizeOpsToken test-token", r.Header.Get("Authorization"))
		_, _ = fmt.Fprintf(w, vropsLatestStatsResponse, time.Now().Add(-time.Minute).UnixMilli())
	})
	mux.HandleFunc("GET /suite-api/api/resources/resource/stats", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "vRealizeOpsToken test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "false", r.URL.Query().Get("currentOnly"))
		assert.Empty(t, r.URL.Query().Get("rollUpType"))
		begin, err := strconv.ParseInt(r.URL.Query().Get("begin"), 10, 64)
		assert.NoError(t, err)
		end, err := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64)
		assert.NoError(t, err)
		server.historyBegin.Store(begin)
		server.historyEnd.Store(end)
		if server.rejectHistory.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, vropsUnavailableResponse)
			return
		}

		// Recent samples fall within every supported monitoring period.
		sampleTime := time.Now().Add(-time.Minute).UnixMilli()
		_, _ = fmt.Fprintf(w, vropsHistoryResponse, sampleTime-60000, sampleTime)
	})
	server.Server = httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	return server
}

const (
	vropsTokenResponse = `{"token":"test-token"}`

	vropsResourcesResponse = `{
	"resourceList": [{
		"identifier": "resource",
		"resourceHealth": "GREEN",
		"resourceStatusStates": [{"resourceStatus": "DATA_RECEIVING"}],
		"resourceKey": {
			"adapterKindKey": "VMWARE",
			"resourceKindKey": "VirtualMachine",
			"resourceIdentifiers": [{
				"identifierType": {"name": "VMEntityInstanceUUID"},
				"value": "%s"
			}]
		}
	}]
}`

	vropsLatestStatsResponse = `{"values":[{"resourceId":"resource","stat-list":{"stat":[
	{"statKey":{"key":"cpu|usage_average"},"data":[42],"timestamps":[%d]}
]}}]}`

	vropsUnavailableResponse = `{"message":"Monitoring temporarily unavailable"}`

	vropsHistoryResponse = `{"values":[{"resourceId":"resource","stat-list":{"stat":[
	{"statKey":{"key":"cpu|usage_average"},"data":[20,60],"timestamps":[%[1]d,%[2]d]},
	{"statKey":{"key":"mem|usage_average"},"data":[0,0],"timestamps":[%[1]d,%[2]d]},
	{"statKey":{"key":"guestfilesystem|usage_total"},"data":[100,120],"timestamps":[%[1]d,%[2]d]},
	{"statKey":{"key":"guestfilesystem|capacity_total"},"data":[256,256],"timestamps":[%[1]d,%[2]d]},
	{"statKey":{"key":"disk|read_average"},"data":[128,256],"timestamps":[%[1]d,%[2]d]}
]}}]}`
)
