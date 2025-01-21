package migration

import (
	"context"
	"errors"
)

type targetService struct {
	repo TargetRepo
}

var _ TargetService = &targetService{}

func NewTargetService(repo TargetRepo) targetService {
	return targetService{
		repo: repo,
	}
}

func (s targetService) Create(ctx context.Context, newTarget Target) (Target, error) {
	err := newTarget.Validate()
	if err != nil {
		return Target{}, err
	}

	return s.repo.Create(ctx, newTarget)
}

func (s targetService) GetAll(ctx context.Context) (Targets, error) {
	return s.repo.GetAll(ctx)
}

func (s targetService) GetByID(ctx context.Context, id int) (Target, error) {
	return s.repo.GetByID(ctx, id)
}

func (s targetService) GetByName(ctx context.Context, name string) (Target, error) {
	if name == "" {
		return Target{}, errors.New("Target name cannot be empty")
	}

	return s.repo.GetByName(ctx, name)
}

func (s targetService) UpdateByName(ctx context.Context, newTarget Target) (Target, error) {
	err := newTarget.Validate()
	if err != nil {
		return Target{}, err
	}

	return s.repo.UpdateByName(ctx, newTarget)
}

func (s targetService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("Instance name cannot be empty")
	}

	return s.repo.DeleteByName(ctx, name)
}
