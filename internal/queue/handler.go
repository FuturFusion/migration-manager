package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// Handler handles interaction between services for the queue.
type Handler struct {
	batchLock util.IDLock[string]

	batch    migration.BatchService
	instance migration.InstanceService
	network  migration.NetworkService
	source   migration.SourceService
	target   migration.TargetService
	queue    migration.QueueService

	workerUpdateCache *util.Cache[uuid.UUID, time.Time]
}

// NewMigrationHandler creates a new handler for queued migrations.
func NewMigrationHandler(b migration.BatchService, i migration.InstanceService, n migration.NetworkService, s migration.SourceService, t migration.TargetService, q migration.QueueService) *Handler {
	return &Handler{
		batchLock:         util.NewIDLock[string](),
		workerUpdateCache: util.NewCache[uuid.UUID, time.Time](),

		batch:    b,
		instance: i,
		network:  n,
		source:   s,
		target:   t,
		queue:    q,
	}
}

// MigrationState is a cache of all migration data for a batch, queued by instance.
type MigrationState struct {
	Batch            migration.Batch
	MigrationWindows migration.MigrationWindows

	Targets      map[uuid.UUID]migration.Target
	QueueEntries map[uuid.UUID]migration.QueueEntry
	Instances    map[uuid.UUID]migration.Instance
	Sources      map[uuid.UUID]migration.Source
}

func (s *Handler) InitWorkerCache(initial map[uuid.UUID]time.Time) error {
	return s.workerUpdateCache.Replace(initial)
}

// RecordWorkerUpdate caches the last worker update that the corresponding instance has received.
func (s *Handler) RecordWorkerUpdate(instanceUUID uuid.UUID) {
	s.workerUpdateCache.Write(instanceUUID, time.Now().UTC(), nil)
}

func (s *Handler) LastWorkerUpdate(instanceUUID uuid.UUID) time.Time {
	lastUpdate, _ := s.workerUpdateCache.Read(instanceUUID)
	return lastUpdate
}

// RemoveFromCache removes the given instanceUUID from the worker cache.
func (s *Handler) RemoveFromCache(instanceUUID uuid.UUID) {
	s.workerUpdateCache.Delete(instanceUUID)
}

// GetMigrationState fetches all migration state information corresponding to the given batch status and migration status.
func (s *Handler) GetMigrationState(ctx context.Context, batchStatus api.BatchStatusType, migrationStatuses ...api.MigrationStatusType) (map[string]MigrationState, error) {
	migrationState := map[string]MigrationState{}

	var entries migration.QueueEntries
	var batches migration.Batches
	var targets migration.Targets
	var sources migration.Sources
	var instances migration.Instances
	windowsByBatch := map[string]migration.MigrationWindows{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		entries, err = s.queue.GetAllByState(ctx, migrationStatuses...)
		if err != nil {
			return err
		}

		batches, err = s.batch.GetAllByState(ctx, batchStatus)
		if err != nil {
			return err
		}

		for _, b := range batches {
			windowsByBatch[b.Name], err = s.batch.GetMigrationWindows(ctx, b.Name)
			if err != nil {
				return err
			}
		}

		targets, err = s.target.GetAll(ctx)
		if err != nil {
			return err
		}

		sources, err = s.source.GetAll(ctx)
		if err != nil {
			return err
		}

		instances, err = s.instance.GetAllQueued(ctx, entries)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch records for migration state: %w", err)
	}

	queueMap := map[string]map[uuid.UUID]migration.QueueEntry{}
	for _, entry := range entries {
		if queueMap[entry.BatchName] == nil {
			queueMap[entry.BatchName] = map[uuid.UUID]migration.QueueEntry{}
		}

		queueMap[entry.BatchName][entry.InstanceUUID] = entry
	}

	batchMap := map[string]migration.Batch{}
	for _, b := range batches {
		_, ok := queueMap[b.Name]
		if ok {
			batchMap[b.Name] = b
		}
	}

	srcMap := make(map[string]migration.Source, len(sources))
	for _, s := range sources {
		srcMap[s.Name] = s
	}

	tgtMap := make(map[string]migration.Target, len(targets))
	for _, t := range targets {
		tgtMap[t.Name] = t
	}

	for _, b := range batchMap {
		state := MigrationState{
			Batch:            b,
			MigrationWindows: windowsByBatch[b.Name],
			QueueEntries:     queueMap[b.Name],
			Instances:        map[uuid.UUID]migration.Instance{},
			Sources:          map[uuid.UUID]migration.Source{},
			Targets:          map[uuid.UUID]migration.Target{},
		}

		for _, inst := range instances {
			q, ok := state.QueueEntries[inst.UUID]
			if ok {
				state.Instances[inst.UUID] = inst
				state.Sources[inst.UUID] = srcMap[inst.Source]

				tgt, ok := tgtMap[q.Placement.TargetName]
				if ok {
					state.Targets[inst.UUID] = tgt
				}
			}
		}

		migrationState[b.Name] = state
	}

	return migrationState, nil
}
