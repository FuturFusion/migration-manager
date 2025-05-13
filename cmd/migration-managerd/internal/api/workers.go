package api

import (
	"context"
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

// beginImports creates the target VMs for all CREATING status instances.
// It sets their batches to RUNNING if they have the necessary files to begin a migration.
// Errors encountered in one batch do not affect the processing of other batches.
//   - cleanupInstances determines whether to delete failed target VMs on errors.
//     If true, errors will not result in the instance state being set to ERROR, to enable retrying this task.
//     If any errors occur after the VM has started, the VM will no longer be cleaned up, and its state will be set to ERROR, preventing retries.
func (d *Daemon) beginImports(ctx context.Context, cleanupInstances bool) error {
	log := slog.With(slog.String("method", "beginImports"))
	migrationState, err := d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_QUEUED, api.MIGRATIONSTATUS_CREATING)
	if err != nil {
		return fmt.Errorf("Failed to compile migration state for batch processing: %w", err)
	}

	ignoredBatches := []string{}
	// Concurrently validate batches, and create the necessary volumes to begin migration.
	err = util.RunConcurrentMap(migrationState, func(batchName string, state queue.MigrationState) error {
		log := log.With(slog.String("batch", state.Batch.Name))
		it, err := d.validateForQueue(ctx, state.Batch, state.MigrationWindows, state.Target, state.Instances)
		if err != nil {
			log.Warn("Batch does not meet requirements to start, ignoring", slog.String("batch", batchName), slog.Any("error", err))
			ignoredBatches = append(ignoredBatches, state.Batch.Name)
			return nil
		}

		// If we fail to set up the initial volumes, set the batch status to errored and skip.
		err = d.ensureISOImagesExistInStoragePool(ctx, it, state.Instances, state.Batch)
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
			err = op.Wait()
			if err != nil {
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

			op, err := it.CreateStoragePoolVolumeFromISO(batch.StoragePool, driversISOPath)
			if err != nil {
				return err
			}

			err = op.Wait()
			if err != nil {
				return err
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
		_, err := d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, errString, true)
		if err != nil {
			log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}
	})

	it, err := target.NewInternalIncusTargetFrom(t.ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to construct target %q: %w", t.Name, err)
	}

	// Connect to the target.
	err = it.Connect(ctx)
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

	if cleanupInstances {
		reverter.Add(func() {
			log := log.With(slog.String("revert", "instance cleanup"))
			err := it.DeleteVM(instanceDef.Name)
			if err != nil {
				log.Error("Failed to delete new instance after failure", logger.Err(err))
			}
		})
	}

	err = it.CreateNewVM(instanceDef, b.StoragePool, util.WorkerVolume(), driverISOName)
	if err != nil {
		return fmt.Errorf("Failed to create new instance %q on migration target %q: %w", instanceDef.Name, it.GetName(), err)
	}

	// Start the instance.
	err = it.StartVM(inst.GetName())
	if err != nil {
		return fmt.Errorf("Failed to start instance %q on target %q: %w", instanceDef.Name, it.GetName(), err)
	}

	// Wait up to 90s for the Incus agent.
	waitCtx, cancel := context.WithTimeout(ctx, time.Second*90)
	defer cancel()
	err = it.CheckIncusAgent(waitCtx, instanceDef.Name)
	if err != nil {
		return err
	}

	// Set the instance state to IDLE before triggering the worker.
	_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_IDLE, string(api.MIGRATIONSTATUS_IDLE), true)
	if err != nil {
		return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_IDLE, err)
	}

	// Now that the VM agent is up, expect a worker update to come soon..
	d.queueHandler.RecordWorkerUpdate(inst.UUID)

	// At this point, the import is about to begin, so we won't try to delete instances anymore.
	// Instead, if an error occurs, we will try to set the instance state to ERROR so that we don't retry.
	cleanupInstances = false

	reverter.Success()

	return nil
}

