package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/common"
	internalUtil "github.com/FuturFusion/migration-manager/util"
	"github.com/FuturFusion/migration-manager/util/incus"
	"github.com/FuturFusion/migration-manager/util/vmware"
)

type appFlags struct {
	common.CmdGlobalFlags
	common.CmdIncusFlags
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
	cmd.Use = "import-vm-metadata"
	cmd.Short = "Import VMware VM metadata into Incus"
	cmd.Long = `Description:
  Import VMware VM metadata into Incus

  This tool imports VM metadata from VMware into Incus. It will setup a
  skeleton VM instance in Incus, copying various configuration from the
  existing VM. You must separately import the backing storage for the VM
  via the `+"`import-disks`"+` command.
`

	cmd.RunE = c.Run

	c.CmdGlobalFlags.AddFlags(cmd)
	c.CmdIncusFlags.AddFlags(cmd)
	c.CmdVMwareFlags.AddFlags(cmd)

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


	// Loop through existing VMware networks.
	networks, err := vmwareClient.GetNetworks()
	if err != nil {
		return err
	}

	for _, network := range networks {
		fmt.Printf("Inspecting VMware Network '%s'...\n", network)
/*
		fmt.Printf("  Summary:\n")
		fmt.Printf("    Network ID: %s\n", network.Summary.GetNetworkSummary().Network.Value)
		fmt.Printf("    Name: %s\n", network.Summary.GetNetworkSummary().Name)
		fmt.Printf("    Accessible: %d\n", network.Summary.GetNetworkSummary().Accessible)
		fmt.Printf("  Hosts: %q\n", network.Host)
		fmt.Printf("  VMs: %q\n", network.Vm)
*/
	}
	fmt.Printf("\n\n\n")


	// Get a list of all VMware VMs.
	vms, err := vmwareClient.GetVMs()
	if err != nil {
		return err
	}

	for _, vm := range vms {
		p, err := vmwareClient.GetVMProperties(vm)
		if err != nil {
			fmt.Printf("  WARNING -- Unable to get VM properties: %q\n", err)
			continue
		}

		fmt.Printf("Inspecting VMware VM '%s'...\n", vm)

		ctkEnabled := false
		if p.Config != nil && p.Config.ExtraConfig != nil {
			for _, v := range p.Config.ExtraConfig {
				if v.GetOptionValue().Key == "ctkEnabled" {
					ctkEnabled = util.IsTrue(v.GetOptionValue().Value.(string))
					break
				}
			}
		}

		if !ctkEnabled {
			fmt.Printf("  WARNING -- VM doesn't have Changed Block Tracking enabled, so we can't perform near-live migration.\n")
		}

		incusInstanceArgs := internalUtil.ConvertVMwareMetadataToIncus(p)

		disks := vmwareClient.GetVMDiskInfo(p)
		nics := vmwareClient.GetVMNetworkInfo(p)

		fmt.Printf("  UUID: %s\n", p.Summary.Config.InstanceUuid)
		fmt.Printf("  Memory: %d MB\n", p.Summary.Config.MemorySizeMB)
		fmt.Printf("  CPU: %d\n", p.Summary.Config.NumCpu)
		fmt.Printf("  Disks: %q\n", disks)
		fmt.Printf("  NICs: %q\n", nics)

		err = incusClient.CreateInstance(incusInstanceArgs, nics)
		if err != nil {
			fmt.Printf("  FAILED to import VM metadata into Incus: %q\n", err)
		}

		fmt.Printf("\n\n")
	}

	return nil
}
