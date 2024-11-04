package worker

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/flosch/pongo2/v4"
	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/windows"
	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"
)

type BitLockerState int
const (
	BITLOCKERSTATE_UNENCRYPTED BitLockerState = iota
	BITLOCKERSTATE_ENCRYPTED
	BITLOCKERSTATE_CLEARKEY
	BITLOCKERSTATE_UNKNOWN
)

const bitLockerMountPath string = "/mnt/dislocker/"
const driversMountDevice string = "/dev/disk/by-id/scsi-0QEMU_QEMU_CD-ROM_incus_drivers"
const driversMountPath string = "/mnt/drivers/"
const windowsMainMountPath string = "/mnt/win_main/"
const windowsRecoveryMountPath string = "/mnt/win_recovery/"

func init() {
	_ = pongo2.RegisterFilter("toHex", toHex)
}

func WindowsDetectBitLockerStatus(partition string) (BitLockerState, error) {
	unencryptedRegex := regexp.MustCompile(`\[ERROR\] The signature of the volume \(.+\) doesn't match the BitLocker's ones \(-FVE-FS- or MSWIN4.1\). Abort.`)
	bitLockerEnabledRegex := regexp.MustCompile(`\[INFO\] =====================\[ BitLocker information structure \]=====================`)
	noClearKeyRegex := regexp.MustCompile(`\[INFO\] No clear key found.`)
	clearKeyRegex := regexp.MustCompile(`\[INFO\] =======\[ There's a clear key here \]========`)

	stdout, err := subprocess.RunCommand("dislocker-metadata", "-V", partition)
	if err != nil {
		return BITLOCKERSTATE_UNKNOWN, err
	}

	if unencryptedRegex.Match([]byte(stdout)) {
		return BITLOCKERSTATE_UNENCRYPTED, nil
	} else if bitLockerEnabledRegex.Match([]byte(stdout)) {
		if noClearKeyRegex.Match([]byte(stdout)) {
			return BITLOCKERSTATE_ENCRYPTED, nil
		} else if clearKeyRegex.Match([]byte(stdout)) {
			return BITLOCKERSTATE_CLEARKEY, nil
		}
	}

	return BITLOCKERSTATE_UNKNOWN, fmt.Errorf("Failed to determine BitLocker status for %s", partition)
}

func WindowsOpenBitLockerPartition(partition string, encryptionKey string) error {
        if !util.PathExists(bitLockerMountPath) {
                err := os.MkdirAll(bitLockerMountPath, 0755)
                if err != nil {
                        return fmt.Errorf("Failed to create mount target %q", bitLockerMountPath)
                }
        }

	if encryptionKey == "" {
		_, err := subprocess.RunCommand("dislocker-fuse", "-V", partition, "--clearkey", "--", bitLockerMountPath)
		return err
	}

	_, err := subprocess.RunCommand("dislocker-fuse", "-V", partition, "--recovery-password=" + encryptionKey, "--", bitLockerMountPath)
	return err
}

