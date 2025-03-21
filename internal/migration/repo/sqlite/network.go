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
	return entities.CreateNetwork(ctx, n.db, in)
}

func (n network) GetAll(ctx context.Context) (migration.Networks, error) {
	return entities.GetNetworks(ctx, n.db)
}

func (n network) GetAllNames(ctx context.Context) ([]string, error) {
	return entities.GetNetworkNames(ctx, n.db)
}

func (n network) GetByName(ctx context.Context, name string) (*migration.Network, error) {
	return entities.GetNetwork(ctx, n.db, name)
}

func (n network) Update(ctx context.Context, in migration.Network) error {
	return transaction.ForceTx(ctx, n.db, func(ctx context.Context, tx transaction.TX) error {
		return entities.UpdateNetwork(ctx, tx, in.Name, in)
	})
}

func (n network) Rename(ctx context.Context, oldName string, newName string) error {
	return entities.RenameNetwork(ctx, n.db, oldName, newName)
}

func (n network) DeleteByName(ctx context.Context, name string) error {
	return entities.DeleteNetwork(ctx, n.db, name)
}