// finalizeCompleteInstances fetches all instances in RUNNING batches whose status is IMPORT COMPLETE, and for each batch, runs configureMigratedInstances.
func (d *Daemon) finalizeCompleteInstances(ctx context.Context) (_err error) {
	log := slog.With(slog.String("method", "finalizeCompleteInstances"))
	var migrationState map[string]queue.MigrationState
	networksByName := map[string]migration.Network{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		networks, err := d.network.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all networks: %w", err)
		}

		for _, net := range networks {
			networksByName[net.Name] = net
		}

		validStates := []api.MigrationStatusType{
			api.MIGRATIONSTATUS_IDLE,
			api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
			api.MIGRATIONSTATUS_FINAL_IMPORT,
			api.MIGRATIONSTATUS_IMPORT_COMPLETE,
		}

		migrationState, err = d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_RUNNING, validStates...)
		return err
	})
	if err != nil {
		return err
	}

	importedBatches := map[string]map[uuid.UUID]bool{}
	for batchName, state := range migrationState {
		importedEntries := map[uuid.UUID]bool{}
		for _, entry := range state.QueueEntries {
			if entry.MigrationStatus != api.MIGRATIONSTATUS_IMPORT_COMPLETE {
				_, err := d.queueHandler.ReceivedWorkerUpdate(entry.InstanceUUID, time.Second*30)
				if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
					_, err = d.queue.UpdateStatusByUUID(ctx, entry.InstanceUUID, api.MIGRATIONSTATUS_ERROR, "Timed out waiting for worker", false)
					if err != nil {
						return fmt.Errorf("Failed to set errored state on instance %q: %w", state.Instances[entry.InstanceUUID].Properties.Location, err)
					}
				}

				// Only consider IMPORT COMPLETE records moving forward.
				continue
			}

			importedEntries[entry.InstanceUUID] = true
		}

		if len(importedEntries) > 0 {
			importedBatches[batchName] = importedEntries
		}
	}

	finishedBatches := make(map[string][]uuid.UUID, len(migrationState))
	for batchName, batches := range importedBatches {
		finishedInstances := []uuid.UUID{}
		state := migrationState[batchName]
		err := util.RunConcurrentMap(state.Instances, func(instUUID uuid.UUID, instance migration.Instance) error {
			if !batches[instance.UUID] {
				// Skip instances that haven't finished background import.
				return nil
			}

			err := d.configureMigratedInstances(ctx, instance, state.Target, state.Batch, networksByName)
			if err != nil {
				return err
			}

			finishedInstances = append(finishedInstances, instUUID)
			return nil
		})
		if err != nil {
			log.Error("Failed to configureMigratedInstances", slog.String("batch", state.Batch.Name))
		}

		finishedBatches[state.Batch.Name] = finishedInstances
	}

	// Remove complete records from the queue cache.
	for batchName := range finishedBatches {
		for instanceUUID := range migrationState[batchName].QueueEntries {
			d.queueHandler.RemoveFromCache(instanceUUID)
		}
	}

	// Set fully completed batches to FINISHED state.
	return transaction.Do(ctx, func(ctx context.Context) error {
		for batch, instUUIDs := range finishedBatches {
			if len(migrationState[batch].Instances) == len(instUUIDs) {
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
func (d *Daemon) configureMigratedInstances(ctx context.Context, i migration.Instance, t migration.Target, batch migration.Batch, allNetworks map[string]migration.Network) (_err error) {
	log := slog.With(
		slog.String("method", "createTargetVMs"),
		slog.String("target", t.Name),
		slog.String("batch", batch.Name),
		slog.String("instance", i.Properties.Location),
	)

	reverter := revert.New()
	defer reverter.Fail()
	reverter.Add(func() {
		log := log.With(slog.String("revert", "set instance failed"))
		var errString string
		if _err != nil {
			errString = _err.Error()
		}

		// Try to set the instance state to ERRORED if it failed.
		_, err := d.queue.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, errString, true)
		if err != nil {
			log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}
	})

	it, err := target.NewInternalIncusTargetFrom(t.ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to construct target %q: %w", t.Name, err)
	}

	// Connect to the target.
	err = it.Connect(ctx)
	if err != nil {
		return fmt.Errorf("Failed to connect to target %q: %w", it.GetName(), err)
	}

	// Set the project.
	err = it.SetProject(batch.TargetProject)
	if err != nil {
		return fmt.Errorf("Failed to set target %q project %q: %w", it.GetName(), batch.TargetProject, err)
	}

	err = it.SetPostMigrationVMConfig(i, allNetworks)
	if err != nil {
		return fmt.Errorf("Failed to update post-migration config for instance %q in %q: %w", i.GetName(), it.GetName(), err)
	}

	// Update the instance status.
	_, err = d.queue.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_FINISHED, string(api.MIGRATIONSTATUS_FINISHED), true)
	if err != nil {
		return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_FINISHED, err)
	}

	reverter.Success()

	return nil
}
