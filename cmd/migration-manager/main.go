package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/lxc/incus/v6/shared/ask"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/version"
)

type cmdGlobal struct {
	asker ask.Asker

	cmd      *cobra.Command
	ret      int

	flagHelp       bool
	flagVersion    bool
}

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
	globalCmd := cmdGlobal{cmd: app, asker: ask.NewAsker(bufio.NewReader(os.Stdin))}

	app.PersistentFlags().BoolVar(&globalCmd.flagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&globalCmd.flagHelp, "help", "h", false, "Print help")

	// Version handling
	app.SetVersionTemplate("{{.Version}}\n")
	app.Version = version.Version

	// source sub-command
	sourceCmd := cmdSource{global: &globalCmd}
	app.AddCommand(sourceCmd.Command())

	// target sub-command
	targetCmd := cmdTarget{global: &globalCmd}
	app.AddCommand(targetCmd.Command())

	// Run the main command and handle errors
	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func (c *cmdGlobal) CheckArgs(cmd *cobra.Command, args []string, minArgs int, maxArgs int) (bool, error) {
	if len(args) < minArgs || (maxArgs != -1 && len(args) > maxArgs) {
		_ = cmd.Help()

		if len(args) == 0 {
			return true, nil
		}

		return true, fmt.Errorf("Invalid number of arguments")
	}

	return false, nil
}
