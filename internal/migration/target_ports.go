package migration

import "context"

type TargetService interface {
	Create(ctx context.Context, target Target) (Target, error)
	GetAll(ctx context.Context) (Targets, error)
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
	GetByID(ctx context.Context, id int) (Target, error)
	GetByName(ctx context.Context, name string) (Target, error)
	UpdateByName(ctx context.Context, target Target) (Target, error)
	DeleteByName(ctx context.Context, name string) error
}
