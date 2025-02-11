package migration

import (
	"context"

	"github.com/google/uuid"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out queue_service_mock_gen_test.go -rm . QueueService

type QueueService interface {
	GetAll(ctx context.Context) (QueueEntries, error)
	GetByInstanceID(ctx context.Context, id uuid.UUID) (QueueEntry, error)
	GetWorkerCommandByInstanceID(ctx context.Context, id uuid.UUID) (WorkerCommand, error)
}
