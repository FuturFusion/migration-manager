package sqlite

import (
	"context"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/sqlite/entities"
	"github.com/FuturFusion/migration-manager/internal/transaction"
)

type network struct {
	db repo.DBTX
}

var _ migration.NetworkRepo = &network{}

func NewNetwork(db repo.DBTX) *network {
	return &network{
		db: db,
	}
}

func (n network) Create(ctx context.Context, in migration.Network) (int64, error) {
	return entities.CreateNetwork(ctx, transaction.GetDBTX(ctx, n.db), in)
}

func (n network) GetAll(ctx context.Context) (migration.Networks, error) {
	return entities.GetNetworks(ctx, transaction.GetDBTX(ctx, n.db))
}

func (n network) GetAllBySource(ctx context.Context, srcName string) (migration.Networks, error) {
	return entities.GetNetworks(ctx, transaction.GetDBTX(ctx, n.db), entities.NetworkFilter{Source: &srcName})
}

func (n network) GetByNameAndSource(ctx context.Context, name string, srcName string) (*migration.Network, error) {
	return entities.GetNetwork(ctx, transaction.GetDBTX(ctx, n.db), name, srcName)
}

func (n network) Update(ctx context.Context, in migration.Network) error {
	return transaction.ForceTx(ctx, transaction.GetDBTX(ctx, n.db), func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateNetwork(ctx, tx, in.Identifier, in.Source, in)
	})
}

func (n network) DeleteByNameAndSource(ctx context.Context, name string, srcName string) error {
	return entities.DeleteNetwork(ctx, transaction.GetDBTX(ctx, n.db), name, srcName)
}
