package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"
	incusTLS "github.com/lxc/incus/v6/shared/tls"

	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/queue"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (d *Daemon) runPeriodicTask(ctx context.Context, task string, f func(context.Context) error, interval time.Duration) {
	go func() {
		for {
			err := f(ctx)
			if err != nil {
				slog.Error("Failed to run periodic task", slog.String("task", task), logger.Err(err))
			}

			t := time.NewTimer(interval)

			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
				t.Stop()
			}
		}
	}()
}

// validateForQueue validates that a set of instances in a batch are capable of being queued for the given target.
// - The batch must be DEFINED or QUEUED.
// - All instances must be ASSIGNED to the batch.
// - All instances must be defined on the source.
// - The batch must be within a valid migration window.
// - Ensures the target and project are reachable.
// - Ensures there are no conflicting instances on the target.
// - Ensures the correct ISO images exist in the target storage pool.
func (d *Daemon) validateForQueue(ctx context.Context, b migration.Batch, w migration.MigrationWindows, t migration.Target, instances map[uuid.UUID]migration.Instance) (*target.InternalIncusTarget, error) {
	err := b.CanStart(w)
	if err != nil {
		return nil, err
	}

	// If no instances apply to this batch, return nil, an error.
	if len(instances) == 0 {
		return nil, fmt.Errorf("Batch %q has no instances assigned", b.Name)
	}

	it, err := target.NewInternalIncusTargetFrom(t.ToAPI())
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to target for batch %q: %w", b.Name, err)
	}

	err = it.ReadyForMigration(ctx, b.TargetProject, instances)
	if err != nil {
		return nil, fmt.Errorf("Failed to validate target for batch %q: %w", b.Name, err)
	}

	// Ensure VMware VIX tarball exists.
	_, err = d.os.GetVMwareVixName()
	if err != nil {
		return nil, fmt.Errorf("Failed to find VMWare vix tarball: %w", err)
	}

	// Ensure exactly zero or one VirtIO drivers ISOs exist.
	_, err = d.os.GetVirtioDriversISOName()
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Failed to find Virtio drivers ISO: %w", err)
	}

	return it, nil
}

func (d *Daemon) reassessBlockedInstances(ctx context.Context) error {
	blockedEntries, err := d.queue.GetAllByState(ctx, api.MIGRATIONSTATUS_BLOCKED)
	if err != nil {
		return fmt.Errorf("Failed to fetch blocked queue entries: %w", err)
	}

	if len(blockedEntries) == 0 {
		return nil
	}

	blockedInstances, err := d.instance.GetAllQueued(ctx, blockedEntries)
	if err != nil {
		return fmt.Errorf("Failed to fetch blocked instances: %w", err)
	}

	for i, inst := range blockedInstances {
		err := inst.DisabledReason()
		if err != nil {
			slog.Warn("Instance is blocked from migration", slog.String("location", inst.Properties.Location), slog.String("reason", err.Error()))
			continue
		}

		_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_WAITING, string(api.MIGRATIONSTATUS_WAITING), blockedEntries[i].ImportStage, blockedEntries[i].GetWindowID())
		if err != nil {
			return fmt.Errorf("Failed to unblock queue entry for %q: %w", inst.Properties.Location, err)
		}
	}

	return nil
}

