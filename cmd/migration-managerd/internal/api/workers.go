package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	incusUtil "github.com/lxc/incus/v6/shared/util"

	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (d *Daemon) runPeriodicTask(f func() bool, interval time.Duration) {
	go func() {
		for {
			done := f()
			if done {
				return
			}

			t := time.NewTimer(interval)

			select {
			case <-d.ShutdownCtx.Done():
				t.Stop()
				return
			case <-t.C:
				t.Stop()
			}
		}
	}()
}

func (d *Daemon) syncInstancesFromSources() bool {
	log := slog.With(slog.String("method", "syncInstancesFromSources"))

	// TODO: context should be passed from the daemon to all the workers.
	ctx := context.TODO()

	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the list of configured sources.
		sources, err := d.source.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all sources: %w", err)
		}

		// Check each source for any net networks and any new, changed, or deleted instances.
		for _, src := range sources {
			log := log.With(slog.String("source", src.Name))

			s, err := source.NewInternalVMwareSourceFrom(api.Source{
				Name:       src.Name,
				DatabaseID: src.ID,
				SourceType: src.SourceType,
				Properties: src.Properties,
			})
			if err != nil {
				log.Warn("Failed to create VMwareSource from source", logger.Err(err))
				continue
			}

			err = s.Connect(d.ShutdownCtx)
			if err != nil {
				log.Warn("Failed to connect to source", logger.Err(err))
				continue
			}

			networks, err := s.GetAllNetworks(d.ShutdownCtx)
			if err != nil {
				log.Warn("Failed to get networks", logger.Err(err))
				continue
			}

			// Iterate each network from this source.
			for _, n := range networks {
				log := log.With(slog.String("network", n.Name))

				// Check if a network already exists with the same name.
				_, err = d.network.GetByName(ctx, n.Name)
				if errors.Is(err, migration.ErrNotFound) {
					log.Info("Adding network from source")
					_, err = d.network.Create(ctx, migration.Network{
						Name:   n.Name,
						Config: n.Config,
					})
					if err != nil {
						log.Warn("Failed to add network", logger.Err(err))
					}

					continue
				}

				if err != nil {
					log.Warn("Failed to get network", logger.Err(err))
				}
			}

			instances, err := s.GetAllVMs(d.ShutdownCtx)
			if err != nil {
				log.Warn("Failed to get VMs", logger.Err(err))
				continue
			}

			currentInstancesFromSource := make(map[uuid.UUID]bool)

			// Iterate each instance from this source.
			for _, i := range instances {
				log := log.With(
					slog.String("instance", i.InventoryPath),
					slog.Any("instance_uuid", i.UUID),
				)

				// Check if this instance is already in the database.
				existingInstance, err := d.instance.GetByID(ctx, i.UUID)
				if err != nil && !errors.Is(err, migration.ErrNotFound) {
					log.Warn("Failed to query DB for instance", slog.Any("instance", i.UUID), logger.Err(err))
					continue
				}

				if err == nil {
					// An instance already exists in the database; update with any changes from the source.
					instanceUpdated := false

					if existingInstance.Annotation != i.Annotation {
						existingInstance.Annotation = i.Annotation
						instanceUpdated = true
					}

					if existingInstance.GuestToolsVersion != i.GuestToolsVersion {
						existingInstance.GuestToolsVersion = i.GuestToolsVersion
						instanceUpdated = true
					}

					if existingInstance.Architecture != i.Architecture {
						existingInstance.Architecture = i.Architecture
						instanceUpdated = true
					}

					if existingInstance.HardwareVersion != i.HardwareVersion {
						existingInstance.HardwareVersion = i.HardwareVersion
						instanceUpdated = true
					}

					if existingInstance.OS != i.OS {
						existingInstance.OS = i.OS
						instanceUpdated = true
					}

					if existingInstance.OSVersion != i.OSVersion {
						existingInstance.OSVersion = i.OSVersion
						instanceUpdated = true
					}

					if !slices.Equal(existingInstance.Devices, i.Devices) {
						existingInstance.Devices = i.Devices
						instanceUpdated = true
					}

					if !slices.Equal(existingInstance.Disks, i.Disks) {
						existingInstance.Disks = i.Disks
						instanceUpdated = true
					}

					if !slices.Equal(existingInstance.NICs, i.NICs) {
						existingInstance.NICs = i.NICs
						instanceUpdated = true
					}

					if !slices.Equal(existingInstance.Snapshots, i.Snapshots) {
						existingInstance.Snapshots = i.Snapshots
						instanceUpdated = true
					}

					if existingInstance.CPU.NumberCPUs != i.CPU.NumberCPUs {
						existingInstance.CPU.NumberCPUs = i.CPU.NumberCPUs
						instanceUpdated = true
					}

					if !slices.Equal(existingInstance.CPU.CPUAffinity, i.CPU.CPUAffinity) {
						existingInstance.CPU.CPUAffinity = i.CPU.CPUAffinity
						instanceUpdated = true
					}

					if existingInstance.CPU.NumberOfCoresPerSocket != i.CPU.NumberOfCoresPerSocket {
						existingInstance.CPU.NumberOfCoresPerSocket = i.CPU.NumberOfCoresPerSocket
						instanceUpdated = true
					}

					if existingInstance.Memory.MemoryInBytes != i.Memory.MemoryInBytes {
						existingInstance.Memory.MemoryInBytes = i.Memory.MemoryInBytes
						instanceUpdated = true
					}

					if existingInstance.Memory.MemoryReservationInBytes != i.Memory.MemoryReservationInBytes {
						existingInstance.Memory.MemoryReservationInBytes = i.Memory.MemoryReservationInBytes
						instanceUpdated = true
					}

					if existingInstance.UseLegacyBios != i.UseLegacyBios {
						existingInstance.UseLegacyBios = i.UseLegacyBios
						instanceUpdated = true
					}

					if existingInstance.SecureBootEnabled != i.SecureBootEnabled {
						existingInstance.SecureBootEnabled = i.SecureBootEnabled
						instanceUpdated = true
					}

					if existingInstance.TPMPresent != i.TPMPresent {
						existingInstance.TPMPresent = i.TPMPresent
						instanceUpdated = true
					}

					if instanceUpdated {
						log.Info("Syncing changes to instance from source")
						existingInstance.LastUpdateFromSource = i.LastUpdateFromSource
						_, err := d.instance.UpdateByID(ctx, existingInstance)
						if err != nil {
							log.Warn("Failed to update instance", logger.Err(err))
							continue
						}
					}
				} else {
					// Add a new instance to the database.
					log.Info("Adding instance from source to database")

					_, err = d.instance.Create(ctx, i)
					if err != nil {
						log.Warn("Failed to add instance", logger.Err(err))
						continue
					}
				}

				// Record that this instance exists.
				currentInstancesFromSource[i.UUID] = true
			}

			// Remove instances that no longer exist in this source.
			allDBInstances, err := d.instance.GetAll(ctx)
			if err != nil {
				log.Warn("Failed to get instances", logger.Err(err))
				continue
			}

			for _, i := range allDBInstances {
				log := log.With(
					slog.String("instance", i.InventoryPath),
					slog.Any("instance_uuid", i.UUID),
				)

				_, instanceExists := currentInstancesFromSource[i.UUID]
				if !instanceExists {
					log.Info("Instance removed from source")
					err := d.instance.DeleteByID(ctx, i.UUID)
					if err != nil {
						log.Warn("Failed to delete instance", logger.Err(err))
						continue
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Warn("Sync Instances From Sources worker failed", logger.Err(err))
	}

	return false
}

func (d *Daemon) processReadyBatches() bool {
	log := slog.With(slog.String("method", "processReadyBatches"))

	// TODO: context should be passed from the daemon to all the workers.
	ctx := context.TODO()

	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_READY)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state: %w", err)
		}

		// Do some basic sanity check of each batch before adding it to the queue.
		for _, b := range batches {
			log := log.With(slog.String("batch", b.Name))

			log.Info("Batch status is 'Ready', processing....")

			// If a migration window is defined, ensure sure it makes sense.
			if !b.MigrationWindowStart.IsZero() && !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(b.MigrationWindowStart) {
				log.Error("Batch window end time is before its start time")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "Migration window end before start")
				if err != nil {
					log.Warn("Failed to update batch status", logger.Err(err))
					continue
				}

				continue
			}

			if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
				log.Error("Batch window end time has already passed")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					log.Warn("Failed to update batch status", logger.Err(err))
					continue
				}

				continue
			}

			// Get all instances for this batch.
			instances, err := d.instance.GetAllByBatchID(ctx, b.ID)
			if err != nil {
				log.Warn("Failed to get instances for batch", logger.Err(err))
				continue
			}

			// If no instances apply to this batch, return an error.
			if len(instances) == 0 {
				log.Error("Batch has no instances")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "No instances assigned")
				if err != nil {
					log.Warn("Failed to update batch status", logger.Err(err))
					continue
				}

				continue
			}

			// No issues detected, move to "queued" status.
			log.Info("Updating batch status to 'Queued'")

			_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_QUEUED, api.BATCHSTATUS_QUEUED.String())
			if err != nil {
				log.Warn("Failed to update batch status", logger.Err(err))
				continue
			}
		}

		return nil
	})
	if err != nil {
		log.Warn("Process Ready Batches worker failed", logger.Err(err))
	}

	return false
}

