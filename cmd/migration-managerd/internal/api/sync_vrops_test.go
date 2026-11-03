package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	incusTLS "github.com/lxc/incus/v7/shared/tls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	endpointMock "github.com/FuturFusion/migration-manager/internal/migration/endpoint/mock"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/testcert"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestVROpsManualSync(t *testing.T) {
	d := daemonSetup(t)
	_, err := d.source.Create(t.Context(), migration.Source{
		Name: "src", SourceType: api.SOURCETYPE_VMWARE,
		Properties: json.RawMessage(`{"endpoint":"https://vcenter","username":"admin","password":"secret","connectivity_status":"OK"}`),
	})
	require.NoError(t, err)
	u := uuidCache{}
	instance := u.newTestInstance("vm", map[int]bool{0: true}, map[int]string{}, api.OSTYPE_LINUX, false)
	instance, err = d.instance.Create(t.Context(), instance)
	require.NoError(t, err)
	failed := false
	mux := http.NewServeMux()
	mux.HandleFunc("POST /suite-api/api/auth/token/acquire", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"token":"token"}`)
	})
	mux.HandleFunc("GET /suite-api/api/resources", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"resourceList":[{"identifier":"resource","resourceHealth":"GREEN","resourceKey":{"adapterKindKey":"VMWARE","resourceKindKey":"VirtualMachine","resourceIdentifiers":[{"identifierType":{"name":"VMEntityInstanceUUID"},"value":"%s"}]}}]}`, instance.UUID)
	})
	mux.HandleFunc("GET /suite-api/api/resources/resource/stats/latest", func(w http.ResponseWriter, r *http.Request) {
		if failed {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		_, _ = fmt.Fprint(w, `{"values":[{"resourceId":"resource","stat-list":{"stat":[
			{"statKey":{"key":"cpu|usage_average"},"data":[42],"timestamps":[1700000002000]},
			{"statKey":{"key":"disk|read_average"},"data":[1024],"timestamps":[1700000002000]},
			{"statKey":{"key":"disk|write_average"},"data":[0],"timestamps":[1700000002000]},
			{"statKey":{"key":"guestfilesystem|usage_total"},"data":[120.5],"timestamps":[1700000002000]},
			{"statKey":{"key":"guestfilesystem|capacity_total"},"data":[256],"timestamps":[1700000002000]},
			{"statKey":{"key":"storage|commandsAveraged_average"},"data":[85],"timestamps":[1700000002000]},
			{"statKey":{"key":"storage|usage_average"},"data":[4096],"timestamps":[1700000002000]},
			{"statKey":{"key":"storage|totalLatency_average"},"data":[2.5],"timestamps":[1700000002000]},
			{"statKey":{"key":"net|received_average"},"data":[64],"timestamps":[1700000002000]},
			{"statKey":{"key":"net|transmitted_average"},"data":[0],"timestamps":[1700000002000]}
		]}}]}`)
	})
	mux.HandleFunc("GET /suite-api/api/resources/resource/stats", func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.Query().Get("rollUpType"))
		if failed {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"message":"Invalid historical query"}`)
			return
		}

		sampleTime := time.Now().Add(-time.Minute).UnixMilli()
		_, _ = fmt.Fprintf(w, `{"values":[{"resourceId":"resource","stat-list":{"stat":[
			{"statKey":{"key":"disk|read_average"},"data":[123],"timestamps":[%d]},
			{"statKey":{"key":"cpu|usage_average"},"data":[20,60],"timestamps":[%d,%d]}
		]}}]}`, sampleTime, sampleTime-60000, sampleTime)
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()
	props, err := json.Marshal(api.VROpsProperties{
		Endpoint: server.URL, Username: "admin", Password: "secret",
		ServerCertificate:                   server.Certificate().Raw,
		TrustedServerCertificateFingerprint: incusTLS.CertFingerprint(server.Certificate()),
		ConnectivityStatus:                  api.EXTERNALCONNECTIVITYSTATUS_OK,
	})
	require.NoError(t, err)
	src, err := d.source.Create(t.Context(), migration.Source{Name: "ops", SourceType: api.SOURCETYPE_VROPS, Properties: props})
	require.NoError(t, err)
	require.NoError(t, d.syncOneSource(t.Context(), src))
	saved, err := d.instance.GetByUUID(t.Context(), instance.UUID)
	require.NoError(t, err)
	require.Equal(t, instance.Properties.Disks, saved.Properties.Disks)
	require.Equal(t, instance.Properties.NICs, saved.Properties.NICs)
	require.Equal(t, instance.LastUpdateFromSource, saved.LastUpdateFromSource)
	require.InDelta(t, 42.0, saved.Properties.Monitoring.Metrics["cpu|usage_average"], 0.001)
	require.InDelta(t, 1024.0, saved.Properties.Monitoring.Metrics["disk|read_average"], 0.001)
	require.Contains(t, saved.Properties.Monitoring.Metrics, "disk|write_average")
	require.Zero(t, saved.Properties.Monitoring.Metrics["disk|write_average"])
	require.Equal(t, time.UnixMilli(1700000002000).UTC(), saved.Properties.Monitoring.SampleTimes["disk|read_average"])
	additionalMetrics := map[string]float64{
		"guestfilesystem|usage_total": 120.5, "guestfilesystem|capacity_total": 256,
		"storage|usage_average":            4096,
		"storage|commandsAveraged_average": 85, "storage|totalLatency_average": 2.5,
		"net|received_average": 64, "net|transmitted_average": 0,
	}

	for key, value := range additionalMetrics {
		require.Contains(t, saved.Properties.Monitoring.Metrics, key)
		require.InDelta(t, value, saved.Properties.Monitoring.Metrics[key], 0.001)
		require.Equal(t, time.UnixMilli(1700000002000).UTC(), saved.Properties.Monitoring.SampleTimes[key])
	}

	require.Equal(t, "ops", saved.Properties.Monitoring.Source)
	successfulAt := saved.Properties.Monitoring.CollectedAt
	require.WithinDuration(t, time.Now(), successfulAt, time.Second)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetPathValue("uuid", instance.UUID.String())
	recorder := httptest.NewRecorder()
	err = instanceMonitoringHistoryGet(d, request).Render(recorder)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	var historyResponse struct {
		Metadata api.InstanceMonitoringHistory `json:"metadata"`
	}

	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &historyResponse))
	require.Len(t, historyResponse.Metadata.Metrics["disk|read_average"], 1)
	require.InDelta(t, 123.0, historyResponse.Metadata.Metrics["disk|read_average"][0].Value, 0.001)
	summaryRequest := httptest.NewRequest(http.MethodGet, "/?period=1h", nil)
	summaryRequest.SetPathValue("uuid", instance.UUID.String())
	summaryRecorder := httptest.NewRecorder()
	require.NoError(t, instanceMonitoringSummaryGet(d, summaryRequest).Render(summaryRecorder))
	require.Equal(t, http.StatusOK, summaryRecorder.Code)
	var summaryResponse struct {
		Metadata api.InstanceMonitoringSummary `json:"metadata"`
	}

	require.NoError(t, json.Unmarshal(summaryRecorder.Body.Bytes(), &summaryResponse))
	require.InDelta(t, 40.0, summaryResponse.Metadata.Metrics["cpu|usage_average"], 0.001)
	require.Equal(t, time.Hour, summaryResponse.Metadata.End.Sub(summaryResponse.Metadata.Begin))
	invalidRequest := httptest.NewRequest(http.MethodGet, "/?period=1mo", nil)
	invalidRequest.SetPathValue("uuid", instance.UUID.String())
	require.Equal(t, http.StatusBadRequest, instanceMonitoringSummaryGet(d, invalidRequest).Code())
	afterHistory, err := d.instance.GetByUUID(t.Context(), instance.UUID)
	require.NoError(t, err)
	require.Equal(t, saved.Properties.Monitoring, afterHistory.Properties.Monitoring)

	failed = true
	require.Equal(t, http.StatusServiceUnavailable, instanceMonitoringSummaryGet(d, summaryRequest).Code())
	failedHistory := instanceMonitoringHistoryGet(d, request)
	require.Equal(t, http.StatusServiceUnavailable, failedHistory.Code())
	recorder = httptest.NewRecorder()
	require.NoError(t, failedHistory.Render(recorder))
	require.Contains(t, recorder.Body.String(), "vROps request failed with status 400 Bad Request")
	require.Contains(t, recorder.Body.String(), "Invalid historical query")
	require.NotContains(t, recorder.Body.String(), "secret")
	require.ErrorContains(t, d.syncOneSource(t.Context(), src), "503")
	saved, err = d.instance.GetByUUID(t.Context(), instance.UUID)
	require.NoError(t, err)
	require.Equal(t, successfulAt, saved.Properties.Monitoring.CollectedAt)
	for key, value := range additionalMetrics {
		require.Contains(t, saved.Properties.Monitoring.Metrics, key)
		require.InDelta(t, value, saved.Properties.Monitoring.Metrics[key], 0.001)
	}

	require.InDelta(t, 1024.0, saved.Properties.Monitoring.Metrics["disk|read_average"], 0.001)
	require.InDelta(t, 42.0, saved.Properties.Monitoring.Metrics["cpu|usage_average"], 0.001)
	require.NotEmpty(t, saved.Properties.Monitoring.Error)
	warnings, err := d.warning.GetAll(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, warnings)

	failed = false
	require.NoError(t, d.syncOneSource(t.Context(), src))
	saved, err = d.instance.GetByUUID(t.Context(), instance.UUID)
	require.NoError(t, err)
	require.Empty(t, saved.Properties.Monitoring.Error)
	warnings, err = d.warning.GetAll(t.Context())
	require.NoError(t, err)
	require.Empty(t, warnings)

	t.Run("certificate rotation persists the new certificate", func(t *testing.T) {
		block, _ := pem.Decode(testcert.LocalhostCert)
		require.NotNil(t, block)
		oldCertificate, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		require.NotEqual(t, oldCertificate.Raw, server.Certificate().Raw)
		properties, err := src.GetVROpsProperties()
		require.NoError(t, err)
		properties.ServerCertificate = oldCertificate.Raw
		src.Properties, err = json.Marshal(properties)
		require.NoError(t, err)
		src.EndpointFunc = func(s api.Source) (migration.SourceEndpoint, error) {
			return source.NewInternalVROpsSourceFrom(s)
		}

		require.NoError(t, d.source.Update(t.Context(), src.Name, &src, d.instance))
		persisted, err := d.source.GetByName(t.Context(), src.Name)
		require.NoError(t, err)
		require.Equal(t, server.Certificate().Raw, persisted.GetServerCertificate().Raw)
		require.NoError(t, d.syncOneSource(t.Context(), *persisted))
	})

	t.Run("manual refresh does not wait for the inventory sync lock", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		syncLock.Lock()
		done := make(chan error, 1)
		go func() { done <- d.syncOneSource(ctx, src) }()
		var syncErr error
		select {
		case syncErr = <-done:
			syncLock.Unlock()
		case <-ctx.Done():
			syncLock.Unlock()
			require.Error(t, <-done)
			syncErr = ctx.Err()
		}

		require.NoError(t, syncErr)
	})

	t.Run("source rename and endpoint changes update monitoring references", func(t *testing.T) {
		renamed := src
		renamed.Name = "renamed-ops"
		var checkedClient *source.InternalVROpsSource
		renamed.EndpointFunc = func(s api.Source) (migration.SourceEndpoint, error) {
			var err error
			checkedClient, err = source.NewInternalVROpsSourceFrom(s)
			return checkedClient, err
		}

		require.NoError(t, d.source.Update(t.Context(), src.Name, &renamed, d.instance))
		require.NotNil(t, checkedClient)
		require.False(t, checkedClient.IsConnected())
		saved, err := d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.Equal(t, renamed.Name, saved.Properties.Monitoring.Source)
		recorder := httptest.NewRecorder()
		require.NoError(t, instanceMonitoringSummaryGet(d, summaryRequest).Render(recorder))
		require.Equal(t, http.StatusOK, recorder.Code)

		properties, err := renamed.GetVROpsProperties()
		require.NoError(t, err)
		properties.Endpoint += "/"
		renamed.Properties, err = json.Marshal(properties)
		require.NoError(t, err)
		require.NoError(t, d.source.Update(t.Context(), renamed.Name, &renamed, d.instance))
		saved, err = d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.NotNil(t, saved.Properties.Monitoring)

		properties.Endpoint = "https://replacement.example.com"
		renamed.Properties, err = json.Marshal(properties)
		require.NoError(t, err)
		renamed.EndpointFunc = func(api.Source) (migration.SourceEndpoint, error) {
			return &endpointMock.SourceEndpointMock{
				DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
					return api.EXTERNALCONNECTIVITYSTATUS_CANNOT_CONNECT, nil
				},
			}, nil
		}

		require.NoError(t, d.source.Update(t.Context(), renamed.Name, &renamed, d.instance))
		saved, err = d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.Nil(t, saved.Properties.Monitoring)
		require.Equal(t, instance.Properties.Disks, saved.Properties.Disks)
		require.Equal(t, instance.Properties.NICs, saved.Properties.NICs)

		restored := src
		restored.EndpointFunc = func(s api.Source) (migration.SourceEndpoint, error) {
			return source.NewInternalVROpsSourceFrom(s)
		}

		require.NoError(t, d.source.Update(t.Context(), renamed.Name, &restored, d.instance))
		require.NoError(t, d.syncOneSource(t.Context(), restored))
	})

	t.Run("overlapping sources do not replace existing monitoring", func(t *testing.T) {
		other, err := d.source.Create(t.Context(), migration.Source{Name: "other", SourceType: api.SOURCETYPE_VROPS, Properties: props})
		require.NoError(t, err)
		before, err := d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.ErrorContains(t, d.syncOneSource(t.Context(), other), "already has monitoring")
		after, err := d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.Equal(t, before.Properties.Monitoring, after.Properties.Monitoring)

		bySource := map[string]map[uuid.UUID]migration.Instance{"src": {instance.UUID: instance}}
		warnings := applyVROpsMonitoring(t.Context(), migration.Sources{src, other}, bySource)
		require.NotEmpty(t, warnings)
		require.Nil(t, bySource["src"][instance.UUID].Properties.Monitoring)

		unreachable := src
		unreachable.EndpointFunc = func(api.Source) (migration.SourceEndpoint, error) {
			return &endpointMock.SourceEndpointMock{
				DoBasicConnectivityCheckFunc: func() (api.ExternalConnectivityStatus, *x509.Certificate) {
					return api.EXTERNALCONNECTIVITYSTATUS_CANNOT_CONNECT, nil
				},
			}, nil
		}

		require.NoError(t, d.source.Update(t.Context(), src.Name, &unreachable, d.instance))
		warnings = applyVROpsMonitoring(t.Context(), migration.Sources{unreachable, other}, bySource)
		require.NotEmpty(t, warnings)
		require.Equal(t, "other", bySource["src"][instance.UUID].Properties.Monitoring.Source)
		warnings, err = d.syncSourceData(t.Context(), bySource, nil, migration.Sources{unreachable, other})
		require.NoError(t, err)
		require.NotEmpty(t, warnings)
		after, err = d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.Equal(t, before.Properties.Monitoring, after.Properties.Monitoring)

		require.NoError(t, d.source.DeleteByName(t.Context(), src.Name, d.instance))
		require.NoError(t, d.syncOneSource(t.Context(), other))
		after, err = d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.Equal(t, "other", after.Properties.Monitoring.Source)
	})

	t.Run("source changes discard stale monitoring without blocking inventory updates", func(t *testing.T) {
		collectedFrom, err := d.source.GetByName(t.Context(), "other")
		require.NoError(t, err)
		before, err := d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		results, err := collectVROpsMonitoring(t.Context(), *collectedFrom, migration.Instances{*before})
		require.NoError(t, err)
		data := results[instance.UUID]
		incoming := instance
		incoming.Properties.Monitoring = &data
		incoming.Properties.Description = "Updated VMware inventory"
		bySource := map[string]map[uuid.UUID]migration.Instance{"src": {instance.UUID: incoming}}

		updated := *collectedFrom
		properties, err := updated.GetVROpsProperties()
		require.NoError(t, err)
		properties.Username = "new-user"
		updated.Properties, err = json.Marshal(properties)
		require.NoError(t, err)
		updated.EndpointFunc = func(s api.Source) (migration.SourceEndpoint, error) {
			return source.NewInternalVROpsSourceFrom(s)
		}

		require.NoError(t, d.source.Update(t.Context(), updated.Name, &updated, d.instance))
		require.ErrorContains(t, d.syncOneSource(t.Context(), *collectedFrom), "changed during collection")
		warnings, err := d.syncSourceData(t.Context(), bySource, nil, migration.Sources{*collectedFrom})
		require.NoError(t, err)
		require.NotEmpty(t, warnings)
		after, err := d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.Equal(t, before.Properties.Monitoring, after.Properties.Monitoring)
		require.Equal(t, incoming.Properties.Description, after.Properties.Description)

		require.NoError(t, d.source.DeleteByName(t.Context(), updated.Name, d.instance))
		require.ErrorContains(t, d.syncOneSource(t.Context(), updated), "changed during collection")
		after, err = d.instance.GetByUUID(t.Context(), instance.UUID)
		require.NoError(t, err)
		require.Nil(t, after.Properties.Monitoring)
		require.Equal(t, incoming.Properties.Description, after.Properties.Description)
	})
}
