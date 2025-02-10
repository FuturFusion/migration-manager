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
	for uuid, inst := range existingInstances {
		log := log.With(
			slog.String("instance", inst.InventoryPath),
			slog.Any("instance_uuid", inst.UUID),
		)

		srcInst, ok := srcInstances[uuid]
		if !ok {
			// Delete the instances that don't exist on the source.
			log.Info("Deleting instance with no source record")
			err := i.DeleteByID(ctx, uuid)
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
	for uuid, inst := range srcInstances {
		_, ok := existingInstances[uuid]
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

func (d *Daemon) processReadyBatches() bool {
	log := slog.With(slog.String("method", "processReadyBatches"))

	// TODO: context should be passed from the daemon to all the workers.
	ctx := context.TODO()

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
					log.Warn("Failed to update batch status", logger.Err(err))
					continue
				}

				continue
			}

			if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
				log.Error("Batch window end time has already passed")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					log.Warn("Failed to update batch status", logger.Err(err))
					continue
				}

				continue
			}

			// Get all instances for this batch.
			instances, err := d.instance.GetAllByBatchID(ctx, b.ID)
			if err != nil {
				log.Warn("Failed to get instances for batch", logger.Err(err))
				continue
			}

			// If no instances apply to this batch, return an error.
			if len(instances) == 0 {
				log.Error("Batch has no instances")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "No instances assigned")
				if err != nil {
					log.Warn("Failed to update batch status", logger.Err(err))
					continue
				}

				continue
			}

			// No issues detected, move to "queued" status.
			log.Info("Updating batch status to 'Queued'")

			_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_QUEUED, api.BATCHSTATUS_QUEUED.String())
			if err != nil {
				log.Warn("Failed to update batch status", logger.Err(err))
				continue
			}
		}

		return nil
	})
	if err != nil {
		log.Warn("Process Ready Batches worker failed", logger.Err(err))
	}

	return false
}

func (d *Daemon) processQueuedBatches() bool {
	log := slog.With(slog.String("method", "processQueuedBatches"))

	// TODO: context should be passed from the daemon to all the workers.
	ctx := context.TODO()

	err := transaction.Do(ctx, func(ctx context.Context) error {
		batches, err := d.batch.GetAllByState(ctx, api.BATCHSTATUS_QUEUED)
		if err != nil {
			return fmt.Errorf("Failed to get batches by state: %w", err)
		}

		// See if we can start running this batch.
		for _, b := range batches {
			log := log.With(slog.String("batch", b.Name))

			log.Info("Batch status is 'Queued', processing....")

			if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
				log.Error("Batch window end time has already passed")

				_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_ERROR, "Migration window end has already passed")
				if err != nil {
					log.Warn("Failed to update batch status", logger.Err(err))
					continue
				}

				continue
			}

			// Get all instances for this batch.
			instances, err := d.instance.GetAllByBatchID(ctx, b.ID)
			if err != nil {
				log.Warn("Failed to get instances for batch", logger.Err(err))
				continue
			}

			target, err := d.target.GetByID(ctx, b.TargetID)
			if err != nil {
				log.Warn("Failed to get target for batch", logger.Err(err))
				continue
			}

			// Make sure the necessary ISO images exist in the Incus storage pool.
			err = d.ensureISOImagesExistInStoragePool(ctx, target, instances, b.TargetProject, b.StoragePool)
			if err != nil {
				log.Warn("Failed to ensure ISO images exist in storage pool", logger.Err(err))
				continue
			}

			// Instantiate each new empty VM in Incus.
			for _, inst := range instances {
				// Create fresh context, since operation is happening in its own go routine.
				ctx := context.Background()
				go d.spinUpMigrationEnv(ctx, inst, b)
			}

			// Move batch to "running" status.
			log.Info("Updating batch status to 'Running'")

			_, err = d.batch.UpdateStatusByID(ctx, b.ID, api.BATCHSTATUS_RUNNING, api.BATCHSTATUS_RUNNING.String())
			if err != nil {
				log.Warn("Failed to update batch status", logger.Err(err))
				continue
			}
		}

		return nil
	})
	if err != nil {
		log.Warn("Process Queued Batches worker failed", logger.Err(err))
	}

	return false
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

