package migration

import (
	"context"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out queue_service_mock_gen_test.go -rm . QueueService

type QueueService interface {
	CreateEntry(ctx context.Context, queue QueueEntry) (QueueEntry, error)
	GetAll(ctx context.Context) (QueueEntries, error)
	GetAllByState(ctx context.Context, status ...api.MigrationStatusType) (QueueEntries, error)
	GetAllByBatch(ctx context.Context, batch string) (QueueEntries, error)
	GetAllByBatchAndState(ctx context.Context, batch string, status api.MigrationStatusType) (QueueEntries, error)
	GetAllNeedingImport(ctx context.Context, batch string, needsDiskImport bool) (QueueEntries, error)
	GetByInstanceUUID(ctx context.Context, id uuid.UUID) (*QueueEntry, error)
	Update(ctx context.Context, entry *QueueEntry) error
	DeleteByUUID(ctx context.Context, id uuid.UUID) error
	DeleteAllByBatch(ctx context.Context, batch string) error

	UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusMessage string, needsDiskImport bool) (*QueueEntry, error)

	NewWorkerCommandByInstanceUUID(ctx context.Context, id uuid.UUID) (WorkerCommand, error)
	ProcessWorkerUpdate(ctx context.Context, id uuid.UUID, workerResponseTypeArg api.WorkerResponseType, statusMessage string) (QueueEntry, error)
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/queue_repo_mock_gen.go -rm . QueueRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i QueueRepo -t ../logger/slog.gotmpl -o ./repo/middleware/queue_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i QueueRepo -t prometheus -o ./repo/middleware/queue_prometheus_gen.go

type QueueRepo interface {
	Create(ctx context.Context, queue QueueEntry) (int64, error)
	GetAll(ctx context.Context) (QueueEntries, error)
	GetAllByState(ctx context.Context, status ...api.MigrationStatusType) (QueueEntries, error)
	GetAllByBatch(ctx context.Context, batch string) (QueueEntries, error)
	GetAllByBatchAndState(ctx context.Context, batch string, status api.MigrationStatusType) (QueueEntries, error)
	GetAllNeedingImport(ctx context.Context, batch string, needsDiskImport bool) (QueueEntries, error)
	GetByInstanceUUID(ctx context.Context, id uuid.UUID) (*QueueEntry, error)
	Update(ctx context.Context, entry QueueEntry) error
	DeleteByUUID(ctx context.Context, id uuid.UUID) error
	DeleteAllByBatch(ctx context.Context, batch string) error
}
