package migration

import (
	"context"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out instance_service_mock_gen_test.go -rm . InstanceService

type InstanceService interface {
	Create(ctx context.Context, instance Instance) (Instance, error)
	GetAll(ctx context.Context, withOverrides bool) (Instances, error)
	GetAllByState(ctx context.Context, status api.MigrationStatusType, withOverrides bool) (Instances, error)
	GetAllByBatch(ctx context.Context, batch string, withOverrides bool) (Instances, error)
	GetAllByBatchAndState(ctx context.Context, batch string, status api.MigrationStatusType, withOverrides bool) (Instances, error)
	GetAllBySource(ctx context.Context, source string, withOverrides bool) (Instances, error)
	GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error)
	GetAllUnassigned(ctx context.Context, withOverrides bool) (Instances, error)
	GetByUUID(ctx context.Context, id uuid.UUID, withOverrides bool) (*Instance, error)

	UnassignFromBatch(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, instance *Instance) error
	UpdateStatusByUUID(ctx context.Context, i uuid.UUID, status api.MigrationStatusType, statusMessage string, needsDiskImport bool) (*Instance, error)
	ProcessWorkerUpdate(ctx context.Context, id uuid.UUID, workerResponseTypeArg api.WorkerResponseType, statusMessage string) (Instance, error)
	DeleteByUUID(ctx context.Context, id uuid.UUID) error

	// Overrride
	CreateOverrides(ctx context.Context, overrides InstanceOverride) (InstanceOverride, error)
	GetOverridesByUUID(ctx context.Context, id uuid.UUID) (*InstanceOverride, error)
	DeleteOverridesByUUID(ctx context.Context, id uuid.UUID) error
	UpdateOverrides(ctx context.Context, overrides *InstanceOverride) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/instance_repo_mock_gen.go -rm . InstanceRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i InstanceRepo -t ../logger/slog.gotmpl -o ./repo/middleware/instance_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i InstanceRepo -t prometheus -o ./repo/middleware/instance_prometheus_gen.go

type InstanceRepo interface {
	Create(ctx context.Context, instance Instance) (int64, error)
	GetAll(ctx context.Context) (Instances, error)
	GetAllByState(ctx context.Context, status api.MigrationStatusType) (Instances, error)
	GetAllByBatch(ctx context.Context, batch string) (Instances, error)
	GetAllByBatchAndState(ctx context.Context, batch string, status api.MigrationStatusType) (Instances, error)
	GetAllBySource(ctx context.Context, source string) (Instances, error)
	GetAllUUIDs(ctx context.Context) ([]uuid.UUID, error)
	GetAllUnassigned(ctx context.Context) (Instances, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*Instance, error)
	Update(ctx context.Context, instance Instance) error
	DeleteByUUID(ctx context.Context, id uuid.UUID) error

	// Overrides
	CreateOverrides(ctx context.Context, overrides InstanceOverride) (int64, error)
	GetOverridesByUUID(ctx context.Context, id uuid.UUID) (*InstanceOverride, error)
	DeleteOverridesByUUID(ctx context.Context, id uuid.UUID) error
	UpdateOverrides(ctx context.Context, overrides InstanceOverride) error
}
