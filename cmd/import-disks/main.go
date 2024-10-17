package main

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/object"

	"github.com/FuturFusion/migration-manager/cmd/common"
	"github.com/FuturFusion/migration-manager/util/vmware"

	"github.com/FuturFusion/migration-manager/util/migratekit/nbdkit"
	vmwareInternal "github.com/FuturFusion/migration-manager/util/migratekit/vmware"
	"github.com/FuturFusion/migration-manager/util/migratekit/vmware_nbdkit"
)

type appFlags struct {
	common.CmdGlobalFlags
	common.CmdVMwareFlags
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

  This tool imports VM disks from VMware into Incus. It supports importing
  incremental disk changes if Changed Block Tracking is enabled.
`

	cmd.RunE = c.Run

	c.CmdGlobalFlags.AddFlags(cmd)
	c.CmdVMwareFlags.AddFlags(cmd)

	return cmd
}

func (c *appFlags) Run(cmd *cobra.Command, args []string) error {
	ctx := context.TODO()

	// Connect to VMware endpoint
	vmwareClient, err := vmware.NewVMwareClient(ctx, c.VmwareEndpoint, c.VmwareInsecure, c.VmwareUsername, c.VmwarePassword)
	if err != nil {
		return err
	}

	endpointUrl := &url.URL{
		Scheme: "https",
		Host:   c.VmwareEndpoint,
		User:   url.UserPassword(c.VmwareUsername, c.VmwarePassword),
		Path:   "sdk",
	}

	thumbprint, err := vmwareInternal.GetEndpointThumbprint(endpointUrl)
	if err != nil {
		return err
	}

	vddkConfig := &vmware_nbdkit.VddkConfig {
		Debug:       false,
		Endpoint:    endpointUrl,
		Thumbprint:  thumbprint,
		Compression: nbdkit.CompressionMethod("none"),
	}


	// Get a list of all VMs
	vms, err := vmwareClient.GetVMs()
	if err != nil {
		return err
	}

	for _, vm := range vms {
		fmt.Printf("Importing disks attached to VM %q...\n", vm.Name())

		err := vmwareClient.DeleteVMSnapshot(vm, "incusMigration")
		if err != nil {
			return err
		}

		err = importDisks(ctx, vm, vddkConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

// Needs to be here, since it depends on internal migratekit code
func importDisks(ctx context.Context, vm *object.VirtualMachine, vddkConfig *vmware_nbdkit.VddkConfig) error {
	servers := vmware_nbdkit.NewNbdkitServers(vddkConfig, vm)
	err := servers.MigrationCycle(ctx, false)
	if err != nil {
		return err
	}

	return nil
}
