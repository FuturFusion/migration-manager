package cmds

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdWarning struct {
	Global *CmdGlobal
}

func (c *CmdWarning) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "warning"
	cmd.Short = "Manage warnings"
	cmd.Long = `Description:

	View and acknowledge warnings
`

	// List
	configListCmd := cmdWarningList{global: c.Global}
	cmd.AddCommand(configListCmd.Command())

	// Show
	configShowCmd := cmdWarningShow{global: c.Global}
	cmd.AddCommand(configShowCmd.Command())

	// Acknowledge
	configAck := cmdWarningAck{global: c.Global}
	cmd.AddCommand(configAck.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

type cmdWarningList struct {
	global     *CmdGlobal
	flagFormat string
}

func (c *cmdWarningList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Aliases = []string{"ls"}
	cmd.Short = "List warnings"
	cmd.Long = `Description:
  List all warnings.
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateFlagFormat(cmd.Flag("format").Value.String())
	}

	return cmd
}

func (c *cmdWarningList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	resp, _, err := c.global.doHTTPRequestV1("/warnings", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	warnings := []api.Warning{}

	err = responseToStruct(resp, &warnings)
	if err != nil {
		return err
	}

	// Render the table.
	header := []string{"UUID", "Status", "Scope", "Entity Type", "Entity", "Type", "Last Updated", "Num Messages", "Count"}
	data := [][]string{}

	for _, w := range warnings {
		data = append(data, []string{w.UUID.String(), string(w.Status), w.Scope.Scope, w.Scope.EntityType, w.Scope.Entity, string(w.Type), w.UpdatedDate.String(), strconv.Itoa(len(w.Messages)), strconv.Itoa(w.Count)})
	}

	sort.Sort(util.SortColumnsNaturally(data))

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, warnings)
}

type cmdWarningAck struct {
	global *CmdGlobal
}

func (c *cmdWarningAck) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "acknowledge <uuid>"
	cmd.Aliases = []string{"ack"}
	cmd.Short = "Acknowledge warning"
	cmd.Long = `Description:
  Acknowledge the warning.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdWarningAck) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	warningUUID := args[0]

	b, err := json.Marshal(api.WarningPut{Status: api.WARNINGSTATUS_ACKNOWLEDGED})
	if err != nil {
		return err
	}

	_, _, err = c.global.doHTTPRequestV1("/warnings/"+warningUUID, http.MethodPut, "", b)
	if err != nil {
		return err
	}

	return nil
}

type cmdWarningShow struct {
	global *CmdGlobal
}

func (c *cmdWarningShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show <uuid>"
	cmd.Short = "Show warning messages"
	cmd.Long = `Description:
	Show warning messages and data as YAML.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdWarningShow) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	warningUUID := args[0]

	resp, _, err := c.global.doHTTPRequestV1("/warnings/"+warningUUID, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	w := api.Warning{}

	err = responseToStruct(resp, &w)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(w)
	if err != nil {
		return err
	}

	fmt.Println(string(b))

	return nil
}
