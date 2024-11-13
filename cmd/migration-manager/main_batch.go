package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type cmdBatch struct {
	global *cmdGlobal
}

func (c *cmdBatch) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "batch"
	cmd.Short = "Interact with migration batches"
	cmd.Long = `Description:
  Interact with migration batches

  Configure batches for use by the migration manager.
`

	// Add
	batchAddCmd := cmdBatchAdd{global: c.global}
	cmd.AddCommand(batchAddCmd.Command())

	// List
	batchListCmd := cmdBatchList{global: c.global}
	cmd.AddCommand(batchListCmd.Command())

	// Remove
	batchRemoveCmd := cmdBatchRemove{global: c.global}
	cmd.AddCommand(batchRemoveCmd.Command())

	// Update
	batchUpdateCmd := cmdBatchUpdate{global: c.global}
	cmd.AddCommand(batchUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Add
type cmdBatchAdd struct {
	global *cmdGlobal
}

func (c *cmdBatchAdd) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "add <name>"
	cmd.Short = "Add a new batch"
	cmd.Long = `Description:
  Add a new batch

  Adds a new batch for the migration manager to use. After defining the batch you can view the instances that would
  be selected, but the batch won't actually run until enabled.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdBatchAdd) Run(cmd *cobra.Command, args []string) error {

	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Add the batch.
	b := api.Batch{
		Name: args[0],
		Status: api.BATCHSTATUS_DEFINED,
	}

	includeRegex, err := c.global.asker.AskString("Regular expression to include instances: ", "", nil)
	if err != nil {
		return err
	}
	b.IncludeRegex = includeRegex

	excludeRegex, err := c.global.asker.AskString("Regular expression to exclude instances: ", "", nil)
	if err != nil {
		return err
	}
	b.ExcludeRegex = excludeRegex

	// TODO handle reading in timestamps for window start/end

	// Insert into database.
	content, err := json.Marshal(b)
	if err != nil {
		return err
	}

	_, err = c.global.DoHttpRequest("/1.0/batches", http.MethodPost, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully added new batch '%s'.\n", b.Name)
	return nil
}

// List
type cmdBatchList struct {
	global *cmdGlobal

	flagFormat string
}

func (c *cmdBatchList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Short = "List available batches"
	cmd.Long = `Description:
  List the available batches
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", "Format (csv|json|table|yaml|compact)")

	return cmd
}

func (c *cmdBatchList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the list of all batches.
	resp, err := c.global.DoHttpRequest("/1.0/batches", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	batches := []api.Batch{}

	// Loop through returned batches.
	for _, anyBatch := range resp.Metadata.([]any) {
		newBatch, err := parseReturnedBatch(anyBatch)
		if err != nil {
			return err
		}
		batches = append(batches, newBatch.(api.Batch))
	}

	// Render the table.
	header := []string{"Name", "Status", "Include Regex", "Exclude Regex", "Window Start", "Window End"}
	data := [][]string{}

	for _, b := range batches {
		startString := ""
		endString := ""
		if !b.MigrationWindowStart.IsZero() {
			startString = b.MigrationWindowStart.String()
		}
		if !b.MigrationWindowEnd.IsZero() {
			endString = b.MigrationWindowEnd.String()
		}
		data = append(data, []string{b.Name, b.Status.String(), b.IncludeRegex, b.ExcludeRegex, startString, endString})
	}

	return util.RenderTable(c.flagFormat, header, data, batches)
}

func parseReturnedBatch(b any) (any, error) {
	reJsonified, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	var ret = api.Batch{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Remove
type cmdBatchRemove struct {
	global *cmdGlobal
}

func (c *cmdBatchRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remove <name>"
	cmd.Short = "Remove batch"
	cmd.Long = `Description:
  Remove batch
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdBatchRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Remove the batch.
	_, err = c.global.DoHttpRequest("/1.0/batches/" + name, http.MethodDelete, "", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully removed batch '%s'.\n", name)
	return nil
}

// Update
type cmdBatchUpdate struct {
	global *cmdGlobal
}

func (c *cmdBatchUpdate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "update <name>"
	cmd.Short = "Update batch"
	cmd.Long = `Description:
  Update batch
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdBatchUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Get the existing batch.
	resp, err := c.global.DoHttpRequest("/1.0/batches/" + name, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	b, err := parseReturnedBatch(resp.Metadata)
	if err != nil {
		return err
	}

	// Prompt for updates.
	origBatchName := ""
	newBatchName := ""
	switch bb := b.(type) {
	case api.Batch:
		origBatchName = bb.Name

		bb.Name, err = c.global.asker.AskString("Batch name: [" + bb.Name + "] ", bb.Name, nil)
		if err != nil {
			return err
		}

		bb.IncludeRegex, err = c.global.asker.AskString("Regular expression to include instances: [" + bb.IncludeRegex + "] ", bb.IncludeRegex, nil)
		if err != nil {
			return err
		}

		bb.ExcludeRegex, err = c.global.asker.AskString("Regular expression to exclude instances: [" + bb.ExcludeRegex + "] ", bb.ExcludeRegex, nil)
		if err != nil {
			return err
		}

		// TODO handle reading in timestamps for window start/end

		newBatchName = bb.Name
		b = bb
	}

	content, err := json.Marshal(b)
	if err != nil {
		return err
	}

	_, err = c.global.DoHttpRequest("/1.0/batches/" + origBatchName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully updated batch '%s'.\n", newBatchName)
	return nil
}
