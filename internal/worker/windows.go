package worker

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
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
	BITLOCKERSTATE_UNKNOWN BitLockerState = iota
	BITLOCKERSTATE_UNENCRYPTED
	BITLOCKERSTATE_ENCRYPTED
	BITLOCKERSTATE_CLEARKEY
)

const (
	bitLockerMountPath       string = "/mnt/dislocker/"
	driversMountDevice       string = "/dev/disk/by-id/scsi-0QEMU_QEMU_CD-ROM_incus_drivers"
	driversMountPath         string = "/mnt/drivers/"
	windowsMainMountPath     string = "/mnt/win_main/"
	windowsRecoveryMountPath string = "/mnt/win_recovery/"
)

func init() {
	_ = pongo2.RegisterFilter("toHex", toHex)
}

func WindowsDetectBitLockerStatus(partition string) (BitLockerState, error) {
	// Regexes to determine the BitLocker status.
	unencryptedRegex := regexp.MustCompile(`\[ERROR\] The signature of the volume \(.+\) doesn't match the BitLocker's ones \(-FVE-FS- or MSWIN4.1\). Abort.`)
	bitLockerEnabledRegex := regexp.MustCompile(`\[INFO\] =====================\[ BitLocker information structure \]=====================`)
	noClearKeyRegex := regexp.MustCompile(`\[INFO\] No clear key found.`)
	clearKeyRegex := regexp.MustCompile(`\[INFO\] =======\[ There's a clear key here \]========`)

	stdout, err := subprocess.RunCommand("dislocker-metadata", "-V", partition)

	// Return the status.
	if unencryptedRegex.Match([]byte(stdout)) {
		return BITLOCKERSTATE_UNENCRYPTED, nil
	}

	if bitLockerEnabledRegex.Match([]byte(stdout)) {
		if noClearKeyRegex.Match([]byte(stdout)) {
			return BITLOCKERSTATE_ENCRYPTED, nil
		}

		if clearKeyRegex.Match([]byte(stdout)) {
			return BITLOCKERSTATE_CLEARKEY, nil
		}
	}

	if err != nil {
		return BITLOCKERSTATE_UNKNOWN, err
	}

	return BITLOCKERSTATE_UNKNOWN, fmt.Errorf("Failed to determine BitLocker status for %s", partition)
}

func WindowsOpenBitLockerPartition(partition string, encryptionKey string) error {
	if !util.PathExists(bitLockerMountPath) {
		err := os.MkdirAll(bitLockerMountPath, 0o755)
		if err != nil {
			return fmt.Errorf("Failed to create mount target %q", bitLockerMountPath)
		}
	}

	if encryptionKey == "" {
		_, err := subprocess.RunCommand("dislocker-fuse", "-V", partition, "--clearkey", "--", bitLockerMountPath)
		return err
	}

	_, err := subprocess.RunCommand("dislocker-fuse", "-V", partition, "--recovery-password="+encryptionKey, "--", bitLockerMountPath)
	return err
}

func WindowsInjectDrivers(ctx context.Context, windowsVersion string, mainPartition string, recoveryPartition string) error {
	slog.Info("Preparing to inject Windows drivers into VM")

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

	slog.Info("Successfully injected drivers!")
	return nil
}

func injectDriversHelper(ctx context.Context, windowsVersion string) error {
	cacheDir := "/tmp/inject-drivers"
	err := os.MkdirAll(cacheDir, 0o700)
	if err != nil {
		return err
	}

	// Distrobuilder does require a logrus Logger.
	log, err := shared.GetLogger(false)
	if err != nil {
		return fmt.Errorf("Failed to get logger: %w\n", err)
	}

	repackUtuil := windows.NewRepackUtil(cacheDir, ctx, log)

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

// Take a full version string and return the abbreviation used by distrobuilder logic.
// Versions supported are an intersection of what's supported by distrobuilder and vCenter.
func MapWindowsVersionToAbbrev(version string) (string, error) {
	switch {
	case strings.Contains(version, "Windows XP"):
		return "xp", nil
	case strings.Contains(version, "Windows 7"):
		return "w7", nil
	case strings.Contains(version, "Windows 8"):
		return "w8", nil
	case strings.Contains(version, "Windows 10"):
		return "w10", nil
	case strings.Contains(version, "Windows 11"):
		return "w11", nil
	case strings.Contains(version, "Server 2003"):
		return "2k3", nil
	case strings.Contains(version, "Server 2008 R2"):
		return "2k8r2", nil
	case strings.Contains(version, "Server 2019"):
		return "2k19", nil
	case strings.Contains(version, "Server 2022"):
		return "2k22", nil
	default:
		return "", fmt.Errorf("'%s' is not currently supported", version)
	}
}
