package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
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
	"github.com/FuturFusion/migration-manager/shared/api/event"
)

type Task string

const (
	SyncTask       Task = "sync"
	ImportTask     Task = "import"
	PostImportTask Task = "post-import"
	ACMEUpdateTask Task = "acme-update"
)

func (d *Daemon) runPeriodicTask(ctx context.Context, task Task, f func(context.Context) error, interval time.Duration) {
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}

			// Don't run the sync task if it's disabled.
			if task != SyncTask || !d.config.Settings.DisableAutoSync {
				slog.Debug("Running periodic task", slog.Any("task", task))
				err := f(ctx)
				if err != nil {
					slog.Error("Failed to run periodic task", slog.String("task", string(task)), logger.Err(err))
				}
			}

			// Override the default duration for sync tasks.
			if task == SyncTask {
				interval = d.config.Settings.SyncInterval.Duration
			}

			now := time.Now().UTC()
			ctx, cancel := context.WithTimeout(ctx, interval)
			defer cancel()

			if task != SyncTask {
				<-ctx.Done()
				cancel()
				continue
			}

			// If this is a sync task, check every 1s in case the interval changed and restart the parent loop if the new interval has already passed.
			var done bool
			for !done {
				select {
				case <-ctx.Done():
					done = true
				default:
					if time.Now().UTC().After(now.Add(d.config.Settings.SyncInterval.Duration)) {
						done = true
						continue
					}

					time.Sleep(1 * time.Second)
				}
			}

			cancel()
		}
	}()
}

func (d *Daemon) reassessBlockedInstances(ctx context.Context) error {
	state, err := d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_RUNNING, api.MIGRATIONSTATUS_BLOCKED, api.MIGRATIONSTATUS_WAITING)
	if err != nil {
		return fmt.Errorf("Failed to fetch blocked and waiting migration state: %w", err)
	}

	entries := state.Queue()
	if len(entries) == 0 {
		return nil
	}

	slog.Info("Assessing queued instances for target import")
	artifacts, err := d.artifact.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("Failed to fetch artifact records: %w", err)
	}

	blockedInstances := map[uuid.UUID]string{}
	for _, q := range entries {
		inst := state[q.BatchName].Instances[q.InstanceUUID]
		err := d.artifact.HasRequiredArtifactsForInstance(artifacts, inst)
		if err != nil {
			slog.Error("Blocking queue entries due to artifact error", slog.Any("error", err))
			blockedInstances[inst.UUID] = fmt.Sprintf("Artifact error: %v", err.Error())
			continue
		}

		_, err = d.os.WorkerImageExists(inst.GetArchitecture())
		if err != nil {
			slog.Error("Blocking queue entries due to filesystem error", slog.Any("error", err))
			blockedInstances[inst.UUID] = fmt.Sprintf("Filesystem error: %v", err.Error())
		}
	}

	instancesBySource := map[string]migration.Instances{}
	for _, q := range entries {
		inst := state[q.BatchName].Instances[q.InstanceUUID]
		if instancesBySource[inst.Source] == nil {
			instancesBySource[inst.Source] = migration.Instances{}
		}

		instancesBySource[inst.Source] = append(instancesBySource[inst.Source], inst)
	}

	for srcName, instances := range instancesBySource {
		q := entries[instances[0].UUID]
		src, ok := state[q.BatchName].Sources[q.InstanceUUID]
		if !ok {
			continue
		}

		verifyImport := slices.ContainsFunc(instances, func(i migration.Instance) bool { return i.NeedsBackgroundImportVerification() })
		if !verifyImport {
			continue
		}

		is, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
		if err != nil {
			return fmt.Errorf("Failed to parse %q source %q: %w", src.SourceType, src.Name, err)
		}

		err = is.Connect(ctx)
		if err != nil {
			return fmt.Errorf("Failed to connect to %q source %q: %w", src.SourceType, src.Name, err)
		}

		verifiedVMs, err := is.VerifyBackgroundImport(ctx, instances)
		if err != nil {
			return fmt.Errorf("Failed to verify VM background import support for source %q: %w", srcName, err)
		}

		for _, inst := range verifiedVMs {
			disks := make([]string, 0, len(inst.Properties.Disks))
			for _, d := range inst.Properties.Disks {
				disks = append(disks, d.Name)
			}

			newInst, err := d.instance.SetBackgroundImportVerified(ctx, inst.UUID, inst.Properties.BackgroundImport, disks)
			if err != nil {
				return fmt.Errorf("Failed to update instance %q background import support: %w", inst.Properties.Location, err)
			}

			// Update the instance in the state cache.
			state[q.BatchName].Instances[inst.UUID] = *newInst
		}
	}

	for _, q := range entries {
		// Block all entries if we failed to validate the filesystem.
		if blockedInstances[q.InstanceUUID] != "" {
			if q.MigrationStatusMessage != blockedInstances[q.InstanceUUID] {
				_, err := d.queue.UpdateStatusByUUID(ctx, q.InstanceUUID, api.MIGRATIONSTATUS_BLOCKED, blockedInstances[q.InstanceUUID], q.ImportStage, q.GetWindowName())
				if err != nil {
					return fmt.Errorf("Failed to unblock queue entry %q: %w", q.InstanceUUID, err)
				}
			}

			continue
		}

		inst := state[q.BatchName].Instances[q.InstanceUUID]

		// Otherwise check why the VM is blocked, and unblock it if needed.
		err := inst.DisabledReason(state[q.BatchName].Batch.Config.RestrictionOverrides)
		if err != nil {
			slog.Warn("Instance is blocked from migration", slog.String("location", inst.Properties.Location), slog.String("reason", err.Error()))
			if err.Error() != q.MigrationStatusMessage {
				_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_BLOCKED, err.Error(), q.ImportStage, q.GetWindowName())
				if err != nil {
					return fmt.Errorf("Failed to update queue entry block message for %q: %w", inst.Properties.Location, err)
				}
			}

			continue
		}

		// If we passed every check, and the old status was Blocked, then unblock the queue entry.
		if q.MigrationStatus == api.MIGRATIONSTATUS_BLOCKED {
			_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_WAITING, "Instance has been unblocked", q.ImportStage, q.GetWindowName())
			if err != nil {
				return fmt.Errorf("Failed to unblock queue entry for %q: %w", inst.Properties.Location, err)
			}
		}
	}

	return nil
}

