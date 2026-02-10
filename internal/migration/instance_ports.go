package migration

import (
	"context"

	"github.com/google/uuid"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out instance_service_mock_gen_test.go -rm . InstanceService

type InstanceService interface {
	Create(ctx context.Context, instance Instance) (Instance, error)
	GetAll(ctx context.Context) (Instances, error)
	GetAllByBatch(ctx context.Context, batch string) (Instances, error)
	GetAllBySource(ctx context.Context, source string) (Instances, error)
	GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error)
	GetAllUUIDsBySource(ctx context.Context, source string) ([]uuid.UUID, error)
	GetAllAssigned(ctx context.Context) (Instances, error)
	GetAllUnassigned(ctx context.Context) (Instances, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*Instance, error)

	GetAllQueued(ctx context.Context, queue QueueEntries) (Instances, error)
	GetBatchesByUUID(ctx context.Context, id uuid.UUID) (Batches, error)

	Update(ctx context.Context, instance *Instance) error
	ResetBackgroundImport(ctx context.Context, instance *Instance) error
	DeleteByUUID(ctx context.Context, id uuid.UUID) error

	RemoveFromQueue(ctx context.Context, id uuid.UUID) error

	GetPostMigrationRetries(id uuid.UUID) int
	RecordPostMigrationRetry(id uuid.UUID)
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/instance_repo_mock_gen.go -rm . InstanceRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i InstanceRepo -t ../logger/slog.gotmpl -o ./repo/middleware/instance_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i InstanceRepo -t prometheus -o ./repo/middleware/instance_prometheus_gen.go

type InstanceRepo interface {
	Create(ctx context.Context, instance Instance) (int64, error)
	GetAll(ctx context.Context) (Instances, error)
	GetAllByBatch(ctx context.Context, batch string) (Instances, error)
	GetAllBySource(ctx context.Context, source string) (Instances, error)
	GetAllByUUIDs(ctx context.Context, id ...uuid.UUID) (Instances, error)
	GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error)
	GetAllUUIDsBySource(ctx context.Context, source string) ([]uuid.UUID, error)
	GetAllAssigned(ctx context.Context) (Instances, error)
	GetAllUnassigned(ctx context.Context) (Instances, error)
	GetBatchesByUUID(ctx context.Context, instanceUUID uuid.UUID) (Batches, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*Instance, error)
	Update(ctx context.Context, instance Instance) error
	DeleteByUUID(ctx context.Context, id uuid.UUID) error

	RemoveFromQueue(ctx context.Context, id uuid.UUID) error
}
