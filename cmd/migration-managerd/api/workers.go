package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/instance"
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
	loggerCtx := logger.Ctx{"method": "syncInstancesFromSources"}

	// Get the list of configured sources.
	sources := []api.Source{}
	err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		sources, err = d.db.GetAllSources(tx)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		logger.Warn(err.Error(), loggerCtx)
		return false
	}

	// Check each source for any net networks and any new, changed, or deleted instances.
	for _, src := range sources {
		s, err := source.NewInternalVMwareSourceFrom(src)
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		err = s.Connect(d.ShutdownCtx)
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		networks, err := s.GetAllNetworks(d.ShutdownCtx)
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		// Iterate each network from this source.
		for _, n := range networks {
			// Check if a network already exists with the same name.
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				_, err := d.db.GetNetwork(tx, n.Name)
				return err
			})
			// Only add the network if it doesn't yet exist
			if err != nil {
				logger.Info("Adding network "+n.Name+" from source "+s.GetName(), loggerCtx)
				err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
					return d.db.AddNetwork(tx, &n)
				})
				if err != nil {
					logger.Warn(err.Error(), loggerCtx)
					continue
				}
			}
		}

		instances, err := s.GetAllVMs(d.ShutdownCtx)
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		currentInstancesFromSource := make(map[uuid.UUID]bool)

		// Iterate each instance from this source.
		for _, i := range instances {
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

				// First, check any fields that cannot be changed through the migration manager
				if existingInstance.Name != i.Name {
					existingInstance.Name = i.Name
					instanceUpdated = true
				}

				if existingInstance.Architecture != i.Architecture {
					existingInstance.Architecture = i.Architecture
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

				if !slices.Equal(existingInstance.Disks, i.Disks) {
					existingInstance.Disks = i.Disks
					instanceUpdated = true
				}

				if !slices.Equal(existingInstance.NICs, i.NICs) {
					existingInstance.NICs = i.NICs
					instanceUpdated = true
				}

				if existingInstance.NumberCPUs != i.NumberCPUs {
					existingInstance.NumberCPUs = i.NumberCPUs
					instanceUpdated = true
				}

				if existingInstance.MemoryInBytes != i.MemoryInBytes {
					existingInstance.MemoryInBytes = i.MemoryInBytes
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
					logger.Info("Syncing changes to instance "+i.GetName()+" ("+i.GetUUID().String()+") from source "+s.GetName(), loggerCtx)
					existingInstance.LastUpdateFromSource = i.LastUpdateFromSource
					err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
						err := d.db.UpdateInstance(tx, existingInstance)
						if err != nil {
							return err
						}

						return nil
					})
					if err != nil {
						logger.Warn(err.Error(), loggerCtx)
						continue
					}
				}
			} else {
				// Add a new instance to the database.
				logger.Info("Adding instance "+i.GetName()+" ("+i.GetUUID().String()+") from source "+s.GetName()+" to database", loggerCtx)
				i.TargetID = internal.INVALID_DATABASE_ID

				err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
					err := d.db.AddInstance(tx, &i)
					if err != nil {
						return err
					}

					return nil
				})
				if err != nil {
					logger.Warn(err.Error(), loggerCtx)
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
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		for _, i := range allDBInstances {
			_, instanceExists := currentInstancesFromSource[i.GetUUID()]
			if !instanceExists {
				logger.Info("Instance "+i.GetName()+" ("+i.GetUUID().String()+") removed from source "+s.GetName(), loggerCtx)
				err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
					err := d.db.DeleteInstance(tx, i.GetUUID())
					if err != nil {
						return err
					}

					return nil
				})
				if err != nil {
					logger.Warn(err.Error(), loggerCtx)
					continue
				}
			}
		}
	}

	return false
}

