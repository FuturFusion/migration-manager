package main

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/target"
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
			case <-d.shutdownCtx.Done():
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

	// Ensure at least one target exists.
	targets := []target.Target{}
	err := d.db.Transaction(d.shutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		targets, err = d.db.GetAllTargets(tx)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		logger.Warn(err.Error(), loggerCtx)
		return false
	}
	if len(targets) == 0 {
		logger.Debug("No targets defined, skipping instance sync from sources", loggerCtx)
		return false
	}

	// For now, just default to the first target defined.
	targetId, err := targets[0].GetDatabaseID()
	if err != nil {
		logger.Warn(err.Error(), loggerCtx)
		return false
	}

	// Get the list of configured sources.
	sources := []source.Source{}
	err = d.db.Transaction(d.shutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
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

	// Check each source for any new, changed, or deleted instances.
	for _, s := range sources {
		err := s.Connect(d.shutdownCtx)
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		instances, err := s.GetAllVMs(d.shutdownCtx)
		if err != nil {
			logger.Warn(err.Error(), loggerCtx)
			continue
		}

		currentInstancesFromSource := make(map[uuid.UUID]bool)

		// Iterate each instance from this source.
		for _, i := range instances {
			// Check if this instance is already in the database.
			existingInstance := &instance.InternalInstance{}
			err := d.db.Transaction(d.shutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
				inst, err := d.db.GetInstance(tx, i.GetUUID())
				if err != nil {
					return err
				}

				existingInstance = inst.(*instance.InternalInstance)
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

				// Next, check fields that can be updated, but only sync if this instance hasn't been manually updated.
				if existingInstance.LastManualUpdate.IsZero() {
					if existingInstance.NumberCPUs != i.NumberCPUs {
						existingInstance.NumberCPUs = i.NumberCPUs
						instanceUpdated = true
					}

					if existingInstance.MemoryInMiB != i.MemoryInMiB {
						existingInstance.MemoryInMiB = i.MemoryInMiB
						instanceUpdated = true
					}
				} else {
					logger.Debug("Instance " +  i.GetName() + " (" + i.GetUUID().String() + ") has been manually updated, skipping some automatic sync updates", loggerCtx)
				}

				if instanceUpdated {
					logger.Info("Syncing changes to instance " + i.GetName() + " (" + i.GetUUID().String() + ") from source " + s.GetName(), loggerCtx)
					existingInstance.LastUpdateFromSource = i.LastUpdateFromSource
					err := d.db.Transaction(d.shutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
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
				logger.Info("Adding instance " + i.GetName() + " (" + i.GetUUID().String() + ") from source " + s.GetName() + " to database", loggerCtx)
				i.TargetID = targetId

				err := d.db.Transaction(d.shutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
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
		err = d.db.Transaction(d.shutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
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
				logger.Info("Instance " + i.GetName() + " (" + i.GetUUID().String() + ") removed from source " + s.GetName(), loggerCtx)
				err := d.db.Transaction(d.shutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
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
