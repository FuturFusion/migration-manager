package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"sync"

	"github.com/google/uuid"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// syncLock ensures source syncing is sequential.
var syncLock sync.Mutex

// trySyncAllSources connects to each source in the database and updates the in-memory record of all networks and instances.
// skipNonResponsiveSources - If true, if a connection to a source returns an error, syncing from that source will be skipped.
func (d *Daemon) trySyncAllSources(ctx context.Context) error {
	log := slog.With(slog.String("method", "syncAllSources"))
	vmSourcesByName := map[string]migration.Source{}
	networkSourcesByName := map[string]migration.Source{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the list of configured sources.
		sources, err := d.source.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all sources: %w", err)
		}

		for _, src := range sources {
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

	networksBySrc := map[string]map[string]migration.Network{}
	instancesBySrc := map[string]map[uuid.UUID]migration.Instance{}
	for _, src := range vmSourcesByName {
		log := log.With(slog.String("source", src.Name))

		if src.GetExternalConnectivityStatus() != api.EXTERNALCONNECTIVITYSTATUS_OK {
			log.Warn("Skipping source that hasn't passed connectivity check")
			continue
		}

		srcNetworks, srcInstances, err := fetchVMWareSourceData(ctx, src)
		if err != nil {
			log.Error("Failed to fetch records from source", logger.Err(err))
			continue
		}

		networksBySrc[src.Name] = srcNetworks
		instancesBySrc[src.Name] = srcInstances
	}

	for _, src := range networkSourcesByName {
		found, err := fetchNSXSourceData(ctx, src, vmSourcesByName, networksBySrc)
		if err != nil {
			log.Error("Failed to fetch records from source", logger.Err(err))
			continue
		}

		if found {
			break
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

	return d.syncSourceData(ctx, vmSourcesByName, instancesBySrc, networksBySrc)
}

// syncSourceData fetches instance and network data from the source and updates our database records.
func (d *Daemon) syncOneSource(ctx context.Context, src migration.Source) error {
	nsxSources, err := d.source.GetAll(ctx, api.SOURCETYPE_NSX)
	if err != nil {
		return fmt.Errorf("Failed to retrieve %q sources: %w", api.SOURCETYPE_NSX, err)
	}

	srcNetworks, srcInstances, err := fetchVMWareSourceData(ctx, src)
	if err != nil {
		return err
	}

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

	if syncNSX {
		for _, nsxSource := range nsxSources {
			found, err := fetchNSXSourceData(ctx, nsxSource, sourcesByName, networksBySrc)
			if err != nil {
				return fmt.Errorf("Failed to fetch network properties from NSX: %w", err)
			}

			if found {
				break
			}
		}
	}

	return d.syncSourceData(ctx, sourcesByName, instancesBySrc, networksBySrc)
}

// syncSourceData is a helper that opens a transaction and updates the internal record of all sources with the supplied data.
func (d *Daemon) syncSourceData(ctx context.Context, sourcesByName map[string]migration.Source, instancesBySrc map[string]map[uuid.UUID]migration.Instance, networksBySrc map[string]map[string]migration.Network) error {
	syncLock.Lock()
	defer syncLock.Unlock()

	return transaction.Do(ctx, func(ctx context.Context) error {
		// Get the list of configured sources.
		allInstances, err := d.instance.GetAllUUIDs(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get internal instance records: %w", err)
		}

		unassignedInstances, err := d.instance.GetAllUnassigned(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get unassigned internal instance records: %w", err)
		}

		unassignedInstancesByUUID := make(map[uuid.UUID]migration.Instance, len(unassignedInstances))
		for _, inst := range unassignedInstances {
			unassignedInstancesByUUID[inst.UUID] = inst
		}

		for srcName, srcNetworks := range networksBySrc {
			// Ensure we only compare networks in the same source.
			existingNetworks := map[string]migration.Network{}
			allNetworks, err := d.network.GetAllBySource(ctx, srcName)
			if err != nil {
				return fmt.Errorf("Failed to get internal network records for source %q: %w", srcName, err)
			}

			// Build maps to make comparison easier.
			unassignedNetworksByName := map[string]migration.Network{}
			for _, net := range migration.FilterUsedNetworks(allNetworks, unassignedInstances) {
				unassignedNetworksByName[net.Name] = net
			}

			for _, dbNetwork := range allNetworks {
				// If the network is already assigned, then omit it from consideration.
				network, ok := unassignedNetworksByName[dbNetwork.Name]
				if !ok {
					_, ok := srcNetworks[dbNetwork.Name]
					if ok {
						delete(srcNetworks, dbNetwork.Name)
					}

					continue
				}

				existingNetworks[network.Name] = network
			}

			err = d.syncNetworksFromSource(ctx, srcName, d.network, existingNetworks, srcNetworks)
			if err != nil {
				return fmt.Errorf("Failed to sync networks from %q: %w", srcName, err)
			}
		}

		for srcName, srcInstances := range instancesBySrc {
			// Ensure we only compare instances in the same source.
			existingInstances := map[uuid.UUID]migration.Instance{}
			for _, instUUID := range allInstances {
				// If the instance is already assigned, then omit it from consideration.
				inst, ok := unassignedInstancesByUUID[instUUID]
				if !ok {
					_, ok := srcInstances[instUUID]
					if ok {
						delete(srcInstances, instUUID)
					}

					continue
				}

				src := sourcesByName[srcName]

				if src.Name == inst.Source {
					existingInstances[inst.UUID] = inst
				}
			}

			err = syncInstancesFromSource(ctx, srcName, d.instance, existingInstances, srcInstances)
			if err != nil {
				return fmt.Errorf("Failed to sync instances from %q: %w", srcName, err)
			}
		}

		return nil
	})
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
		}
	}

	// Create any missing networks.
	for name, network := range srcNetworks {
		_, ok := existingNetworks[name]
		if !ok {
			log := log.With(slog.String("network_id", network.Name), slog.String("network", network.Location))
			log.Info("Recording new network detected on source")
			_, err := n.Create(ctx, network)
			if err != nil {
				return fmt.Errorf("Failed to create network %q (%q): %w", network.Name, network.Location, err)
			}
		}
	}

	return nil
}

// syncInstancesFromSource updates migration manager's internal record of instances from the source.
func syncInstancesFromSource(ctx context.Context, sourceName string, i migration.InstanceService, existingInstances map[uuid.UUID]migration.Instance, srcInstances map[uuid.UUID]migration.Instance) error {
	log := slog.With(
		slog.String("method", "syncInstancesFromSource"),
		slog.String("source", sourceName),
	)
	for instUUID, inst := range existingInstances {
		log := log.With(
			slog.String("instance", inst.Properties.Location),
			slog.Any("instance_uuid", inst.UUID),
		)

		srcInst, ok := srcInstances[instUUID]
		if !ok {
			// Delete the instances that don't exist on the source.
			log.Info("Deleting instance with no source record")
			err := i.DeleteByUUID(ctx, instUUID)
			if err != nil {
				return err
			}

			continue
		}

		instanceUpdated := false

		if inst.Properties.Description != srcInst.Properties.Description {
			inst.Properties.Description = srcInst.Properties.Description
			instanceUpdated = true
		}

		if inst.Properties.Architecture != srcInst.Properties.Architecture {
			inst.Properties.Architecture = srcInst.Properties.Architecture
			instanceUpdated = true
		}

		if inst.Properties.OS != srcInst.Properties.OS {
			inst.Properties.OS = srcInst.Properties.OS
			instanceUpdated = true
		}

		if inst.Properties.OSVersion != srcInst.Properties.OSVersion {
			inst.Properties.OSVersion = srcInst.Properties.OSVersion
			instanceUpdated = true
		}

		if !slices.Equal(inst.Properties.Disks, srcInst.Properties.Disks) {
			inst.Properties.Disks = srcInst.Properties.Disks
			instanceUpdated = true
		}

		if !slices.Equal(inst.Properties.NICs, srcInst.Properties.NICs) {
			inst.Properties.NICs = srcInst.Properties.NICs
			instanceUpdated = true
		}

		if !slices.Equal(inst.Properties.Snapshots, srcInst.Properties.Snapshots) {
			inst.Properties.Snapshots = srcInst.Properties.Snapshots
			instanceUpdated = true
		}

		if inst.Properties.CPUs != srcInst.Properties.CPUs {
			inst.Properties.CPUs = srcInst.Properties.CPUs
			instanceUpdated = true
		}

		if inst.Properties.Memory != srcInst.Properties.Memory {
			inst.Properties.Memory = srcInst.Properties.Memory
			instanceUpdated = true
		}

		if inst.Properties.LegacyBoot != srcInst.Properties.LegacyBoot {
			inst.Properties.LegacyBoot = srcInst.Properties.LegacyBoot
			instanceUpdated = true
		}

		if inst.Properties.SecureBoot != srcInst.Properties.SecureBoot {
			inst.Properties.SecureBoot = srcInst.Properties.SecureBoot
			instanceUpdated = true
		}

		if inst.Properties.TPM != srcInst.Properties.TPM {
			inst.Properties.TPM = srcInst.Properties.TPM
			instanceUpdated = true
		}

		if instanceUpdated {
			log.Info("Syncing changes to instance from source")
			inst.LastUpdateFromSource = srcInst.LastUpdateFromSource
			err := i.Update(ctx, &inst)
			if err != nil {
				return fmt.Errorf("Failed to update instance: %w", err)
			}
		}
	}

	// Create any missing instances.
	for instUUID, inst := range srcInstances {
		_, ok := existingInstances[instUUID]
		if !ok {
			log := log.With(
				slog.String("instance", inst.Properties.Location),
				slog.Any("instance_uuid", inst.UUID),
			)

			log.Info("Recording new instance detected on source")
			_, err := i.Create(ctx, inst)
			if err != nil {
				return fmt.Errorf("Failed to create instance %q (%q): %w", inst.UUID.String(), inst.Properties.Location, err)
			}
		}
	}

	return nil
}

func fetchNSXSourceData(ctx context.Context, src migration.Source, vcenterSources map[string]migration.Source, networksBySrc map[string]map[string]migration.Network) (bool, error) {
	log := slog.With(
		slog.String("method", "fetchNSXSourceData"),
		slog.String("source", src.Name),
	)

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
func fetchVMWareSourceData(ctx context.Context, src migration.Source) (map[string]migration.Network, map[uuid.UUID]migration.Instance, error) {
	s, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create VMwareSource from source: %w", err)
	}

	err = s.Connect(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to connect to source: %w", err)
	}

	instances, err := s.GetAllVMs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get VMs: %w", err)
	}

	allNetworks, err := s.GetAllNetworks(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get networks: %w", err)
	}

	// Only record networks that are actually in use by detected VMs.
	networks := migration.FilterUsedNetworks(allNetworks, instances)

	networkMap := make(map[string]migration.Network, len(networks))
	instanceMap := make(map[uuid.UUID]migration.Instance, len(instances))

	for _, network := range networks {
		networkMap[network.Name] = network
	}

	for _, inst := range instances {
		existing, ok := instanceMap[inst.UUID]
		if ok {
			log := slog.With(
				slog.String("source", src.Name),
				slog.String("location_recorded", existing.Properties.Location),
				slog.String("location_ignored", inst.Properties.Location))

			log.Warn("Detected instance with duplicate UUID. Update instance configuration on source to register this instance")
			continue
		}

		instanceMap[inst.UUID] = inst
	}

	return networkMap, instanceMap, nil
}
