package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
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

func (d *Daemon) reassessBlockedInstances(ctx context.Context) error {
	entries, err := d.queue.GetAllByState(ctx, api.MIGRATIONSTATUS_BLOCKED, api.MIGRATIONSTATUS_WAITING)
	if err != nil {
		return fmt.Errorf("Failed to fetch blocked queue entries: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	queueInstances, err := d.instance.GetAllQueued(ctx, entries)
	if err != nil {
		return fmt.Errorf("Failed to fetch blocked instances: %w", err)
	}

	artifacts, err := d.artifact.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("Failed to fetch artifact records: %w", err)
	}

	blockedInstances := map[uuid.UUID]string{}
	instancesByUUID := make(map[uuid.UUID]migration.Instance, len(queueInstances))
	for _, inst := range queueInstances {
		instancesByUUID[inst.UUID] = inst
		err := d.artifact.HasRequiredArtifactsForInstance(artifacts, inst)
		if err != nil {
			slog.Error("Blocking queue entries due to artifact error", slog.Any("error", err))
			blockedInstances[inst.UUID] = fmt.Sprintf("Artifact error: %v", err.Error())
		}
	}

	batchMap := map[string]*migration.Batch{}
	for _, q := range entries {
		// Block all entries if we failed to validate the filesystem.
		if blockedInstances[q.InstanceUUID] != "" {
			if q.MigrationStatusMessage != blockedInstances[q.InstanceUUID] {
				_, err := d.queue.UpdateStatusByUUID(ctx, q.InstanceUUID, api.MIGRATIONSTATUS_BLOCKED, blockedInstances[q.InstanceUUID], q.ImportStage, q.GetWindowID())
				if err != nil {
					return fmt.Errorf("Failed to unblock queue entry %q: %w", q.InstanceUUID, err)
				}
			}

			continue
		}

		inst, ok := instancesByUUID[q.InstanceUUID]
		if !ok || q.MigrationStatus != api.MIGRATIONSTATUS_BLOCKED {
			continue
		}

		if batchMap[q.BatchName] == nil {
			batchMap[q.BatchName], err = d.batch.GetByName(ctx, q.BatchName)
			if err != nil {
				return fmt.Errorf("Failed to get batch for queue entry %q: %w", inst.Properties.Location, err)
			}
		}

		// Otherwise check why the VM is blocked, and unblock it if needed.
		err := inst.DisabledReason(batchMap[q.BatchName].Config.RestrictionOverrides)
		if err != nil {
			slog.Warn("Instance is blocked from migration", slog.String("location", inst.Properties.Location), slog.String("reason", err.Error()))
			continue
		}

		_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_WAITING, string(api.MIGRATIONSTATUS_WAITING), q.ImportStage, q.GetWindowID())
		if err != nil {
			return fmt.Errorf("Failed to unblock queue entry for %q: %w", inst.Properties.Location, err)
		}
	}

	return nil
}

