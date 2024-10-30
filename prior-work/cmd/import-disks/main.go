package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/common"
	"github.com/FuturFusion/migration-manager/util/vmware"
)

type appFlags struct {
	common.CmdGlobalFlags
	common.CmdVMwareFlags

	vmName  string
}

func main() {
	appCmd := appFlags{}
	app := appCmd.Command()
	app.SilenceUsage = true
	app.CompletionOptions = cobra.CompletionOptions{DisableDefaultCmd: true}

	// Workaround for main command
	app.Args = cobra.ArbitraryArgs

	// Version handling
	app.SetVersionTemplate("{{.Version}}\n")
	app.Version = common.Version

	// Run the main command and handle errors
	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func (c *appFlags) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "import-disks"
	cmd.Short = "Import VMware VM disks into an Incus VM"
	cmd.Long = `Description:
  Import VMware VM disks into an Incus VM

  This tool directly imports VM disks from VMware into an Incus VM. Prior to
  using this tool, you must first run the `+"`import-vm-metadata`"+` utility
  to populate VM metadata in Incus.

  By default, the state of VMs running in VMware will be unchanged. A
  snapshot of the disk(s) will be created, then transferred via this tool.
  If possible, incremental copies are used to speed up the process.

  In the current implementation, tracking of snapshot state will be lost if
  the VM is restarted, resulting in a full disk copy being require when the
  VM is powered on once more.
`

	cmd.RunE = c.Run

	c.CmdGlobalFlags.AddFlags(cmd)
	c.CmdVMwareFlags.AddFlags(cmd)
	cmd.Flags().StringVar(&c.vmName, "vm-name", "", "VM name from which to import disk (required)")
	cmd.MarkFlagRequired("vm-name")

	return cmd
}

func (c *appFlags) Run(cmd *cobra.Command, args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("This tool must be run as root")
	}

	if !util.PathExists("/dev/virtio-ports/org.linuxcontainers.incus") {
		return fmt.Errorf("This tool is designed to be run within an Incus VM")
	}

	ctx := context.TODO()

	// Connect to VMware endpoint.
	vmwareClient, err := vmware.NewVMwareClient(ctx, c.VmwareEndpoint, c.VmwareInsecure, c.VmwareUsername, c.VmwarePassword)
	if err != nil {
		return err
	}

	// Get a list of all VMs.
	vmwareVms, err := vmwareClient.GetVMs()
	if err != nil {
		return err
	}

	for _, vm := range vmwareVms {
		if vm.Name() != c.vmName {
			continue
		}

		fmt.Printf("Exporting disks attached to VM %q...\n", vm.Name())

		err := vmwareClient.DeleteVMSnapshot(vm, "incusMigration")
		if err != nil {
			return err
		}

		err = vmwareClient.ExportDisks(vm)
		if err != nil {
			return err
		}
	}

	return nil
}
