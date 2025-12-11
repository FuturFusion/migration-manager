package cmds

import (
	"net/http"

	"github.com/lxc/incus-os/incus-osd/cli"
	"github.com/spf13/cobra"
)

type CmdAdmin struct {
	Global *CmdGlobal
}

func (c *CmdAdmin) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "admin"
	cmd.Short = "Manage IncusOS"
	cmd.Long = `Description:
  Manage IncusOS
`

	// os
	adminOSCmd := cmdAdminOS{global: c.Global}

	cmd.AddCommand(adminOSCmd.Command())

	return cmd
}

type cmdAdminOS struct {
	global *CmdGlobal
}

func (c *cmdAdminOS) Command() *cobra.Command {
	args := &cli.Args{
		SupportsTarget:    false,
		SupportsRemote:    false,
		DefaultListFormat: "table",
		DoHTTP: func(_ string, req *http.Request) (*http.Response, error) {
			client, url, err := c.global.buildClient(c.global.GetDefaultRemote().Addr + req.URL.String())
			if err != nil {
				return nil, err
			}

			req.URL = url
			return c.global.requestFunc(client)(req)
		},
	}

	cmd := cli.NewCommand(args)
	preFunc := cmd.PersistentPreRun
	preFuncErr := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if preFuncErr != nil {
			err := preFuncErr(cmd, args)
			if err != nil {
				return err
			}
		} else if preFunc != nil {
			preFunc(cmd, args)
		}

		return c.global.PreRun(cmd, args)
	}

	return cmd
}
