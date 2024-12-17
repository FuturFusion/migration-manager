package cmds

import (
	"net/http"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdQueue struct {
	Global *CmdGlobal
}

func (c *CmdQueue) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "queue"
	cmd.Short = "Display the migration queue"
	cmd.Long = `Description:
  Display the migration queue

  Displays a read-only view of the migration queue.
`

	// List
	queueListCmd := cmdQueueList{global: c.Global}
	cmd.AddCommand(queueListCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// List the queues.
type cmdQueueList struct {
	global *CmdGlobal

	flagFormat  string
	flagVerbose bool
}

func (c *cmdQueueList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Short = "List the migration queue"
	cmd.Long = `Description:
  List the migration queue
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)
	cmd.Flags().BoolVarP(&c.flagVerbose, "verbose", "", false, "Enable verbose output")
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateFlagFormat(cmd.Flag("format").Value.String())
	}

	return cmd
}

func (c *cmdQueueList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the current migration queue.
	resp, err := c.global.doHTTPRequestV1("/queue", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	queueEntries := []api.QueueEntry{}

	err = responseToStruct(resp, &queueEntries)
	if err != nil {
		return err
	}

	// Render the table.
	header := []string{"Name", "Batch", "Status", "Status String"}
	if c.flagVerbose {
		header = append(header, "UUID")
	}

	data := [][]string{}

	for _, q := range queueEntries {
		if q.MigrationStatusString == "" {
			q.MigrationStatusString = q.MigrationStatus.String()
		}

		row := []string{q.InstanceName, q.BatchName, q.MigrationStatus.String(), q.MigrationStatusString}
		if c.flagVerbose {
			row = append(row, q.InstanceUUID.String())
		}

		data = append(data, row)
	}

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, queueEntries)
}
