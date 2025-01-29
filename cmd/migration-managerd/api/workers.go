package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	incusUtil "github.com/lxc/incus/v6/shared/util"

	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/target"
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
	// Get the list of configured sources.
	sources, err := d.source.GetAll(ctx)
	if err != nil {
		log.Warn("Failed to get all sources", logger.Err(err))
		return false
	}

	// Check each source for any net networks and any new, changed, or deleted instances.
	for _, src := range sources {
		log := log.With(slog.String("source", src.Name))

		s, err := source.NewInternalVMwareSourceFrom(api.Source{
			Name:       src.Name,
			DatabaseID: src.ID,
			Insecure:   src.Insecure,
			SourceType: src.SourceType,
			Properties: src.Properties,
		})
		if err != nil {
			log.Warn("Failed to create VMWareSource from source", logger.Err(err))
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
				slog.String("instance", i.GetInventoryPath()),
				slog.Any("instance_uuid", i.GetUUID()),
			)

			// Check if this instance is already in the database.
			existingInstance := &instance.InternalInstance{}
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				inst, err := d.db.GetInstance(tx, i.GetUUID())
				if err != nil {
					return err
				}

				var ok bool
				existingInstance, ok = inst.(*instance.InternalInstance)
				if !ok {
					return errors.New("Invalid type for internal instance")
				}

				return nil
			})

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
					err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
						err := d.db.UpdateInstance(tx, existingInstance)
						if err != nil {
							return err
						}

						return nil
					})
					if err != nil {
						log.Warn("Failed to update instance", logger.Err(err))
						continue
					}
				}
			} else {
				// Add a new instance to the database.
				log.Info("Adding instance from source to database")

				err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
					err := d.db.AddInstance(tx, &i)
					if err != nil {
						return err
					}

					return nil
				})
				if err != nil {
					log.Warn("Failed to add instance", logger.Err(err))
					continue
				}
			}

			// Record that this instance exists.
			currentInstancesFromSource[i.GetUUID()] = true
		}

		// Remove instances that no longer exist in this source.
		allDBInstances := []instance.Instance{}
		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			instances, err := d.db.GetAllInstances(tx)
			if err != nil {
				return err
			}

			allDBInstances = instances
			return nil
		})
		if err != nil {
			log.Warn("Failed to get instances", logger.Err(err))
			continue
		}

		for _, i := range allDBInstances {
			log := log.With(
				slog.String("instance", i.GetInventoryPath()),
				slog.Any("instance_uuid", i.GetUUID()),
			)

			_, instanceExists := currentInstancesFromSource[i.GetUUID()]
			if !instanceExists {
				log.Info("Instance removed from source")
				err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
					err := d.db.DeleteInstance(tx, i.GetUUID())
					if err != nil {
						return err
					}

					return nil
				})
				if err != nil {
					log.Warn("Failed to delete instance", logger.Err(err))
					continue
				}
			}
		}
	}

	return false
}

func (d *Daemon) processReadyBatches() bool {
	log := slog.With(slog.String("method", "processReadyBatches"))

	// Get any batches in the "ready" state.
	batches := []batch.Batch{}
	err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		batches, err = d.db.GetAllBatchesByState(tx, api.BATCHSTATUS_READY)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Warn("Failed to get batches by state", logger.Err(err))
		return false
	}

	// Do some basic sanity check of each batch before adding it to the queue.
	for _, b := range batches {
		log := log.With(slog.String("batch", b.GetName()))

		log.Info("Batch status is 'Ready', processing....")
		batchID, err := b.GetDatabaseID()
		if err != nil {
			log.Warn("Failed to get database ID", logger.Err(err))
			continue
		}

		// If a migration window is defined, ensure sure it makes sense.
		if !b.GetMigrationWindowStart().IsZero() && !b.GetMigrationWindowEnd().IsZero() && b.GetMigrationWindowEnd().Before(b.GetMigrationWindowStart()) {
			log.Error("Batch window end time is before its start time")

			err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_ERROR, "Migration window end before start")
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update batch status", logger.Err(err))
				continue
			}

			continue
		}

		if !b.GetMigrationWindowEnd().IsZero() && b.GetMigrationWindowEnd().Before(time.Now().UTC()) {
			log.Error("Batch window end time has already passed")

			err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update batch status", logger.Err(err))
				continue
			}

			continue
		}

		// Get all instances for this batch.
		instances := []instance.Instance{}
		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			var err error
			instances, err = d.db.GetAllInstancesForBatchID(tx, batchID)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to get instances for batch", logger.Err(err))
			continue
		}

		// If no instances apply to this batch, return an error.
		if len(instances) == 0 {
			log.Error("Batch has no instances")

			err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_ERROR, "No instances assigned")
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update batch status", logger.Err(err))
				continue
			}

			continue
		}

		// No issues detected, move to "queued" status.
		log.Info("Updating batch status to 'Queued'")

		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_QUEUED, api.BATCHSTATUS_QUEUED.String())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to update batch status", logger.Err(err))
			continue
		}
	}

	return false
}

