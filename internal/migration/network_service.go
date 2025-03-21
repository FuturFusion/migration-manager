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

func (n networkService) GetAllNames(ctx context.Context) ([]string, error) {
	return n.repo.GetAllNames(ctx)
}

func (n networkService) GetByName(ctx context.Context, name string) (*Network, error) {
	if name == "" {
		return nil, fmt.Errorf("Network name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return n.repo.GetByName(ctx, name)
}

func (n networkService) Update(ctx context.Context, newNetwork Network) error {
	err := newNetwork.Validate()
	if err != nil {
		return err
	}

	return n.repo.Update(ctx, newNetwork)
}

func (n networkService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("Network name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return n.repo.DeleteByName(ctx, name)
}