// beginImports creates the target VMs for all CREATING status instances.
// It sets their batches to RUNNING if they have the necessary files to begin a migration.
// Errors encountered in one batch do not affect the processing of other batches.
//   - cleanupInstances determines whether to delete failed target VMs on errors.
//     If true, errors will not result in the instance state being set to ERROR, to enable retrying this task.
//     If any errors occur after the VM has started, the VM will no longer be cleaned up, and its state will be set to ERROR, preventing retries.
func (d *Daemon) beginImports(ctx context.Context, cleanupInstances bool) error {
	log := slog.With(slog.String("method", "beginImports"))
	var migrationState map[string]queue.MigrationState
	err := transaction.Do(ctx, func(ctx context.Context) error {
		err := d.reassessBlockedInstances(ctx)
		if err != nil {
			return err
		}

		migrationState, err = d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_QUEUED, api.MIGRATIONSTATUS_WAITING)
		if err != nil {
			return fmt.Errorf("Failed to compile migration state for batch processing: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	ignoredBatches := []string{}
	// Concurrently validate batches, and create the necessary volumes to begin migration.
	err = util.RunConcurrentMap(migrationState, func(batchName string, state queue.MigrationState) error {
		// Set a 120s timeout for creating the volumes on the target before instance creation.
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*120)
		defer cancel()

		log := log.With(slog.String("batch", state.Batch.Name))
		it, err := d.validateForQueue(timeoutCtx, state.Batch, state.MigrationWindows, state.Target, state.Instances)
		if err != nil {
			log.Warn("Batch does not meet requirements to start, ignoring", slog.String("batch", batchName), slog.Any("error", err))
			ignoredBatches = append(ignoredBatches, state.Batch.Name)
			return nil
		}

		// If we fail to set up the initial volumes, set the batch status to errored and skip.
		err = d.ensureISOImagesExistInStoragePool(timeoutCtx, it, state.Instances, state.Batch)
		if err != nil {
			log.Error("Failed to validate batch", logger.Err(err))
			_, err := d.batch.UpdateStatusByName(ctx, state.Batch.Name, api.BATCHSTATUS_ERROR, err.Error())
			if err != nil {
				return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_ERROR, err)
			}

			ignoredBatches = append(ignoredBatches, state.Batch.Name)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Move ahead with successful batches only.
	for _, batchName := range ignoredBatches {
		delete(migrationState, batchName)
	}

	// Set the statuses for any batches that made it this far to RUNNING in preparation for instance creation on the target.
	// `finalizeCompleteInstances` will pick up these batches, but won't find any instances in them until their associated VMs are created.
	err = transaction.Do(ctx, func(ctx context.Context) error {
		for _, state := range migrationState {
			log.Info("Updating batch status to 'Running'")
			_, err := d.batch.UpdateStatusByName(ctx, state.Batch.Name, api.BATCHSTATUS_RUNNING, string(api.BATCHSTATUS_RUNNING))
			if err != nil {
				return fmt.Errorf("Failed to update batch status: %w", err)
			}
		}

		// Get all waiting instances in running batches, as they may have been skipped due to concurrency limits before.
		migrationState, err = d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_RUNNING, api.MIGRATIONSTATUS_WAITING)
		if err != nil {
			return fmt.Errorf("Failed to compile migration state for batch processing: %w", err)
		}

		for batchName, state := range migrationState {
			var properties api.IncusProperties
			err = json.Unmarshal(state.Target.Properties, &properties)
			if err != nil {
				return err
			}

			beginningInstances := map[uuid.UUID]migration.Instance{}
			beginningSources := map[uuid.UUID]migration.Source{}
			beginningQueueEntries := map[uuid.UUID]migration.QueueEntry{}
			for _, inst := range state.Instances {
				if properties.CreateLimit > 0 && d.target.GetCachedCreations(state.Target.Name) >= properties.CreateLimit {
					log.Warn("Create limit reached for target, waiting for existing instances to finish creating", slog.String("target", state.Target.Name))
					continue
				}

				d.target.RecordCreation(state.Target.Name)
				_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_CREATING, "Creating target instance definition", state.QueueEntries[inst.UUID].ImportStage, state.QueueEntries[inst.UUID].GetWindowID())
				if err != nil {
					return fmt.Errorf("Failed to unblock queue entry for %q: %w", inst.Properties.Location, err)
				}

				beginningInstances[inst.UUID] = inst
				beginningSources[inst.UUID] = state.Sources[inst.UUID]
				beginningQueueEntries[inst.UUID] = state.QueueEntries[inst.UUID]
			}

			// Prune any deferred instances from the migration state.
			state.QueueEntries = beginningQueueEntries
			state.Sources = beginningSources
			state.Instances = beginningInstances
			migrationState[batchName] = state
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("Process Queued Batches worker failed: %w", err)
	}

	// Create target VMs for all the instances in the remaining batches.
	err = util.RunConcurrentMap(migrationState, func(batchName string, state queue.MigrationState) error {
		return util.RunConcurrentMap(state.Instances, func(instUUID uuid.UUID, inst migration.Instance) error {
			return d.createTargetVM(ctx, state.Batch, inst, state.Target, state.Sources[instUUID], state.QueueEntries[instUUID], cleanupInstances)
		})
	})
	if err != nil {
		return fmt.Errorf("Failed to initialize migration workers: %w", err)
	}

	return nil
}

// ensureISOImagesExistInStoragePool ensures the necessary image files exist on the daemon to be imported to the storage volume.
func (d *Daemon) ensureISOImagesExistInStoragePool(ctx context.Context, it *target.InternalIncusTarget, instances map[uuid.UUID]migration.Instance, batch migration.Batch) error {
	if len(instances) == 0 {
		return fmt.Errorf("No instances in batch")
	}

	log := slog.With(
		slog.String("method", "ensureISOImagesExistInStoragePool"),
		slog.String("storage_pool", batch.StoragePool),
	)

	reverter := revert.New()
	defer reverter.Fail()

	// Key the batch by its constituent parts, as batches with different IDs may share the same target, pool, and project.
	batchKey := it.GetName() + "_" + batch.StoragePool + "_" + batch.TargetProject
	d.batchLock.Lock(batchKey)
	reverter.Add(func() { d.batchLock.Unlock(batchKey) })

	// Connect to the target.
	volumes, err := it.GetStoragePoolVolumeNames(batch.StoragePool)
	if err != nil {
		return err
	}

	volumeMap := make(map[string]bool, len(volumes))
	for _, vol := range volumes {
		volumeMap[vol] = true
	}

	// Verify needed ISO image is in the storage pool.
	var needsDriverISO bool
	for _, inst := range instances {
		if inst.GetOSType() == api.OSTYPE_WINDOWS {
			needsDriverISO = true
			break
		}
	}

	existingDriverName, err := d.os.GetVirtioDriversISOName()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to find Virtio drivers ISO: %w", err)
	}

	missingBaseImg := !volumeMap["custom/"+util.WorkerVolume()]
	missingDriverISO := needsDriverISO && (existingDriverName == "" || !volumeMap["custom/"+existingDriverName])

	// If we need to download missing files, or upload them to the target, set a status message.
	if missingBaseImg || missingDriverISO {
		_, err := d.batch.UpdateStatusByName(ctx, batch.Name, batch.Status, "Downloading artifacts")
		if err != nil {
			return fmt.Errorf("Failed to update batch %q status message: %w", batch.Name, err)
		}
	}

	if missingBaseImg {
		log.Info("Worker image doesn't exist in storage pool, importing...")
		err = d.os.LoadWorkerImage(ctx)
		if err != nil {
			return err
		}

		ops, err := it.CreateStoragePoolVolumeFromBackup(batch.StoragePool, filepath.Join(d.os.CacheDir, util.RawWorkerImage()))
		if err != nil {
			return err
		}

		for _, op := range ops {
			err = op.WaitContext(ctx)
			if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
				return err
			}
		}
	}

	if missingDriverISO {
		driversISOPath, err := d.os.LoadVirtioWinISO()
		if err != nil {
			return err
		}

		driversISO := filepath.Base(driversISOPath)
		if !volumeMap["custom/"+driversISO] {
			log.Info("ISO image doesn't exist in storage pool, importing...")

			ops, err := it.CreateStoragePoolVolumeFromISO(batch.StoragePool, driversISOPath)
			if err != nil {
				return err
			}

			for _, op := range ops {
				err = op.WaitContext(ctx)
				if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
					return err
				}
			}
		}
	}

	d.batchLock.Unlock(batchKey)
	reverter.Success()

	return nil
}

// Concurrently create target VMs for each instance record.
// Any instance that fails the migration has its state set to ERROR.
// - cleanupInstances determines whether a target VM should be deleted if it encounters an error.
func (d *Daemon) createTargetVM(ctx context.Context, b migration.Batch, inst migration.Instance, t migration.Target, s migration.Source, q migration.QueueEntry, cleanupInstances bool) (_err error) {
	log := slog.With(
		slog.String("method", "createTargetVM"),
		slog.String("instance", inst.Properties.Location),
		slog.String("source", s.Name),
		slog.String("target", t.Name),
		slog.String("batch", b.Name),
	)

	reverter := revert.New()
	defer reverter.Fail()
	reverter.Add(func() {
		d.target.RemoveCreation(t.Name)
		log := log.With(slog.String("revert", "set instance failed"))
		var errString string
		if _err != nil {
			errString = _err.Error()
		}

		// If cleanupInstances is true, then we can try to create the VMs again so don't set the instance state to errored.
		if cleanupInstances {
			log.Error("Failed attempt to create target instance. Trying again soon")
			return
		}

		// Try to set the instance state to ERRORED if it failed.
		_, err := d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, errString, migration.IMPORTSTAGE_BACKGROUND, q.GetWindowID())
		if err != nil {
			log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}
	})

	it, err := target.NewInternalIncusTargetFrom(t.ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to construct target %q: %w", t.Name, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*120)
	defer cancel()

	// Connect to the target.
	err = it.Connect(timeoutCtx)
	if err != nil {
		return fmt.Errorf("Failed to connect to target %q: %w", it.GetName(), err)
	}

	// Set the project.
	err = it.SetProject(b.TargetProject)
	if err != nil {
		return fmt.Errorf("Failed to set project %q for target %q: %w", b.TargetProject, it.GetName(), err)
	}

	var driverISOName string
	if inst.GetOSType() == api.OSTYPE_WINDOWS {
		driverISOName, err = d.os.GetVirtioDriversISOName()
		if err != nil {
			return fmt.Errorf("Failed to get driver ISO path: %w", err)
		}
	}

	cert, err := d.ServerCert().PublicKeyX509()
	if err != nil {
		return fmt.Errorf("Failed to parse server certificate: %w", err)
	}

	// Optionally clean up the VMs if we fail to create them.
	instanceDef, err := it.CreateVMDefinition(inst, q.SecretToken, b.StoragePool, incusTLS.CertFingerprint(cert), d.getWorkerEndpoint())
	if err != nil {
		return fmt.Errorf("Failed to create instance definition: %w", err)
	}

	cleanup, err := it.CreateNewVM(timeoutCtx, inst, instanceDef, b.StoragePool, util.WorkerVolume(), driverISOName)
	if err != nil {
		return fmt.Errorf("Failed to create new instance %q on migration target %q: %w", instanceDef.Name, it.GetName(), err)
	}

	if cleanupInstances {
		reverter.Add(func() {
			slog.Error("Cleaning up new instance after failure", slog.String("revert", "instance cleanup"), slog.Any("error", err))
			cleanup()
		})
	}

	// Start the instance.
	err = it.StartVM(timeoutCtx, inst.GetName())
	if err != nil {
		return fmt.Errorf("Failed to start instance %q on target %q: %w", instanceDef.Name, it.GetName(), err)
	}

	// Unblock the concurrency limits for the target so that the Incus agent doesn't block other creations.
	d.target.RemoveCreation(t.Name)

	err = it.CheckIncusAgent(timeoutCtx, instanceDef.Name)
	if err != nil {
		return err
	}

	// Set the instance state to IDLE before triggering the worker.
	_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_IDLE, "Waiting for worker to connect", migration.IMPORTSTAGE_BACKGROUND, q.GetWindowID())
	if err != nil {
		return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_IDLE, err)
	}

	// Now that the VM agent is up, expect a worker update to come soon..
	d.queueHandler.RecordWorkerUpdate(inst.UUID)

	reverter.Success()

	return nil
}

