package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/common"
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
	return nil
}
