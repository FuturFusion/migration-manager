package migration

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/transaction"
)

type networkService struct {
	repo NetworkRepo
}

var _ NetworkService = &networkService{}

func NewNetworkService(repo NetworkRepo) networkService {
	return networkService{
		repo: repo,
	}
}

func (n networkService) Create(ctx context.Context, newNetwork Network) (Network, error) {
	err := newNetwork.Validate()
	if err != nil {
		return Network{}, err
	}

	newNetwork.ID, err = n.repo.Create(ctx, newNetwork)
	if err != nil {
		return Network{}, err
	}

	return newNetwork, nil
}

func (n networkService) GetAll(ctx context.Context) (Networks, error) {
	return n.repo.GetAll(ctx)
}

func (n networkService) GetAllBySource(ctx context.Context, srcName string) (Networks, error) {
	return n.repo.GetAllBySource(ctx, srcName)
}

func (n networkService) GetByNameAndSource(ctx context.Context, name string, srcName string) (*Network, error) {
	if name == "" {
		return nil, fmt.Errorf("Network name cannot be empty: %w", ErrOperationNotPermitted)
	}

	if srcName == "" {
		return nil, fmt.Errorf("Network source cannot be empty: %w", ErrOperationNotPermitted)
	}

	return n.repo.GetByNameAndSource(ctx, name, srcName)
}

func (n networkService) Update(ctx context.Context, newNetwork *Network) error {
	err := newNetwork.Validate()
	if err != nil {
		return err
	}

	return n.repo.Update(ctx, *newNetwork)
}

func (n networkService) DeleteByNameAndSource(ctx context.Context, name string, srcName string) error {
	if name == "" {
		return fmt.Errorf("Network name cannot be empty: %w", ErrOperationNotPermitted)
	}

	if srcName == "" {
		return fmt.Errorf("Network source cannot be empty: %w", ErrOperationNotPermitted)
	}

	return n.repo.DeleteByNameAndSource(ctx, name, srcName)
}

// DeleteByUUID implements NetworkService.
func (n networkService) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		network, err := n.repo.GetByUUID(ctx, id)
		if err != nil {
			return fmt.Errorf("Failed to get network by UUID %q: %w", id.String(), err)
		}

		return n.repo.DeleteByNameAndSource(ctx, network.SourceSpecificID, network.Source)
	})
}

// GetByUUID implements NetworkService.
func (n networkService) GetByUUID(ctx context.Context, id uuid.UUID) (*Network, error) {
	return n.repo.GetByUUID(ctx, id)
}
