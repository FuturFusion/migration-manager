package worker

import(
	"embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus/v6/shared/logger"
	"github.com/lxc/incus/v6/shared/subprocess"
)

//go:embed scripts/*
var embeddedScripts embed.FS

const chrootMountPath string = "/mnt/target/"

func LinuxDoPostMigrationConfig(distro string, rootPartition string) error {
	logger.Info("Preparing to perform post-migration configuration of VM")

	// Mount the migrated root partition.
	err := DoMount(rootPartition, chrootMountPath, nil)
	if err != nil {
		return err
	}
	defer func() { _ = DoUnmount(chrootMountPath) }()

	// Bind-mount /proc/ and /sys/ into the chroot.
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

	// Install incus-agent into the VM.
	err = runScriptInChroot("install-incus-agent.sh")
	if err != nil {
		return err
	}

	// Perform distro-specific post-migration steps.
	if strings.ToLower(distro) == "debian" || strings.ToLower(distro) == "ubuntu" {
		err := runScriptInChroot("debian-purge-open-vm-tools.sh")
		if err != nil {
			return err
		}	
	}

	logger.Info("Post-migration configuration complete!")
	return nil
}

func runScriptInChroot(scriptName string) error {
	// Get the embedded script's contents.
	script, err := embeddedScripts.ReadFile(filepath.Join("scripts/", scriptName))
	if err != nil {
		return err
	}

	// Write script to tmp file.
	tmpFileName := filepath.Join("tmp", scriptName)
	err = os.WriteFile(filepath.Join(chrootMountPath, tmpFileName), script, 0755)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(filepath.Join(chrootMountPath, tmpFileName)) }()

	// Run the script within the chroot.
	_, err = subprocess.RunCommand("chroot", chrootMountPath, tmpFileName)
	return err
}
