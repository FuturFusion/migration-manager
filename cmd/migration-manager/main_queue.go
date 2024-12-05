package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type cmdQueue struct {
	global *cmdGlobal
}

func (c *cmdQueue) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "queue"
	cmd.Short = "Display the migration queue"
	cmd.Long = `Description:
  Display the migration queue

  Displays a read-only view of the migration queue.
`

	// List
	queueListCmd := cmdQueueList{global: c.global}
	cmd.AddCommand(queueListCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// List the queues.
type cmdQueueList struct {
	global *cmdGlobal

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
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", "Format (csv|json|table|yaml|compact)")
	cmd.Flags().BoolVarP(&c.flagVerbose, "verbose", "", false, "Enable verbose output")

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

	metadata, ok := resp.Metadata.([]any)
	if !ok {
		return errors.New("Unexpected API response, invalid type for metadata")
	}

	// Loop through returned entries.
	for _, anyEntry := range metadata {
		newEntry, err := parseReturnedQueueEntry(anyEntry)
		if err != nil {
			return err
		}

		queueEntries = append(queueEntries, newEntry.(api.QueueEntry))
	}

	// Render the table.
	header := []string{"Name", "Batch", "Status", "Status String"}
	if c.flagVerbose {
		header = append(header, "UUID")
	}

	data := [][]string{}

	for _, q := range queueEntries {
		row := []string{q.InstanceName, q.BatchName, q.MigrationStatus.String(), q.MigrationStatusString}
		if c.flagVerbose {
			row = append(row, q.InstanceUUID.String())
		}

		data = append(data, row)
	}

	return util.RenderTable(c.flagFormat, header, data, queueEntries)
}

func parseReturnedQueueEntry(i any) (any, error) {
	reJsonified, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	ret := api.QueueEntry{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
