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

func DetermineWindowsPartitions() (mainParent string, base string, recoveryParent string, recovery string, ok bool, err error) {
	partitions, err := internalUtil.ScanPartitions("")
	if err != nil {
		return "", "", "", "", false, err
	}

	for _, dev := range partitions.BlockDevices {
		if dev.Serial != "incus_root" {
			continue
		}

		for _, child := range dev.Children {
			if child.PartLabel == "Basic data partition" && child.PartTypeName == "Microsoft basic data" {
				base = child.Name
				mainParent = child.PKName
			} else if child.PartTypeName == "Windows recovery environment" {
				recovery = child.Name
				recoveryParent = child.PKName
			}
		}
	}

	if base == "" || mainParent == "" {
		b, err := json.Marshal(partitions)
		if err != nil {
			return "", "", "", "", false, err
		}

		return "", "", "", "", false, fmt.Errorf("Could not determine partitions: %v", string(b))
	}

	if recovery == "" || recoveryParent == "" {
		return "/dev/" + mainParent, "/dev/" + base, "", "", false, nil
	}

	return "/dev/" + mainParent, "/dev/" + base, "/dev/" + recoveryParent, "/dev/" + recovery, true, nil
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

func WindowsInjectDrivers(ctx context.Context, osVersion string, osArchitecture, isoFile string, dryRun bool) error {
	slog.Info("Preparing to inject Windows drivers into VM")
	windowsVersion, err := internalUtil.MapWindowsVersionToAbbrev(osVersion)
	if err != nil {
		return err
	}

	err = cleanupClones()
	if err != nil {
		return fmt.Errorf("Failed to attempt cleanup of stale clone state")
	}

	defer func() { _ = cleanupClones() }()

	mainParent, mainPartition, recoveryParent, recoveryPartition, recoveryExists, err := DetermineWindowsPartitions()
	if err != nil {
		return err
	}

	plan := map[string]mountInfo{mainPartition: {
		Parent:  mainParent,
		Path:    mainPartition,
		Type:    PARTITION_TYPE_PLAIN,
		Options: []string{},
		Root:    true,
	}}

	if !recoveryExists {
		slog.Warn("Windows recovery partition was not found!")
	}

	if dryRun {
		mappings, err := setupDiskClone(plan)
		if err != nil {
			return err
		}

		for src, clone := range mappings[""] {
			if src == mainParent {
				clonePart, err := getMatchingPartition(mainPartition, clone)
				if err != nil {
					return err
				}

				mainPartition = clonePart
				break
			}
		}

		err = ensureMountIsLoop(mainPartition, PARTITION_TYPE_PLAIN)
		if err != nil {
			return fmt.Errorf("Unexpected main partition location: %w", err)
		}

		if recoveryExists {
			for src, clone := range mappings[""] {
				if src == recoveryParent {
					clonePart, err := getMatchingPartition(recoveryPartition, clone)
					if err != nil {
						return err
					}

					recoveryPartition = clonePart
					break
				}
			}

			err = ensureMountIsLoop(recoveryPartition, PARTITION_TYPE_PLAIN)
			if err != nil {
				return fmt.Errorf("Unexpected recovery partition location: %w", err)
			}
		}
	}

	c := internalUtil.UnixHTTPClient("/dev/incus/sock")
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
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

	if recoveryExists {
		err = DoMount(recoveryPartition, windowsRecoveryMountPath, nil)
		if err != nil {
			return err
		}

		defer func() { _ = DoUnmount(windowsRecoveryMountPath) }()
	}

	// The cloned disk takes longer to populate, so wait an initial 10s before commencing.
	if dryRun {
		time.Sleep(10 * time.Second)
	}

	// ntfs-3g is a FUSE-backed file system; the newly mounted file systems might not be ready right away, so wait until they are.
	mountCheckTries := 0
	for !util.PathExists(filepath.Join(windowsMainMountPath, "Windows")) || (recoveryExists && !util.PathExists(filepath.Join(windowsRecoveryMountPath, "Recovery"))) {
		maxTries := 100
		interval := 100 * time.Millisecond
		if dryRun {
			// Increase the wait duration so sub-files populate too.
			maxTries = 10
			interval = 5 * time.Second
		}

		if mountCheckTries > maxTries {
			return fmt.Errorf("Windows partitions failed to mount properly; can't inject drivers")
		}

		slog.Warn("Windows partition failed to mount properly, retrying", slog.Int("retries", mountCheckTries))

		mountCheckTries++
		time.Sleep(interval)
	}

	// Finally get around to injecting the drivers.
	err = injectDriversHelper(ctx, windowsVersion, osArchitecture, recoveryExists)
	if err != nil {
		return err
	}

	hivexScriptName := "hivex-disable-vm-tools.sh"
	hivexScript, err := embeddedScripts.ReadFile(filepath.Join("scripts/", hivexScriptName))
	if err != nil {
		return err
	}

	// Write the hivex script to /tmp.
	err = os.WriteFile(filepath.Join("/tmp", hivexScriptName), hivexScript, 0o755)
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommand("/bin/sh", filepath.Join("/tmp", hivexScriptName))
	if err != nil {
		return err
	}

	firstBootName := "first-boot.ps1"
	firstBootScript, err := embeddedScripts.ReadFile(filepath.Join("scripts/", firstBootName))
	if err != nil {
		return err
	}

	// Write the first-boot script to C:\.
	err = os.WriteFile(filepath.Join(windowsMainMountPath, "migration-manager-"+firstBootName), firstBootScript, 0o755)
	if err != nil {
		return err
	}

	// Write the first-boot script to C:\.
	err = os.WriteFile(filepath.Join(windowsMainMountPath, "migration-manager-"+firstBootName), firstBootScript, 0o755)
	if err != nil {
		return err
	}

	hivexBootName := "hivex-first-boot.sh"
	hivexBootScript, err := embeddedScripts.ReadFile(filepath.Join("scripts/", hivexBootName))
	if err != nil {
		return err
	}

	// Write the hivex script to /tmp.
	err = os.WriteFile(filepath.Join("/tmp", hivexBootName), hivexBootScript, 0o755)
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommand("/bin/sh", filepath.Join("/tmp", hivexBootName))
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

		args := make([]string, 0, len(hwAddrs)+1)
		args = append(args, filepath.Join("/tmp", hivexScriptName))
		args = append(args, hwAddrs...)
		_, err = subprocess.RunCommand("/bin/sh", args...)
		if err != nil {
			return err
		}
	}

	slog.Info("Successfully injected drivers!")
	return nil
}

