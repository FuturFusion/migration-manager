package cmds

import (
	"fmt"
	"net/http"
	"sort"
	"time"

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

	// Delete
	queueRemoveCmd := cmdQueueRemove{global: c.Global}
	cmd.AddCommand(queueRemoveCmd.Command())

	// Cancel
	queueCancelCmd := cmdQueueCancel{global: c.Global}
	cmd.AddCommand(queueCancelCmd.Command())

	// Retry
	queueRetryCmd := cmdQueueRetry{global: c.Global}
	cmd.AddCommand(queueRetryCmd.Command())

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
	resp, _, err := c.global.doHTTPRequestV1("/queue", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return err
	}

	queueEntries := []api.QueueEntry{}

	err = responseToStruct(resp, &queueEntries)
	if err != nil {
		return err
	}

	// Render the table.
	batchesByName := map[string]api.Batch{}
	header := []string{"Name", "Batch", "Last Update", "Status", "Status Message", "Migration Window"}
	if c.flagVerbose {
		header = append(header, "UUID", "Batch Status", "Batch Status Message", "Target", "Target Project`")

		// Get the current migration queue.
		resp, _, err := c.global.doHTTPRequestV1("/batches", http.MethodGet, "recursion=1", nil)
		if err != nil {
			return err
		}

		batches := []api.Batch{}
		err = responseToStruct(resp, &batches)
		if err != nil {
			return err
		}

		for _, b := range batches {
			batchesByName[b.Name] = b
		}
	}

	data := [][]string{}

	for _, q := range queueEntries {
		if q.MigrationStatusMessage == "" {
			q.MigrationStatusMessage = string(q.MigrationStatus)
		}

		lastUpdate := "No update"
		if !q.LastWorkerResponse.IsZero() {
			lastUpdate = time.Now().UTC().Sub(q.LastWorkerResponse).Truncate(time.Second).String() + " ago"
		}

		window := "none"

		if !q.MigrationWindow.End.IsZero() || !q.MigrationWindow.Start.IsZero() {
			window = fmt.Sprintf("%s - %s", q.MigrationWindow.Start.String(), q.MigrationWindow.End.String())
			if !q.MigrationWindow.Lockout.IsZero() {
				window = fmt.Sprintf("%s (lockout %s)", window, q.MigrationWindow.Lockout.String())
			}
		}

		row := []string{q.InstanceName, q.BatchName, lastUpdate, string(q.MigrationStatus), q.MigrationStatusMessage, window}
		if c.flagVerbose {
			row = append(row, q.InstanceUUID.String(), string(batchesByName[q.BatchName].Status), batchesByName[q.BatchName].StatusMessage, q.Placement.TargetName, q.Placement.TargetProject)
		}

		data = append(data, row)
	}

	sort.Sort(util.SortColumnsNaturally(data))

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, queueEntries)
}

// Remove the queue entry.
type cmdQueueRemove struct {
	global *CmdGlobal
}

func (c *cmdQueueRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remove <instance UUID>"
	cmd.Aliases = []string{"rm"}
	cmd.Short = "Remove the queue entry"
	cmd.Long = `Description:
  Remove the queue entry
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdQueueRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	instanceUUID := args[0]

	// Remove the queue.
	_, _, err = c.global.doHTTPRequestV1("/queue/"+instanceUUID, http.MethodDelete, "", nil)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully removed queue entry %q.\n", instanceUUID)
	return nil
}

// Cancel the queue entry.
type cmdQueueCancel struct {
	global *CmdGlobal
}

func (c *cmdQueueCancel) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "cancel <instance UUID>"
	cmd.Short = "Cancel the queue entry"
	cmd.Long = `Description:
  Cancel migration for the queue entry.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdQueueCancel) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	instanceUUID := args[0]

	// Cancel the queue entry.
	_, _, err = c.global.doHTTPRequestV1("/queue/"+instanceUUID+"/:cancel", http.MethodPost, "", nil)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully cancelled queue entry %q.\n", instanceUUID)
	return nil
}

// Retry the queue entry.
type cmdQueueRetry struct {
	global *CmdGlobal
}

func (c *cmdQueueRetry) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "retry <instance UUID>"
	cmd.Short = "Retry the queue entry"
	cmd.Long = `Description:
  Retry migration for the queue entry.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdQueueRetry) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	instanceUUID := args[0]

	// Retry the queue entry.
	_, _, err = c.global.doHTTPRequestV1("/queue/"+instanceUUID+"/:retry", http.MethodPost, "", nil)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully restarted queue entry %q.\n", instanceUUID)
	return nil
}
