package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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

// Add the batch.
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
		Name:         args[0],
		Status:       api.BATCHSTATUS_DEFINED,
		StatusString: api.BATCHSTATUS_DEFINED.String(),
	}

	b.StoragePool, err = c.global.asker.AskString("What storage pool should be used for VMs and the migration ISO images? [local] ", "local", nil)
	if err != nil {
		return err
	}

	b.IncludeRegex, err = c.global.asker.AskString("Regular expression to include instances: ", "", func(s string) error { return nil })
	if err != nil {
		return err
	}

	b.ExcludeRegex, err = c.global.asker.AskString("Regular expression to exclude instances: ", "", func(s string) error { return nil })
	if err != nil {
		return err
	}

	windowStart, err := c.global.asker.AskString("Migration window start (YYYY-MM-DD HH:MM:SS): ", "", func(s string) error {
		if s != "" {
			_, err := time.Parse(time.DateTime, s)
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	if windowStart != "" {
		b.MigrationWindowStart, _ = time.Parse(time.DateTime, windowStart)
	}

	windowEnd, err := c.global.asker.AskString("Migration window end (YYYY-MM-DD HH:MM:SS): ", "", func(s string) error {
		if s != "" {
			_, err := time.Parse(time.DateTime, s)
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	if windowEnd != "" {
		b.MigrationWindowEnd, _ = time.Parse(time.DateTime, windowEnd)
	}

	b.DefaultNetwork, err = c.global.asker.AskString("Default network for instances: ", "", func(s string) error { return nil })
	if err != nil {
		return err
	}

	// Insert into database.
	content, err := json.Marshal(b)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/batches", http.MethodPost, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully added new batch '%s'.\n", b.Name)
	return nil
}

// List the batches.
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
	resp, err := c.global.doHTTPRequestV1("/batches", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	batches := []api.Batch{}

	metadata, ok := resp.Metadata.([]any)
	if !ok {
		return errors.New("Unexpected API response, invalid type for metadata")
	}

	// Loop through returned batches.
	for _, anyBatch := range metadata {
		newBatch, err := parseReturnedBatch(anyBatch)
		if err != nil {
			return err
		}

		batches = append(batches, newBatch.(api.Batch))
	}

	// Render the table.
	header := []string{"Name", "Status", "Status String", "Storage Pool", "Include Regex", "Exclude Regex", "Window Start", "Window End", "Default Network"}
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

		data = append(data, []string{b.Name, b.Status.String(), b.StatusString, b.StoragePool, b.IncludeRegex, b.ExcludeRegex, startString, endString, b.DefaultNetwork})
	}

	return util.RenderTable(c.flagFormat, header, data, batches)
}

func parseReturnedBatch(b any) (any, error) {
	reJsonified, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	ret := api.Batch{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Remove the batch.
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
	_, err = c.global.doHTTPRequestV1("/batches/"+name, http.MethodDelete, "", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully removed batch '%s'.\n", name)
	return nil
}

// Show the batch.
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
	resp, err := c.global.doHTTPRequestV1("/batches/"+name, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	parsed, err := parseReturnedBatch(resp.Metadata)
	if err != nil {
		return err
	}

	b, ok := parsed.(api.Batch)
	if !ok {
		return errors.New("Invalid type for batch")
	}

	// Get all instances for this batch.
	resp, err = c.global.doHTTPRequestV1("/batches/"+name+"/instances", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	instances := []api.Instance{}

	metadata, ok := resp.Metadata.([]any)
	if !ok {
		return errors.New("Unexpected API response, invalid type for metadata")
	}

	// Loop through returned instances.
	for _, anyInstance := range metadata {
		newInstance, err := parseReturnedInstance(anyInstance)
		if err != nil {
			return err
		}

		instances = append(instances, newInstance.(api.Instance))
	}

	// Show the details
	fmt.Printf("Batch: %s\n", b.Name)
	fmt.Printf("  - Status:        %s\n", b.StatusString)
	if b.StoragePool != "" {
		fmt.Printf("  - Storage pool:    %s\n", b.StoragePool)
	}

	if b.IncludeRegex != "" {
		fmt.Printf("  - Include regex:   %s\n", b.IncludeRegex)
	}

	if b.ExcludeRegex != "" {
		fmt.Printf("  - Exclude regex:   %s\n", b.ExcludeRegex)
	}

	if !b.MigrationWindowStart.IsZero() {
		fmt.Printf("  - Window start:    %s\n", b.MigrationWindowStart)
	}

	if !b.MigrationWindowEnd.IsZero() {
		fmt.Printf("  - Window end:      %s\n", b.MigrationWindowEnd)
	}

	if b.DefaultNetwork != "" {
		fmt.Printf("  - Default network: %s\n", b.DefaultNetwork)
	}

	fmt.Printf("\n  - Instances:\n")
	for _, i := range instances {
		fmt.Printf("    - %s (%s)\n", i.Name, i.MigrationStatusString)
	}

	return nil
}

// Start the batch.
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
	_, err = c.global.doHTTPRequestV1("/batches/"+name+"/start", http.MethodPost, "", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully started batch '%s'.\n", name)
	return nil
}

// Stop the batch.
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
	_, err = c.global.doHTTPRequestV1("/batches/"+name+"/stop", http.MethodPost, "", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully stopped batch '%s'.\n", name)
	return nil
}

// Update the batch.
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
	resp, err := c.global.doHTTPRequestV1("/batches/"+name, http.MethodGet, "", nil)
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

		bb.Name, err = c.global.asker.AskString("Batch name: ["+bb.Name+"] ", bb.Name, nil)
		if err != nil {
			return err
		}

		bb.StoragePool, err = c.global.asker.AskString("Storage pool: ["+bb.StoragePool+"] ", bb.StoragePool, nil)
		if err != nil {
			return err
		}

		bb.IncludeRegex, err = c.global.asker.AskString("Regular expression to include instances: ["+bb.IncludeRegex+"] ", bb.IncludeRegex, func(s string) error { return nil })
		if err != nil {
			return err
		}

		bb.ExcludeRegex, err = c.global.asker.AskString("Regular expression to exclude instances: ["+bb.ExcludeRegex+"] ", bb.ExcludeRegex, func(s string) error { return nil })
		if err != nil {
			return err
		}

		windowStartValue := ""
		if !bb.MigrationWindowStart.IsZero() {
			windowStartValue = bb.MigrationWindowStart.Format(time.DateTime)
		}

		windowStart, err := c.global.asker.AskString("Migration window start (YYYY-MM-DD HH:MM:SS): ["+windowStartValue+"] ", windowStartValue, func(s string) error {
			if s != "" {
				_, err := time.Parse(time.DateTime, s)
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		if windowStart != "" {
			bb.MigrationWindowStart, _ = time.Parse(time.DateTime, windowStart)
		}

		windowEndValue := ""
		if !bb.MigrationWindowEnd.IsZero() {
			windowEndValue = bb.MigrationWindowEnd.Format(time.DateTime)
		}

		windowEnd, err := c.global.asker.AskString("Migration window end (YYYY-MM-DD HH:MM:SS): ["+windowEndValue+"] ", windowEndValue, func(s string) error {
			if s != "" {
				_, err := time.Parse(time.DateTime, s)
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		if windowEnd != "" {
			bb.MigrationWindowEnd, _ = time.Parse(time.DateTime, windowEnd)
		}

		bb.DefaultNetwork, err = c.global.asker.AskString("Default network for instances: ["+bb.DefaultNetwork+"] ", bb.DefaultNetwork, func(s string) error { return nil })
		if err != nil {
			return err
		}

		newBatchName = bb.Name
		b = bb
	}

	content, err := json.Marshal(b)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/batches/"+origBatchName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully updated batch '%s'.\n", newBatchName)
	return nil
}
