package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"
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

// trySyncAllSources connects to each source in the database and updates the in-memory record of all networks and instances.
// skipNonResponsiveSources - If true, if a connection to a source returns an error, syncing from that source will be skipped.
func (d *Daemon) trySyncAllSources(ctx context.Context, skipNonResponsiveSources bool) error {
	log := slog.With(slog.String("method", "syncAllSources"))
	var sources migration.Sources
	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get the list of configured sources.
		var err error
		sources, err = d.source.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all sources: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	networksBySrc := map[string]map[string]api.Network{}
	instancesBySrc := map[string]map[uuid.UUID]migration.Instance{}
	for _, src := range sources {
		srcNetworks, srcInstances, err := fetchVMWareSourceData(ctx, src)
		if err != nil {
			if skipNonResponsiveSources {
				log.Error("Failed to fetch records from source", logger.Err(err))
				continue
			}

			return err
		}

		networksBySrc[src.Name] = srcNetworks
		instancesBySrc[src.Name] = srcInstances
	}

	return d.syncSourceData(ctx, instancesBySrc, networksBySrc)
}

// syncSourceData fetches instance and network data from the source and updates our database records.
func (d *Daemon) syncOneSource(ctx context.Context, src migration.Source) error {
	srcNetworks, srcInstances, err := fetchVMWareSourceData(ctx, src)
	if err != nil {
		return err
	}

	instancesBySrc := map[string]map[uuid.UUID]migration.Instance{src.Name: srcInstances}
	networksBySrc := map[string]map[string]api.Network{src.Name: srcNetworks}
	return d.syncSourceData(ctx, instancesBySrc, networksBySrc)
}