func (d *Daemon) processReadyBatches() bool {
	loggerCtx := logger.Ctx{"method": "processReadyBatches"}

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
		logger.Warn(err.Error(), loggerCtx)
		return false
	}

	// Do some basic sanity check of each batch before adding it to the queue.
	for _, b := range batches {
		logger.Info("Batch '"+b.GetName()+"' status is 'Ready', processing....", loggerCtx)
		batchID, err := b.GetDatabaseID()
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		// If a migration window is defined, ensure sure it makes sense.
		if !b.GetMigrationWindowStart().IsZero() && !b.GetMigrationWindowEnd().IsZero() && b.GetMigrationWindowEnd().Before(b.GetMigrationWindowStart()) {
			logger.Error("Batch '"+b.GetName()+"' window end time is before its start time", loggerCtx)

			err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_ERROR, "Migration window end before start")
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
				continue
			}

			continue
		}

		if !b.GetMigrationWindowEnd().IsZero() && b.GetMigrationWindowEnd().Before(time.Now().UTC()) {
			logger.Error("Batch '"+b.GetName()+"' window end time has already passed", loggerCtx)

			err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
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
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		// If no instances apply to this batch, return an error.
		if len(instances) == 0 {
			logger.Error("Batch '"+b.GetName()+"' has no instances", loggerCtx)

			err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_ERROR, "No instances assigned")
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
				continue
			}

			continue
		}

		// No issues detected, move to "queued" status.
		logger.Info("Updating batch '"+b.GetName()+"' status to 'Queued'", loggerCtx)

		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_QUEUED, api.BATCHSTATUS_QUEUED.String())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}
	}

	return false
}

func (d *Daemon) processQueuedBatches() bool {
	loggerCtx := logger.Ctx{"method": "processQueuedBatches"}

	// Make sure global server config has been properly set.
	if d.globalConfig["core.boot_iso_image"] == "" {
		logger.Error("Server config 'core.boot_iso_image' isn't set.")
		return false
	}

	if d.globalConfig["core.drivers_iso_image"] == "" {
		logger.Error("Server config 'core.drivers_iso_image' isn't set.")
		return false
	}

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
		logger.Warn(err.Error(), loggerCtx)
		return false
	}

	// See if we can start running this batch.
	for _, b := range batches {
		logger.Info("Batch '"+b.GetName()+"' status is 'Queued', processing....", loggerCtx)
		batchID, err := b.GetDatabaseID()
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		if !b.GetMigrationWindowEnd().IsZero() && b.GetMigrationWindowEnd().Before(time.Now().UTC()) {
			logger.Error("Batch '"+b.GetName()+"' window end time has already passed", loggerCtx)

			err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
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
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		// Instantiate each new empty VM in Incus.
		for _, inst := range instances {
			go d.spinUpMigrationEnv(inst, b.GetStoragePool())
		}

		// Move batch to "running" status.
		logger.Info("Updating batch '"+b.GetName()+"' status to 'Running'", loggerCtx)

		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateBatchStatus(tx, batchID, api.BATCHSTATUS_RUNNING, api.BATCHSTATUS_RUNNING.String())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}
	}

	return false
}

func (d *Daemon) spinUpMigrationEnv(inst instance.Instance, storagePool string) {
	loggerCtx := logger.Ctx{"method": "spinUpMigrationEnv"}

	// Update the instance status.
	err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_CREATING, api.MIGRATIONSTATUS_CREATING.String(), true)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		logger.Warn(err.Error(), loggerCtx)
		return
	}

	// Get the override for this instance, if any.
	var override api.InstanceOverride
	_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		override, _ = d.db.GetInstanceOverride(tx, inst.GetUUID())
		return nil
	})

	// Get the source for this instance.
	var s api.Source
	err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		s, err = d.db.GetSourceByID(tx, inst.GetSourceID())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		logger.Warn(err.Error(), loggerCtx)
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
	var t target.Target
	err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		t, err = d.db.GetTargetByID(tx, inst.GetTargetID())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		logger.Warn(err.Error(), loggerCtx)
		_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		return
	}

	// Connect to the target.
	err = t.Connect(d.ShutdownCtx)
	if err != nil {
		logger.Warn(err.Error(), loggerCtx)
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
		logger.Warn("Invalid type for internal instance", loggerCtx)
		return
	}

	instanceDef := t.CreateVMDefinition(*internalInstance, override, s.Name, storagePool)
	creationErr := t.CreateNewVM(instanceDef, storagePool, d.globalConfig["core.boot_iso_image"], d.globalConfig["core.drivers_iso_image"])
	if creationErr != nil {
		logger.Warn(creationErr.Error(), loggerCtx)
		err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, creationErr.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
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
		logger.Warn(err.Error(), loggerCtx)
		return
	}

	// Start the instance.
	startErr := t.StartVM(inst.GetName())
	if startErr != nil {
		logger.Warn(startErr.Error(), loggerCtx)
		err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, startErr.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
		}

		return
	}

	// Inject the worker binary.
	pushErr := t.PushFile(inst.GetName(), "./migration-manager-worker", "/root/")
	if pushErr != nil {
		logger.Warn(pushErr.Error(), loggerCtx)
		err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, pushErr.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
		}

		return
	}

	// Start the worker binary.
	workerStartErr := t.ExecWithoutWaiting(inst.GetName(), []string{"/root/migration-manager-worker", "-d", "--endpoint", d.getEndpoint(), "--uuid", inst.GetUUID().String(), "--insecure"}) // FIXME -- currently ignoring TLS cert errors
	if workerStartErr != nil {
		logger.Warn(workerStartErr.Error(), loggerCtx)
		err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			err := d.db.UpdateInstanceStatus(tx, inst.GetUUID(), api.MIGRATIONSTATUS_ERROR, workerStartErr.Error(), true)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
		}

		return
	}
}