func (d *Daemon) processQueuedBatches() bool {
	log := slog.With(slog.String("method", "processQueuedBatches"))

	// TODO: context should be passed from the daemon to all the workers.
	ctx := context.TODO()

	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_QUEUED)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state: %w", err)
		}

		// See if we can start running this batch.
		for _, b := range batches {
			log := log.With(slog.String("batch", b.Name))

			log.Info("Batch status is 'Queued', processing....")

			if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
				log.Error("Batch window end time has already passed")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					log.Warn("Failed to update batch status", logger.Err(err))
					continue
				}

				continue
			}

			// Get all instances for this batch.
			instances, err := d.instance.GetAllByBatchID(ctx, b.ID)
			if err != nil {
				log.Warn("Failed to get instances for batch", logger.Err(err))
				continue
			}

			target, err := d.target.GetByID(ctx, b.TargetID)
			if err != nil {
				log.Warn("Failed to get target for batch", logger.Err(err))
				continue
			}

			// Make sure the necessary ISO images exist in the Incus storage pool.
			err = d.ensureISOImagesExistInStoragePool(ctx, target, instances, b.TargetProject, b.StoragePool)
			if err != nil {
				log.Warn("Failed to ensure ISO images exist in storage pool", logger.Err(err))
				continue
			}

			// Instantiate each new empty VM in Incus.
			for _, inst := range instances {
				// Create fresh context, since operation is happening in its own go routine.
				ctx := context.Background()
				go d.spinUpMigrationEnv(ctx, inst, b)
			}

			// Move batch to "running" status.
			log.Info("Updating batch status to 'Running'")

			_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_RUNNING, api.BATCHSTATUS_RUNNING.String())
			if err != nil {
				log.Warn("Failed to update batch status", logger.Err(err))
				continue
			}
		}

		return nil
	})
	if err != nil {
		log.Warn("Process Queued Batches worker failed", logger.Err(err))
	}

	return false
}