// workerLock synchronizes worker tasks with other API actions.
// Worker tasks should grab the read lock since they can be concurrent with each other,
// but user API actions should grab the write lock, making them exclusive with the full set of periodic tasks.
var workerLock sync.RWMutex

// beginImports creates the target VMs for started batches.
// It fetches all RUNNING batches with WAITING or BLOCKED instances, and moves the instances to CREATING state.
// Errors encountered in one batch do not affect the processing of other batches.
//   - cleanupInstances determines whether to delete failed target VMs on errors.
//     If true, errors will not result in the instance state being set to ERROR, to enable retrying this task.
//     If any errors occur after the VM has started, the VM will no longer be cleaned up, and its state will be set to ERROR, preventing retries.
func (d *Daemon) beginImports(ctx context.Context, cleanupInstances bool) error {
	workerLock.RLock()
	defer workerLock.RUnlock()

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
				placementLock.Lock()
				entry := state.QueueEntries[instUUID]
				placementLock.Unlock()

				if state.Batch.Config.RerunScriptlets {
					usedNetworks := migration.FilterUsedNetworks(allNetworks, migration.Instances{instance})
					windows := make(migration.Windows, 0, len(state.Windows))
					for _, w := range state.Windows {
						windows = append(windows, w)
					}

					placement, err := d.batch.DeterminePlacement(ctx, instance, usedNetworks, state.Batch, windows)
					if err != nil {
						return err
					}

					// Update the migration state with the queue entry's new placement.
					placementLock.Lock()
					entry.Placement = *placement
					state.QueueEntries[instUUID] = entry
					migrationState[batchName] = state
					placementLock.Unlock()
				}

				var info *target.IncusDetails
				for _, t := range targetInfo {
					if t.Name == entry.Placement.TargetName {
						info = &t
						break
					}
				}

				// Verify that the target placement actually exists and the instance can be placed there.
				err := target.CanPlaceInstance(ctx, info, entry.Placement, instance.ToAPI(), state.Batch.ToAPI(nil))
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
					_, err := d.queue.UpdateStatusByUUID(ctx, q.InstanceUUID, api.MIGRATIONSTATUS_BLOCKED, blockedMsg, q.ImportStage, q.GetWindowName())
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
					err := d.ensureISOImagesExistInStoragePool(ctx, state.Instances[instUUID], state.Targets[instUUID], state.Batch, pool, q.Placement.TargetProject)
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
				_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_CREATING, "Creating target instance definition", state.QueueEntries[inst.UUID].ImportStage, state.QueueEntries[inst.UUID].GetWindowName())
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
			return d.createTargetVM(ctx, state.Batch, inst, state.Targets[inst.UUID], state.Sources[instUUID], state.QueueEntries[instUUID], instanceNetworks, state.Windows, cleanupInstances)
		})
	})
	if err != nil {
		return fmt.Errorf("Failed to initialize migration workers: %w", err)
	}

	return nil
}

