package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"

	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/common"
	internalUtil "github.com/FuturFusion/migration-manager/util"
	"github.com/FuturFusion/migration-manager/util/ask"
	"github.com/FuturFusion/migration-manager/util/incus"
	"github.com/FuturFusion/migration-manager/util/vmware"
)

type appFlags struct {
	common.CmdGlobalFlags
	common.CmdIncusFlags
	common.CmdVMwareFlags

	autoImport        bool
	bootableISOPool   string
	bootableISOSource string
	excludeVmRegex    string
	includeVmRegex    string
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

  You may optionally specify regular expressions to include or exclude
  VMs from the import process, based on the VM's inventory path.

  By default confirmation will be asked before importing each VM, as well
  as before deleting an existing Incus VM that conflicts with one that is
  to be imported from VMware. A command line option exists to automate
  this, but use with caution as it may cause destructive actions by
  deleting Incus VMs automatically.
`

	cmd.RunE = c.Run

	c.CmdGlobalFlags.AddFlags(cmd)
	c.CmdIncusFlags.AddFlags(cmd)
	c.CmdVMwareFlags.AddFlags(cmd)
	cmd.Flags().BoolVar(&c.autoImport, "auto-import", false, "Automatically import VMs; may automatically DELETE existing Incus VMs")
	cmd.Flags().StringVar(&c.excludeVmRegex, "exclude-vm-regex", "", "Regular expression to specify which VMs to exclude from import")
	cmd.Flags().StringVar(&c.includeVmRegex, "include-vm-regex", "", "Regular expression to specify which VMs to import")
	cmd.Flags().StringVar(&c.bootableISOPool, "bootable-iso-pool", "iscsi", "Incus storage pool for the bootable migration ISO image")
	cmd.Flags().StringVar(&c.bootableISOSource, "bootable-iso-source", "migration-manager-minimal-boot.iso", "Incus source for the bootable migration ISO image")

	return cmd
}

func (c *appFlags) Run(cmd *cobra.Command, args []string) error {
	asker := ask.NewAsker(bufio.NewReader(os.Stdin))

	ctx := context.TODO()

	// Connect to VMware endpoint.
	vmwareClient, err := vmware.NewVMwareClient(ctx, c.VmwareEndpoint, c.VmwareInsecure, c.VmwareUsername, c.VmwarePassword)
	if err != nil {
		return err
	}

	// Connect to Incus.
	incusClient, err := incus.NewIncusClient(ctx, c.IncusRemoteName, c.bootableISOPool, c.bootableISOSource)
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
	vmwareVms, err := vmwareClient.GetVMs()
	if err != nil {
		return err
	}

	// Get a list of all Incus VMs.
	incusVms, err := incusClient.GetVMNames()
	if err != nil {
		return err
	}

	excludeRegex := regexp.MustCompile(c.excludeVmRegex)
	includeRegex := regexp.MustCompile(c.includeVmRegex)
	for _, vm := range vmwareVms {
		if c.excludeVmRegex != "" && excludeRegex.Match([]byte(vm.InventoryPath)) {
			fmt.Printf("VMware VM '%s' skipped by exclusion pattern.\n\n\n", vm)
			continue
		}

		if c.includeVmRegex != "" && !includeRegex.Match([]byte(vm.InventoryPath)) {
			fmt.Printf("VMware VM '%s' skipped by inclusion pattern.\n\n\n", vm)
			continue
		}

		fmt.Printf("Inspecting VMware VM '%s'...\n", vm)

		p, err := vmwareClient.GetVMProperties(vm)
		if err != nil {
			fmt.Printf("  WARNING -- Unable to get VM properties: %q\n\n\n", err)
			continue
		}

		if slices.Contains(incusVms, vm.Name()) {
			if !c.autoImport {
				ok, err := asker.AskBool("VM '" + vm.Name() + "' already exists in Incus. Delete and re-create? [default=yes]: ", "yes")
				if err != nil {
					fmt.Printf("Got an error, moving to next VM: %q", err)
					continue
				}

				if !ok {
					continue
				}

				err = incusClient.DeleteVM(vm.Name())
				if err != nil {
					fmt.Printf("Error deleting existing VM '%s': %q", vm.Name(), err)
				}
			}
		}

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

		disks := vmware.GetVMDiskInfo(p)
		nics := vmware.GetVMNetworkInfo(p)

		fmt.Printf("  UUID: %s\n", p.Summary.Config.InstanceUuid)
		fmt.Printf("  Memory: %d MB\n", p.Summary.Config.MemorySizeMB)
		fmt.Printf("  CPU: %d\n", p.Summary.Config.NumCpu)

		err = incusClient.CreateInstance(incusInstanceArgs, disks, nics)
		if err != nil {
			fmt.Printf("  FAILED to import VM metadata into Incus: %q\n", err)
		}

		fmt.Printf("\n\n")
	}

	return nil
}
