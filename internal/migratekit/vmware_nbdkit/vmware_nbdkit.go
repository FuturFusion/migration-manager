package vmware_nbdkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"libguestfs.org/libnbd"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdcopy"
	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdkit"
	"github.com/FuturFusion/migration-manager/internal/migratekit/progress"
	"github.com/FuturFusion/migration-manager/internal/migratekit/target"
	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/internal/util"
)

const MaxChunkSize = 64 * 1024 * 1024

type VddkConfig struct {
	Debug       bool
	Endpoint    *url.URL
	Thumbprint  string
	Compression nbdkit.CompressionMethod
}

type NbdkitServers struct {
	VddkConfig     *VddkConfig
	VirtualMachine *object.VirtualMachine
	SnapshotRef    types.ManagedObjectReference
	Servers        []*NbdkitServer
	StatusCallback func(string, bool)
	SDKPath        string
}

type NbdkitServer struct {
	Servers *NbdkitServers
	Disk    *types.VirtualDisk
	Nbdkit  *nbdkit.NbdkitServer
}

func NewNbdkitServers(vddk *VddkConfig, vm *object.VirtualMachine, sdkPath string, statusCallback func(string, bool)) *NbdkitServers {
	return &NbdkitServers{
		VddkConfig:     vddk,
		VirtualMachine: vm,
		Servers:        []*NbdkitServer{},
		StatusCallback: statusCallback,
		SDKPath:        sdkPath,
	}
}

func (s *NbdkitServers) createSnapshot(ctx context.Context) error {
	task, err := s.VirtualMachine.CreateSnapshot(ctx, internal.IncusSnapshotName, "Ephemeral snapshot for Incus migration", false, false)
	if err != nil {
		return fmt.Errorf("Failed to create snapshot: %w", err)
	}

	bar := progress.NewVMwareProgressBar("Creating snapshot")
	s.StatusCallback("Creating snapshot", true)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		bar.Loop(ctx.Done())
	}()
	defer cancel()

	info, err := task.WaitForResult(ctx, bar)
	if err != nil {
		return fmt.Errorf("Snapshot create task error: %w", err)
	}

	s.StatusCallback("Done creating snapshot", true)
	s.SnapshotRef = info.Result.(types.ManagedObjectReference)
	return nil
}

func (s *NbdkitServers) Start(ctx context.Context) error {
	err := s.createSnapshot(ctx)
	if err != nil {
		return err
	}

	var snapshot mo.VirtualMachineSnapshot
	err = s.VirtualMachine.Properties(ctx, s.SnapshotRef, []string{"config.hardware"}, &snapshot)
	if err != nil {
		return err
	}

	for _, device := range snapshot.Config.Hardware.Device {
		switch disk := device.(type) {
		case *types.VirtualDisk:
			// Ignore raw disks or those excluded from snapshots.
			_, err := vmware.IsSupportedDisk(disk)
			if err != nil {
				continue
			}

			diskName, err := vmware.IsSupportedDisk(disk)
			if err != nil {
				return err
			}

			password, _ := s.VddkConfig.Endpoint.User.Password()
			server, err := nbdkit.NewNbdkitBuilder().
				Server(s.VddkConfig.Endpoint.Host).
				Username(s.VddkConfig.Endpoint.User.Username()).
				Password(password).
				Thumbprint(s.VddkConfig.Thumbprint).
				VirtualMachine(s.VirtualMachine.Reference().Value).
				Snapshot(s.SnapshotRef.Value).
				Filename(diskName).
				Compression(s.VddkConfig.Compression).
				SDK(s.SDKPath).
				Build()
			if err != nil {
				return err
			}

			if err := server.Start(); err != nil {
				return err
			}

			s.Servers = append(s.Servers, &NbdkitServer{
				Servers: s,
				Disk:    disk,
				Nbdkit:  server,
			})
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		slog.Warn("Received interrupt signal, cleaning up...")

		err := s.Stop(ctx)
		if err != nil {
			slog.Error("Failed to stop nbdkit servers", slog.Any("error", err))
		}

		os.Exit(1)
	}()

	return nil
}

func (s *NbdkitServers) removeSnapshot(ctx context.Context) error {
	consolidate := true
	task, err := s.VirtualMachine.RemoveSnapshot(ctx, s.SnapshotRef.Value, false, &consolidate)
	if err != nil {
		return fmt.Errorf("Failed to remove snapshot: %w", err)
	}

	bar := progress.NewVMwareProgressBar("Removing snapshot")
	s.StatusCallback("Removing snapshot", true)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		bar.Loop(ctx.Done())
	}()
	defer cancel()

	_, err = task.WaitForResult(ctx, bar)
	if err != nil {
		return fmt.Errorf("Snapshot remove task failed: %w", err)
	}

	s.StatusCallback("Done removing snapshot", true)
	return nil
}

