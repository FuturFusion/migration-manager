package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func collectVROpsMonitoring(ctx context.Context, src migration.Source, instances migration.Instances) (map[uuid.UUID]api.InstanceMonitoring, error) {
	client, err := source.NewInternalVROpsSourceFrom(src.ToAPI())
	if err != nil {
		return nil, err
	}

	defer client.Close()
	err = client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return client.CollectMonitoring(ctx, instances)
}

func applyVROpsMonitoring(ctx context.Context, sources migration.Sources, instancesBySrc map[string]map[uuid.UUID]migration.Instance) migration.Warnings {
	instances := migration.Instances{}
	for _, sourceInstances := range instancesBySrc {
		for _, instance := range sourceInstances {
			instances = append(instances, instance)
		}
	}

	warnings := migration.Warnings{}
	if len(instances) == 0 {
		return warnings
	}

	snapshots := map[uuid.UUID]api.InstanceMonitoring{}
	ambiguous := map[uuid.UUID]bool{}
	for _, src := range sources {
		if src.GetExternalConnectivityStatus() != api.EXTERNALCONNECTIVITYSTATUS_OK {
			warnings = append(warnings, migration.NewSyncWarning(api.SourceUnavailable, src.Name, fmt.Sprintf("status: %q", src.GetExternalConnectivityStatus())))
			continue
		}

		monitoring, err := collectVROpsMonitoring(ctx, src, instances)
		if err != nil {
			warnings = append(warnings, migration.NewSyncWarning(api.SourceUnavailable, src.Name, err.Error()))
		}

		for id, data := range monitoring {
			_, exists := snapshots[id]
			if exists || ambiguous[id] {
				delete(snapshots, id)
				ambiguous[id] = true
				warnings = append(warnings, migration.NewSyncWarning(api.SourceUnavailable, src.Name, fmt.Sprintf("Multiple vROps sources matched VM %s; monitoring was not updated", id)))
				continue
			}

			snapshots[id] = data
		}
	}

	for _, sourceInstances := range instancesBySrc {
		for id, instance := range sourceInstances {
			data, ok := snapshots[id]
			if ok {
				instance.UpdateMonitoring(data)
				sourceInstances[id] = instance
			}
		}
	}

	return warnings
}

func (d *Daemon) configuredVROpsSources(ctx context.Context) (map[string]migration.Source, error) {
	sources, err := d.source.GetAll(ctx, api.SOURCETYPE_VROPS)
	if err != nil {
		return nil, err
	}

	configured := make(map[string]migration.Source, len(sources))
	for _, src := range sources {
		configured[src.Name] = src
	}

	return configured, nil
}

func validateVROpsSource(src migration.Source, configured map[string]migration.Source) error {
	current, exists := configured[src.Name]
	if !exists || current.ID != src.ID || !bytes.Equal(current.Properties, src.Properties) {
		return fmt.Errorf("vROps source %q changed during collection; discard the results and retry", src.Name)
	}

	return nil
}

func validateVROpsOwner(previous *api.InstanceMonitoring, sourceName string, configured map[string]migration.Source) error {
	if previous == nil || previous.Source == sourceName {
		return nil
	}

	_, exists := configured[previous.Source]
	if exists {
		return fmt.Errorf("already has monitoring from source %q; skipping source %q", previous.Source, sourceName)
	}

	return nil
}

func filterVROpsMonitoring(incoming map[uuid.UUID]migration.Instance, existing map[uuid.UUID]migration.Instance, validSources map[string]bool, configured map[string]migration.Source) migration.Warnings {
	warnings := migration.Warnings{}
	for id, instance := range incoming {
		data := instance.Properties.Monitoring
		if data == nil {
			continue
		}

		err := validateVROpsOwner(existing[id].Properties.Monitoring, data.Source, configured)
		if err != nil {
			warnings = append(warnings, migration.NewSyncWarning(api.SourceUnavailable, data.Source, fmt.Sprintf("VM %s %v", id, err)))
		}

		if err != nil || !validSources[data.Source] {
			instance.Properties.Monitoring = nil
			incoming[id] = instance
		}
	}

	return warnings
}

// syncVROpsSource updates monitoring only, without running VMware inventory reconciliation.
func (d *Daemon) syncVROpsSource(ctx context.Context, src migration.Source) error {
	d.syncCache.Write(src.Name, struct{}{}, nil)
	defer d.syncCache.Delete(src.Name)
	var instances migration.Instances
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		instances, err = d.instance.GetAll(ctx)
		return err
	})
	if err != nil {
		return err
	}

	monitoring, collectErr := collectVROpsMonitoring(ctx, src, instances)

	// The database transaction serializes updates without the inventory sync mutex.
	err = transaction.Do(ctx, func(ctx context.Context) error {
		configuredSources, err := d.configuredVROpsSources(ctx)
		if err != nil {
			return err
		}

		err = validateVROpsSource(src, configuredSources)
		if err != nil {
			return err
		}

		for id, data := range monitoring {
			instance, err := d.instance.GetByUUID(ctx, id)
			if errors.Is(err, migration.ErrNotFound) {
				continue
			}

			if err != nil {
				return err
			}

			err = validateVROpsOwner(instance.Properties.Monitoring, src.Name, configuredSources)
			if err != nil {
				collectErr = errors.Join(collectErr, fmt.Errorf("VM %s %w", id, err))
				continue
			}

			if !instance.UpdateMonitoring(data) {
				continue
			}
			// Only monitoring changed, so updates are safe during a migration.
			err = d.instance.Update(ctx, instance, true)
			if err != nil {
				return err
			}
		}

		scope := api.WarningScopeSync()
		scope.Entity = src.Name
		warnings := migration.Warnings{}
		if collectErr != nil {
			warnings = append(warnings, migration.NewSyncWarning(api.SourceUnavailable, src.Name, collectErr.Error()))
		}

		err = d.warning.RemoveStale(ctx, scope, warnings)
		if err != nil {
			return err
		}

		for _, warning := range warnings {
			_, err = d.warning.Emit(ctx, warning)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return collectErr
}
