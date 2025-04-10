package migration

import (
	"context"
	"crypto/x509"

	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out source_service_mock_gen_test.go -rm . SourceService

type SourceService interface {
	Create(ctx context.Context, source Source) (Source, error)
	GetAll(ctx context.Context) (Sources, error)
	GetAllNames(ctx context.Context) ([]string, error)
	GetByName(ctx context.Context, name string) (*Source, error)
	Update(ctx context.Context, name string, source *Source, instanceService InstanceService) error
	DeleteByName(ctx context.Context, name string, instanceService InstanceService) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/source_repo_mock_gen.go -rm . SourceRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i SourceRepo -t ../logger/slog.gotmpl -o ./repo/middleware/source_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i SourceRepo -t prometheus -o ./repo/middleware/source_prometheus_gen.go

type SourceRepo interface {
	Create(ctx context.Context, source Source) (int64, error)
	GetAll(ctx context.Context) (Sources, error)
	GetAllNames(ctx context.Context) ([]string, error)
	GetByName(ctx context.Context, name string) (*Source, error)
	Update(ctx context.Context, name string, source Source) error
	Rename(ctx context.Context, oldName string, newName string) error
	DeleteByName(ctx context.Context, name string) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out endpoint/mock/source_endpoint_mock_gen.go -rm . SourceEndpoint
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i SourceEndpoint -t ../logger/slog.gotmpl -o ./endpoint/middleware/source_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i SourceEndpoint -t prometheus -o ./endpoint/middleware/source_prometheus_gen.go

type SourceEndpoint interface {
	Connect(ctx context.Context) error
	DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate)
}
