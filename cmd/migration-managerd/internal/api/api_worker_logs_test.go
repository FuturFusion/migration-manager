package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type workerLogsQueue struct {
	migration.QueueService
	entry migration.QueueEntry
}

func (q workerLogsQueue) GetByInstanceUUID(ctx context.Context, id uuid.UUID) (*migration.QueueEntry, error) {
	if id != q.entry.InstanceUUID {
		return nil, migration.ErrNotFound
	}

	return &q.entry, nil
}

func TestWorkerLogsAPI(t *testing.T) {
	d := daemonSetup(t)
	instance := uuid.New()
	session := uuid.New()
	token := uuid.New()
	originalQueue := d.queue
	d.queue = workerLogsQueue{QueueService: d.queue, entry: migration.QueueEntry{InstanceUUID: instance, SecretToken: token}}
	client, serverURL := startTestDaemon(t, d, []APIEndpoint{instanceLogsCmd, instanceLogCmd}, []APIEndpoint{workerLogsCmd})

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	workerTransport := transport.Clone()
	workerTransport.TLSClientConfig.Certificates = nil
	defer workerTransport.CloseIdleConnections()
	workerClient := &http.Client{Transport: workerTransport}
	logsURL := serverURL + "/1.0/instances/" + instance.String() + "/logs"
	uploadURL := serverURL + "/internal/worker/" + instance.String() + "/:logs?secret=" + token.String()
	content := strings.Repeat("old message\n", 300) + "last message\n"
	batch, err := json.Marshal(api.WorkerLogBatch{Session: session.String(), Data: []byte(content)})
	require.NoError(t, err)

	for _, query := range []string{"", "&instance=" + strings.ToUpper(instance.String())} {
		status, body := probeAPI(t, workerClient, http.MethodPost, uploadURL+query, bytes.NewReader(batch), nil)
		require.Equal(t, http.StatusOK, status, body)
	}

	for _, query := range []string{uuid.NewString(), "invalid"} {
		status, body := probeAPI(t, workerClient, http.MethodPost, uploadURL+"&instance="+query, bytes.NewReader(batch), nil)
		require.Equal(t, http.StatusUnauthorized, status, body)
	}

	status, body := probeAPI(t, workerClient, http.MethodGet, logsURL+"?secret="+token.String()+"&instance="+instance.String(), nil, nil)
	require.Equal(t, http.StatusUnauthorized, status, body)
	status, body = probeAPI(t, workerClient, http.MethodPost, uploadURL, strings.NewReader(strings.Repeat("x", workerLogUploadLimit+1)), map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusBadRequest, status, body)

	// Retrieval remains available without a queue entry.
	d.queue = originalQueue
	status, body = probeAPI(t, client, http.MethodGet, logsURL, nil, nil)
	require.Equal(t, http.StatusOK, status, body)
	var sessions struct {
		Metadata []api.WorkerLogSession `json:"metadata"`
	}

	require.NoError(t, json.Unmarshal([]byte(body), &sessions))
	require.Len(t, sessions.Metadata, 1)
	require.Equal(t, int64(len(content)), sessions.Metadata[0].Size)

	logURL := logsURL + "/" + session.String()
	status, body = probeAPI(t, client, http.MethodGet, logURL, nil, nil)
	require.Equal(t, http.StatusOK, status, body)
	var preview struct {
		Metadata api.WorkerLogContent `json:"metadata"`
	}

	require.NoError(t, json.Unmarshal([]byte(body), &preview))
	require.Len(t, strings.Split(strings.TrimSuffix(preview.Metadata.Content, "\n"), "\n"), 200)
	require.True(t, preview.Metadata.Truncated)
	status, body = probeAPI(t, client, http.MethodGet, logURL+"?oldest_first=true", nil, nil)
	require.Equal(t, http.StatusOK, status, body)
	require.NoError(t, json.Unmarshal([]byte(body), &preview))
	require.Equal(t, strings.Repeat("old message\n", 200), preview.Metadata.Content)
	require.True(t, preview.Metadata.Truncated)

	status, body = probeAPI(t, client, http.MethodGet, logURL+"?lines=1000", nil, nil)
	require.Equal(t, http.StatusOK, status, body)
	require.NoError(t, json.Unmarshal([]byte(body), &preview))
	require.Equal(t, content, preview.Metadata.Content)

	status, body = probeAPI(t, client, http.MethodGet, logURL+"?download=1", nil, nil)
	require.Equal(t, http.StatusOK, status, body)
	require.Equal(t, content, body)
	status, body = probeAPI(t, client, http.MethodGet, logURL+"?lines=10001", nil, nil)
	require.Equal(t, http.StatusBadRequest, status, body)
	status, body = probeAPI(t, client, http.MethodGet, logURL+"?oldest_first=invalid", nil, nil)
	require.Equal(t, http.StatusBadRequest, status, body)
	status, body = probeAPI(t, client, http.MethodGet, logsURL+"/"+uuid.NewString(), nil, nil)
	require.Equal(t, http.StatusNotFound, status, body)
}

func TestWorkerLogsAuthenticationBoundsFormBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/internal/worker/"+uuid.NewString()+"/:logs", strings.NewReader("padding="+strings.Repeat("x", workerLogUploadLimit)))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err := workerLogsAuthenticate(nil, httptest.NewRecorder(), r)
	var sizeErr *http.MaxBytesError
	require.ErrorAs(t, err, &sizeErr)
}