func (s *NbdkitServers) Stop(ctx context.Context) error {
	for _, server := range s.Servers {
		if err := server.Nbdkit.Stop(); err != nil {
			return err
		}
	}

	err := s.removeSnapshot(ctx)
	if err != nil {
		return err
	}

	return nil
}

func getIncusDisk(ctx context.Context, client *http.Client, diskName string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix.socket/1.0/devices", nil)
	if err != nil {
		return "", false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}

	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}

	devices := map[string]map[string]string{}
	err = json.Unmarshal(out, &devices)
	if err != nil {
		return "", false, err
	}

	var devName string
	for id, cfg := range devices {
		if cfg["user.migration_source"] == diskName {
			devName = id
			break
		}
	}

	if devName == "" {
		return "", false, fmt.Errorf("Failed to find any disk with migration source %q", diskName)
	}

	entries, err := os.ReadDir("/dev/disk/by-id")
	if err != nil {
		return "", false, err
	}

	diskID := "scsi-0QEMU_QEMU_HARDDISK_incus_" + devName
	for _, e := range entries {
		if e.Name() != diskID {
			continue
		}

		diskPath, err := filepath.EvalSymlinks(filepath.Join("/dev/disk/by-id", e.Name()))
		if err != nil {
			return "", false, err
		}

		return diskPath, devName == "root", nil
	}

	return "", false, fmt.Errorf("Failed to find disk with ID %q", diskID)
}