func (d *Daemon) processQueuedBatches() bool {
	log := slog.With(slog.String("method", "processQueuedBatches"))

	// Get any batches in the "queued" state.
	batches := []batch.Batch{}
	err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		batches, err = d.db.GetAllBatchesByState(tx, api.BATCHSTATUS_QUEUED)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Warn("Failed to get batches by state", logger.Err(err))
		return false
	}

	// See if we can start running this batch.
	for _, b := range batches {
		log := log.With(slog.String("batch", b.GetName()))

		log.Info("Batch status is 'Queued', processing....")

		batchID, err := b.GetDatabaseID()
		if err != nil {
			log.Warn("Failed to get database ID", logger.Err(err))
			continue
		}

		if !b.GetMigrationWindowEnd().IsZero() && b.GetMigrationWindowEnd().Before(time.Now().UTC()) {
			log.Error("Batch window end time has already passed")

			err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update batch status", logger.Err(err))
				continue
			}

			continue
		}

		// Get all instances for this batch.
		instances := []instance.Instance{}
		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			var err error
			instances, err = d.db.GetAllInstancesForBatchID(tx, batchID)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to get instances for batch", logger.Err(err))
			continue
		}

		// Make sure the necessary ISO images exist in the Incus storage pool.
		err = d.ensureISOImagesExistInStoragePool(instances, b.GetTargetProject(), b.GetStoragePool())
		if err != nil {
			log.Warn("Failed to ensure ISO images exist in storage pool", logger.Err(err))
			continue
		}

		// Instantiate each new empty VM in Incus.
		for _, inst := range instances {
			go d.spinUpMigrationEnv(inst, b.GetTargetProject(), b.GetStoragePool())
		}

		// Move batch to "running" status.
		log.Info("Updating batch status to 'Running'")

		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_RUNNING, api.BATCHSTATUS_RUNNING.String())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to update batch status", logger.Err(err))
			continue
		}
	}

	return false
}

