package worker

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/lxc/incus/v6/shared/util"

	internalUtil "github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:embed scripts/*
var embeddedScripts embed.FS

type PartitionType string

const (
	PARTITION_TYPE_UNKNOWN PartitionType = "unknown"
	PARTITION_TYPE_PLAIN   PartitionType = "plain"
	PARTITION_TYPE_LVM     PartitionType = "lvm"
)

type LVSOutput struct {
	Report []struct {
		LV []struct {
			VGName string `json:"vg_name"`
			LVName string `json:"lv_name"`
		} `json:"lv"`
		PV []struct {
			VGName string `json:"vg_name"`
			PVName string `json:"pv_name"`
		} `json:"pv"`
	} `json:"report"`
}

const chrootMountPath string = "/run/mount/target/"

func LinuxDoPostMigrationConfig(ctx context.Context, instance api.Instance, osName string, dryRun bool) error {
	// Get the disto's major version, if possible.
	majorVersion := -1
	// VMware API doesn't distinguish openSUSE and Ubuntu versions.
	if !strings.Contains(strings.ToLower(osName), "opensuse") && !strings.Contains(strings.ToLower(osName), "ubuntu") {
		majorVersionRegex := regexp.MustCompile(`^\w+?(\d+)(_64)?$`)
		matches := majorVersionRegex.FindStringSubmatch(osName)
		if len(matches) > 1 {
			majorVersion, _ = strconv.Atoi(majorVersionRegex.FindStringSubmatch(osName)[1])
		}
	}

	distro := ""
	if strings.Contains(strings.ToLower(osName), "centos") {
		distro = "CentOS"
	} else if strings.Contains(strings.ToLower(osName), "debian") {
		distro = "Debian"
	} else if strings.Contains(strings.ToLower(osName), "opensuse") {
		distro = "openSUSE"
	} else if strings.Contains(strings.ToLower(osName), "oracle") {
		distro = "Oracle"
	} else if slices.ContainsFunc([]string{"rhel", "redhat", "red-hat", "red hat"}, func(s string) bool {
		return strings.Contains(strings.ToLower(osName), s)
	}) {
		distro = "RHEL"
	} else if strings.Contains(strings.ToLower(osName), "sles") {
		distro = "SUSE"
	} else if strings.Contains(strings.ToLower(osName), "ubuntu") {
		distro = "Ubuntu"
	}

	if distro == "" {
		slog.Info("Could not determine Linux distribution, not performing any post-migration actions")
		return nil
	}

	slog.Info("Preparing to perform post-migration configuration of VM")

	err := cleanupClones()
	if err != nil {
		return fmt.Errorf("Failed to attempt cleanup of stale clone state")
	}

	defer func() { _ = cleanupClones() }()

	// Determine the root partition.
	rootParent, rootPart, rootType, rootOpts, err := determineRootPartition(looksLikeLinuxRootPartition)
	if err != nil {
		return err
	}

	var mappings map[string]map[string]string
	var plan map[string]mountInfo
	if dryRun {
		plan, err = getRequiredMounts(looksLikeLinuxRootPartition)
		if err != nil {
			return err
		}

		mappings, err = setupDiskClone(plan)
		if err != nil {
			return err
		}

		defer func() { _ = DeactivateVG() }()

		for vgName, sourceToClone := range mappings {
			if vgName != "" {
				vgFilter := []string{}
				// Create a filter to avoid lvm duplication errors.
				for src, clone := range sourceToClone {
					parts := strings.Split(src, "/")
					vgFilter = append(vgFilter, fmt.Sprintf("'a|%s|', 'r|%s|', 'r|/dev/mapper/clone_%s|'", clone, src, parts[len(parts)-1]))
				}

				filter := "devices { filter = [ " + strings.Join(vgFilter, ", ") + " ] }"
				slog.Info("Activating volume groups with filter", slog.String("config", filter), slog.String("vg_name", vgName))
				err := ActivateVG(filter, vgName)
				if err != nil {
					return err
				}
			}

			for src, clone := range sourceToClone {
				partType := PARTITION_TYPE_PLAIN
				if vgName != "" {
					partType = PARTITION_TYPE_LVM
				}

				if partType == PARTITION_TYPE_PLAIN && src == rootParent {
					clonePart, err := getMatchingPartition(rootPart, clone)
					if err != nil {
						return err
					}

					rootPart = clonePart
				}

				// After activating the VG, ensure the mapping is to a loop device if performing dry-run.
				err := ensureMountIsLoop(clone, partType)
				if err != nil {
					return err
				}
			}
		}

		err = ensureMountIsLoop(rootPart, rootType)
		if err != nil {
			return err
		}
	}

	// Activate VG prior to mounting, if needed.
	if !dryRun && rootType == PARTITION_TYPE_LVM {
		err := ActivateVG()
		if err != nil {
			return err
		}

		defer func() { _ = DeactivateVG() }()
	}

	// Mount the migrated root partition.
	err = DoMount(rootPart, chrootMountPath, rootOpts)
	if err != nil {
		return err
	}

	defer func() { _ = DoUnmount(chrootMountPath) }()

	// Bind-mount /dev/, /proc/ and /sys/ into the chroot.
	err = DoMount("/dev/", filepath.Join(chrootMountPath, "dev"), []string{"-o", "bind"})
	if err != nil {
		return err
	}

	defer func() { _ = DoUnmount(filepath.Join(chrootMountPath, "dev")) }()

	err = DoMount("/proc/", filepath.Join(chrootMountPath, "proc"), []string{"-o", "bind"})
	if err != nil {
		return err
	}

	defer func() { _ = DoUnmount(filepath.Join(chrootMountPath, "proc")) }()

	err = DoMount("/sys/", filepath.Join(chrootMountPath, "sys"), []string{"-o", "bind"})
	if err != nil {
		return err
	}

	defer func() { _ = DoUnmount(filepath.Join(chrootMountPath, "sys")) }()

	// Mount additional file systems, such as /var/ on a different partition.
	for _, mnt := range getAdditionalMounts() {
		opts := []string{}
		if mnt["options"] != "" {
			opts = []string{"-o", mnt["options"]}
		}

		log := slog.With(slog.String("fstab", mnt["device"]))
		dev := mnt["device"]
		p, ok := plan[mnt["device"]]
		if dryRun && ok {
			log = log.With(slog.String("path", p.Path), slog.String("parent", p.Parent))
			log.Info("Finding fstab entry in lsblk")
			for _, sourceToClone := range mappings {
				path := p.Parent
				if p.Type == PARTITION_TYPE_LVM {
					path = p.Path
				}

				clone, ok := sourceToClone[path]
				if ok {
					part, err := getMatchingPartition(p.Path, clone)
					if err != nil {
						return err
					}

					log.Info("Found a matching fstab clone", slog.String("clone", part))
					dev = part
					break
				}
			}
		}

		log.Info("Mounting the disk from fstab", slog.String("device", dev), slog.String("target", filepath.Join(chrootMountPath, mnt["path"])), slog.String("opts", strings.Join(opts, ",")))
		err := DoMount(dev, filepath.Join(chrootMountPath, mnt["path"]), opts)
		if err != nil {
			return err
		}

		defer func() { _ = DoUnmount(filepath.Join(chrootMountPath, mnt["path"])) }() //nolint: revive
	}

	// Install incus-agent into the VM.
	err = runScriptInChroot("install-incus-agent.sh")
	if err != nil {
		return err
	}

	// Remove any open-vm-tools packing that might be installed.
	if internalUtil.IsDebianOrDerivative(distro) {
		err := runScriptInChroot("debian-purge-open-vm-tools.sh")
		if err != nil {
			return err
		}
	} else if internalUtil.IsRHELOrDerivative(distro) {
		err := runScriptInChroot("redhat-purge-open-vm-tools.sh")
		if err != nil {
			return err
		}
	} else if internalUtil.IsSUSEOrDerivative(distro) {
		err := runScriptInChroot("suse-purge-open-vm-tools.sh")
		if err != nil {
			return err
		}
	}

	// Add the virtio drivers if needed.
	if internalUtil.IsRHELOrDerivative(distro) || internalUtil.IsSUSEOrDerivative(distro) {
		err := runScriptInChroot("dracut-add-virtio-drivers.sh")
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
	if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
		return err
	}

	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		out, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		// Setup udev rules to create network device aliases.
		err = runScriptInChroot("add-udev-network-rules.sh", string(out))
		if err != nil {
			return err
		}
	}

	// Setup incus-agent service override for older versions of RHEL.
	if internalUtil.IsRHELOrDerivative(distro) && majorVersion <= 7 {
		err := runScriptInChroot("add-incus-agent-override-for-old-systemd.sh")
		if err != nil {
			return err
		}
	}

	if !instance.LegacyBoot {
		err := runScriptInChroot("reinstall-grub-uefi.sh")
		if err != nil {
			return err
		}
	}

	slog.Info("Post-migration configuration complete!")
	return nil
}