// resetQueueEntry starts up the source VM, and sets the queue entry to an earlier step, in the event of a migration window deadline.
// - If the deadline was reached during final import, then it is reset to IDLE and the worker is restarted.
// - If the deadline was reached during post-import, then the target VM is deleted and the queue entry is reset to WAITING for a new instance creation.
func (d *Daemon) resetQueueEntry(ctx context.Context, instUUID uuid.UUID, state queue.MigrationState) error {
	log := slog.With(
		slog.String("method", "resetQueueEntry"),
		slog.String("target", state.Target.Name),
		slog.String("batch", state.Batch.Name),
		slog.String("instance", state.Instances[instUUID].Properties.Location),
		slog.String("source", state.Sources[instUUID].Name),
	)

	src := state.Sources[instUUID]
	is, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to configure %q source-specific configuration for restarting source VM on source %q: %w", src.SourceType, src.Name, err)
	}

	err = is.Connect(ctx)
	if err != nil {
		return fmt.Errorf("Failed to connect to %q source to restart VM on source %q for next migration window: %w", src.SourceType, src.Name, err)
	}

	// First power on the source VM.
	err = is.PowerOnVM(ctx, state.Instances[instUUID].Properties.Location)
	if err != nil {
		return fmt.Errorf("Failed to restart VM on source %q for next migration window: %w", src.Name, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*120)
	defer cancel()

	it, err := target.NewInternalIncusTargetFrom(state.Target.ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to set up %q target-specific configuration: %w", state.Target.TargetType, err)
	}

	err = it.Connect(timeoutCtx)
	if err != nil {
		return fmt.Errorf("Failed to connect to target %q: %w", it.GetName(), err)
	}

	err = it.SetProject(state.Batch.TargetProject)
	if err != nil {
		return fmt.Errorf("Failed to set target %q project %q: %w", it.GetName(), state.Batch.TargetProject, err)
	}

	// If the VM failed in post-import steps, then it needs to be fully cleaned up.
	resetState := api.MIGRATIONSTATUS_IDLE
	resetImportStage := migration.IMPORTSTAGE_FINAL
	if state.QueueEntries[instUUID].MigrationStatus != api.MIGRATIONSTATUS_FINAL_IMPORT {
		resetState = api.MIGRATIONSTATUS_WAITING
		resetImportStage = migration.IMPORTSTAGE_BACKGROUND
		log.Warn("Cleaning up target instance due to migration window deadline")
		err := it.CleanupVM(timeoutCtx, state.Instances[instUUID].Properties.Name)
		if err != nil {
			return fmt.Errorf("Failed to clean up instance %q due to migration window deadline: %w", state.Instances[instUUID].Properties.Location, err)
		}
	} else {
		// Stop the migration worker so it doesn't interfere with our state cleanup.
		err = it.Exec(timeoutCtx, state.Instances[instUUID].Properties.Name, []string{"systemctl", "stop", "migration-manager-worker.service"})
		if err != nil {
			return fmt.Errorf("Failed to stop migration worker on for instance %q: %w", state.Instances[instUUID].Properties.Location, err)
		}
	}

	// Set the migration state to an earlier step.
	reason := "Migration window ended, waiting for next migration window"
	_, err = d.queue.UpdateStatusByUUID(ctx, instUUID, resetState, reason, resetImportStage, nil)
	if err != nil {
		return fmt.Errorf("Failed to reset queue entry %q status: %w", instUUID, err)
	}

	// Restart the migration worker if the instance is still running.
	if state.QueueEntries[instUUID].MigrationStatus == api.MIGRATIONSTATUS_FINAL_IMPORT {
		log.Warn("Restarting migration worker due to migration window deadline")
		err := it.Exec(timeoutCtx, state.Instances[instUUID].Properties.Name, []string{"systemctl", "restart", "migration-manager-worker.service"})
		if err != nil {
			return fmt.Errorf("Failed to restart migration worker on restarting instance %q: %w", state.Instances[instUUID].Properties.Location, err)
		}
	}

	return nil
}

