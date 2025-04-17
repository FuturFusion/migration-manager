package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/revert"
	incusTLS "github.com/lxc/incus/v6/shared/tls"

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

// validateForQueue validates that a set of instances in a batch are capable of being queued for the given target.
// - The batch must be DEFINED or QUEUED.
// - All instances must be ASSIGNED to the batch.
// - All instances must be defined on the source.
// - The batch must be within a valid migration window.
// - Ensures the target and project are reachable.
// - Ensures there are no conflicting instances on the target.
// - Ensures the correct ISO images exist in the target storage pool.
func (d *Daemon) validateForQueue(ctx context.Context, b migration.Batch, t migration.Target, instances migration.Instances) (*target.InternalIncusTarget, error) {
	if b.Status != api.BATCHSTATUS_QUEUED && b.Status != api.BATCHSTATUS_DEFINED {
		return nil, fmt.Errorf("Batch status is %q, not %q or %q", b.Status, api.BATCHSTATUS_QUEUED, api.BATCHSTATUS_DEFINED)
	}

	// If a migration window is defined, ensure sure it makes sense.
	if !b.MigrationWindowStart.IsZero() && !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(b.MigrationWindowStart) {
		return nil, fmt.Errorf("Batch %q window end time is before start time", b.Name)
	}

	if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("Batch %q migration window has already passed", b.Name)
	}

	// If no instances apply to this batch, return nil, an error.
	if len(instances) == 0 {
		return nil, fmt.Errorf("Batch %q has no instances assigned", b.Name)
	}

	it, err := target.NewInternalIncusTargetFrom(t.ToAPI())
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to target for batch %q: %w", b.Name, err)
	}

	// Connect to the target.
	err = it.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to target for batch %q: %w", b.Name, err)
	}

	// Set the project.
	err = it.SetProject(b.TargetProject)
	if err != nil {
		return nil, fmt.Errorf("Failed to set project %q for target of batch %q: %w", b.TargetProject, b.Name, err)
	}

	targetInstances, err := it.GetInstanceNames()
	if err != nil {
		return nil, fmt.Errorf("Failed to get instancs in project %q of target %q for batch %q: %w", b.TargetProject, it.GetName(), b.Name, err)
	}

	targetInstanceMap := make(map[string]bool, len(targetInstances))
	for _, inst := range targetInstances {
		targetInstanceMap[inst] = true
	}

	for _, inst := range instances {
		if b.Name != *inst.Batch {
			return nil, fmt.Errorf("Instance %q is not in batch %q", inst.GetName(), b.Name)
		}

		if inst.MigrationStatus != api.MIGRATIONSTATUS_ASSIGNED_BATCH {
			return nil, fmt.Errorf("Instance %q in batch %q has status %q, expected %q", inst.GetName(), b.Name, inst.MigrationStatus, api.MIGRATIONSTATUS_ASSIGNED_BATCH)
		}

		if targetInstanceMap[inst.GetName()] {
			return nil, fmt.Errorf("Another instance with name %q already exists", inst.GetName())
		}
	}

	// Ensure VMware VIX tarball exists.
	_, err = d.os.GetVMwareVixName()
	if err != nil {
		return nil, fmt.Errorf("Failed to find VMWare vix tarball: %w", err)
	}

	// Ensure exactly zero or one VirtIO drivers ISOs exist.
	_, err = d.os.GetVirtioDriversISOName()
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Failed to find Virtio drivers ISO: %w", err)
	}

	return it, nil
}

