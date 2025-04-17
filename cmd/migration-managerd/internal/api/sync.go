package api

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/google/uuid"

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
	sourcesByName := map[string]migration.Source{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the list of configured sources.
		sources, err := d.source.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all sources: %w", err)
		}

		for _, src := range sources {
			sourcesByName[src.Name] = src
		}

		return nil
	})
	if err != nil {
		return err
	}

	networksBySrc := map[string]map[string]api.Network{}
	instancesBySrc := map[string]map[uuid.UUID]migration.Instance{}
	for _, src := range sourcesByName {
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

	return d.syncSourceData(ctx, sourcesByName, instancesBySrc, networksBySrc)
}

// syncSourceData fetches instance and network data from the source and updates our database records.
func (d *Daemon) syncOneSource(ctx context.Context, src migration.Source) error {
	srcNetworks, srcInstances, err := fetchVMWareSourceData(ctx, src)
	if err != nil {
		return err
	}

	sourcesByName := map[string]migration.Source{src.Name: src}
	instancesBySrc := map[string]map[uuid.UUID]migration.Instance{src.Name: srcInstances}
	networksBySrc := map[string]map[string]api.Network{src.Name: srcNetworks}
	return d.syncSourceData(ctx, sourcesByName, instancesBySrc, networksBySrc)
}

// syncSourceData is a helper that opens a transaction and updates the internal record of all sources with the supplied data.
func (d *Daemon) syncSourceData(ctx context.Context, sourcesByName map[string]migration.Source, instancesBySrc map[string]map[uuid.UUID]migration.Instance, networksBySrc map[string]map[string]api.Network) error {
	syncLock.Lock()
	defer syncLock.Unlock()

	return transaction.Do(ctx, func(ctx context.Context) error {
		// Get the list of configured sources.
		dbNetworks, err := d.network.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get internal network records: %w", err)
		}

		dbInstances, err := d.instance.GetAll(ctx, false)
		if err != nil {
			return fmt.Errorf("Failed to get internal instance records: %w", err)
		}

		// Build maps to make comparison easier.
		existingNetworks := make(map[string]migration.Network, len(dbNetworks))
		for _, net := range dbNetworks {
			existingNetworks[net.Name] = net
		}

		for srcName, srcInstances := range instancesBySrc {
			// Ensure we only compare instances in the same source.
			existingInstances := map[uuid.UUID]migration.Instance{}
			for _, inst := range dbInstances {
				// If the instance is already assigned, then omit it from consideration.
				if inst.Batch != nil {
					_, ok := srcInstances[inst.UUID]
					if ok {
						delete(srcInstances, inst.UUID)
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

		for srcName, srcNetworks := range networksBySrc {
			err = syncNetworksFromSource(ctx, srcName, d.network, existingNetworks, srcNetworks)
			if err != nil {
				return fmt.Errorf("Failed to sync networks from %q: %w", srcName, err)
			}
		}

		return nil
	})
}

// syncNetworksFromSource updates migration manager's internal record of networks from the source.
func syncNetworksFromSource(ctx context.Context, sourceName string, n migration.NetworkService, existingNetworks map[string]migration.Network, srcNetworks map[string]api.Network) error {
	log := slog.With(
		slog.String("method", "syncNetworksFromSource"),
		slog.String("source", sourceName),
	)

	for name, network := range existingNetworks {
		srcNet, ok := srcNetworks[name]
		if !ok {
			// TODO: Do more than pick up new networks, also delete removed networks and update existing networks with any changes.
			// Currently, the entire network list is given in existingNetworks for each source, so we will need to be smarter about filtering that as well.
			continue
		}

		networkUpdated := false
		if network.Location != srcNet.Location {
			network.Location = srcNet.Location
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
			log := log.With(slog.String("network", network.Name))
			log.Info("Recording new network detected on source")
			_, err := n.Create(ctx, migration.Network{Name: network.Name, Config: network.Config, Location: network.Location})
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

// fetchVMWareSourceData connects to a VMWare source and returns the resources we care about, keyed by their unique identifiers.
func fetchVMWareSourceData(ctx context.Context, src migration.Source) (map[string]api.Network, map[uuid.UUID]migration.Instance, error) {
	s, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create VMwareSource from source: %w", err)
	}

	err = s.Connect(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to connect to source: %w", err)
	}

	networks, err := s.GetAllNetworks(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get networks: %w", err)
	}

	instances, err := s.GetAllVMs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get VMs: %w", err)
	}

	networkMap := make(map[string]api.Network, len(networks))
	instanceMap := make(map[uuid.UUID]migration.Instance, len(instances))

	for _, network := range networks {
		networkMap[network.Name] = network
	}

	for _, inst := range instances {
		instanceMap[inst.UUID] = inst
	}

	return networkMap, instanceMap, nil
}
