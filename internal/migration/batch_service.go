package migration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	incusScriptlet "github.com/lxc/incus/v6/shared/scriptlet"

	"github.com/FuturFusion/migration-manager/internal/scriptlet"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type batchService struct {
	repo     BatchRepo
	instance InstanceService

	scriptletLoader *incusScriptlet.Loader
}

var _ BatchService = &batchService{}

func NewBatchService(repo BatchRepo, instance InstanceService) batchService {
	return batchService{
		repo:     repo,
		instance: instance,

		scriptletLoader: incusScriptlet.NewLoader(),
	}
}

func (s batchService) Create(ctx context.Context, batch Batch) (Batch, error) {
	err := batch.Validate()
	if err != nil {
		return Batch{}, err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		var err error

		batch.ID, err = s.repo.Create(ctx, batch)
		if err != nil {
			return fmt.Errorf("Failed to create batch: %w", err)
		}

		return s.UpdateInstancesAssignedToBatch(ctx, batch)
	})
	if err != nil {
		return Batch{}, err
	}

	if batch.Config.PlacementScriptlet != "" {
		err := scriptlet.BatchPlacementSet(s.scriptletLoader, batch.Config.PlacementScriptlet, batch.Name)
		if err != nil {
			return Batch{}, err
		}
	}

	return batch, nil
}

func (s batchService) GetAll(ctx context.Context) (Batches, error) {
	return s.repo.GetAll(ctx)
}

func (s batchService) GetAllByState(ctx context.Context, status api.BatchStatusType) (Batches, error) {
	return s.repo.GetAllByState(ctx, status)
}

func (s batchService) GetAllNames(ctx context.Context) ([]string, error) {
	return s.repo.GetAllNames(ctx)
}

func (s batchService) GetAllNamesByState(ctx context.Context, status api.BatchStatusType) ([]string, error) {
	return s.repo.GetAllNamesByState(ctx, status)
}

