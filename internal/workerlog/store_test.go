package workerlog_test

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/workerlog"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestStoreIsolationAndPreview(t *testing.T) {
	store := workerlog.NewStore(t.TempDir())
	instance := uuid.New()
	session := uuid.New()
	content := strings.Repeat("x", api.WorkerLogChunkSize)
	_, err := store.Append(instance, session, 0, []byte(content))
	require.NoError(t, err)
	_, err = store.Append(instance, session, int64(len(content)), []byte("tail"))
	require.NoError(t, err)

	_, err = store.Append(instance, session, 0, []byte("different content"))
	require.ErrorIs(t, err, workerlog.ErrOffset)
	_, err = store.Append(instance, session, int64(len(content)+100), []byte("gap"))
	require.ErrorIs(t, err, workerlog.ErrOffset)

	preview, err := store.Read(instance, session, 200, false)
	require.NoError(t, err)
	require.Len(t, preview.Content, api.WorkerLogChunkSize+4)
	require.True(t, strings.HasSuffix(preview.Content, "tail"))
	require.Equal(t, int64(len(content)+4), preview.Size)
	oldest, err := store.Read(instance, session, 1, true)
	require.NoError(t, err)
	require.Equal(t, content+"tail", oldest.Content)

	sessions, err := store.Sessions(instance)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, session.String(), sessions[0].Session)
	other, err := store.Sessions(uuid.New())
	require.NoError(t, err)
	require.Empty(t, other)

	info, err := os.Stat(store.Path(instance, session))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestStoreLineLimit(t *testing.T) {
	for _, trailingNewline := range []bool{false, true} {
		store := workerlog.NewStore(t.TempDir())
		instance := uuid.New()
		session := uuid.New()
		content := strings.Repeat("older line\n", 1000) + "latest line"
		if trailingNewline {
			content += "\n"
		}

		_, err := store.Append(instance, session, 0, []byte(content))
		require.NoError(t, err)
		preview, err := store.Read(instance, session, 200, false)
		require.NoError(t, err)
		require.Len(t, strings.Split(strings.TrimSuffix(preview.Content, "\n"), "\n"), 200)
		require.Contains(t, preview.Content, "latest line")
		require.True(t, preview.Truncated)

		preview, err = store.Read(instance, session, 2000, false)
		require.NoError(t, err)
		require.Equal(t, content, preview.Content)
		require.False(t, preview.Truncated)
	}
}

func TestStorePreviewByteLimit(t *testing.T) {
	store := workerlog.NewStore(t.TempDir())
	instance := uuid.New()
	session := uuid.New()
	chunk := []byte(strings.Repeat("x", api.WorkerLogChunkSize))
	for i := range api.WorkerLogPreviewSize/api.WorkerLogChunkSize + 1 {
		_, err := store.Append(instance, session, int64(i*len(chunk)), chunk)
		require.NoError(t, err)
	}

	for _, oldestFirst := range []bool{false, true} {
		preview, err := store.Read(instance, session, 200, oldestFirst)
		require.NoError(t, err)
		require.Len(t, preview.Content, api.WorkerLogPreviewSize)
		require.True(t, preview.Truncated)
	}
}

func TestStoreConcurrentRetries(t *testing.T) {
	store := workerlog.NewStore(t.TempDir())
	instance := uuid.New()
	session := uuid.New()
	errors := make(chan error, 16)
	for range 16 {
		go func() {
			_, err := store.Append(instance, session, 0, []byte("same batch\n"))
			errors <- err
		}()
	}

	for range 16 {
		require.NoError(t, <-errors)
	}

	content, err := os.ReadFile(store.Path(instance, session))
	require.NoError(t, err)
	require.Equal(t, "same batch\n", string(content))
}
