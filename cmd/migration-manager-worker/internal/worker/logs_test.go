package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/workerlog"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestLogsFlushedBeforeTerminalStatus(t *testing.T) {
	cases := []struct {
		name   string
		failed bool
	}{
		{name: "success"},
		{name: "failure", failed: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				paths = append(paths, r.URL.Path)
				var result response.Response = response.EmptySyncResponse
				if r.URL.Path == "/internal/worker/vm/:logs" {
					var batch api.WorkerLogBatch
					err := json.NewDecoder(r.Body).Decode(&batch)
					if err != nil {
						result = response.BadRequest(err)
					} else {
						result = response.SyncResponse(true, batch.Offset+int64(len(batch.Data)))
					}
				}

				_ = result.Render(w)
			}))
			defer server.Close()

			endpoint, err := url.Parse(server.URL)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "worker.log")
			require.NoError(t, os.WriteFile(path, []byte("last message\n"), 0o600))
			worker := &Worker{endpoint: endpoint, uuid: "vm", logFile: path}
			worker.logForwarder = workerlog.NewForwarder(path, uuid.NewString(), worker.sendLogs)
			if tc.failed {
				worker.sendErrorResponse(context.Canceled)
			} else {
				worker.sendStatusResponse(api.WORKERRESPONSE_SUCCESS, "done")
			}

			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, []string{"/internal/worker/vm/:logs", "/internal/worker/vm/:update"}, paths)
		})
	}
}

func TestLogForwardingCleanup(t *testing.T) {
	cases := []struct {
		name         string
		cancelCaller bool
	}{
		{name: "cleanup cancels uploader"},
		{name: "caller already cancelled", cancelCaller: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var uploaded string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var batch api.WorkerLogBatch
				err := json.NewDecoder(r.Body).Decode(&batch)
				if err != nil {
					_ = response.BadRequest(err).Render(w)
					return
				}

				mu.Lock()
				uploaded += string(batch.Data)
				mu.Unlock()
				_ = response.SyncResponse(true, batch.Offset+int64(len(batch.Data))).Render(w)
			}))
			defer server.Close()

			endpoint, err := url.Parse(server.URL)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "worker.log")
			require.NoError(t, os.WriteFile(path, []byte("before cancellation\n"), 0o600))
			worker := &Worker{endpoint: endpoint, uuid: "vm", logFile: path}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stopLogs := worker.StartLogForwarding(ctx)
			if tc.cancelCaller {
				cancel()
			}

			content := "before cancellation\nworker cleanup finished\n"
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
			done := make(chan struct{})
			go func() {
				defer close(done)
				stopLogs()
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("Log forwarding cleanup did not finish")
			}

			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, content, uploaded)
		})
	}
}
