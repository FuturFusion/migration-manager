package main

import (
	"context"
	"database/sql"
	"time"

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

	// Check each source for any new or changed instances.
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
				// TODO handle updating of instances from source
			} else {
				logger.Info("Adding instance " + i.Name + " (" + i.UUID.String() + ") to database", loggerCtx)
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
		}

		// TODO handle removing instances that no longer exist in source
	}

	return false
}