// beginImports creates the target VMs for started batches.
// It fetches all RUNNING batches with WAITING or BLOCKED instances, and moves the instances to CREATING state.
// Errors encountered in one batch do not affect the processing of other batches.
//   - cleanupInstances determines whether to delete failed target VMs on errors.
//     If true, errors will not result in the instance state being set to ERROR, to enable retrying this task.
//     If any errors occur after the VM has started, the VM will no longer be cleaned up, and its state will be set to ERROR, preventing retries.
func (d *Daemon) beginImports(ctx context.Context, cleanupInstances bool) error {
	log := slog.With(slog.String("method", "beginImports"))
	var migrationState map[string]queue.MigrationState
	var allNetworks migration.Networks
	err := transaction.Do(ctx, func(ctx context.Context) error {
		err := d.reassessBlockedInstances(ctx)
		if err != nil {
			return fmt.Errorf("Failed to reassess blocked queue entries: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		migrationState, err = d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_RUNNING, api.MIGRATIONSTATUS_WAITING)
		if err != nil {
			return fmt.Errorf("Failed to compile migration state for batch processing: %w", err)
		}

		if len(migrationState) == 0 {
			return nil
		}

		// Get all networks so we can determine the target network if not overridden via scriptlet.
		allNetworks, err = d.network.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all networks: %w", err)
		}

		// Get data from every registered target to verify placement is valid.
		allTargets, err := d.target.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all targets: %w", err)
		}

		targetInfo := make([]target.IncusDetails, 0, len(allTargets))
		for _, t := range allTargets {
			it, err := target.NewTarget(t.ToAPI())
			if err != nil {
				return err
			}

			err = it.Connect(ctx)
			if err != nil {
				return err
			}

			info, err := it.GetDetails(ctx)
			if err != nil {
				return err
			}

			targetInfo = append(targetInfo, *info)
		}

		placementLock := sync.Mutex{}
		placementErrs := map[uuid.UUID]error{}
		err = util.RunConcurrentMap(migrationState, func(batchName string, state queue.MigrationState) error {
			return util.RunConcurrentMap(state.Instances, func(instUUID uuid.UUID, instance migration.Instance) error {
				if state.Batch.Config.RerunScriptlets {
					usedNetworks := migration.FilterUsedNetworks(allNetworks, migration.Instances{instance})
					placement, err := d.batch.DeterminePlacement(ctx, instance, usedNetworks, state.Batch, state.MigrationWindows)
					if err != nil {
						return err
					}

					// Update the migration state with the queue entry's new placement.
					placementLock.Lock()
					entry := state.QueueEntries[instUUID]
					entry.Placement = *placement
					state.QueueEntries[instUUID] = entry
					migrationState[batchName] = state
					placementLock.Unlock()
				}

				var info *target.IncusDetails
				for _, t := range targetInfo {
					if t.Name == state.QueueEntries[instUUID].Placement.TargetName {
						info = &t
						break
					}
				}

				// Verify that the target placement actually exists and the instance can be placed there.
				err := target.CanPlaceInstance(ctx, info, state.QueueEntries[instUUID].Placement, instance.ToAPI())
				if err != nil {
					placementLock.Lock()
					placementErrs[instUUID] = err
					placementLock.Unlock()
				}

				return nil
			})
		})
		if err != nil {
			return err
		}

		// Write-lock the DB here after running the scriptlets and fetching target info.
		var stateChanged bool
		for _, state := range migrationState {
			for instUUID, q := range state.QueueEntries {
				// Update the db record for any changed placements.
				if state.Batch.Config.RerunScriptlets {
					stateChanged = true
					_, err := d.queue.UpdatePlacementByUUID(ctx, instUUID, state.QueueEntries[instUUID].Placement)
					if err != nil {
						return fmt.Errorf("Failed to update queue %q placement record: %w", instUUID, err)
					}
				}

				// Block any queue entries that we failed to determine placement for. These will be picked up again and retried later.
				err, ok := placementErrs[instUUID]
				if ok {
					stateChanged = true
					blockedMsg := fmt.Sprintf("Cannot place instance: %v", err.Error())
					_, err := d.queue.UpdateStatusByUUID(ctx, q.InstanceUUID, api.MIGRATIONSTATUS_BLOCKED, blockedMsg, q.ImportStage, q.GetWindowID())
					if err != nil {
						return fmt.Errorf("Failed to unblock queue entry %q: %w", q.InstanceUUID, err)
					}
				}
			}
		}

		if stateChanged {
			// Since we just changed the state, we need to re-fetch it.
			migrationState, err = d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_RUNNING, api.MIGRATIONSTATUS_WAITING)
			if err != nil {
				return fmt.Errorf("Failed to compile migration state for batch processing: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// visitedLocations is a map of VM OS type to target name to project name to pool name.
	// This is used so that we ensure each pool in each target is checked only once for volumes in a particular project, for a particular VM OS.
	visitedLocations := map[string]map[string]map[string]bool{}
	ignoredBatches := []string{}
	err = util.RunConcurrentMap(migrationState, func(batchName string, state queue.MigrationState) error {
		// Set a 120s timeout for creating the volumes on the target before instance creation.
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*120)
		defer cancel()

		log := log.With(slog.String("batch", state.Batch.Name))
		for instUUID, q := range state.QueueEntries {
			for _, pool := range q.Placement.StoragePools {
				// for every instance in this batch, check volumes at the corresponding target, unless we did already.
				if visitedLocations == nil {
					visitedLocations = map[string]map[string]map[string]bool{}
				}

				if visitedLocations[q.Placement.TargetName] == nil {
					visitedLocations[q.Placement.TargetName] = map[string]map[string]bool{}
				}

				if visitedLocations[q.Placement.TargetName][q.Placement.TargetProject] == nil {
					visitedLocations[q.Placement.TargetName][q.Placement.TargetProject] = map[string]bool{}
				}

				if !visitedLocations[q.Placement.TargetName][q.Placement.TargetProject][pool] {
					err := d.ensureISOImagesExistInStoragePool(timeoutCtx, state.Targets[instUUID], state.Batch, pool, q.Placement.TargetProject)
					if err != nil {
						log.Error("Failed to validate batch", logger.Err(err))
						_, err := d.batch.UpdateStatusByName(ctx, state.Batch.Name, api.BATCHSTATUS_ERROR, err.Error())
						if err != nil {
							return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_ERROR, err)
						}

						ignoredBatches = append(ignoredBatches, state.Batch.Name)
					} else {
						visitedLocations[q.Placement.TargetName][q.Placement.TargetProject][pool] = true
					}
				}
			}
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
		for batchName, state := range migrationState {
			beginningTargets := map[uuid.UUID]migration.Target{}
			beginningInstances := map[uuid.UUID]migration.Instance{}
			beginningSources := map[uuid.UUID]migration.Source{}
			beginningQueueEntries := map[uuid.UUID]migration.QueueEntry{}
			for _, inst := range state.Instances {
				var properties api.IncusProperties
				err = json.Unmarshal(state.Targets[inst.UUID].Properties, &properties)
				if err != nil {
					return err
				}

				if properties.CreateLimit > 0 && d.target.GetCachedCreations(state.Targets[inst.UUID].Name) >= properties.CreateLimit {
					log.Warn("Create limit reached for target, waiting for existing instances to finish creating", slog.String("target", state.Targets[inst.UUID].Name))
					continue
				}

				d.target.RecordCreation(state.Targets[inst.UUID].Name)
				_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_CREATING, "Creating target instance definition", state.QueueEntries[inst.UUID].ImportStage, state.QueueEntries[inst.UUID].GetWindowID())
				if err != nil {
					return fmt.Errorf("Failed to unblock queue entry for %q: %w", inst.Properties.Location, err)
				}

				beginningInstances[inst.UUID] = inst
				beginningSources[inst.UUID] = state.Sources[inst.UUID]
				beginningTargets[inst.UUID] = state.Targets[inst.UUID]
				beginningQueueEntries[inst.UUID] = state.QueueEntries[inst.UUID]
			}

			// Prune any deferred instances from the migration state.
			state.QueueEntries = beginningQueueEntries
			state.Sources = beginningSources
			state.Instances = beginningInstances
			state.Targets = beginningTargets
			migrationState[batchName] = state
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("Process Queued Batches worker failed: %w", err)
	}

	// Create target VMs for all the instances in the remaining batches.
	err = util.RunConcurrentMap(migrationState, func(batchName string, state queue.MigrationState) error {
		instanceList := make(migration.Instances, 0, len(state.Instances))
		for _, inst := range state.Instances {
			instanceList = append(instanceList, inst)
		}

		instanceNetworks := migration.FilterUsedNetworks(allNetworks, instanceList)
		return util.RunConcurrentMap(state.Instances, func(instUUID uuid.UUID, inst migration.Instance) error {
			return d.createTargetVM(ctx, state.Batch, inst, state.Targets[inst.UUID], state.Sources[instUUID], state.QueueEntries[instUUID], instanceNetworks, cleanupInstances)
		})
	})
	if err != nil {
		return fmt.Errorf("Failed to initialize migration workers: %w", err)
	}

	return nil
}

// ensureISOImagesExistInStoragePool ensures the necessary image files exist on the daemon to be imported to the storage volume.
func (d *Daemon) ensureISOImagesExistInStoragePool(ctx context.Context, tgt migration.Target, batch migration.Batch, pool string, project string) error {
	log := slog.With(
		slog.String("method", "ensureISOImagesExistInStoragePool"),
		slog.String("storage_pool", pool),
		slog.String("target", tgt.Name),
		slog.String("project", project))
	reverter := revert.New()
	defer reverter.Fail()

	// Key the batch by its constituent parts, as batches with different IDs may share the same target, pool, and project.
	batchKey := tgt.Name + "_" + pool + "_" + project
	d.batchLock.Lock(batchKey)
	reverter.Add(func() { d.batchLock.Unlock(batchKey) })

	it, err := target.NewTarget(tgt.ToAPI())
	if err != nil {
		return err
	}

	err = it.Connect(ctx)
	if err != nil {
		return err
	}

	err = it.SetProject(project)
	if err != nil {
		return err
	}

	volumes, err := it.GetStoragePoolVolumeNames(pool)
	if err != nil {
		return err
	}

	var workerVolumeExists bool
	for _, vol := range volumes {
		if vol == "custom/"+util.WorkerVolume() {
			workerVolumeExists = true
			break
		}
	}

	// If we need to download missing files, or upload them to the target, set a status message.
	if !workerVolumeExists {
		_, err := d.batch.UpdateStatusByName(ctx, batch.Name, batch.Status, "Downloading artifacts")
		if err != nil {
			return fmt.Errorf("Failed to update batch %q status message: %w", batch.Name, err)
		}

		log.Info("Worker image doesn't exist in storage pool, importing...")
		err = d.os.LoadWorkerImage(ctx)
		if err != nil {
			return err
		}

		ops, err := it.CreateStoragePoolVolumeFromBackup(pool, filepath.Join(d.os.CacheDir, util.RawWorkerImage()))
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

	d.batchLock.Unlock(batchKey)
	reverter.Success()

	return nil
}

// Concurrently create target VMs for each instance record.
// Any instance that fails the migration has its state set to ERROR.
// - cleanupInstances determines whether a target VM should be deleted if it encounters an error.
func (d *Daemon) createTargetVM(ctx context.Context, b migration.Batch, inst migration.Instance, t migration.Target, s migration.Source, q migration.QueueEntry, networks migration.Networks, cleanupInstances bool) (_err error) {
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
		errString := "Instance creation attempt failed"
		if _err != nil {
			errString = _err.Error()
		}

		// If cleanupInstances is true, then we can try to create the VMs again so don't set the instance state to errored.
		if cleanupInstances {
			log.Error("Failed attempt to create target instance. Trying again soon")
			// Set the state to WAITING so it will be picked up again by beginImports.
			_, err := d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_WAITING, errString, migration.IMPORTSTAGE_BACKGROUND, q.GetWindowID())
			if err != nil {
				log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_WAITING), logger.Err(err))
			}

			return
		}

		// Try to set the instance state to ERRORED if it failed.
		_, err := d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, errString, migration.IMPORTSTAGE_BACKGROUND, q.GetWindowID())
		if err != nil {
			log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}
	})

	it, err := target.NewTarget(t.ToAPI())
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
	err = it.SetProject(q.Placement.TargetProject)
	if err != nil {
		return fmt.Errorf("Failed to set project %q for target %q: %w", q.Placement.TargetProject, it.GetName(), err)
	}

	cert, err := d.ServerCert().PublicKeyX509()
	if err != nil {
		return fmt.Errorf("Failed to parse server certificate: %w", err)
	}

	// Optionally clean up the VMs if we fail to create them.
	usedNetworks := migration.FilterUsedNetworks(networks, migration.Instances{inst})
	instanceDef, err := it.CreateVMDefinition(inst, usedNetworks, q, incusTLS.CertFingerprint(cert), d.getWorkerEndpoint())
	if err != nil {
		return fmt.Errorf("Failed to create instance definition: %w", err)
	}

	cleanup, err := it.CreateNewVM(timeoutCtx, inst, instanceDef, q.Placement, util.WorkerVolume())
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
		slog.String("target", state.Targets[instUUID].Name),
		slog.String("batch", state.Batch.Name),
		slog.String("instance", state.Instances[instUUID].Properties.Location),
		slog.String("source", state.Sources[instUUID].Name),
	)

	// First power on the source VM if it was initially running.
	if state.Instances[instUUID].Properties.Running {
		src := state.Sources[instUUID]
		is, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
		if err != nil {
			return fmt.Errorf("Failed to configure %q source-specific configuration for restarting source VM on source %q: %w", src.SourceType, src.Name, err)
		}

		err = is.Connect(ctx)
		if err != nil {
			return fmt.Errorf("Failed to connect to %q source to restart VM on source %q for next migration window: %w", src.SourceType, src.Name, err)
		}

		err = is.PowerOnVM(ctx, state.Instances[instUUID].Properties.Location)
		if err != nil {
			return fmt.Errorf("Failed to restart VM on source %q for next migration window: %w", src.Name, err)
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*120)
	defer cancel()

	it, err := target.NewTarget(state.Targets[instUUID].ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to set up %q target-specific configuration: %w", state.Targets[instUUID].TargetType, err)
	}

	err = it.Connect(timeoutCtx)
	if err != nil {
		return fmt.Errorf("Failed to connect to target %q: %w", it.GetName(), err)
	}

	err = it.SetProject(state.QueueEntries[instUUID].Placement.TargetProject)
	if err != nil {
		return fmt.Errorf("Failed to set target %q project %q: %w", it.GetName(), state.QueueEntries[instUUID].Placement.TargetProject, err)
	}

	// If the VM failed in post-import steps, then it needs to be fully cleaned up.
	resetState := api.MIGRATIONSTATUS_IDLE
	resetImportStage := migration.IMPORTSTAGE_FINAL
	if state.QueueEntries[instUUID].MigrationStatus != api.MIGRATIONSTATUS_FINAL_IMPORT {
		resetState = api.MIGRATIONSTATUS_WAITING
		resetImportStage = migration.IMPORTSTAGE_BACKGROUND
		log.Warn("Cleaning up target instance due to migration window deadline")
		err := it.CleanupVM(timeoutCtx, state.Instances[instUUID].Properties.Name, false)
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
			err := d.configureMigratedInstances(ctx, state.QueueEntries[instUUID], window, instance, state.Sources[instUUID], state.Targets[instUUID], state.Batch, migration.FilterUsedNetworks(allNetworks, instanceList))
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
			if numRetries < batch.Config.PostMigrationRetries {
				d.instance.RecordPostMigrationRetry(i.UUID)
				log.Error("Instance failed post-migration steps, retrying", slog.String("error", errString), slog.Int("retry_count", numRetries), slog.Int("max_retries", batch.Config.PostMigrationRetries))
				return
			}

			// Only persist the state as errored if the window is still active, because this reverter might have been triggered by the window deadline cleanup.
			_, err := d.queue.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, errString, migration.IMPORTSTAGE_BACKGROUND, q.GetWindowID())
			if err != nil {
				log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
			}
		}

		// VM wasn't initially running, so no need to turn it back on.
		if !i.Properties.Running {
			return
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

	it, err := target.NewTarget(t.ToAPI())
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
	err = it.SetProject(q.Placement.TargetProject)
	if err != nil {
		return fmt.Errorf("Failed to set target %q project %q: %w", it.GetName(), q.Placement.TargetProject, err)
	}

	err = it.SetPostMigrationVMConfig(timeoutCtx, i, q, activeNetworks)
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
