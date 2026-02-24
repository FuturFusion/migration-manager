package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/osarch"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/queue"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
	"github.com/FuturFusion/migration-manager/shared/api/event"
)

// syncLock ensures source syncing is sequential.
var syncLock sync.Mutex

func (d *Daemon) syncActiveBatches(ctx context.Context) error {
	var states map[string]queue.MigrationState
	err := transaction.Do(ctx, func(ctx context.Context) error {
		var err error
		states, err = d.queueHandler.GetMigrationState(ctx, api.BATCHSTATUS_RUNNING, api.MIGRATIONSTATUS_BACKGROUND_IMPORT, api.MIGRATIONSTATUS_FINAL_IMPORT, api.MIGRATIONSTATUS_CREATING)
		if err != nil {
			return fmt.Errorf("Failed to get running migration states: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	importingFromSource := map[string]int{}
	importingToTarget := map[string]int{}
	creatingOnTarget := map[string]int{}
	workerUpdates := map[uuid.UUID]time.Time{}
	now := time.Now().UTC()
	for _, state := range states {
		for instUUID, entry := range state.QueueEntries {
			switch entry.MigrationStatus {
			case api.MIGRATIONSTATUS_IDLE:
				workerUpdates[instUUID] = now

			case api.MIGRATIONSTATUS_WORKER_DONE:
				workerUpdates[instUUID] = now

			case api.MIGRATIONSTATUS_POST_IMPORT:
				workerUpdates[instUUID] = now

			case api.MIGRATIONSTATUS_ERROR:
				workerUpdates[instUUID] = now

			case api.MIGRATIONSTATUS_BACKGROUND_IMPORT:
				importingFromSource[state.Sources[instUUID].Name] = importingFromSource[state.Sources[instUUID].Name] + 1
				importingToTarget[state.Targets[instUUID].Name] = importingToTarget[state.Targets[instUUID].Name] + 1
				workerUpdates[instUUID] = now

			case api.MIGRATIONSTATUS_FINAL_IMPORT:
				importingFromSource[state.Sources[instUUID].Name] = importingFromSource[state.Sources[instUUID].Name] + 1
				importingToTarget[state.Targets[instUUID].Name] = importingToTarget[state.Targets[instUUID].Name] + 1
				workerUpdates[instUUID] = now

			case api.MIGRATIONSTATUS_CREATING:
				creatingOnTarget[state.Targets[instUUID].Name] = creatingOnTarget[state.Targets[instUUID].Name] + 1
			}
		}
	}

	err = d.source.InitImportCache(importingFromSource)
	if err != nil {
		return fmt.Errorf("Failed to initialize source import cache: %w", err)
	}

	err = d.target.InitImportCache(importingToTarget)
	if err != nil {
		return fmt.Errorf("Failed to initialize target import cache: %w", err)
	}

	err = d.target.InitCreateCache(creatingOnTarget)
	if err != nil {
		return fmt.Errorf("Failed to initialize target create cache: %w", err)
	}

	err = d.queueHandler.InitWorkerCache(workerUpdates)
	if err != nil {
		return fmt.Errorf("Failed to initialize worker update cache: %w", err)
	}

	return nil
}

// trySyncAllSources connects to each source in the database and updates the in-memory record of all networks and instances.
// skipNonResponsiveSources - If true, if a connection to a source returns an error, syncing from that source will be skipped.
func (d *Daemon) trySyncAllSources(ctx context.Context) (_err error) {
	log := slog.With(slog.String("method", "syncAllSources"))
	log.Info("Syncing all sources")
	vmSourcesByName := map[string]migration.Source{}
	networkSourcesByName := map[string]migration.Source{}
	var sources migration.Sources
	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the list of configured sources.
		var err error
		sources, err = d.source.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all sources: %w", err)
		}

		for _, src := range sources {
			if src.GetExternalConnectivityStatus() != api.EXTERNALCONNECTIVITYSTATUS_OK {
				log.Warn("Skipping instance sync for unreachable source", slog.String("name", src.Name))
				continue
			}

			if slices.Contains(api.VMSourceTypes(), src.SourceType) {
				vmSourcesByName[src.Name] = src
			}

			if slices.Contains(api.NetworkSourceTypes(), src.SourceType) {
				networkSourcesByName[src.Name] = src
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	warnings := migration.Warnings{}
	defer func() {
		err := transaction.Do(ctx, func(ctx context.Context) error {
			// If this sync succeeded, then prune warning messages and remove resolved ones.
			if _err == nil {
				log.Info("Cleaning up stale warnings")
				err := d.warning.RemoveStale(ctx, api.WarningScopeSync(), warnings)
				if err != nil {
					return fmt.Errorf("Failed to clean up warnings: %w", err)
				}
			}

			if len(warnings) > 0 {
				log.Info("Emitting warnings", slog.Int("warnings", len(warnings)))
			}

			for _, w := range warnings {
				_, err := d.warning.Emit(ctx, w)
				if err != nil {
					return fmt.Errorf("Failed to trigger warning: %w", err)
				}
			}

			return nil
		})
		if err != nil {
			log.Error("Failed to update sync warnings", slog.Any("error", err))
		}
	}()

	networksBySrc := map[string]map[string]migration.Network{}
	instancesBySrc := map[string]map[uuid.UUID]migration.Instance{}
	for _, src := range vmSourcesByName {
		props, err := src.GetVMwareProperties()
		if err != nil {
			return err
		}

		log := log.With(slog.String("source", src.Name))

		if src.GetExternalConnectivityStatus() != api.EXTERNALCONNECTIVITYSTATUS_OK {
			warnings = append(warnings, migration.NewSyncWarning(api.SourceUnavailable, src.Name, fmt.Sprintf("status: %q", src.GetExternalConnectivityStatus())))
			log.Warn("Skipping source that hasn't passed connectivity check")
			continue
		}

		ctx, cancel := context.WithTimeout(ctx, props.ConnectionTimeout.Duration)
		srcNetworks, srcInstances, importWarnings, err := fetchVMWareSourceData(ctx, src)
		if err != nil {
			cancel()
			warnings = append(warnings, migration.NewSyncWarning(api.InstanceImportFailed, src.Name, err.Error()))
			log.Error("Failed to fetch records from source", logger.Err(err))
			continue
		}

		cancel()

		warnings = append(warnings, importWarnings...)

		networksBySrc[src.Name] = srcNetworks
		instancesBySrc[src.Name] = srcInstances
	}

	for _, src := range networkSourcesByName {
		props, err := src.GetVMwareProperties()
		if err != nil {
			return err
		}

		if src.GetExternalConnectivityStatus() != api.EXTERNALCONNECTIVITYSTATUS_OK {
			warnings = append(warnings, migration.NewSyncWarning(api.SourceUnavailable, src.Name, fmt.Sprintf("status: %q", src.GetExternalConnectivityStatus())))
			continue
		}

		ctx, cancel := context.WithTimeout(ctx, props.ConnectionTimeout.Duration)
		found, err := fetchNSXSourceData(ctx, src, vmSourcesByName, networksBySrc)
		if err != nil {
			cancel()
			warnings = append(warnings, migration.NewSyncWarning(api.NetworkImportFailed, src.Name, err.Error()))
			log.Error("Failed to fetch records from source", logger.Err(err))
			continue
		}

		cancel()

		if found {
			break
		}
	}

	for srcName, networks := range networksBySrc {
		for _, net := range networks {
			if net.Type == api.NETWORKTYPE_VMWARE_NSX || net.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX {
				var props internalAPI.NSXNetworkProperties
				err := json.Unmarshal(net.Properties, &props)
				if err != nil {
					return err
				}

				if props.Segment.Name == "" {
					warnings = append(warnings, migration.NewSyncWarning(api.InstanceMissingNetworkSource, srcName, fmt.Sprintf("No NSX source for network %q", net.Location)))
				}
			}
		}
	}

	// Filter out instances with duplicate UUIDs.
	allInstancesByUUID := map[uuid.UUID]migration.Instance{}
	for srcName, instancesByUUID := range instancesBySrc {
		duplicateUUIDs := []uuid.UUID{}
		for _, inst := range instancesByUUID {
			existing, ok := allInstancesByUUID[inst.UUID]
			if !ok {
				allInstancesByUUID[inst.UUID] = inst
				continue
			}

			msg := fmt.Sprintf("Duplicate UUIDs: %q. Skipped instance %q on source %q, keeping instance %q on source %q", inst.UUID.String(), inst.Properties.Location, srcName, existing.Properties.Location, existing.Source)
			warnings = append(warnings, migration.NewSyncWarning(api.InstanceImportFailed, srcName, msg))

			// Record the instance to ignore.
			duplicateUUIDs = append(duplicateUUIDs, inst.UUID)

			log := log.With(
				slog.String("source_recorded", existing.Source),
				slog.String("location_recorded", existing.Properties.Location),
				slog.String("source_ignored", inst.Source),
				slog.String("location_ignored", inst.Properties.Location))

			log.Warn("Detected instance with duplicate UUID on different sources. Update instance configuration on one source to register both instances")
		}

		// Remove duplicate UUID instances from consideration for this source.
		for _, instUUID := range duplicateUUIDs {
			delete(instancesByUUID, instUUID)
		}

		instancesBySrc[srcName] = instancesByUUID
	}

	srcWarnings, err := d.syncSourceData(ctx, instancesBySrc, networksBySrc)
	if err != nil {
		for srcName := range instancesBySrc {
			warnings = append(warnings, migration.NewSyncWarning(api.InstanceImportFailed, srcName, fmt.Sprintf("Failed to update records: %v", err)))
		}

		return err
	}

	warnings = append(warnings, srcWarnings...)

	return nil
}

// syncSourceData fetches instance and network data from the source and updates our database records.
func (d *Daemon) syncOneSource(ctx context.Context, src migration.Source) error {
	slog.Info("Syncing source", slog.String("source", src.Name))
	nsxSources, err := d.source.GetAll(ctx, api.SOURCETYPE_NSX)
	if err != nil {
		return fmt.Errorf("Failed to retrieve %q sources: %w", api.SOURCETYPE_NSX, err)
	}

	warnings := migration.Warnings{}
	defer func() {
		err := transaction.Do(ctx, func(ctx context.Context) error {
			sourceScope := api.WarningScopeSync()
			sourceScope.Entity = src.Name
			err := d.warning.RemoveStale(ctx, sourceScope, warnings)
			if err != nil {
				return fmt.Errorf("Failed to clean up warnings: %w", err)
			}

			for _, w := range warnings {
				_, err := d.warning.Emit(ctx, w)
				if err != nil {
					return fmt.Errorf("Failed to trigger warning: %w", err)
				}
			}

			return nil
		})
		if err != nil {
			slog.Error("Failed to update sync warnings", slog.Any("error", err))
		}
	}()

	props, err := src.GetVMwareProperties()
	if err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, props.ConnectionTimeout.Duration)
	defer cancel()

	srcNetworks, srcInstances, importWarnings, err := fetchVMWareSourceData(timeoutCtx, src)
	if err != nil {
		warnings = append(warnings, migration.NewSyncWarning(api.InstanceImportFailed, src.Name, err.Error()))
		return err
	}

	warnings = append(warnings, importWarnings...)

	sourcesByName := map[string]migration.Source{src.Name: src}
	instancesBySrc := map[string]map[uuid.UUID]migration.Instance{src.Name: srcInstances}
	networksBySrc := map[string]map[string]migration.Network{src.Name: srcNetworks}

	syncNSX := false
	for _, net := range srcNetworks {
		if net.Type == api.NETWORKTYPE_VMWARE_NSX || net.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX {
			syncNSX = true
			break
		}
	}

	var nsxIP string
	if syncNSX {
		var matchingNSXSource bool
		for _, nsxSource := range nsxSources {
			if nsxSource.GetExternalConnectivityStatus() != api.EXTERNALCONNECTIVITYSTATUS_OK {
				warnings = append(warnings, migration.NewSyncWarning(api.SourceUnavailable, src.Name, fmt.Sprintf("status: %q", src.GetExternalConnectivityStatus())))
				continue
			}

			matchingNSXSource, err = fetchNSXSourceData(timeoutCtx, nsxSource, sourcesByName, networksBySrc)
			if err != nil {
				warnings = append(warnings, migration.NewSyncWarning(api.NetworkImportFailed, src.Name, err.Error()))
				return fmt.Errorf("Failed to fetch network properties from NSX: %w", err)
			}

			if matchingNSXSource {
				break
			}
		}

		// If there are no matching NSX sources, try to see if any exist, and record them.
		if !matchingNSXSource {
			for _, net := range srcNetworks {
				if net.Type == api.NETWORKTYPE_VMWARE_NSX || net.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX {
					var props internalAPI.NSXNetworkProperties
					err := json.Unmarshal(net.Properties, &props)
					if err != nil {
						return err
					}

					if props.Segment.Name == "" {
						warnings = append(warnings, migration.NewSyncWarning(api.InstanceMissingNetworkSource, src.Name, fmt.Sprintf("No NSX source for network %q", net.Location)))
					}
				}
			}

			vmwareSrc, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
			if err != nil {
				return fmt.Errorf("Failed to convert source %q to %q source: %w", src.Name, src.SourceType, err)
			}

			err = vmwareSrc.Connect(ctx)
			if err != nil {
				return fmt.Errorf("Failed to connect to source %q: %w", src.Name, err)
			}

			nsxIP, err = vmwareSrc.GetNSXManagerIP(timeoutCtx)
			if err != nil {
				return fmt.Errorf("Failed to look for NSX Managers for source %q: %w", src.Name, err)
			}
		}
	}

	return transaction.Do(ctx, func(ctx context.Context) error {
		nsxURL, err := url.Parse(nsxIP)
		if err == nil && nsxURL.String() != "" {
			// If we detected an unrecorded NSX Manager, try to add a basic source entry with AUTH ERROR hinting to the user to setup authentication.
			props := api.VMwareProperties{
				Endpoint:           nsxIP,
				ConnectivityStatus: api.EXTERNALCONNECTIVITYSTATUS_AUTH_ERROR,
			}

			b, err := json.Marshal(props)
			if err != nil {
				return fmt.Errorf("Failed to marshal source properties: %w", err)
			}

			_, err = d.source.Create(ctx, migration.Source{
				Name:       nsxURL.Hostname(),
				SourceType: api.SOURCETYPE_NSX,
				Properties: b,
				EndpointFunc: func(s api.Source) (migration.SourceEndpoint, error) {
					return source.NewInternalNSXSourceFrom(s)
				},
			})
			if err != nil {
				return fmt.Errorf("Failed to record %q source for %q source %q: %w", api.SOURCETYPE_NSX, api.SOURCETYPE_VMWARE, src.Name, err)
			}
		}

		srcWarnings, err := d.syncSourceData(ctx, instancesBySrc, networksBySrc)
		if err != nil {
			return err
		}

		warnings = append(warnings, srcWarnings...)

		return nil
	})
}

// syncSourceData is a helper that opens a transaction and updates the internal record of all sources with the supplied data.
func (d *Daemon) syncSourceData(ctx context.Context, instancesBySrc map[string]map[uuid.UUID]migration.Instance, networksBySrc map[string]map[string]migration.Network) (migration.Warnings, error) {
	syncLock.Lock()
	defer syncLock.Unlock()

	warnings := migration.Warnings{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		assignedInstances, err := d.instance.GetAllAssigned(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get unassigned internal instance records: %w", err)
		}

		instanceIsAssigned := make(map[uuid.UUID]bool, len(assignedInstances))
		for _, inst := range assignedInstances {
			instanceIsAssigned[inst.UUID] = true
		}

		// Generate UUIDs for networks.
		dbNetworksBySrc := map[string]migration.Networks{}
		for srcName, srcNetworks := range networksBySrc {
			dbNetworksBySrc[srcName], err = d.network.GetAllBySource(ctx, srcName)
			if err != nil {
				return fmt.Errorf("Failed to get internal network records for source %q: %w", srcName, err)
			}

			dbNetworksByID := map[string]migration.Network{}
			netUUIDsByID := map[string]uuid.UUID{}
			for _, net := range dbNetworksBySrc[srcName] {
				dbNetworksByID[net.SourceSpecificID] = net
				netUUIDsByID[net.SourceSpecificID] = net.UUID
			}

			// Generate a UUID for each newly discovered network.
			for _, srcNet := range srcNetworks {
				dbNet, ok := dbNetworksByID[srcNet.SourceSpecificID]
				if ok {
					srcNet.UUID = dbNet.UUID
				} else {
					srcNet.UUID, err = uuid.NewRandom()
					if err != nil {
						return fmt.Errorf("Failed to generate UUID for network %q in source %q: %w", srcNet.Location, srcName, err)
					}

					netUUIDsByID[srcNet.SourceSpecificID] = srcNet.UUID
				}

				networksBySrc[srcName][srcNet.SourceSpecificID] = srcNet
			}

			// Update instances NICs with the network UUID too.
			for _, inst := range instancesBySrc[srcName] {
				for i, nic := range inst.Properties.NICs {
					netUUID, ok := netUUIDsByID[nic.SourceSpecificID]
					if ok {
						inst.Properties.NICs[i].UUID = netUUID
					}
				}

				instancesBySrc[srcName][inst.UUID] = inst
			}
		}

		// Sync instances before networks so we can prune networks according to the most up-to-date information.
		for srcName, srcInstances := range instancesBySrc {
			// Ensure we only compare instances in the same source.
			existingInstances := map[uuid.UUID]migration.Instance{}
			allInstances, err := d.instance.GetAllBySource(ctx, srcName)
			if err != nil {
				return fmt.Errorf("Failed to get internal instance records for source %q: %w", srcName, err)
			}

			for _, inst := range allInstances {
				// If the instance is already assigned, then omit it from consideration, unless it is disabled.
				if instanceIsAssigned[inst.UUID] && inst.DisabledReason(api.InstanceRestrictionOverride{}) == nil {
					delete(srcInstances, inst.UUID)
					continue
				}

				existingInstances[inst.UUID] = inst
			}

			srcWarnings, err := d.syncInstancesFromSource(ctx, srcName, d.instance, existingInstances, srcInstances)
			if err != nil {
				return fmt.Errorf("Failed to sync instances from %q: %w", srcName, err)
			}

			warnings = append(warnings, srcWarnings...)
		}

		for srcName, srcNetworks := range networksBySrc {
			// Ensure we only compare networks in the same source.
			existingNetworks := map[string]migration.Network{}

			allInstancesBySrc, err := d.instance.GetAllBySource(ctx, srcName)
			if err != nil {
				return fmt.Errorf("Failed to get all instances in source %q: %w", srcName, err)
			}

			// Build maps to make comparison easier.
			assignedNetworksByName := map[string]migration.Network{}
			for _, net := range migration.FilterUsedNetworks(dbNetworksBySrc[srcName], assignedInstances) {
				assignedNetworksByName[net.SourceSpecificID] = net
			}

			usedNetworksByName := map[string]migration.Network{}
			for _, net := range migration.FilterUsedNetworks(dbNetworksBySrc[srcName], allInstancesBySrc) {
				usedNetworksByName[net.SourceSpecificID] = net
			}

			for _, dbNetwork := range dbNetworksBySrc[srcName] {
				// If the network is already assigned to a batch, or has no instances using it, then omit it from consideration.
				_, netHasQueuedInstance := assignedNetworksByName[dbNetwork.SourceSpecificID]
				_, netHasInstance := usedNetworksByName[dbNetwork.SourceSpecificID]
				if netHasQueuedInstance || !netHasInstance {
					_, ok := srcNetworks[dbNetwork.SourceSpecificID]
					if ok {
						delete(srcNetworks, dbNetwork.SourceSpecificID)
					}

					continue
				}

				// If the network data came from the instance, but we already have a network record from an NSX source, then don't overwrite it.
				// We may get here if NSX somehow returns an error in this sync, but not in an earlier one.
				srcNet, ok := srcNetworks[dbNetwork.SourceSpecificID]
				if ok && slices.Contains([]api.NetworkType{api.NETWORKTYPE_VMWARE_NSX, api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX}, dbNetwork.Type) {
					var existingProps internalAPI.NSXNetworkProperties
					err := json.Unmarshal(dbNetwork.Properties, &existingProps)
					if err != nil {
						return err
					}

					var newProps internalAPI.VCenterNetworkProperties
					err = json.Unmarshal(srcNet.Properties, &newProps)
					if err != nil {
						return err
					}

					if existingProps.Segment.Name != "" && newProps.SegmentPath != "" {
						slog.Info("Not syncing NSX network due to missing NSX configuration", slog.String("source", dbNetwork.Source), slog.String("identifier", dbNetwork.SourceSpecificID), slog.String("location", dbNetwork.Location))
						// Also remove the source network entry so that the network is ignored.
						_, ok := srcNetworks[dbNetwork.SourceSpecificID]
						if ok {
							delete(srcNetworks, dbNetwork.SourceSpecificID)
						}

						continue
					}
				}

				existingNetworks[dbNetwork.SourceSpecificID] = dbNetwork
			}

			err = d.syncNetworksFromSource(ctx, srcName, d.network, existingNetworks, srcNetworks)
			if err != nil {
				return fmt.Errorf("Failed to sync networks from %q: %w", srcName, err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return warnings, nil
}

// syncNetworksFromSource updates migration manager's internal record of networks from the source.
func (d *Daemon) syncNetworksFromSource(ctx context.Context, sourceName string, n migration.NetworkService, existingNetworks map[string]migration.Network, srcNetworks map[string]migration.Network) error {
	log := slog.With(
		slog.String("method", "syncNetworksFromSource"),
		slog.String("source", sourceName),
	)

	for name, network := range existingNetworks {
		srcNet, ok := srcNetworks[name]
		if !ok {
			// Delete the instances that don't exist on the source.
			log.Info("Deleting instance with no source record")
			err := n.DeleteByNameAndSource(ctx, name, network.Source)
			if err != nil {
				return err
			}

			apiNet, err := network.ToAPI()
			if err != nil {
				return err
			}

			d.logHandler.SendLifecycle(ctx, event.NewNetworkEvent(event.NetworkRemoved, nil, *apiNet, apiNet.UUID))

			continue
		}

		networkUpdated := false
		if network.Location != srcNet.Location {
			network.Location = srcNet.Location
			networkUpdated = true
		}

		if network.Type != srcNet.Type {
			network.Type = srcNet.Type
			networkUpdated = true
		}

		if !bytes.Equal(network.Properties, srcNet.Properties) {
			network.Properties = srcNet.Properties
			networkUpdated = true
		}

		if networkUpdated {
			log.Info("Syncing changes to network from source")
			err := n.Update(ctx, &network)
			if err != nil {
				return fmt.Errorf("Failed to update network: %w", err)
			}

			apiNet, err := network.ToAPI()
			if err != nil {
				return err
			}

			d.logHandler.SendLifecycle(ctx, event.NewNetworkEvent(event.NetworkModified, nil, *apiNet, apiNet.UUID))
		}
	}

	// Create any missing networks.
	for name, network := range srcNetworks {
		_, ok := existingNetworks[name]
		if !ok {
			log := log.With(slog.String("network_id", network.SourceSpecificID), slog.String("network", network.Location))
			log.Info("Recording new network detected on source")
			newNet, err := n.Create(ctx, network)
			if err != nil {
				return fmt.Errorf("Failed to create network %q (%q): %w", network.SourceSpecificID, network.Location, err)
			}

			apiNet, err := newNet.ToAPI()
			if err != nil {
				return err
			}

			d.logHandler.SendLifecycle(ctx, event.NewNetworkEvent(event.NetworkImported, nil, *apiNet, apiNet.UUID))
		}
	}

	return nil
}

// syncInstancesFromSource updates migration manager's internal record of instances from the source.
func (d *Daemon) syncInstancesFromSource(ctx context.Context, sourceName string, i migration.InstanceService, existingInstances map[uuid.UUID]migration.Instance, srcInstances map[uuid.UUID]migration.Instance) (migration.Warnings, error) {
	log := slog.With(
		slog.String("method", "syncInstancesFromSource"),
		slog.String("source", sourceName),
	)

	warnings := migration.Warnings{}
	for instUUID, inst := range existingInstances {
		log := log.With(slog.Any("uuid", instUUID), slog.String("location", inst.Properties.Location))

		srcInst, ok := srcInstances[instUUID]
		if !ok {
			// Delete the instances that don't exist on the source.
			log.Info("Deleting instance with no source record")
			err := i.DeleteByUUID(ctx, instUUID)
			if err != nil && !errors.Is(err, migration.ErrOperationNotPermitted) {
				return nil, fmt.Errorf("Failed to delete instance %q: %w", instUUID, err)
			}

			if err != nil {
				log.Error("Failed to delete instance", slog.Any("error", err))
			} else {
				d.logHandler.SendLifecycle(ctx, event.NewInstanceEvent(event.InstanceRemoved, nil, inst.ToAPI(), inst.UUID))
			}

			continue
		}

		log.Debug("Comparing instance")
		instanceUpdated := false

		if inst.Properties.Location != srcInst.Properties.Location {
			log.Debug("Instance location changed", slog.String("new_location", srcInst.Properties.Location))
			inst.Properties.Location = srcInst.Properties.Location
			instanceUpdated = true
		}

		if inst.Properties.Name != srcInst.Properties.Name {
			log.Debug("Instance name changed", slog.String("new", srcInst.Properties.Name), slog.String("old", inst.Properties.Name))
			inst.Properties.Name = srcInst.Properties.Name
			instanceUpdated = true
		}

		if inst.Properties.BackgroundImport != srcInst.Properties.BackgroundImport {
			log.Debug("Instance background import changed", slog.Bool("new", srcInst.Properties.BackgroundImport), slog.Bool("old", inst.Properties.BackgroundImport))
			inst.Properties.BackgroundImport = srcInst.Properties.BackgroundImport
			instanceUpdated = true
		}

		if inst.Properties.Description != srcInst.Properties.Description {
			log.Debug("Instance description changed", slog.String("new", srcInst.Properties.Description), slog.String("old", inst.Properties.Description))
			inst.Properties.Description = srcInst.Properties.Description
			instanceUpdated = true
		}

		if inst.Properties.Architecture != srcInst.Properties.Architecture && srcInst.Properties.Architecture != "" {
			log.Debug("Instance architecture changed", slog.String("new", srcInst.Properties.Architecture), slog.String("old", inst.Properties.Architecture))
			inst.Properties.Architecture = srcInst.Properties.Architecture
			instanceUpdated = true
		}

		instanceIncomplete := false
		if srcInst.Properties.Architecture == "" || srcInst.Properties.OS == "" || srcInst.Properties.OSVersion == "" {
			instanceIncomplete = true
		}

		for _, nic := range srcInst.Properties.NICs {
			if nic.IPv4Address == "" {
				instanceIncomplete = true
				break
			}
		}

		if instanceIncomplete {
			warnings = append(warnings, migration.NewSyncWarning(api.InstanceIncomplete, inst.Source, fmt.Sprintf("%q has incomplete properties. Ensure VM is powered on and guest agent is running", inst.Properties.Location)))
		}

		// Set fallback architecture.
		if inst.Properties.Architecture == "" {
			arch, err := osarch.ArchitectureName(osarch.ARCH_64BIT_INTEL_X86)
			if err != nil {
				return nil, err
			}

			inst.Properties.Architecture = arch
			instanceUpdated = true
			log.Debug("Unable to determine architecture; Using fallback", slog.String("architecture", arch))
		}

		if inst.Properties.OS != srcInst.Properties.OS && srcInst.Properties.OS != "" {
			log.Debug("Instance os changed", slog.String("new", srcInst.Properties.OS), slog.String("old", inst.Properties.OS))
			inst.Properties.OS = srcInst.Properties.OS
			instanceUpdated = true
		}

		if inst.Properties.OSVersion != srcInst.Properties.OSVersion && srcInst.Properties.OSVersion != "" {
			log.Debug("Instance os version changed", slog.String("new", srcInst.Properties.OSVersion), slog.String("old", inst.Properties.OSVersion))
			inst.Properties.OSVersion = srcInst.Properties.OSVersion
			instanceUpdated = true
		}

		if !slices.Equal(inst.Properties.Disks, srcInst.Properties.Disks) {
			log.Debug("Instance disks changed")
			inst.Properties.Disks = srcInst.Properties.Disks
			instanceUpdated = true
		}

		if !slices.Equal(inst.Properties.NICs, srcInst.Properties.NICs) {
			oldNics := map[string]api.InstancePropertiesNIC{}
			for _, nic := range inst.Properties.NICs {
				oldNics[nic.SourceSpecificID] = nic
			}

			// Preserve IPs from the previous sync in case the VM has turned off.
			newNics := make([]api.InstancePropertiesNIC, len(srcInst.Properties.NICs))
			for i, nic := range srcInst.Properties.NICs {
				oldNIC, ok := oldNics[nic.SourceSpecificID]
				if ok {
					if nic.IPv4Address == "" && oldNIC.IPv4Address != "" {
						nic.IPv4Address = oldNIC.IPv4Address
					}

					if nic.IPv6Address == "" && oldNIC.IPv6Address != "" {
						nic.IPv6Address = oldNIC.IPv6Address
					}
				}

				newNics[i] = nic
			}

			if !slices.Equal(inst.Properties.NICs, newNics) {
				log.Debug("Instance nics changed")
				instanceUpdated = true
				inst.Properties.NICs = srcInst.Properties.NICs
			}
		}

		if !slices.Equal(inst.Properties.Snapshots, srcInst.Properties.Snapshots) {
			log.Debug("Instance snapshots changed")
			inst.Properties.Snapshots = srcInst.Properties.Snapshots
			instanceUpdated = true
		}

		if inst.Properties.CPUs != srcInst.Properties.CPUs {
			log.Debug("Instance cpu limit changed", slog.Int64("new", srcInst.Properties.CPUs), slog.Int64("old", inst.Properties.CPUs))
			inst.Properties.CPUs = srcInst.Properties.CPUs
			instanceUpdated = true
		}

		if inst.Properties.Memory != srcInst.Properties.Memory {
			log.Debug("Instance memory limit changed", slog.Int64("new", srcInst.Properties.Memory), slog.Int64("old", inst.Properties.Memory))
			inst.Properties.Memory = srcInst.Properties.Memory
			instanceUpdated = true
		}

		if inst.Properties.LegacyBoot != srcInst.Properties.LegacyBoot {
			log.Debug("Instance CSM mode changed", slog.Bool("new", srcInst.Properties.LegacyBoot), slog.Bool("old", inst.Properties.LegacyBoot))
			inst.Properties.LegacyBoot = srcInst.Properties.LegacyBoot
			instanceUpdated = true
		}

		if inst.Properties.SecureBoot != srcInst.Properties.SecureBoot {
			log.Debug("Instance secure boot changed", slog.Bool("new", srcInst.Properties.SecureBoot), slog.Bool("old", inst.Properties.SecureBoot))
			inst.Properties.SecureBoot = srcInst.Properties.SecureBoot
			instanceUpdated = true
		}

		if inst.Properties.TPM != srcInst.Properties.TPM {
			log.Debug("Instance tpm state changed", slog.Bool("new", srcInst.Properties.TPM), slog.Bool("old", inst.Properties.TPM))
			inst.Properties.TPM = srcInst.Properties.TPM
			instanceUpdated = true
		}

		if inst.Properties.Running != srcInst.Properties.Running {
			log.Debug("Instance running state changed", slog.Bool("new", srcInst.Properties.Running), slog.Bool("old", inst.Properties.Running))
			inst.Properties.Running = srcInst.Properties.Running
			instanceUpdated = true
		}

		if instanceUpdated {
			log.Info("Syncing changes to instance from source")
			inst.LastUpdateFromSource = srcInst.LastUpdateFromSource
			err := i.Update(ctx, &inst)
			if err != nil && !errors.Is(err, migration.ErrOperationNotPermitted) {
				return nil, fmt.Errorf("Failed to update instance: %w", err)
			}

			if err != nil {
				log.Error("Failed to update instance", slog.Any("error", err))
			} else {
				d.logHandler.SendLifecycle(ctx, event.NewInstanceEvent(event.InstanceModified, nil, inst.ToAPI(), inst.UUID))
			}
		}
	}

	// Create any missing instances.
	for instUUID, inst := range srcInstances {
		_, ok := existingInstances[instUUID]
		if !ok {
			log := log.With(
				slog.String("location", inst.Properties.Location),
				slog.Any("uuid", inst.UUID),
			)

			// Set fallback architecture.
			if inst.Properties.Architecture == "" {
				arch, err := osarch.ArchitectureName(osarch.ARCH_64BIT_INTEL_X86)
				if err != nil {
					return nil, err
				}

				inst.Properties.Architecture = arch
				log.Debug("Unable to determine architecture; Using fallback", slog.String("architecture", arch))
			}

			log.Info("Recording new instance detected on source")
			newInst, err := i.Create(ctx, inst)
			if err != nil {
				return nil, fmt.Errorf("Failed to create instance %q (%q): %w", inst.UUID.String(), inst.Properties.Location, err)
			}

			d.logHandler.SendLifecycle(ctx, event.NewInstanceEvent(event.InstanceImported, nil, newInst.ToAPI(), newInst.UUID))
		}
	}

	return warnings, nil
}

func fetchNSXSourceData(ctx context.Context, src migration.Source, vcenterSources map[string]migration.Source, networksBySrc map[string]map[string]migration.Network) (bool, error) {
	log := slog.With(
		slog.String("method", "fetchNSXSourceData"),
		slog.String("source", src.Name),
	)

	log.Debug("Fetching NSX data for source")
	s, err := source.NewInternalNSXSourceFrom(src.ToAPI())
	if err != nil {
		return false, fmt.Errorf("Failed to create NSX source from source %q: %w", src.Name, err)
	}

	err = s.Connect(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to connect to source %q: %w", src.Name, err)
	}

	vcenters, err := s.GetComputeManagers(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to fetch compute managers from source %q: %w", src.Name, err)
	}

	// Collect all vcenter compute managers.
	vcentersByURL := map[string]internalAPI.NSXComputeManager{}
	for _, vcenter := range vcenters {
		if vcenter.Type == "vCenter" {
			vcentersByURL[vcenter.Server] = vcenter
		}
	}

	// If we have fetched networks that belong to a vcenter that has an associated NSX manager, then fetch all the NSX segments.
	var fetchSegments bool
	for _, vcenter := range vcenterSources {
		var props api.VMwareProperties
		err := json.Unmarshal(vcenter.Properties, &props)
		if err != nil {
			return false, err
		}

		vcURL, err := url.Parse(props.Endpoint)
		if err != nil {
			return false, fmt.Errorf("Failed to parse vCenter URL: %q", props.Endpoint)
		}

		_, ok := vcentersByURL[vcURL.Host]
		if !ok {
			continue
		}

		log.Info("Detected a vCenter in use by the NSX source", slog.String("vcenter", vcenter.Name))
		if networksBySrc[vcenter.Name] != nil {
			for _, network := range networksBySrc[vcenter.Name] {
				if network.Type == api.NETWORKTYPE_VMWARE_NSX || network.Type == api.NETWORKTYPE_VMWARE_DISTRIBUTED_NSX {
					fetchSegments = true
					log.Info("Detected networks in use by the NSX source", slog.String("vcenter", vcenter.Name), slog.String("network", network.Location))
					break
				}
			}
		}

		if fetchSegments {
			break
		}
	}

	if !fetchSegments {
		return false, nil
	}

	segments, err := s.GetSegments(ctx, false)
	if err != nil {
		return false, err
	}

	vms, err := s.GetVMs(ctx)
	if err != nil {
		return false, err
	}

	segmentData := map[string]*internalAPI.NSXSegment{}
	for _, baseSegment := range segments {
		for src, networks := range networksBySrc {
			for name, network := range networks {
				var vcProps internalAPI.VCenterNetworkProperties
				err := json.Unmarshal(network.Properties, &vcProps)
				if err != nil {
					return false, err
				}

				if vcProps.SegmentPath == "" {
					continue
				}

				if vcProps.SegmentPath == baseSegment.Path {
					segment, ok := segmentData[baseSegment.Path]
					if !ok {
						segment, err = s.AddSegmentData(ctx, &baseSegment, vms)
						if err != nil {
							return false, err
						}

						segmentData[segment.Path] = segment
					}

					log.Info("Recording NSX segment data", slog.String("segment", segment.Path), slog.String("network", network.Location))
					nsxProps := internalAPI.NSXNetworkProperties{
						Source:  s.Name,
						Segment: *segment,
					}

					if vcProps.TransportZoneUUID != uuid.Nil {
						log.Info("Recording NSX transport zone data", slog.String("zone", vcProps.TransportZoneUUID.String()), slog.String("network", network.Location))
						zone, err := s.GetTransportZone(ctx, vcProps.TransportZoneUUID)
						if err != nil {
							return false, err
						}

						nsxProps.TransportZone = *zone
					}

					b, err := json.Marshal(nsxProps)
					if err != nil {
						return false, err
					}

					network.Properties = b
					networks[name] = network
				}
			}

			networksBySrc[src] = networks
		}
	}

	return true, nil
}

// fetchVMWareSourceData connects to a VMWare source and returns the resources we care about, keyed by their unique identifiers.
func fetchVMWareSourceData(ctx context.Context, src migration.Source) (map[string]migration.Network, map[uuid.UUID]migration.Instance, migration.Warnings, error) {
	slog.Debug("Fetching VMware data for source", slog.String("source", src.Name))
	s, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to create VMwareSource from source: %w", err)
	}

	err = s.Connect(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to connect to source: %w", err)
	}

	instances, allNetworks, warnings, err := s.GetAllVMs(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to get VMs: %w", err)
	}

	// Only record networks that are actually in use by detected VMs.
	networks := migration.FilterUsedNetworks(allNetworks, instances)

	networkMap := make(map[string]migration.Network, len(networks))
	instanceMap := make(map[uuid.UUID]migration.Instance, len(instances))

	for _, network := range networks {
		networkMap[network.SourceSpecificID] = network
	}

	for _, inst := range instances {
		existing, ok := instanceMap[inst.UUID]
		if ok {
			log := slog.With(
				slog.String("source", src.Name),
				slog.String("location_recorded", existing.Properties.Location),
				slog.String("location_ignored", inst.Properties.Location))

			msg := fmt.Sprintf("Duplicate UUIDs: %q. Skipped instance %q, keeping instance %q", inst.UUID.String(), inst.Properties.Location, existing.Properties.Location)
			warnings = append(warnings, migration.NewSyncWarning(api.InstanceImportFailed, src.Name, msg))
			log.Warn("Detected instance with duplicate UUID. Update instance configuration on source to register this instance")
			continue
		}

		instanceMap[inst.UUID] = inst
	}

	return networkMap, instanceMap, warnings, nil
}
