package cmds

import (
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

type cmdSourceSync struct {
	global *CmdGlobal
}

func (c *cmdSourceSync) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <name>",
		Short: "Refresh source data",
		RunE:  c.Run,
	}
}

func (c *cmdSourceSync) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	_, _, err = c.global.doHTTPRequestV1("/sources/"+url.PathEscape(args[0])+"/:sync", http.MethodPost, "", nil)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully synced source %q.\n", args[0])
	return nil
}
