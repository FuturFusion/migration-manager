package migration

import (
	"context"
	"fmt"
)

type sourceService struct {
	repo SourceRepo
}

var _ SourceService = &sourceService{}

func NewSourceService(repo SourceRepo) sourceService {
	return sourceService{
		repo: repo,
	}
}

func (s sourceService) Create(ctx context.Context, newSource Source) (Source, error) {
	err := newSource.Validate()
	if err != nil {
		return Source{}, err
	}

	return s.repo.Create(ctx, newSource)
}

func (s sourceService) GetAll(ctx context.Context) (Sources, error) {
	return s.repo.GetAll(ctx)
}

func (s sourceService) GetAllNames(ctx context.Context) ([]string, error) {
	return s.repo.GetAllNames(ctx)
}

func (s sourceService) GetByID(ctx context.Context, id int) (Source, error) {
	return s.repo.GetByID(ctx, id)
}

func (s sourceService) GetByName(ctx context.Context, name string) (Source, error) {
	if name == "" {
		return Source{}, fmt.Errorf("Source name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.GetByName(ctx, name)
}

func (s sourceService) UpdateByID(ctx context.Context, newSource Source) (Source, error) {
	err := newSource.Validate()
	if err != nil {
		return Source{}, err
	}

	return s.repo.UpdateByID(ctx, newSource)
}

func (s sourceService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("Source name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.DeleteByName(ctx, name)
}
