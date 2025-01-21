package migration

import (
	"context"
	"errors"
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
		return Source{}, errors.New("Source name cannot be empty")
	}

	return s.repo.GetByName(ctx, name)
}

func (s sourceService) UpdateByName(ctx context.Context, newSource Source) (Source, error) {
	err := newSource.Validate()
	if err != nil {
		return Source{}, err
	}

	return s.repo.UpdateByName(ctx, newSource)
}

func (s sourceService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("Source name cannot be empty")
	}

	return s.repo.DeleteByName(ctx, name)
}