func ActivateVG(opts ...string) error {
	if len(opts) > 0 {
		args := make([]string, 0, len(opts)+3)
		args = append(args, "-a", "y", "--config")
		args = append(args, opts...)
		_, err := subprocess.RunCommand("vgchange", args...)
		return err
	}

	_, err := subprocess.RunCommand("vgchange", "-a", "y")
	return err
}

func DeactivateVG() error {
	_, err := subprocess.RunCommand("vgchange", "-a", "n")
	return err
}

func determineRootPartition(looksLikeRootPartition func(partition string, opts []string) bool) (string, string, PartitionType, []string, error) {
	lvs, err := scanVGs()
	if err != nil {
		return "", "", PARTITION_TYPE_UNKNOWN, nil, err
	}

	// If a VG(s) exists, check if any LVs look like the root partition.
	if len(lvs.Report[0].LV) > 0 {
		err := ActivateVG()
		if err != nil {
			return "", "", PARTITION_TYPE_UNKNOWN, nil, err
		}

		defer func() { _ = DeactivateVG() }()

		for _, lv := range lvs.Report[0].LV {
			if looksLikeRootPartition(fmt.Sprintf("/dev/%s/%s", lv.VGName, lv.LVName), nil) {
				rootPath := "/dev/" + lv.VGName + "/" + lv.LVName
				return rootPath, rootPath, PARTITION_TYPE_LVM, nil, nil
			}
		}
	}

	partitions, err := internalUtil.ScanPartitions("")
	if err != nil {
		return "", "", PARTITION_TYPE_UNKNOWN, nil, err
	}

	for _, dev := range partitions.BlockDevices {
		if dev.Serial != "incus_root" {
			continue
		}

		// Loop through any partitions on /dev/sda and check if they look like the root partition.
		for _, p := range dev.Children {
			if p.Name == "" || p.PKName == "" {
				return "", "", PARTITION_TYPE_UNKNOWN, nil, fmt.Errorf("Unable to determine root disk: %+v", dev)
			}

			partition := "/dev/" + p.Name
			parent := "/dev/" + p.PKName

			if p.FSType == "btrfs" {
				btrfsSubvol, err := getBTRFSTopSubvol(partition)
				if err != nil {
					return "", "", PARTITION_TYPE_UNKNOWN, nil, err
				}

				opts := []string{"-o", fmt.Sprintf("subvol=%s", btrfsSubvol)}
				if looksLikeRootPartition(partition, opts) {
					return parent, partition, PARTITION_TYPE_PLAIN, opts, nil
				}
			} else if looksLikeRootPartition(partition, nil) {
				return parent, partition, PARTITION_TYPE_PLAIN, nil, nil
			}
		}
	}

	return "", "", PARTITION_TYPE_UNKNOWN, nil, fmt.Errorf("Failed to determine the root partition")
}

