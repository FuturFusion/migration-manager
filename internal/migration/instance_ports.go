package migration

import (
	"context"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out instance_service_mock_gen_test.go -rm . InstanceService

type InstanceService interface {
	Create(ctx context.Context, instance Instance) (Instance, error)
	GetAll(ctx context.Context) (Instances, error)
	GetAllByState(ctx context.Context, status api.MigrationStatusType) (Instances, error)
	GetAllByBatchID(ctx context.Context, batchID int) (Instances, error)
	GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error)
	GetAllUnassigned(ctx context.Context) (Instances, error)
	GetByID(ctx context.Context, id uuid.UUID) (Instance, error)
	GetByIDWithDetails(ctx context.Context, id uuid.UUID) (InstanceWithDetails, error)
	UnassignFromBatch(ctx context.Context, id uuid.UUID) error
	UpdateByID(ctx context.Context, instance Instance) (Instance, error)
	UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (Instance, error)
	ProcessWorkerUpdate(ctx context.Context, id uuid.UUID, workerResponseTypeArg api.WorkerResponseType, statusString string) (Instance, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// Overrride
	CreateOverrides(ctx context.Context, overrides Overrides) (Overrides, error)
	GetOverridesByID(ctx context.Context, id uuid.UUID) (Overrides, error)
	DeleteOverridesByID(ctx context.Context, id uuid.UUID) error
	UpdateOverridesByID(ctx context.Context, overrides Overrides) (Overrides, error)
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/instance_repo_mock_gen.go -rm . InstanceRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i InstanceRepo -t ../logger/slog.gotmpl -o ./repo/middleware/instance_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i InstanceRepo -t prometheus -o ./repo/middleware/instance_prometheus_gen.go

type InstanceRepo interface {
	Create(ctx context.Context, instance Instance) (Instance, error)
	GetAll(ctx context.Context) (Instances, error)
	GetAllByState(ctx context.Context, status api.MigrationStatusType) (Instances, error)
	GetAllByBatchID(ctx context.Context, batchID int) (Instances, error)
	GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error)
	GetAllUnassigned(ctx context.Context) (Instances, error)
	GetByID(ctx context.Context, id uuid.UUID) (Instance, error)
	UpdateByID(ctx context.Context, instance Instance) (Instance, error)
	UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.MigrationStatusType, statusString string, needsDiskImport bool) (Instance, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// Overrides
	CreateOverrides(ctx context.Context, overrides Overrides) (Overrides, error)
	GetOverridesByID(ctx context.Context, id uuid.UUID) (Overrides, error)
	DeleteOverridesByID(ctx context.Context, id uuid.UUID) error
	UpdateOverridesByID(ctx context.Context, overrides Overrides) (Overrides, error)
}
