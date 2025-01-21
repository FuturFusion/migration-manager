package migration

import "context"

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out target_service_mock_gen_test.go -rm . TargetService

type TargetService interface {
	Create(ctx context.Context, target Target) (Target, error)
	GetAll(ctx context.Context) (Targets, error)
	GetAllNames(ctx context.Context) ([]string, error)
	GetByID(ctx context.Context, id int) (Target, error)
	GetByName(ctx context.Context, name string) (Target, error)
	UpdateByName(ctx context.Context, target Target) (Target, error)
	DeleteByName(ctx context.Context, name string) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/target_repo_mock_gen.go -rm . TargetRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i TargetRepo -t ../logger/slog.gotmpl -o ./repo/middleware/target_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i TargetRepo -t prometheus -o ./repo/middleware/target_prometheus_gen.go

type TargetRepo interface {
	Create(ctx context.Context, target Target) (Target, error)
	GetAll(ctx context.Context) (Targets, error)
	GetAllNames(ctx context.Context) ([]string, error)
	GetByID(ctx context.Context, id int) (Target, error)
	GetByName(ctx context.Context, name string) (Target, error)
	UpdateByName(ctx context.Context, target Target) (Target, error)
	DeleteByName(ctx context.Context, name string) error
}