func (d *Daemon) ensureISOImagesExistInStoragePool(ctx context.Context, t migration.Target, instances migration.Instances, project string, storagePool string) error {
	if len(instances) == 0 {
		return fmt.Errorf("No instances in batch")
	}

	inst := instances[0]
	log := slog.With(
		slog.String("method", "ensureISOImagesExistInStoragePool"),
		slog.String("instance", inst.InventoryPath),
		slog.String("storage_pool", storagePool),
	)

	// Determine the ISO names.
	workerISOName, err := d.os.GetMigrationManagerISOName()
	if err != nil {
		return err
	}

	workerISOPath := filepath.Join(d.os.CacheDir, workerISOName)
	workerISOExists := incusUtil.PathExists(workerISOPath)
	if !workerISOExists {
		return fmt.Errorf("Worker ISO not found at %q", workerISOPath)
	}

	importISOs := []string{workerISOName}
	for _, inst := range instances {
		if inst.GetOSType() == api.OSTYPE_WINDOWS {
			driverISOName, err := d.os.GetVirtioDriversISOName()
			if err != nil {
				return err
			}

			driverISOPath := filepath.Join(d.os.CacheDir, driverISOName)
			driverISOExists := incusUtil.PathExists(driverISOPath)
			if !driverISOExists {
				return fmt.Errorf("VirtIO drivers ISO not found at %q", driverISOPath)
			}

			importISOs = append(importISOs, driverISOName)

			break
		}
	}

	it, err := target.NewInternalIncusTargetFrom(api.Target{
		Name:       t.Name,
		DatabaseID: t.ID,
		TargetType: t.TargetType,
		Properties: t.Properties,
	})
	if err != nil {
		return err
	}

	// Connect to the target.
	err = it.Connect(ctx)
	if err != nil {
		return err
	}

	// Set the project.
	err = it.SetProject(project)
	if err != nil {
		return err
	}

	// Verify needed ISO images are in the storage pool.
	for _, iso := range importISOs {
		log := log.With(slog.String("iso", iso))

		_, _, err = it.GetStoragePoolVolume(storagePool, "custom", iso)
		if err != nil && incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
			log.Info("ISO image doesn't exist in storage pool, importing...")

			op, err := it.CreateStoragePoolVolumeFromISO(storagePool, filepath.Join(d.os.CacheDir, iso))
			if err != nil {
				return err
			}

			err = op.Wait()
			if err != nil {
				return err
			}
		}

		if err != nil {
			return fmt.Errorf("Failed checking for storage volume %q: %w", iso, err)
		}
	}

	return nil
}

