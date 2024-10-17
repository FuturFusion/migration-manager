package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/common"
	"github.com/FuturFusion/migration-manager/util/incus"
	"github.com/FuturFusion/migration-manager/util/vmware"
)

type appFlags struct {
	common.CmdGlobalFlags
	common.CmdIncusFlags
	common.CmdVMwareFlags

	cutover bool
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
	cmd.Short = "Import VMware VM disks into Incus"
	cmd.Long = `Description:
  Import VMware VM disks into Incus

  This tool imports VM disks from VMware into Incus. Prior to importing any
  disks, you must first run the `+"`import-vm-metadata`"+` utility to
  populate VM metadata in Incus.

  By default, the state of VMs running in VMware will be unchanged. A
  snapshot of the disk(s) will be created, then transferred via this tool.
  If possible, incremental copies are used to speed up the process.

  When you are ready to cut over to Incus, pass the `+"`--cutover`"+` flag
  which will shutdown the VMware VMs, do a final data transfer and then
  start the VMs in Incus.
`

	cmd.RunE = c.Run

	c.CmdGlobalFlags.AddFlags(cmd)
	c.CmdIncusFlags.AddFlags(cmd)
	c.CmdVMwareFlags.AddFlags(cmd)
	cmd.Flags().BoolVar(&c.cutover, "cutover", false, "Shutdown VMware VMs, perform a final data transfer and start Incus VMs")

	return cmd
}

func (c *appFlags) Run(cmd *cobra.Command, args []string) error {
	ctx := context.TODO()

	// Connect to VMware endpoint.
	vmwareClient, err := vmware.NewVMwareClient(ctx, c.VmwareEndpoint, c.VmwareInsecure, c.VmwareUsername, c.VmwarePassword)
	if err != nil {
		return err
	}

	// Connect to Incus.
	incusClient, err := incus.NewIncusClient(ctx, c.IncusRemoteName)
	if err != nil {
		return err
	}

	// Get a list of all VMs.
	vmwareVms, err := vmwareClient.GetVMs()
	if err != nil {
		return err
	}

	// Get a list of all Incus VMs.
	incusVms, err := incusClient.GetVMNames()
	if err != nil {
		return err
	}

	if c.cutover {
		fmt.Printf("cut over not currently supported!\n")
		return nil
	}

	for _, vm := range vmwareVms {
		if !slices.Contains(incusVms, vm.Name()) {
			fmt.Printf("VMware VM '%s' hasn't yet been imported into Incus; skipping disk migration.\n", vm.Name())
			continue
		}

		fmt.Printf("Importing disks attached to VM %q...\n", vm.Name())

		err := vmwareClient.DeleteVMSnapshot(vm, "incusMigration")
		if err != nil {
			return err
		}

		err = vmwareClient.ExportDisks(vm)
		if err != nil {
			fmt.Printf("  ERROR: Failed to import disk(s): %q\n", err)
		}

		// TODO sync freshly imported disk images into Incus
	}

	return nil
}