func (s *NbdkitServers) MigrationCycle(ctx context.Context, runV2V bool) error {
	err := s.Start(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := s.Stop(ctx)
		if err != nil {
			slog.Error("Failed to stop nbdkit servers", slog.Any("error", err))
		}
	}()

	devIncus := util.UnixHTTPClient("/dev/incus/sock")
	for _, server := range s.Servers {
		diskName, err := vmware.IsSupportedDisk(server.Disk)
		if err != nil {
			return err
		}

		diskID, isRoot, err := getIncusDisk(ctx, devIncus, diskName)
		if err != nil {
			return err
		}

		if isRoot {
			runV2V = false
		}

		t, err := target.NewDiskTarget(s.VirtualMachine, server.Disk, diskID)
		if err != nil {
			return err
		}

		err = server.SyncToTarget(ctx, t, runV2V, s.StatusCallback)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *NbdkitServer) FullCopyToTarget(t target.Target, path string, targetIsClean bool, statusCallback func(string, bool)) error {
	diskName, err := vmware.IsSupportedDisk(s.Disk)
	if err != nil {
		return err
	}

	log := slog.With(
		slog.String("vm", s.Servers.VirtualMachine.Name()),
		slog.String("disk", diskName),
	)

	log.Info("Starting full copy")

	index := 1
	for i, server := range s.Servers.Servers {
		serverDiskName, err := vmware.IsSupportedDisk(server.Disk)
		if err != nil {
			return err
		}

		if serverDiskName == diskName {
			index = i + 1
			break
		}
	}

	msg := fmt.Sprintf("Importing disk (%d/%d)", index, len(s.Servers.Servers))
	err = nbdcopy.Run(
		msg,
		s.Nbdkit.LibNBDExportName(),
		path,
		s.Disk.CapacityInBytes,
		targetIsClean,
		diskName,
		statusCallback,
	)
	if err != nil {
		return err
	}

	log.Info("Full copy completed")

	return nil
}

func (s *NbdkitServer) IncrementalCopyToTarget(ctx context.Context, t target.Target, path string, statusCallback func(string, bool)) error {
	diskName, err := vmware.IsSupportedDisk(s.Disk)
	if err != nil {
		return err
	}

	index := 1
	for i, server := range s.Servers.Servers {
		serverDiskName, err := vmware.IsSupportedDisk(server.Disk)
		if err != nil {
			return err
		}

		if serverDiskName == diskName {
			index = i + 1
			break
		}
	}

	log := slog.With(
		slog.String("vm", s.Servers.VirtualMachine.Name()),
		slog.String("disk", diskName),
	)

	log.Info("Starting incremental copy")

	currentChangeId, err := t.GetCurrentChangeID(ctx)
	if err != nil {
		return err
	}

	handle, err := libnbd.Create()
	if err != nil {
		return err
	}

	err = handle.ConnectUri(s.Nbdkit.LibNBDExportName())
	if err != nil {
		return err
	}

	// We have removed os.O_EXCL, as it was causing some weird failure when attempting to perform followup incremental disk syncs.
	// For our use, we know nothing else in the migration environment will be doing anything with the raw disk device.
	fd, err := os.OpenFile(path, os.O_WRONLY|syscall.O_DIRECT, 0o644)
	if err != nil {
		return err
	}
	defer fd.Close()

	startOffset := int64(0)
	bar := progress.DataProgressBar("Incremental copy", s.Disk.CapacityInBytes)

	for {
		req := types.QueryChangedDiskAreas{
			This:        s.Servers.VirtualMachine.Reference(),
			Snapshot:    &s.Servers.SnapshotRef,
			DeviceKey:   s.Disk.Key,
			StartOffset: startOffset,
			ChangeId:    currentChangeId.Value,
		}

		res, err := methods.QueryChangedDiskAreas(ctx, s.Servers.VirtualMachine.Client(), &req)
		if err != nil {
			return fmt.Errorf("Failed to query disk changes: %w", err)
		}

		diskChangeInfo := res.Returnval

		for _, area := range diskChangeInfo.ChangedArea {
			for offset := area.Start; offset < area.Start+area.Length; {
				chunkSize := area.Length - (offset - area.Start)
				if chunkSize > MaxChunkSize {
					chunkSize = MaxChunkSize
				}

				buf := make([]byte, chunkSize)
				err = handle.Pread(buf, uint64(offset), nil)
				if err != nil {
					return err
				}

				_, err = fd.WriteAt(buf, offset)
				if err != nil {
					return err
				}

				bar.Set64(offset + chunkSize)
				statusCallback(fmt.Sprintf("Importing disk (%d/%d) %q: %02.2f%% complete", index, len(s.Servers.Servers), diskName, float64(offset+chunkSize)/float64(s.Disk.CapacityInBytes)*100.0), false)
				offset += chunkSize
			}
		}

		startOffset = diskChangeInfo.StartOffset + diskChangeInfo.Length
		bar.Set64(startOffset)

		if startOffset == s.Disk.CapacityInBytes {
			break
		}
	}

	return nil
}

func (s *NbdkitServer) SyncToTarget(ctx context.Context, t target.Target, runV2V bool, statusCallback func(string, bool)) error {
	snapshotChangeId, err := vmware.GetChangeID(s.Disk)
	if err != nil {
		// Rather than returning an error when CBT isn't enabled, just proceed with a dummy change ID.
		// This will always force a full disk copy, which is OK for our use case.
		snapshotChangeId = &vmware.ChangeID{
			UUID:   "",
			Number: "",
			Value:  "",
		}
	}

	needFullCopy, targetIsClean, err := target.NeedsFullCopy(ctx, t)
	if err != nil {
		return err
	}

	err = t.Connect(ctx)
	if err != nil {
		return err
	}
	defer t.Disconnect(ctx)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		slog.Warn("Received interrupt signal, cleaning up...")

		err := t.Disconnect(ctx)
		if err != nil {
			slog.Error("Failed to disconnect from target", slog.Any("error", err))
		}

		os.Exit(1)
	}()

	path, err := t.GetPath(ctx)
	if err != nil {
		return err
	}

	if needFullCopy {
		err = s.FullCopyToTarget(t, path, targetIsClean, statusCallback)
		if err != nil {
			return err
		}
	} else {
		err = s.IncrementalCopyToTarget(ctx, t, path, statusCallback)
		if err != nil {
			return err
		}
	}

	if runV2V {
		slog.Info("Running virt-v2v-in-place")

		os.Setenv("LIBGUESTFS_BACKEND", "direct")

		var cmd *exec.Cmd
		if s.Servers.VddkConfig.Debug {
			cmd = exec.Command("virt-v2v-in-place", "-v", "-x", "--no-selinux-relabel", "-i", "disk", path)
		} else {
			cmd = exec.Command("virt-v2v-in-place", "--no-selinux-relabel", "-i", "disk", path)
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			return err
		}

		err = t.WriteChangeID(ctx, &vmware.ChangeID{})
		if err != nil {
			return err
		}
	} else {
		err = t.WriteChangeID(ctx, snapshotChangeId)
		if err != nil {
			return err
		}
	}

	return nil
}
