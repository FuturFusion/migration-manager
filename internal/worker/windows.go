package worker

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/flosch/pongo2/v4"
	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/windows"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"

	"github.com/FuturFusion/migration-manager/internal/logger"
	internalUtil "github.com/FuturFusion/migration-manager/internal/util"
)

type BitLockerState int

const (
	BITLOCKERSTATE_UNKNOWN BitLockerState = iota
	BITLOCKERSTATE_UNENCRYPTED
	BITLOCKERSTATE_ENCRYPTED
	BITLOCKERSTATE_CLEARKEY
)

const (
	bitLockerMountPath       string = "/run/mount/dislocker/"
	driversMountPath         string = "/run/mount/drivers/"
	windowsMainMountPath     string = "/run/mount/win_main/"
	windowsRecoveryMountPath string = "/run/mount/win_recovery/"
)

func init() {
	_ = pongo2.RegisterFilter("toHex", toHex)
}

func DetermineWindowsPartitions() (base string, recovery string, err error) {
	partitions, err := internalUtil.ScanPartitions("")
	if err != nil {
		return "", "", err
	}

	for _, dev := range partitions.BlockDevices {
		if dev.Serial != "incus_root" {
			continue
		}

		for _, child := range dev.Children {
			if child.PartLabel == "Basic data partition" && child.PartTypeName == "Microsoft basic data" {
				base = child.Name
			} else if child.PartTypeName == "Windows recovery environment" {
				recovery = child.Name
			}
		}
	}

	if base == "" || recovery == "" {
		b, err := json.Marshal(partitions)
		if err != nil {
			return "", "", err
		}

		return "", "", fmt.Errorf("Could not determine partitions: %v", string(b))
	}

	return "/dev/" + base, "/dev/" + recovery, nil
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

func WindowsInjectDrivers(ctx context.Context, osVersion string, isoFile string, dryRun bool) error {
	slog.Info("Preparing to inject Windows drivers into VM")
	windowsVersion, err := internalUtil.MapWindowsVersionToAbbrev(osVersion)
	if err != nil {
		return err
	}

	mainPartition, recoveryPartition, err := DetermineWindowsPartitions()
	if err != nil {
		return err
	}

	if dryRun {
		mainPartition, _, err = setupDiskClone(mainPartition, PARTITION_TYPE_PLAIN, nil)
		if err != nil {
			return err
		}

		defer func() { _ = cleanupDiskClone(PARTITION_TYPE_PLAIN) }()

		recoveryPartition, err = getMatchingPartition(recoveryPartition, mainPartition)
		if err != nil {
			return err
		}
	}

	c := internalUtil.UnixHTTPClient("/dev/incus/sock")
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix.socket/1.0/config/user.migration.hwaddrs", nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		return err
	}

	var hwAddrs []string
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		out, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		hwAddrs = strings.Split(string(out), " ")
	}

	// Mount the virtio drivers image.
	err = DoMount(isoFile, driversMountPath, []string{"-o", "loop"})
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

	// Re-assign network configs to the new NIC if we have MACs.
	if len(hwAddrs) > 0 {
		hivexScriptName := "hivex-assign-netcfg.sh"
		ps1ScriptName := "virtio-assign-netcfg.ps1"
		ps1Script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", ps1ScriptName))
		if err != nil {
			return err
		}

		// Write the ps1 script to C:\.
		err = os.WriteFile(filepath.Join(windowsMainMountPath, ps1ScriptName), ps1Script, 0o755)
		if err != nil {
			return err
		}

		hivexScript, err := embeddedScripts.ReadFile(filepath.Join("scripts/", hivexScriptName))
		if err != nil {
			return err
		}

		// Write the hivex script to /tmp.
		err = os.WriteFile(filepath.Join("/tmp", hivexScriptName), hivexScript, 0o755)
		if err != nil {
			return err
		}

		args := []string{filepath.Join("/tmp", hivexScriptName)}
		args = append(args, hwAddrs...)
		_, err = subprocess.RunCommand("/bin/sh", args...)
		if err != nil {
			return err
		}
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

	repackUtuil := windows.NewRepackUtil(cacheDir, ctx, logger.SlogBackedLogrus())

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