func (d *Daemon) ensureISOImagesExistInStoragePool(instances []instance.Instance, project string, storagePool string) error {
	if len(instances) == 0 {
		return fmt.Errorf("No instances in batch")
	}

	inst := instances[0]
	log := slog.With(
		slog.String("method", "ensureISOImagesExistInStoragePool"),
		slog.String("instance", inst.GetInventoryPath()),
		slog.String("storage_pool", storagePool),
	)

	// Determine the ISO names.
	workerISOName, err := d.os.GetMigrationManagerISOName()
	if err != nil {
		return err
	}

	driverISOName, err := d.os.GetVirtioDriversISOName()
	if err != nil {
		return err
	}

	workerISOPath := filepath.Join(d.os.CacheDir, workerISOName)
	workerISOExists := incusUtil.PathExists(workerISOPath)
	if !workerISOExists {
		return fmt.Errorf("Worker ISO not found at %q", workerISOPath)
	}

	for _, inst := range instances {
		if inst.GetOSType() == api.OSTYPE_WINDOWS {
			driverISOPath := filepath.Join(d.os.CacheDir, driverISOName)
			driverISOExists := incusUtil.PathExists(driverISOPath)
			if !driverISOExists {
				return fmt.Errorf("VirtIO drivers ISO not found at %q", driverISOPath)
			}

			break
		}
	}

	// Get the target.
	ctx := context.TODO()
	t, err := d.target.GetByID(ctx, *inst.GetTargetID())
	if err != nil {
		return err
	}

	// TODO: The methods on the target.InternalIncusTarget should be moved to migration
	// which would then make this conversion obsolete.
	it := target.InternalIncusTarget{
		IncusTarget: api.IncusTarget{
			DatabaseID:    t.ID,
			Name:          t.Name,
			Endpoint:      t.Endpoint,
			TLSClientKey:  t.TLSClientKey,
			TLSClientCert: t.TLSClientCert,
			OIDCTokens:    t.OIDCTokens,
			Insecure:      t.Insecure,
		},
	}

	// Connect to the target.
	err = it.Connect(d.ShutdownCtx)
	if err != nil {
		return err
	}

	// Set the project.
	err = it.SetProject(project)
	if err != nil {
		return err
	}

	// Verify needed ISO images are in the storage pool.
	for _, iso := range []string{workerISOName, driverISOName} {
		log := log.With(slog.String("iso", iso))

		_, _, err = it.GetStoragePoolVolume(storagePool, "custom", iso)
		if err != nil {
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
	}

	return nil
}

func (d *Daemon) spinUpMigrationEnv(inst instance.Instance, project string, storagePool string) {
	log := slog.With(
		slog.String("method", "spinUpMigrationEnv"),
		slog.String("instance", inst.GetInventoryPath()),
	)

	// Update the instance status.
	err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_CREATING, api.MIGRATIONSTATUS_CREATING.String(), true)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Warn("Failed to update instance status", logger.Err(err))
		return
	}

	// TODO: Context should be passed from Daemon to all the workers.
	ctx := context.TODO()
	s, err := d.source.GetByID(ctx, inst.GetSourceID())
	if err != nil {
		log.Warn("Failed to get source by ID", logger.Err(err))
		_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		return
	}

	// Get the target for this instance.
	t, err := d.target.GetByID(ctx, *inst.GetTargetID())
	if err != nil {
		log.Warn("Failed to get target by ID", slog.Int("target_id", *inst.GetTargetID()), logger.Err(err))
		_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		return
	}

	// TODO: The methods on the target.InternalIncusTarget should be moved to migration
	// which would then make this conversion obsolete.
	it := target.InternalIncusTarget{
		IncusTarget: api.IncusTarget{
			DatabaseID:    t.ID,
			Name:          t.Name,
			Endpoint:      t.Endpoint,
			TLSClientKey:  t.TLSClientKey,
			TLSClientCert: t.TLSClientCert,
			OIDCTokens:    t.OIDCTokens,
			Insecure:      t.Insecure,
		},
	}

	// Connect to the target.
	err = it.Connect(d.ShutdownCtx)
	if err != nil {
		log.Warn("Failed to connect to target", slog.String("target", it.GetName()), logger.Err(err))
		_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		return
	}

	// Set the project.
	err = it.SetProject(project)
	if err != nil {
		log.Warn("Failed to set target project", slog.String("project", project), logger.Err(err))
		_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		return
	}

	// Create the instance.
	internalInstance, ok := inst.(*instance.InternalInstance)
	if !ok {
		log.Warn("Invalid type for internal instance")
		return
	}

	workerISOName, _ := d.os.GetMigrationManagerISOName()
	driverISOName, _ := d.os.GetVirtioDriversISOName()
	instanceDef := it.CreateVMDefinition(*internalInstance, s.Name, storagePool)
	creationErr := it.CreateNewVM(instanceDef, storagePool, workerISOName, driverISOName)
	if creationErr != nil {
		log.Warn("Failed to create new VM", slog.String("instance", instanceDef.Name), logger.Err(creationErr))
		err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, creationErr.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to update instance status", logger.Err(err))
		}

		return
	}

	// Creation was successful, update the instance state to 'Idle'.
	err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_IDLE, api.MIGRATIONSTATUS_IDLE.String(), true)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Warn("Failed to update instance status", logger.Err(err))
		return
	}

	// Start the instance.
	startErr := it.StartVM(inst.GetName())
	if startErr != nil {
		log.Warn("Failed to start VM", logger.Err(startErr))
		err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, startErr.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to update instance status", logger.Err(err))
		}

		return
	}

	// Inject the worker binary.
	workerBinaryName := filepath.Join(d.os.VarDir, "migration-manager-worker")
	pushErr := it.PushFile(inst.GetName(), workerBinaryName, "/root/")
	if pushErr != nil {
		log.Warn("Failed to push file to instance", slog.String("filename", workerBinaryName), logger.Err(pushErr))
		err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, pushErr.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to update instance status", logger.Err(err))
		}

		return
	}

	// Start the worker binary.
	workerStartErr := it.ExecWithoutWaiting(inst.GetName(), []string{"/root/migration-manager-worker", "-d", "--endpoint", d.getWorkerEndpoint(), "--uuid", inst.GetUUID().String(), "--token", inst.GetSecretToken().String()})
	if workerStartErr != nil {
		log.Warn("Failed to execute without waiting", logger.Err(workerStartErr))
		err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, workerStartErr.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to update instance status", logger.Err(err))
		}

		return
	}
}

