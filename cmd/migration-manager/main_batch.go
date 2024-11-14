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

	// Show
	batchShowCmd := cmdBatchShow{global: c.global}
	cmd.AddCommand(batchShowCmd.Command())

	// Start
	batchStartCmd := cmdBatchStart{global: c.global}
	cmd.AddCommand(batchStartCmd.Command())

	// Stop
	batchStopCmd := cmdBatchStop{global: c.global}
	cmd.AddCommand(batchStopCmd.Command())

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

// Show
type cmdBatchShow struct {
	global *cmdGlobal
}

func (c *cmdBatchShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show <name>"
	cmd.Short = "Show information about a batch"
	cmd.Long = `Description:
  Show information about a batch, including all instances assigned to it.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdBatchShow) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Get the batch.
	resp, err := c.global.DoHttpRequest("/1.0/batches/" + name, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	parsed, err := parseReturnedBatch(resp.Metadata)
	if err != nil {
		return err
	}
	b := parsed.(api.Batch)

	// Get all instances for this batch.
	resp, err = c.global.DoHttpRequest("/1.0/batches/" + name + "/instances", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	instances := []api.Instance{}

	// Loop through returned instances.
	for _, anyInstance := range resp.Metadata.([]any) {
		newInstance, err := parseReturnedInstance(anyInstance)
		if err != nil {
			return err
		}
		instances = append(instances, newInstance.(api.Instance))
	}

	// Show the details
	fmt.Printf("Batch: %s\n", b.Name)
	fmt.Printf("  - Status:        %s\n", b.Status)
	if b.IncludeRegex != "" {
		fmt.Printf("  - Include regex: %s\n", b.IncludeRegex)
	}
	if b.ExcludeRegex != "" {
		fmt.Printf("  - Exclude regex: %s\n", b.ExcludeRegex)
	}
	if !b.MigrationWindowStart.IsZero() {
		fmt.Printf("  - Window start:  %s\n", b.MigrationWindowStart)
	}
	if !b.MigrationWindowEnd.IsZero() {
		fmt.Printf("  - Window end:    %s\n", b.MigrationWindowEnd)
	}

	fmt.Printf("\n  - Instances:\n")
	for _, i := range instances {
		fmt.Printf("    - %s (%s)\n", i.Name, i.MigrationStatusString)
	}
	return nil
}

// Start
type cmdBatchStart struct {
	global *cmdGlobal
}

func (c *cmdBatchStart) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "start <name>"
	cmd.Short = "Start batch"
	cmd.Long = `Description:
  Activate a batch and start the migration process for its instances.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdBatchStart) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Start the batch.
	_, err = c.global.DoHttpRequest("/1.0/batches/" + name + "/start", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully started batch '%s'.\n", name)
	return nil
}

// Stop
type cmdBatchStop struct {
	global *cmdGlobal
}

func (c *cmdBatchStop) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "stop <name>"
	cmd.Short = "Stop batch"
	cmd.Long = `Description:
  Deactivate a batch and stop the migration process for its instances.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdBatchStop) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Start the batch.
	_, err = c.global.DoHttpRequest("/1.0/batches/" + name + "/stop", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully stopped batch '%s'.\n", name)
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