// ensureISOImagesExistInStoragePool ensures the necessary image files exist on the daemon to be imported to the storage volume.
func (d *Daemon) ensureISOImagesExistInStoragePool(ctx context.Context, instance migration.Instance, tgt migration.Target, batch migration.Batch, pool string, project string) error {
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

	ctx, cancel := context.WithTimeout(ctx, it.Timeout())
	defer cancel()

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

	arch := instance.GetArchitecture()
	workerVolumeExists := slices.Contains(volumes, "custom/"+util.WorkerVolume(arch))

	// If we need to download missing files, or upload them to the target, set a status message.
	if !workerVolumeExists {
		_, err := d.batch.UpdateStatusByName(ctx, batch.Name, batch.Status, "Uploading worker volume")
		if err != nil {
			return fmt.Errorf("Failed to update batch %q status message: %w", batch.Name, err)
		}

		log.Info("Worker image doesn't exist in storage pool, importing...")
		workerPath, err := d.os.LoadWorkerImage(ctx, arch)
		if err != nil {
			return err
		}

		ops, cleanup, err := it.CreateStoragePoolVolumeFromBackup(ctx, pool, workerPath, arch, util.WorkerVolume(arch))
		if err != nil {
			return err
		}

		// Always clean up created files.
		defer cleanup()

		for _, op := range ops {
			err = op.WaitContext(ctx)
			if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
				return err
			}
		}

		_, err = d.batch.UpdateStatusByName(ctx, batch.Name, batch.Status, string(api.BATCHSTATUS_RUNNING))
		if err != nil {
			return fmt.Errorf("Failed to update batch %q status message: %w", batch.Name, err)
		}
	}

	d.batchLock.Unlock(batchKey)
	reverter.Success()

	return nil
}

