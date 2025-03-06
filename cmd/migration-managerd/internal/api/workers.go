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

func (d *Daemon) runPeriodicTask(ctx context.Context, task string, f func(context.Context) error, interval time.Duration) {
	go func() {
		for {
			err := f(ctx)
			if err != nil {
				slog.Error("Failed to run periodic task", slog.String("task", task), logger.Err(err))
			}

			t := time.NewTimer(interval)

			select {
			case <-ctx.Done():
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

		for srcName, srcInstances := range instancesBySrc {
			// Ensure we only compare instances in the same source.
			existingInstances := make(map[uuid.UUID]migration.Instance, len(dbInstances))
			for _, inst := range dbInstances {
				src := sourcesByName[srcName]

				if src.ID == inst.SourceID {
					existingInstances[inst.UUID] = inst
				}
			}

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

	// TODO: Do more than pick up new networks, also delete removed networks and update existing networks with any changes.
	// Currently, the entire network list is given in existingNetworks for each source, so we will need to be smarter about filtering that as well.

	// Create any missing networks.
	for name, network := range srcNetworks {
		_, ok := existingNetworks[name]
		if !ok {
			log := log.With(slog.String("network", network.Name))
			log.Info("Recording new network detected on source")
			_, err := n.Create(ctx, migration.Network{Name: network.Name, Config: network.Config})
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

	for _, network := range networks {
		networkMap[network.Name] = network
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

		// Sets the batch to errored status with the given message, and logs the message as well.
		setBatchErr := func(msg string, batchID int, log *slog.Logger) error {
			log.Error("Failed to start batch", slog.String("message", msg), slog.Int("batch_id", batchID))
			_, err := d.batch.UpdateStatusByID(ctx, batchID, api.BATCHSTATUS_ERROR, msg)
			if err != nil {
				return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_ERROR.String(), err)
			}

			return nil
		}

		// Do some basic sanity check of each batch before adding it to the queue.
		for _, b := range batches {
			log := log.With(slog.String("batch", b.Name))

			log.Info("Batch status is 'Ready', processing....")

			// If a migration window is defined, ensure sure it makes sense.
			if !b.MigrationWindowStart.IsZero() && !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(b.MigrationWindowStart) {
				err := setBatchErr("Batch migration window end time is before its start time", b.ID, log)
				if err != nil {
					return err
				}

				continue
			}

			if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
				err := setBatchErr("Batch migration window end time has already passed", b.ID, log)
				if err != nil {
					return err
				}

				continue
			}

			// Get all instances and targets for this batch. Fail the transaction on errors as it could mean a database issue.
			instances, err := d.instance.GetAllByBatchID(ctx, b.ID)
			if err != nil {
				return fmt.Errorf("Failed to get instances for batch %q: %w", b.Name, err)
			}

			// If no instances apply to this batch, return an error.
			if len(instances) == 0 {
				err := setBatchErr("Batch has no instances assigned", b.ID, log)
				if err != nil {
					return err
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
// `beginImports` can differentiate between the various states of instances in a RUNNING batch.
func (d *Daemon) processQueuedBatches(ctx context.Context) error {
	log := slog.With(slog.String("method", "processQueuedBatches"))
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
			log := log.With(slog.String("batch", b.Name))
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

	reverter := revert.New()
	defer reverter.Fail()

	// Key the batch by its constituent parts, as batches with different IDs may share the same target, pool, and project.
	batchKey := t.Name + "_" + storagePool + "_" + project
	d.batchLock.Lock(batchKey)
	reverter.Add(func() { d.batchLock.Unlock(batchKey) })

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

	d.batchLock.Unlock(batchKey)
	reverter.Success()

	return nil
}

// beginImports creates the target VMs for all CREATING status instances in a RUNNING batch.
// Errors encountered in one batch do not affect the processing of other batches.
//   - cleanupInstances determines whether to delete failed target VMs on errors.
//     If true, errors will not result in the instance state being set to ERROR, to enable retrying this task.
//     If any errors occur after the VM has started, the VM will no longer be cleaned up, and its state will be set to ERROR, preventing retries.
func (d *Daemon) beginImports(ctx context.Context, cleanupInstances bool) error {
	log := slog.With(
		slog.String("method", "beginImports"),
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
		}
	}

	return nil
}

// Concurrently create target VMs for each instance record.
// Any instance that fails the migration has its state set to ERROR.
// - cleanupInstances determines whether a target VM should be deleted if it encounters an error.
func (d *Daemon) createTargetVMs(ctx context.Context, b migration.Batch, instances migration.Instances, t migration.Target, sources map[uuid.UUID]migration.Source, cleanupInstances bool) error {
	log := slog.With(
		slog.String("method", "createTargetVMs"),
		slog.String("target", t.Name),
		slog.String("batch", b.Name),
	)

	err := util.RunConcurrentList(instances, func(inst migration.Instance) (_err error) {
		s := sources[inst.UUID]
		log := log.With(
			slog.String("instance", inst.InventoryPath),
			slog.String("source", s.Name),
		)

		reverter := revert.New()
		defer reverter.Fail()
		reverter.Add(func() {
			log := log.With(slog.String("revert", "set instance failed"))
			var errString string
			if _err != nil {
				errString = _err.Error()
			}

			// If cleanupInstances is true, then we can try to create the VMs again so don't set the instance state to errored.
			if cleanupInstances {
				log.Error("Failed attempt to create target instance. Trying again soon")
				return
			}

			// Try to set the instance state to ERRORED if it failed.
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
		workerISOName, err := d.os.GetMigrationManagerISOName()
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
				log := log.With(slog.String("revert", "instance cleanup"))
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

		// Set the instance state to IDLE before triggering the worker.
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_IDLE, api.MIGRATIONSTATUS_IDLE.String(), true)
		if err != nil {
			return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_IDLE.String(), err)
		}

		// At this point, the import is about to begin, so we won't try to delete instances anymore.
		// Instead, if an error occurs, we will try to set the instance state to ERROR so that we don't retry.
		cleanupInstances = false

		// Start the worker binary.
		// TODO: Periodically check that the worker is actually running.
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

// finalizeCompleteInstances fetches all instances in RUNNING batches whose status is IMPORT COMPLETE, and for each batch, runs configureMigratedInstances.
func (d *Daemon) finalizeCompleteInstances(ctx context.Context) (_err error) {
	log := slog.With(slog.String("method", "finalizeCompleteInstances"))
	batchesByID := map[int]migration.Batch{}
	networksByName := map[string]migration.Network{}
	completeInstancesByBatch := map[int]migration.Instances{}
	targetsByBatch := map[int]migration.Target{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_RUNNING)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state %q: %w", api.BATCHSTATUS_RUNNING.String(), err)
		}

		for _, b := range batches {
			batchesByID[b.ID] = b
		}

		// Get any instances in the "complete" state.
		instances, err := d.instance.GetAllByState(ctx, api.MIGRATIONSTATUS_IMPORT_COMPLETE)
		if err != nil {
			return fmt.Errorf("Failed to get instances by state %q: %w", api.MIGRATIONSTATUS_IMPORT_COMPLETE.String(), err)
		}

		for _, i := range instances {
			if completeInstancesByBatch[*i.BatchID] == nil {
				completeInstancesByBatch[*i.BatchID] = migration.Instances{}
			}

			completeInstancesByBatch[*i.BatchID] = append(completeInstancesByBatch[*i.BatchID], i)
		}

		for batchID := range completeInstancesByBatch {
			var err error
			b := batchesByID[batchID]
			targetsByBatch[b.ID], err = d.target.GetByID(ctx, b.TargetID)
			if err != nil {
				return fmt.Errorf("Failed to get all targets: %w", err)
			}
		}

		networks, err := d.network.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all networks: %w", err)
		}

		for _, net := range networks {
			networksByName[net.Name] = net
		}

		return nil
	})
	if err != nil {
		return err
	}

	for batchID, instances := range completeInstancesByBatch {
		err := d.configureMigratedInstances(ctx, instances, targetsByBatch[batchID], batchesByID[batchID], networksByName)
		if err != nil {
			log.Error("Failed to configureMigratedInstances", slog.String("batch", batchesByID[batchID].Name))
		}
	}

	return nil
}

// configureMigratedInstances updates the configuration of instances concurrently after they have finished migrating. Errors will result in the instance state becoming ERRORED.
// If an instance succeeds, its state will be moved to FINISHED.
func (d *Daemon) configureMigratedInstances(ctx context.Context, instances migration.Instances, t migration.Target, batch migration.Batch, allNetworks map[string]migration.Network) error {
	log := slog.With(
		slog.String("method", "createTargetVMs"),
		slog.String("target", t.Name),
		slog.String("batch", batch.Name),
	)

	return util.RunConcurrentList(instances, func(i migration.Instance) (_err error) {
		log := log.With(slog.String("instance", i.InventoryPath))

		reverter := revert.New()
		defer reverter.Fail()
		reverter.Add(func() {
			log := log.With(slog.String("revert", "set instance failed"))
			var errString string
			if _err != nil {
				errString = _err.Error()
			}

			// Try to set the instance state to ERRORED if it failed.
			_, err := d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, errString, true)
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

		// Update the instance status.
		_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_FINISHED, api.MIGRATIONSTATUS_FINISHED.String(), true)
		if err != nil {
			return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_FINISHED.String(), err)
		}

		reverter.Success()

		return nil
	})
}
