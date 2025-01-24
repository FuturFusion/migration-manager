package migration

import (
	"context"
	"errors"
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

	return n.repo.Create(ctx, newNetwork)
}

func (n networkService) GetAll(ctx context.Context) (Networks, error) {
	return n.repo.GetAll(ctx)
}

func (n networkService) GetAllNames(ctx context.Context) ([]string, error) {
	return n.repo.GetAllNames(ctx)
}

func (n networkService) GetByID(ctx context.Context, id int) (Network, error) {
	return n.repo.GetByID(ctx, id)
}

func (n networkService) GetByName(ctx context.Context, name string) (Network, error) {
	if name == "" {
		return Network{}, errors.New("Network name cannot be empty")
	}

	return n.repo.GetByName(ctx, name)
}

func (n networkService) UpdateByID(ctx context.Context, newNetwork Network) (Network, error) {
	err := newNetwork.Validate()
	if err != nil {
		return Network{}, err
	}

	return n.repo.UpdateByID(ctx, newNetwork)
}

func (n networkService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("Network name cannot be empty")
	}

	return n.repo.DeleteByName(ctx, name)
}