// Concurrently create target VMs for each instance record.
// Any instance that fails the migration has its state set to ERROR.
// - cleanupInstances determines whether a target VM should be deleted if it encounters an error.
func (d *Daemon) createTargetVM(ctx context.Context, b migration.Batch, inst migration.Instance, t migration.Target, s migration.Source, q migration.QueueEntry, networks migration.Networks, windows map[string]migration.Window, cleanupInstances bool) (_err error) {
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
			_, err := d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_WAITING, errString, migration.IMPORTSTAGE_BACKGROUND, q.GetWindowName())
			if err != nil {
				log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_WAITING), logger.Err(err))
			}

			return
		}

		// Try to set the instance state to ERRORED if it failed.
		_, err := d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, errString, migration.IMPORTSTAGE_BACKGROUND, q.GetWindowName())
		if err != nil {
			log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}
	})

	it, err := target.NewTarget(t.ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to construct target %q: %w", t.Name, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, it.Timeout())
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
	var migrationNetwork api.MigrationNetworkPlacement
	for _, n := range b.Defaults.MigrationNetwork {
		if n.Target == q.Placement.TargetName && n.TargetProject == q.Placement.TargetProject {
			migrationNetwork = n
			break
		}
	}

	instanceDef, err := it.CreateVMDefinition(inst, usedNetworks, q, incusTLS.CertFingerprint(cert), d.getWorkerEndpoint(), migrationNetwork)
	if err != nil {
		return fmt.Errorf("Failed to create instance definition: %w", err)
	}

	cleanup, err := it.CreateNewVM(timeoutCtx, inst, instanceDef, q.Placement, util.WorkerVolume(inst.GetArchitecture()))
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

	var window migration.Window
	if q.GetWindowName() != nil {
		window = windows[*q.GetWindowName()]
	} else {
		w, err := d.queue.GetNextWindow(ctx, q)
		if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
			return err
		}

		if w != nil {
			window = *w
		}
	}

	d.logHandler.SendLifecycle(ctx, event.NewMigrationEvent(event.MigrationCreated, inst.ToAPI(), q.ToAPI(inst.GetName(), d.queueHandler.LastWorkerUpdate(inst.UUID), window)))

	// Unblock the concurrency limits for the target so that the Incus agent doesn't block other creations.
	d.target.RemoveCreation(t.Name)

	err = it.CheckIncusAgent(timeoutCtx, instanceDef.Name)
	if err != nil {
		return err
	}

	// Set the instance state to IDLE before triggering the worker.
	_, err = d.queue.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_IDLE, "Waiting for worker to connect", migration.IMPORTSTAGE_BACKGROUND, q.GetWindowName())
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

	it, err := target.NewTarget(state.Targets[instUUID].ToAPI())
	if err != nil {
		return fmt.Errorf("Failed to set up %q target-specific configuration: %w", state.Targets[instUUID].TargetType, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, it.Timeout())
	defer cancel()

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
		err := it.CleanupVM(timeoutCtx, state.Instances[instUUID].GetName(), false)
		if err != nil {
			return fmt.Errorf("Failed to clean up instance %q due to migration window deadline: %w", state.Instances[instUUID].Properties.Location, err)
		}
	} else {
		// Stop the migration worker so it doesn't interfere with our state cleanup.
		err = it.Exec(timeoutCtx, state.Instances[instUUID].GetName(), []string{"systemctl", "stop", "migration-manager-worker.service"})
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
		err := it.Exec(timeoutCtx, state.Instances[instUUID].GetName(), []string{"systemctl", "restart", "migration-manager-worker.service"})
		if err != nil {
			return fmt.Errorf("Failed to restart migration worker on restarting instance %q: %w", state.Instances[instUUID].Properties.Location, err)
		}
	}

	return nil
}