func (d *Daemon) finalizeCompleteInstances() bool {
	log := slog.With(slog.String("method", "finalizeCompleteInstances"))

	// Get any instances in the "complete" state.
	instances := []instance.Instance{}
	err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		instances, err = d.db.GetAllInstancesByState(tx, api.MIGRATIONSTATUS_IMPORT_COMPLETE)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Warn("Failed to get instances by state", logger.Err(err))
		return false
	}

	for _, i := range instances {
		log := log.With(slog.String("instance", i.GetInventoryPath()))

		log.Info("Finalizing migration steps for instance")

		// Get the target for this instance.
		ctx := context.TODO()
		t, err := d.target.GetByID(ctx, *i.GetTargetID())
		if err != nil {
			log.Warn("Failed to get target", slog.Int("target_id", *i.GetTargetID()), logger.Err(err))
			_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			continue
		}

		// TODO: The methods on the target.InternalIncusTarget should be moved to migration
		// which would then make this conversion obsolete.
		it := target.InternalIncusTarget{
			IncusTarget: api.IncusTarget{
				DatabaseID:    t.ID,
				Name:          t.Name,
				Endpoint:      t.Endpoint,
				TLSClientKey:  t.TLSClientKey,
				TLSClientCert: t.TLSClientCert,
				OIDCTokens:    t.OIDCTokens,
				Insecure:      t.Insecure,
			},
		}

		// Get the batch for this instance.
		batchID := *i.GetBatchID()
		var dbBatch batch.Batch
		batchErr := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			var err error
			dbBatch, err = d.db.GetBatchByID(tx, batchID)
			if err != nil {
				return err
			}

			return nil
		})
		if batchErr != nil {
			log.Warn("Failed to get batch by ID", slog.Int("batch_id", batchID), logger.Err(batchErr))
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, batchErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update instance status", logger.Err(err))
			}

			continue
		}

		// Connect to the target.
		err = it.Connect(d.ShutdownCtx)
		if err != nil {
			log.Warn("Failed to connect to target", slog.String("target", it.GetName()), logger.Err(err))
			_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			continue
		}

		// Set the project.
		err = it.SetProject(dbBatch.GetTargetProject())
		if err != nil {
			log.Warn("Failed to set target project", slog.String("project", dbBatch.GetTargetProject()), logger.Err(err))
			_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			continue
		}

		// Stop the instance.
		stopErr := it.StopVM(i.GetName(), true)
		if stopErr != nil {
			log.Warn("Failed to stop VM", logger.Err(stopErr))
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, stopErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update instance status", logger.Err(err))
			}

			continue
		}

		// Get the instance definition.
		apiDef, etag, err := it.GetInstance(i.GetName())
		if err != nil {
			log.Warn("Failed to get instance", logger.Err(err))
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update instance status", logger.Err(err))
			}

			continue
		}

		// Add NIC(s).
		for idx, nic := range i.(*instance.InternalInstance).NICs {
			log := log.With(slog.String("network_hwaddr", nic.Hwaddr))

			nicDeviceName := fmt.Sprintf("eth%d", idx)

			baseNetwork, netErr := d.network.GetByName(ctx, nic.Network)
			if netErr != nil {
				log.Warn("Failed to get network", logger.Err(netErr))
				err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
					err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, netErr.Error(), true)
					if err != nil {
						return err
					}

					return nil
				})
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
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
		apiDef.Config["volatile.uuid"] = i.GetUUID().String()
		apiDef.Config["volatile.uuid.generation"] = i.GetUUID().String()

		// Update the instance in Incus.
		op, updateErr := it.UpdateInstance(i.GetName(), apiDef.Writable(), etag)
		if updateErr != nil {
			log.Warn("Failed to update instance", logger.Err(updateErr))
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, updateErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update instance status", logger.Err(err))
			}

			continue
		}

		updateErr = op.Wait()
		if updateErr != nil {
			log.Warn("Failed to wait for operation", logger.Err(updateErr))
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, updateErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Warn("Failed to update instance status", logger.Err(err))
			}

			continue
		}

		// Update the instance status.
		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_FINISHED, api.MIGRATIONSTATUS_FINISHED.String(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Warn("Failed to update instance status", logger.Err(err))
			continue
		}

		// Power on the completed instance.
		startErr := it.StartVM(i.GetName())
		if startErr != nil {
			log.Warn("Failed to start VM", logger.Err(startErr))
			continue
		}
	}
	return false
}
