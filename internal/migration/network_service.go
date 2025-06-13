package migration

import (
	"context"
	"fmt"
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
