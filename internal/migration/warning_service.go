package migration

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type warningService struct {
	repo WarningRepo
}

var _ WarningService = &warningService{}

func NewWarningService(repo WarningRepo) warningService {
	return warningService{repo: repo}
}

// DeleteByUUID implements WarningService.
func (w warningService) DeleteByUUID(ctx context.Context, id uuid.UUID) error {
	return w.repo.DeleteByUUID(ctx, id)
}

// Emit records the given warning. If another warning of the same scope and type already exists,
// their messages and count will be merged, with new messages appearing at the end of the list.
func (w warningService) Emit(ctx context.Context, warning Warning) (Warning, error) {
	err := warning.Validate()
	if err != nil {
		return Warning{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		scope := api.WarningScope{Scope: warning.Scope, EntityType: warning.EntityType, Entity: warning.Entity}
		dbWarnings, err := w.repo.GetByScopeAndType(ctx, scope, warning.Type)
		if err != nil {
			return err
		}

		if len(dbWarnings) > 1 {
			return fmt.Errorf("Invalid warning state for scope %v", scope)
		}

		// If the warning already exists, re-use it and increment its count.
		now := time.Now().UTC()
		if len(dbWarnings) == 1 {
			dbWarning := dbWarnings[0]
			warning.UUID = dbWarning.UUID
			warning.Count += dbWarning.Count
			warning.FirstSeenDate = dbWarning.FirstSeenDate
			warning.LastSeenDate = dbWarning.UpdatedDate
			newMessages := []string{}
			for _, msg := range dbWarning.Messages {
				if !slices.Contains(warning.Messages, msg) {
					newMessages = append(newMessages, msg)
				}
			}

			// Append new messages at the end.
			newMessages = append(newMessages, warning.Messages...)
			warning.Messages = newMessages
		} else {
			warning.FirstSeenDate = now
			warning.LastSeenDate = now
		}

		warning.UpdatedDate = now
		warning.ID, err = w.repo.Upsert(ctx, warning)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return Warning{}, err
	}

	return warning, nil
}

// GetAll implements WarningService.
func (w warningService) GetAll(ctx context.Context) (Warnings, error) {
	return w.repo.GetAll(ctx)
}

// GetByScopeAndType implements WarningService.
func (w warningService) GetByScopeAndType(ctx context.Context, scope api.WarningScope, wType api.WarningType) (Warnings, error) {
	return w.repo.GetByScopeAndType(ctx, scope, wType)
}

// GetByUUID implements WarningService.
func (w warningService) GetByUUID(ctx context.Context, id uuid.UUID) (*Warning, error) {
	return w.repo.GetByUUID(ctx, id)
}

// RemoveStale prunes all warning messages in the scope which are different from the provided set.
// Only duplicates of the provided set, or warnings of other scopes will remain.
func (w warningService) RemoveStale(ctx context.Context, scope api.WarningScope, newWarnings Warnings) error {
	messagesByType := map[api.WarningType]map[string]bool{}
	for _, w := range newWarnings {
		if !scope.Match(w.ToAPI()) {
			continue
		}

		if messagesByType[w.Type] == nil {
			messagesByType[w.Type] = map[string]bool{}
		}

		for _, msg := range w.Messages {
			messagesByType[w.Type][msg] = true
		}
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		allWarnings, err := w.repo.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all warnings: %w", err)
		}

		for _, warning := range allWarnings {
			if !scope.Match(warning.ToAPI()) {
				continue
			}

			seenMessages := []string{}
			for _, msg := range warning.Messages {
				if messagesByType[warning.Type][msg] {
					seenMessages = append(seenMessages, msg)
				}
			}

			// All messages for this warning are stale, so delete it.
			if len(seenMessages) == 0 {
				err := w.repo.DeleteByUUID(ctx, warning.UUID)
				if err != nil {
					return fmt.Errorf("Failed to delete stale warning: %w", err)
				}
			} else if !slices.Equal(warning.Messages, seenMessages) {
				warning.Messages = seenMessages
				err := w.repo.Update(ctx, warning.UUID, warning)
				if err != nil {
					return fmt.Errorf("Failed to prune stale warning messages: %w", err)
				}
			}
		}

		return nil
	})
}

// Update implements WarningService.
func (w warningService) Update(ctx context.Context, id uuid.UUID, warning *Warning) error {
	return w.repo.Update(ctx, id, *warning)
}

// UpdateStatusByUUID implements WarningService.
func (w warningService) UpdateStatusByUUID(ctx context.Context, id uuid.UUID, status api.WarningStatus) (*Warning, error) {
	var warning Warning
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		dbWarning, err := w.repo.GetByUUID(ctx, id)
		if err != nil {
			return err
		}

		warning = *dbWarning

		warning.Status = status
		warning.LastSeenDate = dbWarning.UpdatedDate
		warning.UpdatedDate = time.Now().UTC()
		return w.repo.Update(ctx, warning.UUID, warning)
	})
	if err != nil {
		return nil, err
	}

	return &warning, nil
}
