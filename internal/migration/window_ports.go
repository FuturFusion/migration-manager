package migration

import (
	"context"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out window_service_mock_gen_test.go -rm . WindowService

type WindowService interface {
	Create(ctx context.Context, window Window) (Window, error)
	GetByNameAndBatch(ctx context.Context, name string, batchName string) (*Window, error)
	GetAll(ctx context.Context) (Windows, error)
	GetAllByBatch(ctx context.Context, batchName string) (Windows, error)
	Update(ctx context.Context, window *Window) error
	ReplaceByBatch(ctx context.Context, queueSvc QueueService, batchName string, windows Windows) error
	DeleteByNameAndBatch(ctx context.Context, queueSvc QueueService, name string, batchName string) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/window_repo_mock_gen.go -rm . WindowRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i WindowRepo -t ../logger/slog.gotmpl -o ./repo/middleware/window_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i WindowRepo -t prometheus -o ./repo/middleware/window_prometheus_gen.go

type WindowRepo interface {
	Create(ctx context.Context, window Window) (int64, error)
	GetByNameAndBatch(ctx context.Context, name string, batchName string) (*Window, error)
	GetAll(ctx context.Context) (Windows, error)
	GetAllByBatch(ctx context.Context, batchName string) (Windows, error)
	Update(ctx context.Context, window Window) error
	DeleteByNameAndBatch(ctx context.Context, name string, batchName string) error
}
