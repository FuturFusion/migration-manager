package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/workerlog"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// StartLogForwarding returns a function that stops uploads and flushes remaining logs.
func (w *Worker) StartLogForwarding(ctx context.Context) func() {
	// The boot ID keeps the same session when systemd restarts the worker process.
	bootID, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	session, parseErr := uuid.Parse(strings.TrimSpace(string(bootID)))
	if err != nil || parseErr != nil {
		session = uuid.New()
	}

	w.logForwarder = workerlog.NewForwarder(w.logFile, session.String(), w.sendLogs)
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		failed := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				flushCtx, stop := context.WithTimeout(ctx, 10*time.Second)
				err := w.logForwarder.Flush(flushCtx)
				stop()
				if err != nil && ctx.Err() == nil && !failed {
					slog.Warn("Failed to forward worker logs, will retry", logger.Err(err))
				}

				failed = err != nil
			}
		}
	}()

	return func() {
		cancel()
		<-done
		w.flushLogs()
	}
}

func (w *Worker) flushLogs() {
	if w.logForwarder == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := w.logForwarder.Flush(ctx)
	if err != nil {
		slog.Warn("Failed to flush worker logs", logger.Err(err))
	}
}

func (w *Worker) sendLogs(ctx context.Context, batch api.WorkerLogBatch) (int64, error) {
	content, err := json.Marshal(batch)
	if err != nil {
		return 0, err
	}

	req, client, err := w.makeRequest("/internal/worker/"+w.uuid+"/:logs", http.MethodPost, "secret="+w.token, bytes.NewReader(content))
	if err != nil {
		return 0, err
	}

	defer client.CloseIdleConnections()

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	return decodeLogOffset(resp)
}

func decodeLogOffset(resp *http.Response) (int64, error) {
	var result struct {
		Metadata int64  `json:"metadata"`
		Error    string `json:"error"`
	}

	err := json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Log upload failed (%d): %s", resp.StatusCode, result.Error)
	}

	return result.Metadata, nil
}