func (s batchService) GetByName(ctx context.Context, name string) (*Batch, error) {
	if name == "" {
		return nil, fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return s.repo.GetByName(ctx, name)
}

// canUpdateRunningBatch returns an error if the modified batch cannot be committed because the batch is already running.
// - Placement and instance filtering cannot be modified for a running batch.
// - Constraints that match to queue entries that have already entered final import cannot be added or removed.
func (s batchService) canUpdateRunningBatch(ctx context.Context, queueSvc QueueService, newBatch Batch, oldBatch Batch) error {
	if oldBatch.Status != api.BATCHSTATUS_RUNNING {
		return nil
	}

	// If the constraints changed, keep a list of all old and new constraints to check if any matching queue entry is already committed.
	var constraintsToCheck []BatchConstraint
	if !slices.Equal(oldBatch.Constraints, newBatch.Constraints) {
		constraintsToCheck = oldBatch.Constraints
		for _, c := range newBatch.Constraints {
			if !slices.Contains(constraintsToCheck, c) {
				constraintsToCheck = append(constraintsToCheck, c)
			}
		}
	}

	if len(constraintsToCheck) > 0 {
		instances, err := s.instance.GetAllByBatch(ctx, oldBatch.Name)
		if err != nil {
			return fmt.Errorf("Failed to get instances for batch %q: %w", oldBatch.Name, err)
		}

		queueEntries, err := queueSvc.GetAllByBatch(ctx, oldBatch.Name)
		if err != nil {
			return fmt.Errorf("Failed to get queue entries for batch %q: %w", oldBatch.Name, err)
		}

		queueMap := make(map[uuid.UUID]QueueEntry, len(queueEntries))
		for _, q := range queueEntries {
			queueMap[q.InstanceUUID] = q
		}

		for i, c := range constraintsToCheck {
			// If the constraint at this index hasn't changed, then we don't need to check it.
			if i < len(oldBatch.Constraints) && oldBatch.Constraints[i] == c {
				continue
			}

			for _, inst := range instances {
				match, err := inst.MatchesCriteria(c.IncludeExpression)
				if err != nil {
					return fmt.Errorf("Failed to check constraint %q against instance %q: %w", c.IncludeExpression, inst.Properties.Location, err)
				}

				if match {
					q, ok := queueMap[inst.UUID]
					if !ok {
						continue
					}

					if q.IsCommitted() {
						return fmt.Errorf("Matching constraint %q cannot be modified for committed queue entry with status %q: %w", c.IncludeExpression, q.MigrationStatus, ErrOperationNotPermitted)
					}
				}
			}
		}
	}

	if oldBatch.Name != newBatch.Name {
		return fmt.Errorf("Cannot rename running batch %q: %w", oldBatch.Name, ErrOperationNotPermitted)
	}

	if oldBatch.Defaults.Placement.StoragePool != newBatch.Defaults.Placement.StoragePool ||
		oldBatch.Defaults.Placement.Target != newBatch.Defaults.Placement.Target ||
		oldBatch.Defaults.Placement.TargetProject != newBatch.Defaults.Placement.TargetProject ||
		oldBatch.Config.PlacementScriptlet != newBatch.Config.PlacementScriptlet ||
		!slices.Equal(oldBatch.Defaults.MigrationNetwork, newBatch.Defaults.MigrationNetwork) {
		return fmt.Errorf("Cannot modify placement of running batch %q: %w", oldBatch.Name, ErrOperationNotPermitted)
	}

	if oldBatch.IncludeExpression != newBatch.IncludeExpression {
		return fmt.Errorf("Cannot modify include expression of running batch %q: %w", oldBatch.Name, ErrOperationNotPermitted)
	}

	return nil
}

func (s batchService) Update(ctx context.Context, queueSvc QueueService, name string, batch *Batch) error {
	// Reset batch state in testing mode.
	if util.InTestingMode() {
		batch.Status = api.BATCHSTATUS_DEFINED
		batch.StatusMessage = string(api.BATCHSTATUS_DEFINED)
		batch.StartDate = time.Time{}
	}

	err := batch.Validate()
	if err != nil {
		return err
	}

	var updateScriptlet bool
	err = transaction.Do(ctx, func(ctx context.Context) error {
		oldBatch, err := s.repo.GetByName(ctx, name)
		if err != nil {
			return err
		}

		if oldBatch.Status == api.BATCHSTATUS_RUNNING && !util.InTestingMode() {
			err := s.canUpdateRunningBatch(ctx, queueSvc, *batch, *oldBatch)
			if err != nil {
				return err
			}
		}

		err = s.repo.Update(ctx, name, *batch)
		if err != nil {
			return err
		}

		// Only modify instances and placement if the batch is not running.
		if oldBatch.Status != api.BATCHSTATUS_RUNNING || util.InTestingMode() {
			if oldBatch.Config.PlacementScriptlet != batch.Config.PlacementScriptlet && batch.Config.PlacementScriptlet != "" {
				updateScriptlet = true
			}

			return s.UpdateInstancesAssignedToBatch(ctx, *batch)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if updateScriptlet {
		err := scriptlet.BatchPlacementSet(s.scriptletLoader, batch.Config.PlacementScriptlet, batch.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s batchService) UpdateStatusByName(ctx context.Context, name string, status api.BatchStatusType, statusMessage string) (*Batch, error) {
	var batch *Batch
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		batch, err = s.repo.GetByName(ctx, name)
		if err != nil {
			return err
		}

		batch.Status = status
		batch.StatusMessage = statusMessage

		return s.repo.Update(ctx, batch.Name, *batch)
	})
	if err != nil {
		return nil, err
	}

	return batch, nil
}

func (s batchService) UpdateInstancesAssignedToBatch(ctx context.Context, batch Batch) error {
	if batch.Status == api.BATCHSTATUS_RUNNING {
		return fmt.Errorf("Cannot update batch %q: Currently in a migration phase: %w", batch.Name, ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		instances, err := s.instance.GetAllByBatch(ctx, batch.Name)
		if err != nil {
			return fmt.Errorf("Failed to get instance for batch %q (%d): %w", batch.Name, batch.ID, err)
		}

		// Update each instance for this batch.
		assignedInstances := map[uuid.UUID]bool{}
		for _, instance := range instances {
			isMatch, err := instance.MatchesCriteria(batch.IncludeExpression)
			if err != nil {
				return err
			}

			if isMatch {
				assignedInstances[instance.UUID] = true
			} else {
				// Instance does not belong to this batch
				err := s.repo.UnassignBatch(ctx, batch.Name, instance.UUID)
				if err != nil {
					return fmt.Errorf("Failed to unassign instance %q from batch: %w", instance.Properties.Location, err)
				}
			}
		}

		// Get a list of all instances.
		instances, err = s.instance.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get instances for match checking with batch %q (%d): %w", batch.Name, batch.ID, err)
		}

		// Check if any unassigned instances should be assigned to this batch.
		for _, instance := range instances {
			isMatch, err := instance.MatchesCriteria(batch.IncludeExpression)
			if err != nil {
				return err
			}

			if isMatch && !assignedInstances[instance.UUID] {
				err := s.repo.AssignBatch(ctx, batch.Name, instance.UUID)
				if err != nil {
					return fmt.Errorf("Failed to assign instance %q to batch: %w", instance.Properties.Location, err)
				}
			}
		}

		// Reset instance state in testing mode.
		if util.InTestingMode() {
			for _, inst := range instances {
				err := s.instance.RemoveFromQueue(ctx, inst.UUID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return fmt.Errorf("Failed to remove from queue: %w", err)
				}
			}
		}

		return nil
	})
}

func (s batchService) Rename(ctx context.Context, oldName string, newName string) error {
	return s.repo.Rename(ctx, oldName, newName)
}

func (s batchService) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		oldBatch, err := s.repo.GetByName(ctx, name)
		if err != nil {
			return err
		}

		if oldBatch.Status == api.BATCHSTATUS_RUNNING {
			return fmt.Errorf("Cannot delete batch %q: Currently in a migration phase: %w", name, ErrOperationNotPermitted)
		}

		instances, err := s.instance.GetAllByBatch(ctx, oldBatch.Name)
		if err != nil {
			return err
		}

		// Verify all instances for this batch aren't in a migration phase and remove their association with this batch.
		for _, inst := range instances {
			err = s.repo.UnassignBatch(ctx, name, inst.UUID)
			if err != nil {
				return err
			}
		}

		err = s.repo.UnassignMigrationWindows(ctx, name)
		if err != nil {
			return err
		}

		return s.repo.DeleteByName(ctx, name)
	})
}