// processQueuedBatches fetches all QUEUED batches which are in an active migration window,
// and sets them to RUNNING if they have the necessary files to begin a migration.
// All of ASSIGNED instances in the RUNNING batch are also set to CREATING, so that
// `beginImports` can differentiate between the various states of instances in a RUNNING batch.
func (d *Daemon) processQueuedBatches(ctx context.Context) error {
	log := slog.With(slog.String("method", "processQueuedBatches"))
	// Fetch all QUEUED batches, and their instances and targets.
	instancesByBatch := map[string]migration.Instances{}
	targetsByBatch := map[string]migration.Target{}
	batchesByName := map[string]migration.Batch{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_QUEUED)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state: %w", err)
		}

		for _, b := range batches {
			// Get the target and all instances for this batch.
			instances, err := d.instance.GetAllByBatchAndState(ctx, b.Name, api.MIGRATIONSTATUS_ASSIGNED_BATCH, false)
			if err != nil {
				return fmt.Errorf("Failed to get instances for batch %q: %w", b.Name, err)
			}

			t, err := d.target.GetByName(ctx, b.Target)
			if err != nil {
				return fmt.Errorf("Failed to get target for batch %q: %w", b.Name, err)
			}

			instancesByBatch[b.Name] = instances
			targetsByBatch[b.Name] = *t
			batchesByName[b.Name] = b
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Validate the batches before updating their state.
	// If a batch is invalid or unable to be validated, it will be set to ERROR and removed from consideration.
	for batchName, availableInstances := range instancesByBatch {
		t := targetsByBatch[batchName]
		b := batchesByName[batchName]
		log := log.With(slog.String("batch", b.Name))
		it, err := d.validateForQueue(ctx, b, t, availableInstances)
		if err == nil {
			err = d.ensureISOImagesExistInStoragePool(ctx, it, availableInstances, b)
		}

		if err != nil {
			log.Error("Failed to validate batch", logger.Err(err))
			_, err := d.batch.UpdateStatusByName(ctx, b.Name, api.BATCHSTATUS_ERROR, err.Error())
			if err != nil {
				return fmt.Errorf("Failed to set batch status to %q: %w", api.BATCHSTATUS_ERROR, err)
			}

			delete(batchesByName, batchName)
		}
	}

	// Set the statuses for any batches that made it this far to RUNNING in preparation for instance creation on the target.
	// `finalizeCompleteInstances` will pick up these batches, but won't find any instances in them until their associated VMs are created.
	err = transaction.Do(ctx, func(ctx context.Context) error {
		for _, b := range batchesByName {
			log.Info("Updating batch status to 'Running'")
			_, err := d.batch.UpdateStatusByName(ctx, b.Name, api.BATCHSTATUS_RUNNING, string(api.BATCHSTATUS_RUNNING))
			if err != nil {
				return fmt.Errorf("Failed to update batch status: %w", err)
			}

			for _, inst := range instancesByBatch[b.Name] {
				_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_CREATING, string(api.MIGRATIONSTATUS_CREATING), true, false)
				if err != nil {
					return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_CREATING, err)
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

// ensureISOImagesExistInStoragePool ensures the necessary image files exist on the daemon to be imported to the storage volume.
func (d *Daemon) ensureISOImagesExistInStoragePool(ctx context.Context, it *target.InternalIncusTarget, instances migration.Instances, batch migration.Batch) error {
	if len(instances) == 0 {
		return fmt.Errorf("No instances in batch")
	}

	log := slog.With(
		slog.String("method", "ensureISOImagesExistInStoragePool"),
		slog.String("storage_pool", batch.StoragePool),
	)

	reverter := revert.New()
	defer reverter.Fail()

	// Key the batch by its constituent parts, as batches with different IDs may share the same target, pool, and project.
	batchKey := it.GetName() + "_" + batch.StoragePool + "_" + batch.TargetProject
	d.batchLock.Lock(batchKey)
	reverter.Add(func() { d.batchLock.Unlock(batchKey) })

	// Connect to the target.
	volumes, err := it.GetStoragePoolVolumeNames(batch.StoragePool)
	if err != nil {
		return err
	}

	volumeMap := make(map[string]bool, len(volumes))
	for _, vol := range volumes {
		volumeMap[vol] = true
	}

	// Verify needed ISO image is in the storage pool.
	var needsDriverISO bool
	for _, inst := range instances {
		if inst.GetOSType() == api.OSTYPE_WINDOWS {
			needsDriverISO = true
			break
		}
	}

	existingDriverName, err := d.os.GetVirtioDriversISOName()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to find Virtio drivers ISO: %w", err)
	}

	missingBaseImg := !volumeMap["custom/"+util.WorkerVolume()]
	missingDriverISO := needsDriverISO && (existingDriverName == "" || !volumeMap["custom/"+existingDriverName])

	// If we need to download missing files, or upload them to the target, set a status message.
	if missingBaseImg || missingDriverISO {
		_, err := d.batch.UpdateStatusByName(ctx, batch.Name, batch.Status, "Downloading artifacts")
		if err != nil {
			return fmt.Errorf("Failed to update batch %q status message: %w", batch.Name, err)
		}
	}

	if missingBaseImg {
		log.Info("Worker image doesn't exist in storage pool, importing...")
		err = d.os.LoadWorkerImage(ctx)
		if err != nil {
			return err
		}

		ops, err := it.CreateStoragePoolVolumeFromBackup(batch.StoragePool, filepath.Join(d.os.CacheDir, util.RawWorkerImage()))
		if err != nil {
			return err
		}

		for _, op := range ops {
			err = op.Wait()
			if err != nil {
				return err
			}
		}
	}

	if missingDriverISO {
		driversISOPath, err := d.os.LoadVirtioWinISO()
		if err != nil {
			return err
		}

		driversISO := filepath.Base(driversISOPath)
		if !volumeMap["custom/"+driversISO] {
			log.Info("ISO image doesn't exist in storage pool, importing...")

			op, err := it.CreateStoragePoolVolumeFromISO(batch.StoragePool, driversISOPath)
			if err != nil {
				return err
			}

			err = op.Wait()
			if err != nil {
				return err
			}
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

	var batchesByName map[string]migration.Batch
	var instancesByBatch map[string]migration.Instances
	targetsByBatch := map[string]migration.Target{}
	sourcesByInstance := map[uuid.UUID]migration.Source{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_RUNNING)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state %q: %w", api.BATCHSTATUS_RUNNING, err)
		}

		allInstances, err := d.instance.GetAllByState(ctx, false, api.MIGRATIONSTATUS_CREATING)
		if err != nil {
			return fmt.Errorf("Failed to get instances by state %q: %w", api.MIGRATIONSTATUS_CREATING, err)
		}

		batchesByName = make(map[string]migration.Batch, len(batches))
		for _, batch := range batches {
			batchesByName[batch.Name] = batch
		}

		// Collect only CREATING instances (and their target and source) for RUNNING batches.
		instancesByBatch = map[string]migration.Instances{}
		for _, inst := range allInstances {
			b, ok := batchesByName[*inst.Batch]
			if !ok {
				continue
			}

			if instancesByBatch[*inst.Batch] == nil {
				instancesByBatch[*inst.Batch] = migration.Instances{}
			}

			instancesByBatch[*inst.Batch] = append(instancesByBatch[*inst.Batch], inst)

			target, err := d.target.GetByName(ctx, b.Target)
			if err != nil {
				return fmt.Errorf("Failed to get target for instance %q: %w", inst.UUID, err)
			}

			targetsByBatch[*inst.Batch] = *target
			source, err := d.source.GetByName(ctx, inst.Source)
			if err != nil {
				return fmt.Errorf("Failed to get source for instance %q: %w", inst.UUID, err)
			}

			sourcesByInstance[inst.UUID] = *source
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = util.RunConcurrentMap(instancesByBatch, func(batchName string, instances migration.Instances) error {
		return d.createTargetVMs(ctx, batchesByName[batchName], instances, targetsByBatch[batchName], sourcesByInstance, cleanupInstances)
	})
	if err != nil {
		log.Error("Failed to initialize migration workers", logger.Err(err))
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
			slog.String("instance", inst.Properties.Location),
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
			_, err := d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, errString, true, false)
			if err != nil {
				log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
			}
		})

		it, err := target.NewInternalIncusTargetFrom(t.ToAPI())
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

		var driverISOName string
		if inst.GetOSType() == api.OSTYPE_WINDOWS {
			driverISOName, err = d.os.GetVirtioDriversISOName()
			if err != nil {
				return fmt.Errorf("Failed to get driver ISO path: %w", err)
			}
		}

		cert, err := d.ServerCert().PublicKeyX509()
		if err != nil {
			return fmt.Errorf("Failed to parse server certificate: %w", err)
		}

		// Optionally clean up the VMs if we fail to create them.
		instanceDef, err := it.CreateVMDefinition(inst, s.Name, b.StoragePool, incusTLS.CertFingerprint(cert), d.getWorkerEndpoint())
		if err != nil {
			return fmt.Errorf("Failed to create instance definition: %w", err)
		}

		if cleanupInstances {
			reverter.Add(func() {
				log := log.With(slog.String("revert", "instance cleanup"))
				err := it.DeleteVM(instanceDef.Name)
				if err != nil {
					log.Error("Failed to delete new instance after failure", logger.Err(err))
				}
			})
		}

		err = it.CreateNewVM(instanceDef, b.StoragePool, util.WorkerVolume(), driverISOName)
		if err != nil {
			return fmt.Errorf("Failed to create new instance %q on migration target %q: %w", instanceDef.Name, it.GetName(), err)
		}

		// Start the instance.
		err = it.StartVM(inst.GetName())
		if err != nil {
			return fmt.Errorf("Failed to start instance %q on target %q: %w", instanceDef.Name, it.GetName(), err)
		}

		// Wait up to 90s for the Incus agent.
		waitCtx, cancel := context.WithTimeout(ctx, time.Second*90)
		defer cancel()
		err = it.CheckIncusAgent(waitCtx, instanceDef.Name)
		if err != nil {
			return err
		}

		// Set the instance state to IDLE before triggering the worker.
		// Consider this a worker update as it may take some time for the worker to actually start.
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_IDLE, string(api.MIGRATIONSTATUS_IDLE), true, true)
		if err != nil {
			return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_IDLE, err)
		}

		// At this point, the import is about to begin, so we won't try to delete instances anymore.
		// Instead, if an error occurs, we will try to set the instance state to ERROR so that we don't retry.
		cleanupInstances = false

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
	batchesByName := map[string]migration.Batch{}
	networksByName := map[string]migration.Network{}
	completeInstancesByBatch := map[string]migration.Instances{}
	targetsByBatch := map[string]*migration.Target{}
	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_RUNNING)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state %q: %w", api.BATCHSTATUS_RUNNING, err)
		}

		for _, b := range batches {
			batchesByName[b.Name] = b
		}

		// Get any instances with a status that indicates they have a running worker.
		instances, err := d.instance.GetAllByState(ctx, true,
			api.MIGRATIONSTATUS_IDLE,
			api.MIGRATIONSTATUS_BACKGROUND_IMPORT,
			api.MIGRATIONSTATUS_FINAL_IMPORT,
			api.MIGRATIONSTATUS_IMPORT_COMPLETE)
		if err != nil {
			return fmt.Errorf("Failed to get instances by state %q: %w", api.MIGRATIONSTATUS_IMPORT_COMPLETE, err)
		}

		for _, i := range instances {
			if i.LastUpdateFromWorker.Add(30 * time.Second).Before(time.Now().UTC()) {
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, "Timed out waiting for worker", false, false)
				if err != nil {
					return fmt.Errorf("Failed to set errored state on instance %q: %w", i.Properties.Location, err)
				}

				continue
			}

			// Only consider IMPORT COMPLETE instances moving forward.
			if i.MigrationStatus != api.MIGRATIONSTATUS_IMPORT_COMPLETE {
				continue
			}

			if completeInstancesByBatch[*i.Batch] == nil {
				completeInstancesByBatch[*i.Batch] = migration.Instances{}
			}

			completeInstancesByBatch[*i.Batch] = append(completeInstancesByBatch[*i.Batch], i)
		}

		for batchName := range completeInstancesByBatch {
			var err error
			b := batchesByName[batchName]
			targetsByBatch[b.Name], err = d.target.GetByName(ctx, b.Target)
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

	for batchName, instances := range completeInstancesByBatch {
		err := d.configureMigratedInstances(ctx, instances, *targetsByBatch[batchName], batchesByName[batchName], networksByName)
		if err != nil {
			log.Error("Failed to configureMigratedInstances", slog.String("batch", batchesByName[batchName].Name))
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
		log := log.With(slog.String("instance", i.Properties.Location))

		reverter := revert.New()
		defer reverter.Fail()
		reverter.Add(func() {
			log := log.With(slog.String("revert", "set instance failed"))
			var errString string
			if _err != nil {
				errString = _err.Error()
			}

			// Try to set the instance state to ERRORED if it failed.
			_, err := d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, errString, true, false)
			if err != nil {
				log.Error("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
			}
		})

		it, err := target.NewInternalIncusTargetFrom(t.ToAPI())
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

		err = it.SetPostMigrationVMConfig(i, allNetworks)
		if err != nil {
			return fmt.Errorf("Failed to update post-migration config for instance %q in %q: %w", i.GetName(), it.GetName(), err)
		}

		// Update the instance status.
		_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_FINISHED, string(api.MIGRATIONSTATUS_FINISHED), true, false)
		if err != nil {
			return fmt.Errorf("Failed to update instance status to %q: %w", api.MIGRATIONSTATUS_FINISHED, err)
		}

		reverter.Success()

		return nil
	})
}