// syncSourceData is a helper that opens a transaction and updates the internal record of all sources with the supplied data.
func (d *Daemon) syncSourceData(ctx context.Context, instancesBySrc map[string]map[uuid.UUID]migration.Instance, networksBySrc map[string]map[string]api.Network) error {
	return transaction.Do(ctx, func(ctx context.Context) error {
		// Get the list of configured sources.
		dbNetworks, err := d.network.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get internal network records: %w", err)
		}

		dbInstances, err := d.instance.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get internal instance records: %w", err)
		}

		// Build maps to make comparison easier.
		existingNetworks := make(map[string]migration.Network, len(dbNetworks))
		for _, net := range dbNetworks {
			existingNetworks[net.Name] = net
		}

		existingInstances := make(map[uuid.UUID]migration.Instance, len(dbInstances))
		for _, inst := range dbInstances {
			existingInstances[inst.UUID] = inst
		}

		for srcName, srcInstances := range instancesBySrc {
			err = syncInstancesFromSource(ctx, srcName, d.instance, existingInstances, srcInstances)
			if err != nil {
				return err
			}
		}

		for srcName, srcNetworks := range networksBySrc {
			err = syncNetworksFromSource(ctx, srcName, d.network, existingNetworks, srcNetworks)
			if err != nil {
				return err
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
	for name, net := range existingNetworks {
		log := log.With(slog.String("network", net.Name))
		_, ok := srcNetworks[name]
		if !ok {
			log.Info("Deleting network with no source record")
			err := n.DeleteByName(ctx, name)
			if err != nil {
				return err
			}

			continue
		}

		// TODO: Replace `networks update` with `networks override` and fetch changes to the source network as well?
	}

	// Create any missing networks.
	for name, net := range srcNetworks {
		_, ok := existingNetworks[name]
		if !ok {
			log := log.With(slog.String("network", net.Name))
			log.Info("Recording new network detected on source")
			_, err := n.Create(ctx, migration.Network{Name: net.Name, Config: net.Config})
			if err != nil {
				return err
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
			slog.String("instance", inst.InventoryPath),
			slog.Any("instance_uuid", inst.UUID),
		)

		srcInst, ok := srcInstances[instUUID]
		if !ok {
			// Delete the instances that don't exist on the source.
			log.Info("Deleting instance with no source record")
			err := i.DeleteByID(ctx, instUUID)
			if err != nil {
				return err
			}

			continue
		}

		instanceUpdated := false

		if inst.Annotation != srcInst.Annotation {
			inst.Annotation = srcInst.Annotation
			instanceUpdated = true
		}

		if inst.GuestToolsVersion != srcInst.GuestToolsVersion {
			inst.GuestToolsVersion = srcInst.GuestToolsVersion
			instanceUpdated = true
		}

		if inst.Architecture != srcInst.Architecture {
			inst.Architecture = srcInst.Architecture
			instanceUpdated = true
		}

		if inst.HardwareVersion != srcInst.HardwareVersion {
			inst.HardwareVersion = srcInst.HardwareVersion
			instanceUpdated = true
		}

		if inst.OS != srcInst.OS {
			inst.OS = srcInst.OS
			instanceUpdated = true
		}

		if inst.OSVersion != srcInst.OSVersion {
			inst.OSVersion = srcInst.OSVersion
			instanceUpdated = true
		}

		if !slices.Equal(inst.Devices, srcInst.Devices) {
			inst.Devices = srcInst.Devices
			instanceUpdated = true
		}

		if !slices.Equal(inst.Disks, srcInst.Disks) {
			inst.Disks = srcInst.Disks
			instanceUpdated = true
		}

		if !slices.Equal(inst.NICs, srcInst.NICs) {
			inst.NICs = srcInst.NICs
			instanceUpdated = true
		}

		if !slices.Equal(inst.Snapshots, srcInst.Snapshots) {
			inst.Snapshots = srcInst.Snapshots
			instanceUpdated = true
		}

		if inst.CPU.NumberCPUs != srcInst.CPU.NumberCPUs {
			inst.CPU.NumberCPUs = srcInst.CPU.NumberCPUs
			instanceUpdated = true
		}

		if !slices.Equal(inst.CPU.CPUAffinity, srcInst.CPU.CPUAffinity) {
			inst.CPU.CPUAffinity = srcInst.CPU.CPUAffinity
			instanceUpdated = true
		}

		if inst.CPU.NumberOfCoresPerSocket != srcInst.CPU.NumberOfCoresPerSocket {
			inst.CPU.NumberOfCoresPerSocket = srcInst.CPU.NumberOfCoresPerSocket
			instanceUpdated = true
		}

		if inst.Memory.MemoryInBytes != srcInst.Memory.MemoryInBytes {
			inst.Memory.MemoryInBytes = srcInst.Memory.MemoryInBytes
			instanceUpdated = true
		}

		if inst.Memory.MemoryReservationInBytes != srcInst.Memory.MemoryReservationInBytes {
			inst.Memory.MemoryReservationInBytes = srcInst.Memory.MemoryReservationInBytes
			instanceUpdated = true
		}

		if inst.UseLegacyBios != srcInst.UseLegacyBios {
			inst.UseLegacyBios = srcInst.UseLegacyBios
			instanceUpdated = true
		}

		if inst.SecureBootEnabled != srcInst.SecureBootEnabled {
			inst.SecureBootEnabled = srcInst.SecureBootEnabled
			instanceUpdated = true
		}

		if inst.TPMPresent != srcInst.TPMPresent {
			inst.TPMPresent = srcInst.TPMPresent
			instanceUpdated = true
		}

		if instanceUpdated {
			log.Info("Syncing changes to instance from source")
			inst.LastUpdateFromSource = srcInst.LastUpdateFromSource
			_, err := i.UpdateByID(ctx, inst)
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
				slog.String("instance", inst.InventoryPath),
				slog.Any("instance_uuid", inst.UUID),
			)

			log.Info("Recording new instance detected on source")
			_, err := i.Create(ctx, inst)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// fetchVMWareSourceData connects to a VMWare source and returns the resources we care about, keyed by their unique identifiers.
func fetchVMWareSourceData(ctx context.Context, src migration.Source) (map[string]api.Network, map[uuid.UUID]migration.Instance, error) {
	s, err := source.NewInternalVMwareSourceFrom(api.Source{
		Name:       src.Name,
		DatabaseID: src.ID,
		SourceType: src.SourceType,
		Properties: src.Properties,
	})
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

	for _, net := range networks {
		networkMap[net.Name] = net
	}

	for _, inst := range instances {
		instanceMap[inst.UUID] = inst
	}

	return networkMap, instanceMap, nil
}

func (d *Daemon) processReadyBatches(ctx context.Context) error {
	log := slog.With(slog.String("method", "processReadyBatches"))
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
					return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_ERROR.String(), err)
				}

				continue
			}

			if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
				log.Error("Batch window end time has already passed")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_ERROR.String(), err)
				}

				continue
			}

			// Get all instances for this batch.
			instances, err := d.instance.GetAllByBatchID(ctx, b.ID)
			if err != nil {
				return fmt.Errorf("Failed to get instances for batch: %w", err)
			}

			// If no instances apply to this batch, return an error.
			if len(instances) == 0 {
				log.Error("Batch has no instances")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "No instances assigned")
				if err != nil {
					return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_ERROR.String(), err)
				}

				continue
			}

			// No issues detected, move to "queued" status.
			log.Info("Updating batch status to 'Queued'")

			_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_QUEUED, api.BATCHSTATUS_QUEUED.String())
			if err != nil {
				return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_QUEUED.String(), err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("Process Ready Batches worker failed: %w", err)
	}

	return nil
}