func (d *Daemon) finalizeCompleteInstances() bool {
	loggerCtx := logger.Ctx{"method": "finalizeCompleteInstances"}

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
		logger.Warn(err.Error(), loggerCtx)
		return false
	}

	for _, i := range instances {
		logger.Info("Finalizing migration steps for instance "+i.GetName(), loggerCtx)
		// Get the target for this instance.
		var t target.Target
		err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			var err error
			t, err = d.db.GetTargetByID(tx, i.GetTargetID())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			_ = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			continue
		}

		// Connect to the target.
		err = t.Connect(d.ShutdownCtx)
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
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
		stopErr := t.StopVM(i.GetName(), true)
		if stopErr != nil {
			logger.Warn(stopErr.Error(), loggerCtx)
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, stopErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
			}

			continue
		}

		// Get the instance definition.
		apiDef, etag, err := t.GetInstance(i.GetName())
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
			}

			continue
		}

		// Get the batch for this instance.
		batchID := i.GetBatchID()
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
			logger.Warn(batchErr.Error(), loggerCtx)
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, batchErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
			}

			continue
		}

		// Get the default network to use for this instance.
		var defaultNetwork api.Network
		defNetErr := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
			var err error
			defaultNetwork, err = d.db.GetNetwork(tx, dbBatch.GetDefaultNetwork())
			if err != nil {
				return err
			}

			return nil
		})
		if defNetErr != nil {
			logger.Warn(defNetErr.Error(), loggerCtx)
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, defNetErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
			}

			continue
		}

		// Add NIC(s).
		for idx, nic := range i.(*instance.InternalInstance).NICs {
			nicDeviceName := fmt.Sprintf("eth%d", idx)
			baseNetwork := defaultNetwork

			// If the NIC has a network set, and it's not the default, fetch the network definition.
			if nic.Network != "" && nic.Network != baseNetwork.Name {
				netErr := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
					var err error
					baseNetwork, err = d.db.GetNetwork(tx, nic.Network)
					if err != nil {
						return err
					}

					return nil
				})
				if netErr != nil {
					logger.Warn(netErr.Error(), loggerCtx)
					err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
						err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, netErr.Error(), true)
						if err != nil {
							return err
						}

						return nil
					})
					if err != nil {
						logger.Warn(err.Error(), loggerCtx)
					}

					continue
				}
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
		op, updateErr := t.UpdateInstance(i.GetName(), apiDef.Writable(), etag)
		if updateErr != nil {
			logger.Warn(updateErr.Error(), loggerCtx)
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, updateErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
			}

			continue
		}

		updateErr = op.Wait()
		if updateErr != nil {
			logger.Warn(updateErr.Error(), loggerCtx)
			err := d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				err := d.db.UpdateInstanceStatus(tx, i.GetUUID(), api.MIGRATIONSTATUS_ERROR, updateErr.Error(), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				logger.Warn(err.Error(), loggerCtx)
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
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		// Power on the completed instance.
		startErr := t.StartVM(i.GetName())
		if startErr != nil {
			logger.Warn(startErr.Error(), loggerCtx)
			continue
		}
	}
	return false
}