func runScriptInChroot(scriptName string, args ...string) error {
	// Get the embedded script's contents.
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return err
	}

	// Write script to tmp file.
	err = os.WriteFile(filepath.Join(chrootMountPath, scriptName), script, 0o755)
	if err != nil {
		return err
	}

	defer func() { _ = os.Remove(filepath.Join(chrootMountPath, scriptName)) }()

	// Run the script within the chroot.
	cmd := make([]string, 0, len(args)+2)
	cmd = append(cmd, chrootMountPath, filepath.Join("/", scriptName))
	cmd = append(cmd, args...)
	_, err = subprocess.RunCommand("chroot", cmd...)
	return err
}

func scanVGs() (LVSOutput, error) {
	ret := LVSOutput{}
	output, err := subprocess.RunCommand("lvs", "-o", "vg_name,lv_name", "--reportformat", "json")
	if err != nil {
		return ret, err
	}

	err = json.Unmarshal([]byte(output), &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func scanPVs() (LVSOutput, error) {
	ret := LVSOutput{}
	output, err := subprocess.RunCommand("pvs", "-o", "vg_name,pv_name", "--reportformat", "json")
	if err != nil {
		return ret, err
	}

	err = json.Unmarshal([]byte(output), &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func looksLikeLinuxRootPartition(partition string, opts []string) bool {
	// Mount the potential root partition.
	err := DoMount(partition, chrootMountPath, opts)
	if err != nil {
		return false
	}

	defer func() { _ = DoUnmount(chrootMountPath) }()

	// If /usr/ and /etc/ exist, this is probably the root partition.
	return util.PathExists(filepath.Join(chrootMountPath, "usr")) && util.PathExists(filepath.Join(chrootMountPath, "etc"))
}

func getAdditionalMounts() []map[string]string {
	ret := []map[string]string{}

	fstab, err := os.Open(filepath.Join(chrootMountPath, "etc/fstab"))
	if err != nil {
		return ret
	}

	defer func() { _ = fstab.Close() }()

	sc := bufio.NewScanner(fstab)
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())

		if len(text) > 0 && !strings.HasPrefix(text, "#") {
			fields := regexp.MustCompile(`\s+`).Split(text, -1)
			if strings.HasPrefix(fields[1], "/boot") || strings.HasPrefix(fields[1], "/var") || strings.HasPrefix(fields[1], "/usr") {
				ret = append(ret, map[string]string{"device": fields[0], "path": fields[1], "options": fields[3]})
			}
		}
	}

	return ret
}

func getBTRFSTopSubvol(partition string) (string, error) {
	// Mount the partition so we can get the list of subvolumes.
	err := DoMount(partition, chrootMountPath, nil)
	if err != nil {
		return "", err
	}

	defer func() { _ = DoUnmount(chrootMountPath) }()

	// Get the subvolumes.
	output, err := subprocess.RunCommand("btrfs", "subvolume", "list", chrootMountPath)
	if err != nil {
		return "", err
	}

	// Get the top level subvolume.
	submatch := regexp.MustCompile(` top level 5 path (.+)`).FindStringSubmatch(output)
	if submatch != nil {
		return submatch[1], nil
	}

	return "", fmt.Errorf("Unable to determine top level subvolume for partition %s", partition)
}

// getRequiredMounts returns a list of disks that must be mounted according to /etc/fstab on the root partition.
func getRequiredMounts(partFunc func(string, []string) bool) (map[string]mountInfo, error) {
	lvs, err := scanVGs()
	if err != nil {
		return nil, err
	}

	// Determine the root partition.
	_, rootPartition, rootPartitionType, rootMountOpts, err := determineRootPartition(partFunc)
	if err != nil {
		return nil, err
	}

	if len(lvs.Report[0].LV) > 0 {
		err := ActivateVG()
		if err != nil {
			return nil, err
		}

		defer func() { _ = DeactivateVG() }()
	}

	lsblk, err := internalUtil.ScanPartitions("")
	if err != nil {
		return nil, err
	}

	parent, _, err := lsblk.FindDisk(rootPartition)
	if err != nil {
		return nil, err
	}

	plan := map[string]mountInfo{rootPartition: {
		Parent:  parent,
		Path:    rootPartition,
		Type:    rootPartitionType,
		Options: rootMountOpts,
		Root:    true,
	}}

	// Mount everything here as read-only as we are just inspecting /etc/fstab.
	readOnly := []string{"-o", "ro"}
	if len(rootMountOpts) >= 2 && rootMountOpts[0] == "-o" {
		readOnly[1] += "," + strings.Join(rootMountOpts[1:], ",")
	}

	// Mount the migrated root partition.
	err = DoMount(rootPartition, chrootMountPath, readOnly)
	if err != nil {
		return nil, err
	}

	defer func() { _ = DoUnmount(chrootMountPath) }()

	for _, mnt := range getAdditionalMounts() {
		dev, after, _ := strings.Cut(mnt["device"], "=")
		if after != "" {
			dev = after
		}

		parent, path, err := lsblk.FindDisk(dev)
		if err != nil {
			return nil, fmt.Errorf("Unknown disk %q", mnt["device"])
		}

		partType := PARTITION_TYPE_PLAIN
		for _, lv := range lvs.Report[0].LV {
			if path == "/dev/"+lv.VGName+"/"+lv.LVName {
				partType = PARTITION_TYPE_LVM
			}
		}

		next := mountInfo{Parent: parent, Path: path, Type: partType}

		if mnt["options"] != "" {
			next.Options = []string{"-o", mnt["options"]}
		}

		plan[mnt["device"]] = next
	}

	return plan, nil
}