// processQueuedBatches fetches all QUEUED batches which are in an active migration window,
// and sets them to RUNNING if they have the necessary files to begin a migration.
// All of ASSIGNED instances in the RUNNING batch are also set to CREATING, so that
// `initMigrationWorkers` can differentiate between the various states of instances in a RUNNING batch.
func (d *Daemon) processQueuedBatches(ctx context.Context) error {
	log := slog.With(slog.String("method", "processQueuedBatches"))
	reverter := revert.New()
	defer reverter.Fail()

	// Fetch all QUEUED batches, and their instances and targets.
	instancesByBatch := map[int]migration.Instances{}
	targetsByBatch := map[int]migration.Target{}
	batchesByID := map[int]migration.Batch{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_QUEUED)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state: %w", err)
		}

		// Skip any batches outside the migration window.
		for _, b := range batches {
			log := log.With(slog.String("batch", b.Name))

			log.Info("Batch status is 'Queued', processing....")
			if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
				log.Error("Batch window end time has already passed")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_ERROR.String(), err)
				}

				continue
			}

			// Get all instances for this batch.
			instances, err := d.instance.GetAllByBatchID(ctx, b.ID)
			if err != nil {
				return fmt.Errorf("Failed to get instances for batch: %w", err)
			}

			target, err := d.target.GetByID(ctx, b.TargetID)
			if err != nil {
				return err
			}

			availableInstances := migration.Instances{}
			for _, inst := range instances {
				if inst.MigrationStatus == api.MIGRATIONSTATUS_ASSIGNED_BATCH {
					availableInstances = append(availableInstances, inst)
				}
			}

			batchesByID[b.ID] = b
			instancesByBatch[b.ID] = availableInstances
			targetsByBatch[b.ID] = target
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Skip any batches that are lacking the necessary ISO images in the Incus storage pool.
	for batchID, instances := range instancesByBatch {
		t := targetsByBatch[batchID]
		b := batchesByID[batchID]
		err := d.ensureISOImagesExistInStoragePool(ctx, t, instances, b.TargetProject, b.StoragePool)
		if err != nil {
			// Skip the batch if it's missing needed resources.
			delete(batchesByID, batchID)
			log.Error("Failed to ensure ISO images exist in storage pool", logger.Err(err))
		}
	}

	// Set the statuses for any batches that made it this far to RUNNING in preparation for instance creation on the target.
	// `finalizeCompleteInstances` will pick up these batches, but won't find any instances in them until their associated VMs are created.
	sourcesByInstance := map[uuid.UUID]migration.Source{}
	err = transaction.Do(ctx, func(ctx context.Context) error {
		for _, b := range batchesByID {
			log.Info("Updating batch status to 'Running'")
			_, err := d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_RUNNING, api.BATCHSTATUS_RUNNING.String())
			if err != nil {
				return fmt.Errorf("Failed to update batch status: %w", err)
			}

			for _, inst := range instancesByBatch[b.ID] {
				sourcesByInstance[inst.UUID], err = d.source.GetByID(ctx, inst.SourceID)
				if err != nil {
					return err
				}

				_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_CREATING, api.MIGRATIONSTATUS_CREATING.String(), true)
				if err != nil {
					return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_CREATING.String(), err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("Process Queued Batches worker failed: %w", err)
	}

	return nil
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

// initMigrationWorkers creates the target VMs for all CREATING status instances in a RUNNING batch.
// Errors encountered in one batch do not affect the processing of other batches.
// - cleanupInstances determines whether to delete failed target VMs on errors.
func (d *Daemon) initMigrationWorkers(ctx context.Context, cleanupInstances bool) error {
	log := slog.With(
		slog.String("method", "initMigrationWorkers"),
	)

	var batchesByID map[int]migration.Batch
	var instancesByBatch map[int]migration.Instances
	targetsByBatch := map[int]migration.Target{}
	sourcesByInstance := map[uuid.UUID]migration.Source{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_RUNNING)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state %q: %w", api.BATCHSTATUS_RUNNING.String(), err)
		}

		allInstances, err := d.instance.GetAllByState(ctx, api.MIGRATIONSTATUS_CREATING)
		if err != nil {
			return fmt.Errorf("Failed to get instances by state %q: %w", api.MIGRATIONSTATUS_CREATING.String(), err)
		}

		batchesByID = make(map[int]migration.Batch, len(batches))
		for _, batch := range batches {
			batchesByID[batch.ID] = batch
		}

		// Collect only CREATING instances (and their target and source) for RUNNING batches.
		instancesByBatch = map[int]migration.Instances{}
		for _, inst := range allInstances {
			b, ok := batchesByID[*inst.BatchID]
			if !ok {
				continue
			}

			if instancesByBatch[*inst.BatchID] == nil {
				instancesByBatch[*inst.BatchID] = migration.Instances{}
			}

			instancesByBatch[*inst.BatchID] = append(instancesByBatch[*inst.BatchID], inst)

			targetsByBatch[*inst.BatchID], err = d.target.GetByID(ctx, b.TargetID)
			if err != nil {
				return fmt.Errorf("Failed to get target for instance %q: %w", inst.UUID, err)
			}

			sourcesByInstance[inst.UUID], err = d.source.GetByID(ctx, inst.SourceID)
			if err != nil {
				return fmt.Errorf("Failed to get source for instance %q: %w", inst.UUID, err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	for bID, instances := range instancesByBatch {
		err := d.createTargetVMs(ctx, batchesByID[bID], instances, targetsByBatch[bID], sourcesByInstance, cleanupInstances)
		if err != nil {
			log.Error("Failed to initialize migration workers", slog.String("target", targetsByBatch[bID].Name), slog.String("batch", batchesByID[bID].Name), logger.Err(err))
			// TODO: Set batch status to failed? Not sure what this would affect.
		}
	}

	return nil
}

// Concurrently create target VMs for each instance record.
// Any instance that fails the migration has its state set to ERROR.
// - cleanupInstances determines whether a target VM should be deleted if it encounters an error.
func (d *Daemon) createTargetVMs(ctx context.Context, b migration.Batch, instances migration.Instances, t migration.Target, sources map[uuid.UUID]migration.Source, cleanupInstances bool) error {
	log := slog.With(slog.String("method", "createTargetVMs"))
	err := util.RunConcurrentList(instances, func(inst migration.Instance) error {
		s := sources[inst.UUID]
		log := log.With(
			slog.String("instance", inst.InventoryPath),
			slog.String("source", s.Name),
			slog.String("target", t.Name),
		)

		// Try to set the instance state to ERRORED if it failed at any point.
		var err error
		reverter := revert.New()
		defer reverter.Fail()
		reverter.Add(func() {
			var errString string
			if err != nil {
				errString = err.Error()
			}

			_, err := d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, errString, true)
			if err != nil {
				log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR.String()), logger.Err(err))
			}
		})

		it, err := target.NewInternalIncusTargetFrom(api.Target{
			Name:       t.Name,
			DatabaseID: t.ID,
			TargetType: t.TargetType,
			Properties: t.Properties,
		})
		if err != nil {
			return fmt.Errorf("Failed to construct target %q: %w", t.Name, err)
		}

		// Connect to the target.
		err = it.Connect(ctx)
		if err != nil {
			return fmt.Errorf("Failed to connect to target %q: %w", it.GetName(), err)
		}

		// Set the project.
		err = it.SetProject(b.TargetProject)
		if err != nil {
			return fmt.Errorf("Failed to set project %q for target %q: %w", b.TargetProject, it.GetName(), err)
		}

		// Create the instance.
		var workerISOName string
		workerISOName, err = d.os.GetMigrationManagerISOName()
		if err != nil {
			return fmt.Errorf("Failed to get worker ISO path: %w", err)
		}

		var driverISOName string
		if inst.GetOSType() == api.OSTYPE_WINDOWS {
			driverISOName, err = d.os.GetVirtioDriversISOName()
			if err != nil {
				return fmt.Errorf("Failed to get driver ISO path: %w", err)
			}
		}

		// Optionally clean up the VMs if we fail to create them.
		instanceDef := it.CreateVMDefinition(inst, s.Name, b.StoragePool)
		if cleanupInstances {
			reverter.Add(func() {
				err := it.DeleteVM(instanceDef.Name)
				if err != nil {
					log.Error("Failed to delete new instance after failure", logger.Err(err))
				}
			})
		}

		err = it.CreateNewVM(instanceDef, b.StoragePool, workerISOName, driverISOName)
		if err != nil {
			return fmt.Errorf("Failed to create new instance %q on migration target %q: %w", instanceDef.Name, it.GetName(), err)
		}

		// Set the instance state to IDLE now that the VM is fully created.
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_IDLE, api.MIGRATIONSTATUS_IDLE.String(), true)
		if err != nil {
			return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_IDLE.String(), err)
		}

		// Start the instance.
		err = it.StartVM(inst.GetName())
		if err != nil {
			return fmt.Errorf("Failed to start instance %q on target %q: %w", instanceDef.Name, it.GetName(), err)
		}

		// Inject the worker binary.
		workerBinaryName := filepath.Join(d.os.VarDir, "migration-manager-worker")
		err = it.PushFile(inst.GetName(), workerBinaryName, "/root/")
		if err != nil {
			return fmt.Errorf("Failed to push %q to instance %q on target %q: %w", workerBinaryName, instanceDef.Name, it.GetName(), err)
		}

		// Start the worker binary.
		err = it.ExecWithoutWaiting(inst.GetName(), []string{"/root/migration-manager-worker", "-d", "--endpoint", d.getWorkerEndpoint(), "--uuid", inst.UUID.String(), "--token", inst.SecretToken.String()})
		if err != nil {
			return fmt.Errorf("Failed to execute worker on instance %q on target %q: %w", instanceDef.Name, it.GetName(), err)
		}

		reverter.Success()

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (d *Daemon) finalizeCompleteInstances(ctx context.Context) error {
	log := slog.With(slog.String("method", "finalizeCompleteInstances"))
	reverter := revert.New()
	defer reverter.Fail()

	var err error
	var instances migration.Instances
	var targetsByBatch map[int]*target.InternalIncusTarget
	var batchesByInstance map[uuid.UUID]migration.Batch
	var networksByName map[string]migration.Network
	reverter.Add(func() {
		var errString string
		if err != nil {
			errString = err.Error()
		}

		err = transaction.Do(ctx, func(ctx context.Context) error {
			for _, i := range instances {
				_, err := d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, errString, true)
				if err != nil {
					log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR.String()), logger.Err(err))
				}
			}

			return nil
		})
	})

	err = transaction.Do(ctx, func(ctx context.Context) error {
		// Get any instances in the "complete" state.
		var err error
		instances, err = d.instance.GetAllByState(ctx, api.MIGRATIONSTATUS_IMPORT_COMPLETE)
		if err != nil {
			return fmt.Errorf("Failed to get instances by state: %w", err)
		}

		networks, err := d.network.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all networks: %w", err)
		}

		targetsByBatch = make(map[int]*target.InternalIncusTarget, len(instances))
		batchesByInstance = make(map[uuid.UUID]migration.Batch, len(instances))
		networksByName = make(map[string]migration.Network, len(networks))

		for _, net := range networks {
			networksByName[net.Name] = net
		}

		batches, err := d.batch.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all batches: %w", err)
		}

		batchesByID := make(map[int]migration.Batch, len(batches))
		for _, b := range batches {
			batchesByID[b.ID] = b

			t, err := d.target.GetByID(ctx, b.TargetID)
			if err != nil {
				return fmt.Errorf("Failed to get all targets: %w", err)
			}

			targetsByBatch[b.ID], err = target.NewInternalIncusTargetFrom(api.Target{
				Name:       t.Name,
				DatabaseID: t.ID,
				TargetType: t.TargetType,
				Properties: t.Properties,
			})
			if err != nil {
				return err
			}
		}

		for _, i := range instances {
			batch, ok := batchesByID[*i.BatchID]
			if !ok {
				return fmt.Errorf("No batch found for instance %q: %w", i.UUID, err)
			}

			batchesByInstance[i.UUID] = batch
		}

		return nil
	})
	if err != nil {
		return err
	}

	instanceErrs := make([]error, 0, len(instances))
	for _, i := range instances {
		log := log.With(slog.String("instance", i.InventoryPath))
		batch := batchesByInstance[i.UUID]
		it := targetsByBatch[*i.BatchID]
		err = configureMigratedInstance(ctx, i, it, batch, networksByName)
		if err != nil {
			instanceErrs = append(instanceErrs, err)
			log.Error("Failed to finalize instance", logger.Err(err))
		}
	}

	if len(instanceErrs) > 0 {
		return fmt.Errorf("Failed to finalize %d instances. Last error: %w", len(instanceErrs), instanceErrs[len(instanceErrs)-1])
	}

	err = transaction.Do(ctx, func(ctx context.Context) error {
		for _, i := range instances {
			// Update the instance status.
			_, err := d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_FINISHED, api.MIGRATIONSTATUS_FINISHED.String(), true)
			if err != nil {
				return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_FINISHED.String(), err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	reverter.Success()

	return nil
}

// configureMigratedInstance updates the configuration of an instance after it has finished migrating.
func configureMigratedInstance(ctx context.Context, i migration.Instance, it *target.InternalIncusTarget, batch migration.Batch, allNetworks map[string]migration.Network) error {
	// Connect to the target.
	err := it.Connect(ctx)
	if err != nil {
		return fmt.Errorf("Failed to connect to target %q: %w", it.GetName(), err)
	}

	// Set the project.
	err = it.SetProject(batch.TargetProject)
	if err != nil {
		return fmt.Errorf("Failed to set target %q project %q: %w", it.GetName(), batch.TargetProject, err)
	}

	// Stop the instance.
	err = it.StopVM(i.GetName(), true)
	if err != nil {
		return fmt.Errorf("Failed to stop instance %q on target %q: %w", i.GetName(), it.GetName(), err)
	}

	// Get the instance definition.
	apiDef, _, err := it.GetInstance(i.GetName())
	if err != nil {
		return fmt.Errorf("Failed to get configuration for instance %q on target %q: %w", i.GetName(), it.GetName(), err)
	}

	for idx, nic := range i.NICs {
		nicDeviceName := fmt.Sprintf("eth%d", idx)
		baseNetwork, ok := allNetworks[nic.Network]
		if !ok {
			err = fmt.Errorf("No network %q associated with instance %q on target %q", nic.Network, i.GetName(), it.GetName())
			return err
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
	op, err := it.UpdateInstance(i.GetName(), apiDef.Writable(), "")
	if err != nil {
		return fmt.Errorf("Failed to update instance %q on target %q: %w", i.GetName(), it.GetName(), err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Failed to wait for update to instance %q on target %q: %w", i.GetName(), it.GetName(), err)
	}

	err = it.StartVM(i.GetName())
	if err != nil {
		return fmt.Errorf("Failed to start instance %q on target %q: %w", i.GetName(), it.GetName(), err)
	}

	return nil
}
