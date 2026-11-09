package workerlog_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/workerlog"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestForwarderRetriesAndRestarts(t *testing.T) {
	store := workerlog.NewStore(t.TempDir())
	instance := uuid.New()
	session := uuid.New()
	path := filepath.Join(t.TempDir(), "worker.log")
	content := strings.Repeat("migration progress\n", api.WorkerLogChunkSize/10)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	lostResponse := true
	send := func(ctx context.Context, batch api.WorkerLogBatch) (int64, error) {
		require.Equal(t, session.String(), batch.Session)
		require.LessOrEqual(t, len(batch.Data), api.WorkerLogChunkSize)
		offset, err := store.Append(instance, session, batch.Offset, batch.Data)
		require.NoError(t, err)
		if lostResponse {
			lostResponse = false
			return 0, fmt.Errorf("Response lost after storing chunk")
		}

		return offset, nil
	}

	forwarder := workerlog.NewForwarder(path, session.String(), send)
	require.Error(t, forwarder.Flush(context.Background()))
	require.NoError(t, forwarder.Flush(context.Background()))
	stored, err := os.ReadFile(store.Path(instance, session))
	require.NoError(t, err)
	require.Equal(t, content, string(stored))

	// A new process starts at zero and resumes from the server's acknowledged offset.
	content += "migration complete\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	forwarder = workerlog.NewForwarder(path, session.String(), send)
	require.NoError(t, forwarder.Flush(context.Background()))
	stored, err = os.ReadFile(store.Path(instance, session))
	require.NoError(t, err)
	require.Equal(t, content, string(stored))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	forwarder = workerlog.NewForwarder(path, session.String(), send)
	require.ErrorIs(t, forwarder.Flush(ctx), context.Canceled)
}

func TestForwarderDeadlineWhileAnotherFlushRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.log")
	require.NoError(t, os.WriteFile(path, []byte("message\n"), 0o600))
	started := make(chan struct{})
	release := make(chan struct{})
	first := make(chan error, 1)
	defer func() {
		close(release)
		require.NoError(t, <-first)
	}()
	forwarder := workerlog.NewForwarder(path, uuid.NewString(), func(ctx context.Context, batch api.WorkerLogBatch) (int64, error) {
		close(started)
		<-release
		return int64(len(batch.Data)), nil
	})
	go func() { first <- forwarder.Flush(context.Background()) }()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- forwarder.Flush(ctx) }()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("Flush ignored its deadline while waiting for another upload")
	}
}

func TestForwarderReportsTruncatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.log")
	require.NoError(t, os.WriteFile(path, []byte("original log\n"), 0o600))
	forwarder := workerlog.NewForwarder(path, uuid.NewString(), func(ctx context.Context, batch api.WorkerLogBatch) (int64, error) {
		return batch.Offset + int64(len(batch.Data)), nil
	})
	require.NoError(t, forwarder.Flush(context.Background()))
	require.NoError(t, os.Truncate(path, 0))
	require.ErrorContains(t, forwarder.Flush(context.Background()), "truncated below acknowledged offset")
}
