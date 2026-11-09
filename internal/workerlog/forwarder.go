package workerlog

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// Forwarder uploads a worker log incrementally from its last acknowledged offset.
type Forwarder struct {
	flushing chan struct{}
	file     string
	session  string
	offset   int64
	send     func(context.Context, api.WorkerLogBatch) (int64, error)
}

// NewForwarder creates a forwarder whose send callback returns the stored byte offset.
func NewForwarder(file string, session string, send func(context.Context, api.WorkerLogBatch) (int64, error)) *Forwarder {
	return &Forwarder{
		flushing: make(chan struct{}, 1),
		file:     file,
		session:  session,
		send:     send,
	}
}

// Flush uploads a snapshot of pending log bytes, respecting the context deadline.
func (f *Forwarder) Flush(ctx context.Context) error {
	// Waiting for another flush must respect the caller's shutdown deadline too.
	select {
	case f.flushing <- struct{}{}:
		defer func() { <-f.flushing }()
	case <-ctx.Done():
		return ctx.Err()
	}

	file, err := os.Open(f.file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Snapshot the size so new log messages cannot keep a flush running indefinitely.
	size := info.Size()
	if size < f.offset {
		return fmt.Errorf("Worker log file was truncated below acknowledged offset %d", f.offset)
	}

	for f.offset < size {
		err = ctx.Err()
		if err != nil {
			return err
		}

		data := make([]byte, min(api.WorkerLogChunkSize, size-f.offset))
		_, err = file.ReadAt(data, f.offset)
		if err != nil {
			return err
		}

		next, err := f.send(ctx, api.WorkerLogBatch{Session: f.session, Offset: f.offset, Data: data})
		if err != nil {
			return err
		}

		// A restarted worker can replay a chunk already followed by more stored data.
		if next < f.offset+int64(len(data)) || next > size {
			return fmt.Errorf("Invalid acknowledged log offset %d", next)
		}

		f.offset = next
	}

	return nil
}
