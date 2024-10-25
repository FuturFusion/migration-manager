package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/ask"
	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/cmd/common"
	internalUtil "github.com/FuturFusion/migration-manager/util"
	"github.com/FuturFusion/migration-manager/util/incus"
	migratekitVmware "github.com/FuturFusion/migration-manager/util/migratekit/vmware"
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
	networkMapping    string
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
	cmd.Flags().StringVar(&c.bootableISOPool, "bootable-iso-pool", "iscsi", "Incus storage pool for the bootable migration ISO image")
	cmd.Flags().StringVar(&c.bootableISOSource, "bootable-iso-source", "migration-manager-minimal-boot.iso", "Incus source for the bootable migration ISO image")
	cmd.Flags().StringVar(&c.excludeVmRegex, "exclude-vm-regex", "", "Regular expression to specify which VMs to exclude from import")
	cmd.Flags().StringVar(&c.includeVmRegex, "include-vm-regex", "", "Regular expression to specify which VMs to import")
	cmd.Flags().StringVar(&c.networkMapping, "network-mapping", "", "Comma separated list of vmware:incus network mappings")

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
	incusClient, err := incus.NewIncusClient(ctx, c.IncusRemoteName, c.IncusProject, c.IncusProfile, c.bootableISOPool, c.bootableISOSource)
	if err != nil {
		return err
	}

	/*
		Network mapping from VMware to Incus.
	*/

	// Get a list of all VMware networks.
	vmwareNetworks, err := vmwareClient.GetNetworks()
	if err != nil {
		return err
	}

	// Get a list of all Incus networks.
	incusNetworks, err := incusClient.GetNetworkNames()
	if err != nil {
		return err
	}

	networkMapping := make(map[string]string)

	if c.networkMapping != "" {
		for _, split := range strings.Split(c.networkMapping, ",") {
			networks := strings.Split(split, ":")
			if len(networks) != 2 {
				continue
			}

			if !slices.ContainsFunc(vmwareNetworks, func(n object.NetworkReference) bool { return n.Reference().Value == networks[0] }) {
				fmt.Printf("WARNING: '%s' is not a VMware network, skipping provided mapping.\n", networks[0])
				continue
			}

			if !slices.Contains(incusNetworks, networks[1]) {
				fmt.Printf("WARNING: '%s' is not an Incus network, skipping provided mapping.\n", networks[1])
				continue
			}

			networkMapping[networks[0]] = networks[1]
		}
	} else {
		fmt.Printf("The following networks exist in Incus:\n")
		for _, network := range incusNetworks {
			fmt.Printf("  %s\n", network)
		}

		fmt.Printf("Please specify a mapping (if any) for existing VMware networks:\n")
		for _, network := range vmwareNetworks {
			fmt.Printf("  VMware Network '%s'...\n", network)

			selectedNetwork, err := asker.AskString("    Which Incus network should this be mapped to (empty to ignore)? ", "", func(answer string) error {
				if answer == "" || slices.Contains(incusNetworks, answer) {
					return nil
				}

				return fmt.Errorf("Please enter a valid Incus network name")
			})

			if err != nil {
				fmt.Printf("Got an error, moving to next network: %q", err)
				continue
			}

			if selectedNetwork != "" {
				networkMapping[network.Reference().Value] = selectedNetwork
			}
		}
	}

	fmt.Printf("VMware -> Incus network mapping(s):\n")
	for k, v := range networkMapping {
		fmt.Printf("  %s -> %s\n", k, v)
	}
	fmt.Printf("\n\n")

	/*
		Import VMware VMs into Incus.
	*/

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
			fmt.Printf("  WARNING: Unable to get VM properties: %q\n\n\n", err)
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

		// Check if CBT is enabled for VM disk(s).
		for _, disk := range vmwareClient.GetVMDisks(vm) {
			_, err := migratekitVmware.GetChangeID(disk)
			if err != nil {
				b, ok := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
				if ok {
					fmt.Printf("  WARNING: Changed Block Tracking not enabled for disk %q; will only be able to perform full-disk migration\n", b.GetVirtualDeviceFileBackingInfo().FileName)
				} else {
					return fmt.Errorf("Changed Block Tracking not enabled for disk, and unable to determine the disk name?")
				}
			}
		}

		// If this appears to be a Windows VM, ask if BitLocker is enabled.
		bitlockerRecoveryKey := ""
		if strings.Contains(p.Summary.Config.GuestId, "windows") {
			bitlockerEnabled, err := asker.AskBool("Does this VM have BitLocker encryption enabled? [default=no]: ", "no")
			if err != nil {
				fmt.Printf("Got an error, moving to next VM: %q", err)
				continue
			}

			if bitlockerEnabled {
				bitlockerRecoveryKey, err = asker.AskString("Please enter the BitLocker recovery key for this VM: ", "", nil)
				if err != nil {
					fmt.Printf("Got an error, moving to next VM: %q", err)
					continue
				}
			}
		}

		incusInstanceArgs := internalUtil.ConvertVMwareMetadataToIncus(p)

		disks := vmware.GetVMDiskInfo(p)
		nics := vmware.GetVMNetworkInfo(p, networkMapping)

		fmt.Printf("  UUID: %s\n", p.Summary.Config.InstanceUuid)
		fmt.Printf("  Memory: %d MB\n", p.Summary.Config.MemorySizeMB)
		fmt.Printf("  CPU: %d\n", p.Summary.Config.NumCpu)
		if bitlockerRecoveryKey != "" {
			fmt.Printf("  BitLocker recovery key: %s\n", bitlockerRecoveryKey)
		}

		err = incusClient.CreateInstance(incusInstanceArgs, disks, nics)
		if err != nil {
			fmt.Printf("  FAILED to import VM metadata into Incus: %q\n", err)
		}

		fmt.Printf("\n\n")
	}

	return nil
}
