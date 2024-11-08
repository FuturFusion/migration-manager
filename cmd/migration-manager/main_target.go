package main

import (
	"github.com/spf13/cobra"
)

type cmdTarget struct {
	global *cmdGlobal
}

func (c *cmdTarget) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "target"
	cmd.Short = "Interact with migration targets"
	cmd.Long = `Description:
  Interact with migration targets

  Configure migration targets for use by the migration manager.
`
	cmd.RunE = c.Run

	return cmd
}

func (c *cmdTarget) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}
	err = c.global.CheckConfigStatus()
	if err != nil {
		return err
	}

	return nil
}
