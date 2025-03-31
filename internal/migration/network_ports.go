package migration

import "context"

//go:generate go run github.com/matryer/moq -fmt goimports -pkg migration_test -out network_service_mock_gen_test.go -rm . NetworkService

type NetworkService interface {
	Create(ctx context.Context, network Network) (Network, error)
	GetAll(ctx context.Context) (Networks, error)
	GetAllNames(ctx context.Context) ([]string, error)
	GetByName(ctx context.Context, name string) (*Network, error)
	Update(ctx context.Context, network *Network) error
	DeleteByName(ctx context.Context, name string) error
}

//go:generate go run github.com/matryer/moq -fmt goimports -pkg mock -out repo/mock/network_repo_mock_gen.go -rm . NetworkRepo
//go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i NetworkRepo -t ../logger/slog.gotmpl -o ./repo/middleware/network_slog_gen.go
// disabled go:generate go run github.com/hexdigest/gowrap/cmd/gowrap gen -g -i NetworkRepo -t prometheus -o ./repo/middleware/network_prometheus_gen.go

type NetworkRepo interface {
	Create(ctx context.Context, network Network) (int64, error)
	GetAll(ctx context.Context) (Networks, error)
	GetAllNames(ctx context.Context) ([]string, error)
	GetByName(ctx context.Context, name string) (*Network, error)
	Update(ctx context.Context, network Network) error
	Rename(ctx context.Context, oldName string, newName string) error
	DeleteByName(ctx context.Context, name string) error
}