func (s batchService) StartBatchByName(ctx context.Context, name string) (err error) {
	if name == "" {
		return fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		// Get the batch to start.
		batch, err := s.GetByName(ctx, name)
		if err != nil {
			return err
		}

		// Ensure batch is in a state that is ready to start.
		switch batch.Status {
		case
			api.BATCHSTATUS_DEFINED,
			api.BATCHSTATUS_STOPPED,
			api.BATCHSTATUS_ERROR:
			// States, where starting a batch is allowed.
		default:
			return fmt.Errorf("Cannot start batch %q in its current state '%s': %w", batch.Name, batch.Status, ErrOperationNotPermitted)
		}

		batch.StartDate = time.Now().UTC()
		batch.Status = api.BATCHSTATUS_RUNNING
		batch.StatusMessage = string(batch.Status)
		return s.repo.Update(ctx, batch.Name, *batch)
	})
}

func (s batchService) StopBatchByName(ctx context.Context, name string) (err error) {
	if name == "" {
		return fmt.Errorf("Batch name cannot be empty: %w", ErrOperationNotPermitted)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		// Get the batch to stop.
		batch, err := s.GetByName(ctx, name)
		if err != nil {
			return err
		}

		if batch.Status != api.BATCHSTATUS_RUNNING {
			return fmt.Errorf("Cannot stop batch %q in its current state '%s': %w", batch.Name, batch.Status, ErrOperationNotPermitted)
		}

		// Move batch status to "stopped".
		batch.Status = api.BATCHSTATUS_STOPPED
		batch.StatusMessage = string(batch.Status)
		return s.repo.Update(ctx, batch.Name, *batch)
	})
}

func (s batchService) AssignMigrationWindows(ctx context.Context, batch string, windows MigrationWindows) error {
	err := windows.Validate()
	if err != nil {
		return fmt.Errorf("Failed to assign migration window to batch %q: %w", batch, err)
	}

	return s.repo.AssignMigrationWindows(ctx, batch, windows)
}

func (s batchService) ChangeMigrationWindows(ctx context.Context, queueSvc QueueService, batchName string, newWindows MigrationWindows) error {
	err := newWindows.Validate()
	if err != nil {
		return fmt.Errorf("Failed to assign migration window to batch %q: %w", batchName, err)
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		batch, err := s.repo.GetByName(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get batch %q: %w", batchName, err)
		}

		oldWindows, err := s.repo.GetMigrationWindowsByBatch(ctx, batch.Name)
		if err != nil {
			return fmt.Errorf("Failed to get current migration windows for batch %q: %w", batch.Name, err)
		}

		// Keep this list nil by default so we don't write anything unless the windows actually change.
		var changedWindows MigrationWindows
		if len(oldWindows) != len(newWindows) {
			changedWindows = newWindows
		} else {
			windowMap := map[string]bool{}
			for _, w := range oldWindows {
				windowMap[w.Key()] = true
			}

			changed := false
			for _, w := range newWindows {
				if !windowMap[w.Key()] {
					changed = true
					break
				}
			}

			if changed {
				changedWindows = append(MigrationWindows{}, newWindows...)
			}
		}

		newWindowMap := make(map[string]MigrationWindow, len(newWindows))
		for _, w := range newWindows {
			newWindowMap[w.Key()] = w
		}

		removedWindows := MigrationWindows{}
		for _, w := range oldWindows {
			_, ok := newWindowMap[w.Key()]
			if !ok {
				removedWindows = append(removedWindows, w)
			}
		}

		if batch.Status == api.BATCHSTATUS_RUNNING {
			entries, err := queueSvc.GetAllByBatch(ctx, batch.Name)
			if err != nil {
				return fmt.Errorf("Failed to get queue entries for batch %q: %w", batch.Name, err)
			}

			for _, e := range entries {
				if e.IsCommitted() && len(removedWindows) > 0 {
					return fmt.Errorf("Cannot remove migration windows from batch %q with committed queue entries: %w", batch.Name, ErrOperationNotPermitted)
				}
			}
		}

		if changedWindows != nil {
			return s.repo.UpdateMigrationWindows(ctx, batch.Name, changedWindows)
		}

		return nil
	})
}