func injectDriversHelper(ctx context.Context, windowsVersion string, windowsArchitecture string, recoveryExists bool) error {
	cacheDir := "/tmp/inject-drivers"
	err := os.MkdirAll(cacheDir, 0o700)
	if err != nil {
		return err
	}

	repackUtuil := windows.NewRepackUtil(cacheDir, ctx, logger.SlogBackedLogrus())

	var reWim string
	var reWimInfo windows.WimInfo
	if recoveryExists {
		reWim, err = shared.FindFirstMatch(windowsRecoveryMountPath, "Recovery/WindowsRE", "winre.wim")
		if err != nil {
			return fmt.Errorf("Unable to find winre.wim: %w", err)
		}

		reWimInfo, err = repackUtuil.GetWimInfo(reWim)
		if err != nil {
			return fmt.Errorf("Failed to get RE wim info: %w", err)
		}

		windowsArchitecture = windows.DetectWindowsArchitecture(reWimInfo.Architecture(1))
		if windowsVersion == "" {
			windowsVersion = windows.DetectWindowsVersion(reWimInfo.Name(1))
		}
	} else {
		windowsArchitecture = windows.DetectWindowsArchitecture(windowsArchitecture)
		windowsVersion = windows.DetectWindowsVersion(windowsVersion)
	}

	if windowsVersion == "" {
		return fmt.Errorf("Failed to detect Windows version")
	}

	if windowsArchitecture == "" {
		return fmt.Errorf("Failed to detect Windows architecture")
	}

	repackUtuil.SetWindowsVersionArchitecture(windowsVersion, windowsArchitecture)

	// Inject drivers into the RE wim image.
	if recoveryExists {
		err = repackUtuil.InjectDriversIntoWim(reWim, reWimInfo, driversMountPath)
		if err != nil {
			return fmt.Errorf("Failed to modify wim %q: %w", reWim, err)
		}
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
