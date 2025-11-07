package migration

import (
	"context"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal/transaction"
)

type windowService struct {
	repo WindowRepo
}

func NewWindowService(repo WindowRepo) WindowService {
	return windowService{repo: repo}
}

// Create implements WindowService.
func (s windowService) Create(ctx context.Context, window Window) (Window, error) {
	err := window.Validate()
	if err != nil {
		return Window{}, err
	}

	window.ID, err = s.repo.Create(ctx, window)
	if err != nil {
		return Window{}, err
	}

	return window, nil
}

// DeleteByNameAndBatch implements WindowService.
func (s windowService) DeleteByNameAndBatch(ctx context.Context, queueSvc QueueService, name string, batchName string) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		qs, err := queueSvc.GetAllByBatch(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get existing migration windows for batch %q: %w", batchName, err)
		}

		for _, q := range qs {
			windowName := q.GetWindowName()
			if windowName == nil {
				continue
			}

			if *windowName == name {
				return fmt.Errorf("Cannot delete migration window %q because it is assigned to queue entry %q", name, q.InstanceUUID.String())
			}
		}

		return s.repo.DeleteByNameAndBatch(ctx, name, batchName)
	})
}

// GetAll implements WindowService.
func (s windowService) GetAll(ctx context.Context) (Windows, error) {
	return s.repo.GetAll(ctx)
}

// GetAllByBatch implements WindowService.
func (s windowService) GetAllByBatch(ctx context.Context, batchName string) (Windows, error) {
	return s.repo.GetAllByBatch(ctx, batchName)
}

// GetByNameAndBatch implements WindowService.
func (s windowService) GetByNameAndBatch(ctx context.Context, name string, batchName string) (*Window, error) {
	return s.repo.GetByNameAndBatch(ctx, name, batchName)
}

// ReplaceByBatch implements WindowService.
func (s windowService) ReplaceByBatch(ctx context.Context, queueSvc QueueService, batchName string, windows Windows) error {
	newWindowsByName := map[string]Window{}
	for _, w := range windows {
		err := w.Validate()
		if err != nil {
			return err
		}

		newWindowsByName[w.Name] = w
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		oldWindows, err := s.repo.GetAllByBatch(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get existing migration windows for batch %q: %w", batchName, err)
		}

		oldWindowsByName := map[string]Window{}
		for _, w := range oldWindows {
			oldWindowsByName[w.Name] = w
		}

		qs, err := queueSvc.GetAllByBatch(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get queue entries for batch %q: %w", batchName, err)
		}

		// Validate that the window is not restricted by assignment to queue entries.
		for _, q := range qs {
			windowName := q.GetWindowName()
			if windowName == nil {
				continue
			}

			oldWindow, ok := oldWindowsByName[*windowName]
			if !ok {
				continue
			}

			newWindow, ok := newWindowsByName[*windowName]
			if !ok {
				return fmt.Errorf("Window %q cannot be removed or renamed after assignment to queue entry %q", oldWindow.Name, q.InstanceUUID)
			}

			if newWindow.Start.Sub(oldWindow.Start) > 0 {
				return fmt.Errorf("Window %q start time cannot be increased because it is assigned to queue entry %q", newWindow.Name, q.InstanceUUID)
			}

			if !newWindow.End.IsZero() && newWindow.End.Sub(oldWindow.End) < 0 {
				return fmt.Errorf("Window %q end time cannot be reduced because it is assigned to queue entry %q", newWindow.Name, q.InstanceUUID)
			}

			if newWindow.Config.Capacity != 0 && (newWindow.Config.Capacity < oldWindow.Config.Capacity || oldWindow.Config.Capacity == 0) {
				return fmt.Errorf("Window %q capacity must be cannot be reduced after assignment to queue entry %q", oldWindow.Name, q.InstanceUUID)
			}
		}

		// Update and prune old windows.
		for _, oldWindow := range oldWindows {
			newWindow, ok := newWindowsByName[oldWindow.Name]
			if !ok {
				err = s.repo.DeleteByNameAndBatch(ctx, oldWindow.Name, oldWindow.Batch)
				if err != nil {
					return fmt.Errorf("Failed to delete migration window %q from batch %q: %w", oldWindow.Name, oldWindow.Batch, err)
				}
			} else {
				newWindow.Batch = batchName
				// The new window won't have an ID if it came from the API, so unset it before comparison.
				oldWindow.ID = newWindow.ID
				if oldWindow != newWindow {
					err = s.repo.Update(ctx, newWindow)
					if err != nil {
						return fmt.Errorf("Failed to update migration window %q in batch %q: %w", newWindow.Name, newWindow.Batch, err)
					}
				}
			}
		}

		// Create new windows.
		for _, newWindow := range newWindowsByName {
			_, ok := oldWindowsByName[newWindow.Name]
			if !ok {
				newWindow.Batch = batchName
				_, err = s.repo.Create(ctx, newWindow)
				if err != nil {
					return fmt.Errorf("Failed to create migration window %q in batch %q: %w", newWindow.Name, newWindow.Batch, err)
				}
			}
		}

		return nil
	})
}

// Update implements WindowService.
func (s windowService) Update(ctx context.Context, window *Window) error {
	err := window.Validate()
	if err != nil {
		return err
	}

	return s.repo.Update(ctx, *window)
}