func (d *Daemon) spinUpMigrationEnv(ctx context.Context, inst migration.Instance, b migration.Batch) {
	log := slog.With(
		slog.String("method", "spinUpMigrationEnv"),
		slog.String("instance", inst.InventoryPath),
	)

	// Update the instance status.
	_, err := d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_CREATING, api.MIGRATIONSTATUS_CREATING.String(), true)
	if err != nil {
		log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		return
	}

	s, err := d.source.GetByID(ctx, inst.SourceID)
	if err != nil {
		log.Warn("Failed to get source by ID", logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Get the target for this instance.
	t, err := d.target.GetByID(ctx, b.TargetID)
	if err != nil {
		log.Warn("Failed to get target by ID", slog.Int("target_id", b.TargetID), logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	it, err := target.NewInternalIncusTargetFrom(api.Target{
		Name:       t.Name,
		DatabaseID: t.ID,
		TargetType: t.TargetType,
		Properties: t.Properties,
	})
	if err != nil {
		log.Warn("Failed to construct target", slog.String("target", it.GetName()), logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Connect to the target.
	err = it.Connect(d.ShutdownCtx)
	if err != nil {
		log.Warn("Failed to connect to target", slog.String("target", it.GetName()), logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Set the project.
	err = it.SetProject(b.TargetProject)
	if err != nil {
		log.Warn("Failed to set target project", slog.String("project", b.TargetProject), logger.Err(err))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Create the instance.
	workerISOName, _ := d.os.GetMigrationManagerISOName()
	var driverISOName string
	if inst.GetOSType() == api.OSTYPE_WINDOWS {
		driverISOName, _ = d.os.GetVirtioDriversISOName()
	}

	instanceDef := it.CreateVMDefinition(inst, s.Name, b.StoragePool)
	creationErr := it.CreateNewVM(instanceDef, b.StoragePool, workerISOName, driverISOName)
	if creationErr != nil {
		log.Warn("Failed to create new VM", slog.String("instance", instanceDef.Name), logger.Err(creationErr))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, creationErr.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Creation was successful, update the instance state to 'Idle'.
	_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_IDLE, api.MIGRATIONSTATUS_IDLE.String(), true)
	if err != nil {
		log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		return
	}

	// Start the instance.
	startErr := it.StartVM(inst.GetName())
	if startErr != nil {
		log.Warn("Failed to start VM", logger.Err(startErr))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, startErr.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Inject the worker binary.
	workerBinaryName := filepath.Join(d.os.VarDir, "migration-manager-worker")
	pushErr := it.PushFile(inst.GetName(), workerBinaryName, "/root/")
	if pushErr != nil {
		log.Warn("Failed to push file to instance", slog.String("filename", workerBinaryName), logger.Err(pushErr))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, pushErr.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}

	// Start the worker binary.
	workerStartErr := it.ExecWithoutWaiting(inst.GetName(), []string{"/root/migration-manager-worker", "-d", "--endpoint", d.getWorkerEndpoint(), "--uuid", inst.UUID.String(), "--token", inst.SecretToken.String()})
	if workerStartErr != nil {
		log.Warn("Failed to execute without waiting", logger.Err(workerStartErr))
		_, err = d.instance.UpdateStatusByUUID(ctx, inst.UUID, api.MIGRATIONSTATUS_ERROR, workerStartErr.Error(), true)
		if err != nil {
			log.Warn("Failed to update instance status", slog.Any("status", api.MIGRATIONSTATUS_ERROR), logger.Err(err))
		}

		return
	}
}

func (d *Daemon) finalizeCompleteInstances() bool {
	log := slog.With(slog.String("method", "finalizeCompleteInstances"))

	// TODO: context should be passed from the daemon to all the workers.
	ctx := context.TODO()

	err := transaction.Do(ctx, func(ctx context.Context) error {
		// Get any instances in the "complete" state.
		instances, err := d.instance.GetAllByState(ctx, api.MIGRATIONSTATUS_IMPORT_COMPLETE)
		if err != nil {
			return fmt.Errorf("Failed to get instances by state: %w", err)
		}

		batches, err := d.batch.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get all batches: %w", err)
		}

		batchesByID := make(map[int]migration.Batch, len(batches))
		for _, b := range batches {
			batchesByID[b.ID] = b
		}

		for _, i := range instances {
			log := log.With(slog.String("instance", i.InventoryPath))

			log.Info("Finalizing migration steps for instance")

			// Get the target for this instance.
			batch := batchesByID[*i.BatchID]
			t, err := d.target.GetByID(ctx, batch.TargetID)
			if err != nil {
				log.Warn("Failed to get target", slog.Int("target_id", batch.TargetID), logger.Err(err))
				_, err := d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			it, err := target.NewInternalIncusTargetFrom(api.Target{
				Name:       t.Name,
				DatabaseID: t.ID,
				TargetType: t.TargetType,
				Properties: t.Properties,
			})
			if err != nil {
				log.Warn("Failed to construct target", slog.String("target", it.GetName()), logger.Err(err))
				_, err := d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Connect to the target.
			err = it.Connect(d.ShutdownCtx)
			if err != nil {
				log.Warn("Failed to connect to target", slog.String("target", it.GetName()), logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Set the project.
			err = it.SetProject(batch.TargetProject)
			if err != nil {
				log.Warn("Failed to set target project", slog.String("project", batch.TargetProject), logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Stop the instance.
			stopErr := it.StopVM(i.GetName(), true)
			if stopErr != nil {
				log.Warn("Failed to stop VM", logger.Err(stopErr))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Get the instance definition.
			apiDef, etag, err := it.GetInstance(i.GetName())
			if err != nil {
				log.Warn("Failed to get instance", logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Add NIC(s).
			for idx, nic := range i.NICs {
				log := log.With(slog.String("network_hwaddr", nic.Hwaddr))

				nicDeviceName := fmt.Sprintf("eth%d", idx)

				baseNetwork, err := d.network.GetByName(ctx, nic.Network)
				if err != nil {
					log.Warn("Failed to get network", logger.Err(err))
					_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
					if err != nil {
						log.Warn("Failed to update instance status", logger.Err(err))
						continue
					}

					continue
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
			op, err := it.UpdateInstance(i.GetName(), apiDef.Writable(), etag)
			if err != nil {
				log.Warn("Failed to update instance", logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			err = op.Wait()
			if err != nil {
				log.Warn("Failed to wait for operation", logger.Err(err))
				_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_ERROR, err.Error(), true)
				if err != nil {
					log.Warn("Failed to update instance status", logger.Err(err))
					continue
				}

				continue
			}

			// Update the instance status.
			_, err = d.instance.UpdateStatusByUUID(ctx, i.UUID, api.MIGRATIONSTATUS_FINISHED, api.MIGRATIONSTATUS_FINISHED.String(), true)
			if err != nil {
				log.Warn("Failed to update instance status", logger.Err(err))
				continue
			}

			// Power on the completed instance.
			err = it.StartVM(i.GetName())
			if err != nil {
				log.Warn("Failed to start VM", logger.Err(err))
				continue
			}
		}

		return nil
	})
	if err != nil {
		log.Warn("Sync Instances From Sources worker failed", logger.Err(err))
		return false
	}

	return false
}
