package main

import (
	"github.com/spf13/cobra"
)

type cmdSource struct {
	global *cmdGlobal
}

func (c *cmdSource) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "source"
	cmd.Short = "Interact with migration sources"
	cmd.Long = `Description:
  Interact with migration sources

  Configure migration sources for use by the migration manager.
`
	cmd.RunE = c.Run

	return cmd
}

func (c *cmdSource) Run(cmd *cobra.Command, args []string) error {
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