// finalizeCompleteInstances fetches all instances in RUNNING batches whose status is WORKER DONE, and for each batch, runs configureMigratedInstances.
func (d *Daemon) finalizeCompleteInstances(ctx context.Context) (_err error) {
	workerLock.RLock()
	defer workerLock.RUnlock()

	log := slog.With(slog.String("method", "finalizeCompleteInstances"))
	var migrationState queue.BatchMigrationState

	queueEntriesToReset := map[uuid.UUID]bool{}
	windowsByQueueUUID := map[uuid.UUID]migration.Window{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		migrationState, err = d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_RUNNING, api.MIGRATIONSTATUS_WORKER_DONE, api.MIGRATIONSTATUS_FINAL_IMPORT, api.MIGRATIONSTATUS_POST_IMPORT)
		if err != nil {
			return fmt.Errorf("Failed to compile migration state for final import steps: %w", err)
		}

		for _, s := range migrationState {
			for _, q := range s.QueueEntries {
				windowName := q.GetWindowName()
				if windowName == nil {
					continue
				}

				windowsByQueueUUID[q.InstanceUUID] = s.Windows[*windowName]
				if windowsByQueueUUID[q.InstanceUUID].Ended() {
					queueEntriesToReset[q.InstanceUUID] = true
				}
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = d.finalSourceAndTargetChecks(ctx, migrationState)
	if err != nil {
		return fmt.Errorf("Failed to perform final source and target checks: %w", err)
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

			window := windowsByQueueUUID[instUUID]
			err := d.configureMigratedInstances(ctx, state.QueueEntries[instUUID], window, instance, state.Sources[instUUID], state.Targets[instUUID], state.Batch)
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

// finalSourceAndTargetChecks performs one final source sync for each instance in the migration state.
// If the instance has changed, the instance record will be updated, new networks will be recorded, and several checks will be performed:
// - Ensure the instance UUID, name, and disks did not change, otherwise place the queue entry into an ERROR state.
// - Ensure the instance can still be placed on the associated target, otherwise place the queue entry into a recoverable CONFLICT state.
func (d *Daemon) finalSourceAndTargetChecks(ctx context.Context, migrationState queue.BatchMigrationState) error {
	log := slog.With(slog.String("method", "finalSourceAndTargetChecks"))
	existingNetworks := migration.Networks{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		existingNetworks, err = d.network.GetAll(ctx)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	instancesByUUID := map[uuid.UUID]migration.Instance{}
	instanceSourceIDs := map[string][]string{}
	sourcesByName := map[string]migration.Source{}
	batchesByInstance := map[uuid.UUID]string{}
	for _, s := range migrationState {
		for _, inst := range s.Instances {
			if s.QueueEntries[inst.UUID].MigrationStatus != api.MIGRATIONSTATUS_WORKER_DONE {
				continue
			}

			batchesByInstance[inst.UUID] = s.Batch.Name
			instancesByUUID[inst.UUID] = inst
			if instanceSourceIDs[inst.Source] == nil {
				instanceSourceIDs[inst.Source] = []string{}
			}

			instanceSourceIDs[inst.Source] = append(instanceSourceIDs[inst.Source], inst.Properties.SourceSpecificID)
		}

		for instUUID, src := range s.Sources {
			if s.QueueEntries[instUUID].MigrationStatus != api.MIGRATIONSTATUS_WORKER_DONE {
				continue
			}

			_, ok := sourcesByName[src.Name]
			if !ok {
				sourcesByName[src.Name] = src
			}
		}
	}

	// Key networks by their source and source ID because imported networks won't have a UUID yet.
	networksBySourceAndID := map[string]map[string]migration.Network{}
	for _, n := range existingNetworks {
		if networksBySourceAndID[n.Source] == nil {
			networksBySourceAndID[n.Source] = map[string]migration.Network{}
		}

		networksBySourceAndID[n.Source][n.SourceSpecificID] = n
	}

	// Get new source information.
	srcInsts := migration.Instances{}
	srcNetworks := migration.Networks{}
	clientsBySourceName := map[string]source.Source{}
	for srcName, ids := range instanceSourceIDs {
		s, err := source.NewInternalVMwareSourceFrom(sourcesByName[srcName].ToAPI())
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(ctx, s.ConnectionTimeout.Duration)
		err = s.Connect(ctx)
		if err != nil {
			cancel()
			return err
		}

		insts, networks, _, err := s.GetAllVMs(ctx, ids...)
		cancel()
		if err != nil {
			return err
		}

		if clientsBySourceName[srcName] == nil {
			clientsBySourceName[srcName] = s
		}

		srcInsts = append(srcInsts, insts...)
		srcNetworks = append(srcNetworks, networks...)
	}

	type conflictState struct {
		status     api.MigrationStatusType
		message    string
		properties api.InstanceProperties
		placement  *api.Placement
	}

	targetDetails := map[string]*target.IncusDetails{}
	updatedInstances := map[uuid.UUID]conflictState{}
	for _, srcInst := range srcInsts {
		log := log.With(slog.String("location", srcInst.Properties.Location), slog.String("uuid", srcInst.UUID.String()), slog.String("source", srcInst.Source), slog.String("id", srcInst.Properties.SourceSpecificID))
		var matchingBatch string
		var oldUUID uuid.UUID
		for batchName, state := range migrationState {
			for _, inst := range state.Instances {
				if inst.Source == srcInst.Source && inst.Properties.SourceSpecificID == srcInst.Properties.SourceSpecificID {
					matchingBatch = batchName
					oldUUID = inst.UUID
					break
				}
			}

			if matchingBatch != "" {
				break
			}
		}

		if matchingBatch == "" || oldUUID == uuid.Nil {
			log.Error("Found instance not matching any queue entry")
			continue
		}

		state := migrationState[matchingBatch]
		oldInst := state.Instances[oldUUID]

		// Only consider instances that have finished migrating.
		if state.QueueEntries[oldUUID].MigrationStatus != api.MIGRATIONSTATUS_WORKER_DONE {
			continue
		}

		log.Info("Performing final source and target checks")

		if srcInst.Properties.Running {
			log.Info("Detected source instance is still running, powering off before proceeding")
			s := clientsBySourceName[srcInst.Source]
			ctx, cancel := context.WithTimeout(ctx, s.Timeout())
			err = s.PowerOffVM(ctx, srcInst.Properties.Location)
			cancel()
			if err != nil {
				return fmt.Errorf("Failed to power off source VM for final source and target checks: %w", err)
			}

			srcInst.Properties.Running = false
		}

		// ApplyUpdates won't change the UUID so check it explicitly.
		if oldInst.UUID != srcInst.UUID {
			log.Error("Instance identifying information changed, canceling migration")
			updatedInstances[oldInst.UUID] = conflictState{
				status:     api.MIGRATIONSTATUS_ERROR,
				message:    fmt.Sprintf("Instance source UUID has changed (expected %q, found %q)", oldInst.UUID, srcInst.UUID),
				properties: srcInst.Properties,
			}

			continue
		}

		updatedInst, updated := oldInst.ApplyUpdates(srcInst)
		if !updated {
			log.Info("Instance has not changed, skipping further checks")
			continue
		}

		if oldInst.GetName() != updatedInst.GetName() {
			log.Error("Instance identifying information changed, canceling migration")
			updatedInstances[oldInst.UUID] = conflictState{
				status:     api.MIGRATIONSTATUS_ERROR,
				message:    fmt.Sprintf("Instance source name has changed (expected %q, found %q)", oldInst.GetName(), updatedInst.GetName()),
				properties: updatedInst.Properties,
			}

			continue
		}

		if !slices.Equal(oldInst.Properties.Disks, updatedInst.Properties.Disks) {
			log.Error("Instance identifying information changed, canceling migration")
			updatedInstances[oldInst.UUID] = conflictState{
				status:     api.MIGRATIONSTATUS_ERROR,
				message:    "Instance source disks have changed",
				properties: updatedInst.Properties,
			}

			continue
		}

		windows := migration.Windows{}
		for _, w := range state.Windows {
			windows = append(windows, w)
		}

		allNetworks := existingNetworks
		for i, n := range srcNetworks {
			if networksBySourceAndID[n.Source] == nil {
				continue
			}

			_, ok := networksBySourceAndID[n.Source][n.SourceSpecificID]
			if !ok {
				n.UUID, err = uuid.NewRandom()
				if err != nil {
					return err
				}

				srcNetworks[i] = n
				allNetworks = append(allNetworks, n)
			}
		}

		oldPlacement := state.QueueEntries[updatedInst.UUID].Placement
		usedNetworks := migration.FilterUsedNetworks(allNetworks, migration.Instances{updatedInst})
		log.Info("Recomputing placement", slog.Bool("use_scriptlet", state.Batch.Config.PlacementScriptlet != ""))
		newPlacement, err := d.batch.DeterminePlacement(ctx, updatedInst, usedNetworks, state.Batch, windows)
		if err != nil {
			return err
		}

		var replacePlacement *api.Placement
		if oldPlacement.TargetName != newPlacement.TargetName || oldPlacement.TargetProject != newPlacement.TargetProject || !maps.Equal(oldPlacement.StoragePools, newPlacement.StoragePools) {
			log.Warn("Instance placement determination changed", slog.Bool("blocking", !state.Batch.Defaults.ForceConflictResolution))
			if !state.Batch.Defaults.ForceConflictResolution {
				updatedInstances[oldInst.UUID] = conflictState{
					status:     api.MIGRATIONSTATUS_CONFLICT,
					message:    fmt.Sprintf("Instance placement changed: %v", err),
					properties: updatedInst.Properties,
				}

				continue
			}

			log.Info("Using previous placement to force conflict resolution")
		} else if !maps.Equal(oldPlacement.Networks, newPlacement.Networks) {
			log.Info("Placement contains new networks")
			replacePlacement = newPlacement

			// Preserve the initial placement's running state, as the source VM will always be powered off at this point.
			replacePlacement.Running = oldPlacement.Running
		}

		details, ok := targetDetails[state.Targets[oldInst.UUID].Name]
		if !ok {
			t, err := target.NewTarget(state.Targets[oldInst.UUID].ToAPI())
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(ctx, t.Timeout())
			err = t.Connect(ctx)
			if err != nil {
				cancel()
				return err
			}

			details, err = t.GetDetails(ctx)
			cancel()
			if err != nil {
				return err
			}

			targetDetails[state.Targets[oldInst.UUID].Name] = details
		}

		q := state.QueueEntries[oldInst.UUID]
		if replacePlacement != nil {
			q.Placement = *replacePlacement
		}

		err = target.CanPlaceInstance(ctx, details, q, updatedInst.ToAPI(), state.Batch.ToAPI(windows))
		if err != nil {
			log.Warn("Instance cannot be placed", slog.Bool("blocking", !state.Batch.Defaults.ForceConflictResolution), slog.Any("error", err))
			if state.Batch.Defaults.ForceConflictResolution {
				log.Info("Using previous instance configuration to force conflict resolution")
				continue
			}

			updatedInstances[oldInst.UUID] = conflictState{
				status:     api.MIGRATIONSTATUS_CONFLICT,
				message:    fmt.Sprintf("Instance can no longer be placed on the target: %v", err),
				properties: updatedInst.Properties,
				placement:  replacePlacement,
			}

			continue
		}

		// If we got this far, just update the instance with no conflict or error.
		updatedInstances[oldInst.UUID] = conflictState{properties: updatedInst.Properties, placement: replacePlacement}
	}

	// No conflicts. Update the instance and create additional network records.
	return transaction.Do(ctx, func(ctx context.Context) error {
		for _, n := range srcNetworks {
			if networksBySourceAndID[n.Source] == nil {
				continue
			}

			_, ok := networksBySourceAndID[n.Source][n.SourceSpecificID]
			if !ok {
				log.Info("Creating new detected network record", slog.String("network", n.Location))
				n, err = d.network.Create(ctx, n)
				if err != nil {
					return err
				}

				networksBySourceAndID[n.Source][n.SourceSpecificID] = n
			}
		}

		for id, data := range updatedInstances {
			batchName := batchesByInstance[id]
			state := migrationState[batchName]
			inst := instancesByUUID[id]
			inst.Properties = data.properties
			for i, nic := range inst.Properties.NICs {
				network, ok := networksBySourceAndID[inst.Source][nic.SourceSpecificID]
				if nic.UUID == uuid.Nil && ok {
					inst.Properties.NICs[i].UUID = network.UUID
				}
			}

			log.Info("Updating instance record")
			err := d.instance.Update(ctx, &inst, true)
			if err != nil {
				return err
			}

			if data.message != "" {
				log.Info("Updating queue migration status", slog.String("status", string(data.status)))
				q, err := d.queue.UpdateStatusByUUID(ctx, id, data.status, data.message, state.QueueEntries[id].ImportStage, state.QueueEntries[id].GetWindowName())
				if err != nil {
					return err
				}

				state.QueueEntries[id] = *q
				migrationState[batchName] = state
			}

			if data.placement != nil {
				log.Info("Updating queue placement")
				q, err := d.queue.UpdatePlacementByUUID(ctx, id, *data.placement)
				if err != nil {
					return err
				}

				state.QueueEntries[id] = *q
				migrationState[batchName] = state
			}

			state.Instances[inst.UUID] = inst
			migrationState[batchName] = state
		}

		return nil
	})
}

// configureMigratedInstances updates the configuration of instances concurrently after they have finished migrating. Errors will result in the instance state becoming ERRORED.
// If an instance succeeds, its state will be moved to FINISHED.
func (d *Daemon) configureMigratedInstances(ctx context.Context, q migration.QueueEntry, w migration.Window, i migration.Instance, s migration.Source, t migration.Target, batch migration.Batch) (_err error) {
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
			_, err := d.queue.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, errString, migration.IMPORTSTAGE_BACKGROUND, q.GetWindowName())
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

	timeoutCtx, cancel := context.WithTimeout(ctx, it.Timeout())
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

	err = it.SetPostMigrationVMConfig(timeoutCtx, i, q)
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