// finalizeCompleteInstances fetches all instances in RUNNING batches whose status is WORKER DONE, and for each batch, runs configureMigratedInstances.
func (d *Daemon) finalizeCompleteInstances(ctx context.Context) (_err error) {
	log := slog.With(slog.String("method", "finalizeCompleteInstances"))
	var migrationState map[string]queue.MigrationState
	var allNetworks migration.Networks

	queueEntriesToReset := map[uuid.UUID]bool{}
	windowsByQueueUUID := map[uuid.UUID]migration.MigrationWindow{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		allNetworks, err = d.network.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all networks: %w", err)
		}

		migrationState, err = d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_RUNNING, api.MIGRATIONSTATUS_WORKER_DONE, api.MIGRATIONSTATUS_FINAL_IMPORT, api.MIGRATIONSTATUS_POST_IMPORT)
		if err != nil {
			return fmt.Errorf("Failed to compile migration state for final import steps: %w", err)
		}

		for _, s := range migrationState {
			for _, q := range s.QueueEntries {
				windowID := q.GetWindowID()
				if windowID == nil {
					continue
				}

				window, err := d.batch.GetMigrationWindow(ctx, *windowID)
				if err != nil {
					return fmt.Errorf("Failed to get migration window for queue entry %q: %w", q.InstanceUUID, err)
				}

				windowsByQueueUUID[q.InstanceUUID] = *window
				if window.Ended() {
					queueEntriesToReset[q.InstanceUUID] = true
				}
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	finishedInstances := []uuid.UUID{}
	err = util.RunConcurrentMap(migrationState, func(batchName string, state queue.MigrationState) error {
		return util.RunConcurrentMap(state.Instances, func(instUUID uuid.UUID, instance migration.Instance) error {
			if queueEntriesToReset[instUUID] {
				return d.resetQueueEntry(ctx, instUUID, state)
			}

			// Skip queue entries that are still performing sync.
			if state.QueueEntries[instUUID].MigrationStatus != api.MIGRATIONSTATUS_WORKER_DONE {
				return nil
			}

			instanceList := make(migration.Instances, 0, len(state.Instances))
			for _, inst := range state.Instances {
				instanceList = append(instanceList, inst)
			}

			window := windowsByQueueUUID[instUUID]
			err := d.configureMigratedInstances(ctx, state.QueueEntries[instUUID], window, instance, state.Sources[instUUID], state.Target, state.Batch, migration.FilterUsedNetworks(allNetworks, instanceList))
			if err != nil {
				return err
			}

			finishedInstances = append(finishedInstances, instUUID)
			return nil
		})
	})
	if err != nil {
		log.Error("Failed to configure migrated instances for all batches", slog.Any("error", err))
	}

	// Remove complete records from the queue cache.
	for _, instanceUUID := range finishedInstances {
		d.queueHandler.RemoveFromCache(instanceUUID)
	}

	// Set fully completed batches to FINISHED state.
	return transaction.Do(ctx, func(ctx context.Context) error {
		for batch := range migrationState {
			entries, err := d.queue.GetAllByBatch(ctx, batch)
			if err != nil {
				return err
			}

			finished := true
			for _, entry := range entries {
				if entry.MigrationStatus != api.MIGRATIONSTATUS_FINISHED {
					finished = false
					break
				}
			}

			if finished {
				_, err := d.batch.UpdateStatusByName(ctx, batch, api.BATCHSTATUS_FINISHED, string(api.BATCHSTATUS_FINISHED))
				if err != nil {
					return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_FINISHED, err)
				}
			}
		}

		return nil
	})
}