func WindowsInjectDrivers(ctx context.Context, windowsVersion string, mainPartition string, recoveryPartition string) error {
	fmt.Printf("Preparing to inject Windows drivers into VM....\n")

	// Mount the virtio drivers image.
	err := DoMount(driversMountDevice, driversMountPath, nil)
	if err != nil {
		return err
	}
	defer func() { _ = DoUnmount(driversMountPath) }()

	// Get the BitLocker status.
	bitLockerStatus, err := WindowsDetectBitLockerStatus(mainPartition)
	if err != nil {
		return err
	}

	// Mount the main Windows partition.
	switch bitLockerStatus {
	case BITLOCKERSTATE_UNENCRYPTED:
		err = DoMount(mainPartition, windowsMainMountPath, nil)
		if err != nil {
			return err
		}
		defer func() { _ = DoUnmount(windowsMainMountPath) }()
	case BITLOCKERSTATE_CLEARKEY:
		err = WindowsOpenBitLockerPartition(mainPartition, "")
		if err != nil {
			return err
		}
		defer func() { _ = DoUnmount(bitLockerMountPath) }()

		err = DoMount(filepath.Join(bitLockerMountPath, "dislocker-file"), windowsMainMountPath, nil)
		if err != nil {
			return err
		}
		defer func() { _ = DoUnmount(windowsMainMountPath) }()
	default:
		// TODO -- Handle passing in of a recovery key for mounting BitLocker partition.
		return fmt.Errorf("BitLocker without a clear key detected; bailing out for now")
	}

	// Mount the Windows recovery partition.
	err = DoMount(recoveryPartition, windowsRecoveryMountPath, nil)
	if err != nil {
		return err
	}
	defer func() { _ = DoUnmount(windowsRecoveryMountPath) }()

	// ntfs-3g is a FUSE-backed file system; the newly mounted file systems might not be ready right away, so wait until they are.
	mountCheckTries := 0
	for !util.PathExists(filepath.Join(windowsMainMountPath, "Windows")) || !util.PathExists(filepath.Join(windowsRecoveryMountPath, "Recovery")) {
		if mountCheckTries > 100 {
			return fmt.Errorf("Windows partitions failed to mount properly; can't inject drivers")
		}
		mountCheckTries++
		time.Sleep(100 * time.Millisecond)
	}

	// Finally get around to injecting the drivers.
	err = injectDriversHelper(ctx, windowsVersion)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully injected drivers!\n")
	return nil
}

func injectDriversHelper(ctx context.Context, windowsVersion string) error {
	cacheDir := "/tmp/inject-drivers"
	err := os.MkdirAll(cacheDir, 0700)
	if err != nil {
		return err
	}

	logger, err := shared.GetLogger(false)
	if err != nil {
		return fmt.Errorf("Failed to get logger: %w\n", err)
	}

	repackUtuil := windows.NewRepackUtil(cacheDir, ctx, logger)

	reWim, err := shared.FindFirstMatch(windowsRecoveryMountPath, "Recovery/WindowsRE", "winre.wim")
	if err != nil {
		return fmt.Errorf("Unable to find winre.wim: %w", err)
	}
	reWimInfo, err := repackUtuil.GetWimInfo(reWim)
	if err != nil {
		return fmt.Errorf("Failed to get RE wim info: %w", err)
	}

	if windowsVersion == "" {
		windowsVersion = windows.DetectWindowsVersion(reWimInfo.Name(1))
	}

	windowsArchitecture := windows.DetectWindowsArchitecture(reWimInfo.Architecture(1))

	if windowsVersion == "" {
		return fmt.Errorf("Failed to detect Windows version. Please provide the version using the --windows-version flag")
	}

	if windowsArchitecture == "" {
		return fmt.Errorf("Failed to detect Windows architecture. Please provide the architecture using the --windows-architecture flag")
	}

	repackUtuil.SetWindowsVersionArchitecture(windowsVersion, windowsArchitecture)

	// Inject drivers into the RE wim image.
	err = repackUtuil.InjectDriversIntoWim(reWim, reWimInfo, driversMountPath)
	if err != nil {
		return fmt.Errorf("Failed to modify wim %q: %w", reWim, err)
	}

	// Inject drivers into the Windows install.
	err = repackUtuil.InjectDrivers(windowsMainMountPath, driversMountPath)
	if err != nil {
		return fmt.Errorf("Failed to inject drivers: %w", err)
	}

	return nil
}

// toHex is a pongo2 filter which converts the provided value to a hex value understood by the Windows registry.
func toHex(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
	dst := make([]byte, hex.EncodedLen(len(in.String())))
	hex.Encode(dst, []byte(in.String()))

	var builder strings.Builder

	for i := 0; i < len(dst); i += 2 {
		_, err := builder.Write(dst[i : i+2])
		if err != nil {
			return &pongo2.Value{}, &pongo2.Error{Sender: "filter:toHex", OrigError: err}
		}

		_, err = builder.WriteString(",00,")
		if err != nil {
			return &pongo2.Value{}, &pongo2.Error{Sender: "filter:toHex", OrigError: err}
		}
	}

	return pongo2.AsValue(strings.TrimSuffix(builder.String(), ",")), nil
}