func (s batchService) GetMigrationWindows(ctx context.Context, batch string) (MigrationWindows, error) {
	return s.repo.GetMigrationWindowsByBatch(ctx, batch)
}

func (s batchService) GetMigrationWindow(ctx context.Context, windowID int64) (*MigrationWindow, error) {
	return s.repo.GetMigrationWindow(ctx, windowID)
}

func (s batchService) GetEarliestWindow(ctx context.Context, batch string) (*MigrationWindow, error) {
	windows, err := s.repo.GetMigrationWindowsByBatch(ctx, batch)
	if err != nil {
		return nil, fmt.Errorf("Failed to get batch %q migration windows: %w", batch, err)
	}

	// If there are no explicit windows exist, return a zeroed MigrationWindow.
	if len(windows) == 0 {
		return &MigrationWindow{Start: time.Time{}, End: time.Time{}, Lockout: time.Time{}}, nil
	}

	earliest, err := windows.GetEarliest(0)
	if err != nil {
		return nil, fmt.Errorf("Failed to get earliest migration window for batch %q: %w", batch, err)
	}

	return earliest, nil
}

func (s batchService) DeterminePlacement(ctx context.Context, instance Instance, usedNetworks Networks, batch Batch, migrationWindows MigrationWindows) (*api.Placement, error) {
	if batch.Config.PlacementScriptlet == "" {
		return batch.GetIncusPlacement(instance, usedNetworks, api.Placement{})
	}

	apiNetworks := make([]api.Network, len(usedNetworks))
	for _, n := range usedNetworks {
		apiNet, err := n.ToAPI()
		if err != nil {
			return nil, err
		}

		apiNetworks = append(apiNetworks, *apiNet)
	}

	err := scriptlet.BatchPlacementSet(s.scriptletLoader, batch.Config.PlacementScriptlet, batch.Name)
	if err != nil {
		return nil, err
	}

	rawPlacement, err := scriptlet.BatchPlacementRun(ctx, s.scriptletLoader, instance.ToAPI(), batch.ToAPI(migrationWindows), apiNetworks)
	if err != nil {
		return nil, err
	}

	return batch.GetIncusPlacement(instance, usedNetworks, *rawPlacement)
}

// ResetBatchByName returns the batch to Defined state, and removes all associated queue entries. Also cleans up target and source concurrency limits.
func (s batchService) ResetBatchByName(ctx context.Context, name string, queueSvc QueueService, sourceSvc SourceService, targetSvc TargetService) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		batch, err := s.repo.GetByName(ctx, name)
		if err != nil {
			return err
		}

		// Batch is not started, so nothing to do.
		if batch.Status == api.BATCHSTATUS_DEFINED {
			return nil
		}

		entries, err := queueSvc.GetAllByBatch(ctx, name)
		if err != nil {
			return err
		}

		var entriesCommitted bool
		for _, q := range entries {
			// Check if there are any committed queue entries that prevent resetting the batch.
			if q.IsCommitted() {
				entriesCommitted = true
				break
			}
		}

		if entriesCommitted {
			return fmt.Errorf("Queue entries have already begun final import or post-migration steps: %w", ErrOperationNotPermitted)
		}

		instances, err := s.instance.GetAllQueued(ctx, entries)
		if err != nil {
			return err
		}

		err = queueSvc.DeleteAllByBatch(ctx, name)
		if err != nil {
			return fmt.Errorf("Failed to remove all queue entries for batch %q: %w", name, err)
		}

		batch.Status = api.BATCHSTATUS_DEFINED
		batch.StatusMessage = string(api.BATCHSTATUS_DEFINED)
		batch.StartDate = time.Time{}
		err = s.repo.Update(ctx, name, *batch)
		if err != nil {
			return fmt.Errorf("Failed to reset batch %q: %w", name, err)
		}

		instMap := make(map[uuid.UUID]Instance, len(entries))
		for _, inst := range instances {
			instMap[inst.UUID] = inst
		}

		for _, q := range entries {
			if q.MigrationStatus == api.MIGRATIONSTATUS_BACKGROUND_IMPORT || q.MigrationStatus == api.MIGRATIONSTATUS_FINAL_IMPORT {
				sourceSvc.RemoveActiveImport(instMap[q.InstanceUUID].Source)
				targetSvc.RemoveActiveImport(q.Placement.TargetName)
			}
		}

		return nil
	})
}
