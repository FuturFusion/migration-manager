package main

import (
	"bufio"
	"os"

	"github.com/lxc/incus/v6/shared/ask"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/internal/cmds"
	"github.com/FuturFusion/migration-manager/internal/version"
)

func main() {
	// Setup the parser
	app := &cobra.Command{}
	app.Use = "migration-manager"
	app.Short = "Command line client for migration manager"
	app.Long = `Description:
  Command line client for migration manager

  The migration manager can be interacted with through the various commands
  below. For help with any of those, simply call them with --help.
`

	app.SilenceUsage = true
	app.SilenceErrors = true
	app.CompletionOptions = cobra.CompletionOptions{HiddenDefaultCmd: true}

	// Global flags
	asker := ask.NewAsker(bufio.NewReader(os.Stdin))
	globalCmd := cmds.CmdGlobal{Cmd: app, Asker: &asker}

	app.PersistentFlags().BoolVar(&globalCmd.FlagForceLocal, "force-local", false, "Force using the local unix socket")
	app.PersistentFlags().BoolVar(&globalCmd.FlagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&globalCmd.FlagHelp, "help", "h", false, "Print help")

	// Wrappers
	app.PersistentPreRunE = globalCmd.PreRun

	// Version handling
	app.SetVersionTemplate("{{.Version}}\n")
	app.Version = version.Version

	// admin sub-command
	adminCmd := cmds.CmdAdmin{Global: &globalCmd}
	app.AddCommand(adminCmd.Command())

	// artifact sub-command
	artifactCmd := cmds.CmdArtifact{Global: &globalCmd}
	app.AddCommand(artifactCmd.Command())

	// batch sub-command
	batchCmd := cmds.CmdBatch{Global: &globalCmd}
	app.AddCommand(batchCmd.Command())

	configCmd := cmds.CmdConfig{Global: &globalCmd}
	app.AddCommand(configCmd.Command())

	// instance sub-command
	instanceCmd := cmds.CmdInstance{Global: &globalCmd}
	app.AddCommand(instanceCmd.Command())

	// network sub-command
	networkCmd := cmds.CmdNetwork{Global: &globalCmd}
	app.AddCommand(networkCmd.Command())

	// query sub-command
	queryCmd := cmds.CmdQuery{Global: &globalCmd}
	queryCmdHidden := queryCmd.Command()
	queryCmdHidden.Hidden = true // Don't show query command by default.
	app.AddCommand(queryCmdHidden)

	// queue sub-command
	queueCmd := cmds.CmdQueue{Global: &globalCmd}
	app.AddCommand(queueCmd.Command())

	// remote sub-command
	remoteCmd := cmds.CmdRemote{Global: &globalCmd}
	app.AddCommand(remoteCmd.Command())

	// source sub-command
	sourceCmd := cmds.CmdSource{Global: &globalCmd}
	app.AddCommand(sourceCmd.Command())

	// target sub-command
	targetCmd := cmds.CmdTarget{Global: &globalCmd}
	app.AddCommand(targetCmd.Command())

	// Run the main command and handle errors
	err := app.Execute()
	if err != nil {
		app.Printf("%s\n", err)
		os.Exit(1)
	}
}
