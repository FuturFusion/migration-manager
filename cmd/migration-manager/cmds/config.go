package cmds

import (
	"github.com/spf13/cobra"
)

type CmdConfig struct {
	Global *CmdGlobal
}

func (c *CmdConfig) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "config"
	cmd.Short = "Show/update server config"
	cmd.Long = `Description:
  Show/update server config
`

	// Trust
	configTrustCmd := cmdConfigTrust{global: c.Global}
	cmd.AddCommand(configTrustCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}
