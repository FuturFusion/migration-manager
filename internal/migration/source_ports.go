package migration

import "context"

type SourceService interface {
	Create(ctx context.Context, source Source) (Source, error)
	GetAll(ctx context.Context) (Sources, error)
	GetByID(ctx context.Context, id int) (Source, error)
	GetByName(ctx context.Context, name string) (Source, error)
	UpdateByName(ctx context.Context, source Source) (Source, error)
	DeleteByName(ctx context.Context, name string) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/source_repo_mock_gen.go -rm . SourceRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i SourceRepo -t ../logger/slog.gotmpl -o ./repo/middleware/source_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i SourceRepo -t prometheus -o ./repo/middleware/source_prometheus_gen.go

type SourceRepo interface {
	Create(ctx context.Context, source Source) (Source, error)
	GetAll(ctx context.Context) (Sources, error)
	GetByID(ctx context.Context, id int) (Source, error)
	GetByName(ctx context.Context, name string) (Source, error)
	UpdateByName(ctx context.Context, source Source) (Source, error)
	DeleteByName(ctx context.Context, name string) error
}