func (d *Daemon) spinUpMigrationEnv(ctx context.Context, inst migration.Instance, b migration.Batch) {
	log := slog.With(
		slog.String("method", "spinUpMigrationEnv"),
		slog.String("instance", inst.InventoryPath),
	)

	// Update the instance status.
	_, err := d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_CREATING, api.MIGRATIONSTATUS_CREATING.String(), true)
	if err != nil {
		log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		return
	}

	s, err := d.source.GetByID(ctx, inst.SourceID)
	if err != nil {
		log.Warn("Failed to get source by ID", logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Get the target for this instance.
	t, err := d.target.GetByID(ctx, b.TargetID)
	if err != nil {
		log.Warn("Failed to get target by ID", slog.Int("target_id", b.TargetID), logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	it, err := target.NewInternalIncusTargetFrom(api.Target{
		Name:       t.Name,
		DatabaseID: t.ID,
		TargetType: t.TargetType,
		Properties: t.Properties,
	})
	if err != nil {
		log.Warn("Failed to construct target", slog.String("target", it.GetName()), logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Connect to the target.
	err = it.Connect(d.ShutdownCtx)
	if err != nil {
		log.Warn("Failed to connect to target", slog.String("target", it.GetName()), logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Set the project.
	err = it.SetProject(b.TargetProject)
	if err != nil {
		log.Warn("Failed to set target project", slog.String("project", b.TargetProject), logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Create the instance.
	workerISOName, _ := d.os.GetMigrationManagerISOName()
	var driverISOName string
	if inst.GetOSType() == api.OSTYPE_WINDOWS {
		driverISOName, _ = d.os.GetVirtioDriversISOName()
	}

	instanceDef := it.CreateVMDefinition(inst, s.Name, b.StoragePool)
	creationErr := it.CreateNewVM(instanceDef, b.StoragePool, workerISOName, driverISOName)
	if creationErr != nil {
		log.Warn("Failed to create new VM", slog.String("instance", instanceDef.Name), logger.Err(creationErr))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, creationErr.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Creation was successful, update the instance state to 'Idle'.
	_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_IDLE, api.MIGRATIONSTATUS_IDLE.String(), true)
	if err != nil {
		log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		return
	}

	// Start the instance.
	startErr := it.StartVM(inst.GetName())
	if startErr != nil {
		log.Warn("Failed to start VM", logger.Err(startErr))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, startErr.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Inject the worker binary.
	workerBinaryName := filepath.Join(d.os.VarDir, "migration-manager-worker")
	pushErr := it.PushFile(inst.GetName(), workerBinaryName, "/root/")
	if pushErr != nil {
		log.Warn("Failed to push file to instance", slog.String("filename", workerBinaryName), logger.Err(pushErr))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, pushErr.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Start the worker binary.
	workerStartErr := it.ExecWithoutWaiting(inst.GetName(), []string{"/root/migration-manager-worker", "-d", "--endpoint", d.getWorkerEndpoint(), "--uuid", inst.UUID.String(), "--token", inst.SecretToken.String()})
	if workerStartErr != nil {
		log.Warn("Failed to execute without waiting", logger.Err(workerStartErr))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, workerStartErr.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}
}

func (d *Daemon) finalizeCompleteInstances() bool {
	log := slog.With(slog.String("method", "finalizeCompleteInstances"))

	// TODO: context should be passed from the daemon to all the workers.
	ctx := context.TODO()

	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get any instances in the "complete" state.
		instances, err := d.instance.GetAllByState(ctx, api.MIGRATIONSTATUS_IMPORT_COMPLETE)
		if err != nil {
			return fmt.Errorf("Failed to get instances by state: %w", err)
		}

		batches, err := d.batch.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all batches: %w", err)
		}

		batchesByID := make(map[int]migration.Batch, len(batches))
		for _, b := range batches {
			batchesByID[b.ID] = b
		}

		for _, i := range instances {
			log := log.With(slog.String("instance", i.InventoryPath))

			log.Info("Finalizing migration steps for instance")

			// Get the target for this instance.
			batch := batchesByID[*i.BatchID]
			t, err := d.target.GetByID(ctx, batch.TargetID)
			if err != nil {
				log.Warn("Failed to get target", slog.Int("target_id", batch.TargetID), logger.Err(err))
				_, err := d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			it, err := target.NewInternalIncusTargetFrom(api.Target{
				Name:       t.Name,
				DatabaseID: t.ID,
				TargetType: t.TargetType,
				Properties: t.Properties,
			})
			if err != nil {
				log.Warn("Failed to construct target", slog.String("target", it.GetName()), logger.Err(err))
				_, err := d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Connect to the target.
			err = it.Connect(d.ShutdownCtx)
			if err != nil {
				log.Warn("Failed to connect to target", slog.String("target", it.GetName()), logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Set the project.
			err = it.SetProject(batch.TargetProject)
			if err != nil {
				log.Warn("Failed to set target project", slog.String("project", batch.TargetProject), logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Stop the instance.
			stopErr := it.StopVM(i.GetName(), true)
			if stopErr != nil {
				log.Warn("Failed to stop VM", logger.Err(stopErr))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Get the instance definition.
			apiDef, etag, err := it.GetInstance(i.GetName())
			if err != nil {
				log.Warn("Failed to get instance", logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Add NIC(s).
			for idx, nic := range i.NICs {
				log := log.With(slog.String("network_hwaddr", nic.Hwaddr))

				nicDeviceName := fmt.Sprintf("eth%d", idx)

				baseNetwork, err := d.network.GetByName(ctx, nic.Network)
				if err != nil {
					log.Warn("Failed to get network", logger.Err(err))
					_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
					if err != nil {
						log.Warn("Failed to update instance status", logger.Err(err))
						continue
					}

					continue
				}

				// Pickup device name override if set.
				deviceName, ok := baseNetwork.Config["name"]
				if ok {
					nicDeviceName = deviceName
				}

				// Copy the base network definitions.
				apiDef.Devices[nicDeviceName] = make(map[string]string, len(baseNetwork.Config))
				for k, v := range baseNetwork.Config {
					apiDef.Devices[nicDeviceName][k] = v
				}

				// Set a few forced overrides.
				apiDef.Devices[nicDeviceName]["type"] = "nic"
				apiDef.Devices[nicDeviceName]["name"] = nicDeviceName
				apiDef.Devices[nicDeviceName]["hwaddr"] = nic.Hwaddr
			}

			// Remove the migration ISO image.
			delete(apiDef.Devices, "migration-iso")

			// Don't set any profiles by default.
			apiDef.Profiles = []string{}

			// Handle Windows-specific completion steps.
			if strings.Contains(apiDef.Config["image.os"], "swodniw") {
				// Remove the drivers ISO image.
				delete(apiDef.Devices, "drivers")

				// Fixup the OS name.
				apiDef.Config["image.os"] = strings.Replace(apiDef.Config["image.os"], "swodniw", "windows", 1)
			}

			// Handle RHEL (and derivative) specific completion steps.
			if util.IsRHELOrDerivative(apiDef.Config["image.os"]) {
				// RHEL7+ don't support 9p, so make agent config available via cdrom.
				apiDef.Devices["agent"] = map[string]string{
					"type":   "disk",
					"source": "agent:config",
				}
			}

			// Set the instance's UUID copied from the source.
			apiDef.Config["volatile.uuid"] = i.UUID.String()
			apiDef.Config["volatile.uuid.generation"] = i.UUID.String()

			// Update the instance in Incus.
			op, err := it.UpdateInstance(i.GetName(), apiDef.Writable(), etag)
			if err != nil {
				log.Warn("Failed to update instance", logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			err = op.Wait()
			if err != nil {
				log.Warn("Failed to wait for operation", logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Update the instance status.
			_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_FINISHED, api.MIGRATIONSTATUS_FINISHED.String(), true)
			if err != nil {
				log.Warn("Failed to update instance status", logger.Err(err))
				continue
			}

			// Power on the completed instance.
			err = it.StartVM(i.GetName())
			if err != nil {
				log.Warn("Failed to start VM", logger.Err(err))
				continue
			}
		}

		return nil
	})
	if err != nil {
		log.Warn("Sync Instances From Sources worker failed", logger.Err(err))
		return false
	}

	return false
}
