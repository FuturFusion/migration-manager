package cmds

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

type CmdQuery struct {
	Global *CmdGlobal

	flagData    string
	flagRequest string
}

func (c *CmdQuery) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "query <API path>"
	cmd.Short = "Send a raw query to the server"
	cmd.Long = `Description:
  Send a raw query to the server
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagData, "data", "d", "", "Input data")
	cmd.Flags().StringVarP(&c.flagRequest, "request", "X", "GET", "Action (defaults to GET)")

	return cmd
}

func (c *CmdQuery) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.Global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	query := args[0]

	// Run the query.
	resp, err := c.Global.makeHTTPRequest(query, c.flagRequest, []byte(c.flagData))
	if err != nil {
		return err
	}

	marshalled, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	cmd.Printf("%s\n", marshalled)

	return nil
}