// configureMigratedInstances updates the configuration of instances concurrently after they have finished migrating. Errors will result in the instance state becoming ERRORED.
// If an instance succeeds, its state will be moved to FINISHED.
func (d *Daemon) configureMigratedInstances(ctx context.Context, q migration.QueueEntry, w migration.MigrationWindow, i migration.Instance, s migration.Source, t migration.Target, batch migration.Batch, activeNetworks migration.Networks) (_err error) {
	log := slog.With(
		slog.String("method", "configureMigratedInstances"),
		slog.String("target", t.Name),
		slog.String("batch", batch.Name),
		slog.String("instance", i.Properties.Location),
		slog.String("source", s.Name),
	)

	log.Info("Finalizing target instance")
	reverter := revert.New()
	defer reverter.Fail()
	reverter.Add(func() {
		log := log.With(slog.String("revert", "set instance failed"))
		var errString string
		if _err != nil {
			errString = _err.Error()
		}

		// If the migration window has already ended, we have no capacity to retry.
		if !w.Ended() {
			numRetries := d.instance.GetPostMigrationRetries(i.UUID)
			if numRetries < batch.PostMigrationRetries {
				d.instance.RecordPostMigrationRetry(i.UUID)
				log.Error("Instance failed post-migration steps, retrying", slog.String("error", errString), slog.Int("retry_count", numRetries), slog.Int("max_retries", batch.PostMigrationRetries))
				return
			}

			// Only persist the state as errored if the window is still active, because this reverter might have been triggered by the window deadline cleanup.
			_, err := d.queue.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, errString, migration.IMPORTSTAGE_BACKGROUND, q.GetWindowID())
			if err != nil {
				log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
			}
		}

		is, err := source.NewInternalVMwareSourceFrom(s.ToAPI())
		if err != nil {
			log.Error("Failed to establish source-specific type to restart VM after migration failure", logger.Err(err))
			return
		}

		err = is.Connect(ctx)
		if err != nil {
			log.Error("Failed to connect to source to restart VM after migration failure", logger.Err(err))
			return
		}

		err = is.PowerOnVM(ctx, i.Properties.Location)
		if err != nil {
			log.Error("Failed to restart VM after migration failure", logger.Err(err))
			return
		}
	})

	it, err := target.NewInternalIncusTargetFrom(t.ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to construct target %q: %w", t.Name, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*120)
	defer cancel()

	// Connect to the target.
	err = it.Connect(timeoutCtx)
	if err != nil {
		return fmt.Errorf("Failed to connect to target %q: %w", it.GetName(), err)
	}

	// Set the project.
	err = it.SetProject(batch.TargetProject)
	if err != nil {
		return fmt.Errorf("Failed to set target %q project %q: %w", it.GetName(), batch.TargetProject, err)
	}

	err = it.SetPostMigrationVMConfig(timeoutCtx, i, activeNetworks)
	if err != nil {
		return fmt.Errorf("Failed to update post-migration config for instance %q in %q: %w", i.GetName(), it.GetName(), err)
	}

	// Update the instance status to finished, and remove its migration window.
	_, err = d.queue.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_FINISHED, string(api.MIGRATIONSTATUS_FINISHED), q.ImportStage, nil)
	if err != nil {
		return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_FINISHED, err)
	}

	reverter.Success()

	return nil
}
