package migration

import (
	"context"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out warning_service_mock_gen_test.go -rm . WarningService

type WarningService interface {
	GetAll(ctx context.Context) (Warnings, error)
	GetByScopeAndType(ctx context.Context, scope api.WarningScope, wType api.WarningType) (Warnings, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*Warning, error)
	Update(ctx context.Context, id uuid.UUID, w *Warning) error
	UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.WarningStatus) (*Warning, error)
	DeleteByUUID(ctx context.Context, id uuid.UUID) error

	Emit(ctx context.Context, w Warning) (Warning, error)
	RemoveStale(ctx context.Context, scope api.WarningScope, newWarnings Warnings) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/warning_repo_mock_gen.go -rm . WarningRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i WarningRepo -t ../logger/slog.gotmpl -o ./repo/middleware/warning_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i WarningRepo -t prometheus -o ./repo/middleware/warning_prometheus_gen.go

type WarningRepo interface {
	Upsert(ctx context.Context, w Warning) (int64, error)
	GetAll(ctx context.Context) (Warnings, error)
	GetByScopeAndType(ctx context.Context, scope api.WarningScope, wType api.WarningType) (Warnings, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*Warning, error)
	Update(ctx context.Context, id uuid.UUID, w Warning) error
	DeleteByUUID(ctx context.Context, id uuid.UUID) error
}
