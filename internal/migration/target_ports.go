package migration

import (
	"context"
	"crypto/x509"

	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out target_service_mock_gen_test.go -rm . TargetService

type TargetService interface {
	Create(ctx context.Context, target Target) (Target, error)
	GetAll(ctx context.Context) (Targets, error)
	GetAllNames(ctx context.Context) ([]string, error)
	GetByName(ctx context.Context, name string) (*Target, error)
	Update(ctx context.Context, target *Target) error
	DeleteByName(ctx context.Context, name string) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/target_repo_mock_gen.go -rm . TargetRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i TargetRepo -t ../logger/slog.gotmpl -o ./repo/middleware/target_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i TargetRepo -t prometheus -o ./repo/middleware/target_prometheus_gen.go

type TargetRepo interface {
	Create(ctx context.Context, target Target) (int64, error)
	GetAll(ctx context.Context) (Targets, error)
	GetAllNames(ctx context.Context) ([]string, error)
	GetByName(ctx context.Context, name string) (*Target, error)
	Update(ctx context.Context, target Target) error
	Rename(ctx context.Context, oldName string, newName string) error
	DeleteByName(ctx context.Context, name string) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out endpoint/mock/target_endpoint_mock_gen.go -rm . TargetEndpoint
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i TargetEndpoint -t ../logger/slog.gotmpl -o ./endpoint/middleware/target_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i TargetEndpoint -t prometheus -o ./endpoint/middleware/target_prometheus_gen.go

type TargetEndpoint interface {
	Connect(ctx context.Context) error
	DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate)
	IsWaitingForOIDCTokens() bool
}
