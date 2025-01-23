package cmds

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdBatch struct {
	Global *CmdGlobal
}

func (c *CmdBatch) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "batch"
	cmd.Short = "Interact with migration batches"
	cmd.Long = `Description:
  Interact with migration batches

  Configure batches for use by the migration manager.
`

	// Add
	batchAddCmd := cmdBatchAdd{global: c.Global}
	cmd.AddCommand(batchAddCmd.Command())

	// List
	batchListCmd := cmdBatchList{global: c.Global}
	cmd.AddCommand(batchListCmd.Command())

	// Remove
	batchRemoveCmd := cmdBatchRemove{global: c.Global}
	cmd.AddCommand(batchRemoveCmd.Command())

	// Show
	batchShowCmd := cmdBatchShow{global: c.Global}
	cmd.AddCommand(batchShowCmd.Command())

	// Start
	batchStartCmd := cmdBatchStart{global: c.Global}
	cmd.AddCommand(batchStartCmd.Command())

	// Stop
	batchStopCmd := cmdBatchStop{global: c.Global}
	cmd.AddCommand(batchStopCmd.Command())

	// Update
	batchUpdateCmd := cmdBatchUpdate{global: c.Global}
	cmd.AddCommand(batchUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Add the batch.
type cmdBatchAdd struct {
	global *CmdGlobal
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

	// Get any defined targets.
	targetMap, err := c.global.getTargetMap()
	if err != nil {
		return err
	}

	if len(targetMap) == 0 {
		return fmt.Errorf("No targets have been defined, cannot add a batch.")
	}

	definedTargets := []string{}
	for _, v := range targetMap {
		definedTargets = append(definedTargets, v)
	}

	// Add the batch.
	b := api.Batch{
		Name: args[0],
	}

	targetString, err := c.global.Asker.AskChoice(fmt.Sprintf("What target should be used by this batch? (Choices: '%s') ", strings.Join(definedTargets, "', '")), definedTargets, "")
	if err != nil {
		return err
	}

	for k, v := range targetMap {
		if v == targetString {
			b.TargetID = k
			break
		}
	}

	b.TargetProject, err = c.global.Asker.AskString("What Incus project should this batch use? [default] ", "default", nil)
	if err != nil {
		return err
	}

	b.StoragePool, err = c.global.Asker.AskString("What storage pool should be used for VMs and the migration ISO images? [local] ", "local", nil)
	if err != nil {
		return err
	}

	b.IncludeExpression, err = c.global.Asker.AskString("Expression to include instances: ", "", func(s string) error { return nil })
	if err != nil {
		return err
	}

	windowStart, err := c.global.Asker.AskString("Migration window start (YYYY-MM-DD HH:MM:SS): ", "", func(s string) error {
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

	windowEnd, err := c.global.Asker.AskString("Migration window end (YYYY-MM-DD HH:MM:SS): ", "", func(s string) error {
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

	// Insert into database.
	content, err := json.Marshal(b)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/batches", http.MethodPost, "", content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully added new batch '%s'.\n", b.Name)
	return nil
}

// List the batches.
type cmdBatchList struct {
	global *CmdGlobal

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
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateFlagFormat(cmd.Flag("format").Value.String())
	}

	return cmd
}

func (c *cmdBatchList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get any defined targets.
	targetMap, err := c.global.getTargetMap()
	if err != nil {
		return err
	}

	// Get the list of all batches.
	resp, err := c.global.doHTTPRequestV1("/batches", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return err
	}

	batches := []api.Batch{}

	err = responseToStruct(resp, &batches)
	if err != nil {
		return err
	}

	// Render the table.
	header := []string{"Name", "Status", "Status String", "Target", "Project", "Storage Pool", "Include Expression", "Window Start", "Window End"}
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

		data = append(data, []string{b.Name, b.Status.String(), b.StatusString, targetMap[b.TargetID], b.TargetProject, b.StoragePool, b.IncludeExpression, startString, endString})
	}

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, batches)
}

// Remove the batch.
type cmdBatchRemove struct {
	global *CmdGlobal
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

	cmd.Printf("Successfully removed batch '%s'.\n", name)
	return nil
}

// Show the batch.
type cmdBatchShow struct {
	global *CmdGlobal
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

	// Get any defined targets.
	targetMap, err := c.global.getTargetMap()
	if err != nil {
		return err
	}

	// Get the batch.
	resp, err := c.global.doHTTPRequestV1("/batches/"+name, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	b := api.Batch{}

	err = responseToStruct(resp, &b)
	if err != nil {
		return err
	}

	// Get all instances for this batch.
	resp, err = c.global.doHTTPRequestV1("/batches/"+name+"/instances", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return err
	}

	instances := []api.Instance{}

	err = responseToStruct(resp, &instances)
	if err != nil {
		return err
	}

	// Show the details
	cmd.Printf("Batch: %s\n", b.Name)
	cmd.Printf("  - Status:             %s\n", b.StatusString)
	cmd.Printf("  - Target:             %s\n", targetMap[b.TargetID])
	if b.TargetProject != "" {
		cmd.Printf("  - Project:            %s\n", b.TargetProject)
	}

	if b.StoragePool != "" {
		cmd.Printf("  - Storage pool:       %s\n", b.StoragePool)
	}

	if b.IncludeExpression != "" {
		cmd.Printf("  - Include expression: %s\n", b.IncludeExpression)
	}

	if !b.MigrationWindowStart.IsZero() {
		cmd.Printf("  - Window start:       %s\n", b.MigrationWindowStart)
	}

	if !b.MigrationWindowEnd.IsZero() {
		cmd.Printf("  - Window end:         %s\n", b.MigrationWindowEnd)
	}

	cmd.Printf("\n  - Instances:\n")
	for _, i := range instances {
		cmd.Printf("    - %s (%s)\n", i.InventoryPath, i.MigrationStatusString)
	}

	return nil
}

// Start the batch.
type cmdBatchStart struct {
	global *CmdGlobal
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

	cmd.Printf("Successfully started batch '%s'.\n", name)
	return nil
}

// Stop the batch.
type cmdBatchStop struct {
	global *CmdGlobal
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

	cmd.Printf("Successfully stopped batch '%s'.\n", name)
	return nil
}

// Update the batch.
type cmdBatchUpdate struct {
	global *CmdGlobal
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

	// Get any defined targets.
	targetMap, err := c.global.getTargetMap()
	if err != nil {
		return err
	}

	if len(targetMap) == 0 {
		return fmt.Errorf("No targets have been defined, cannot update the batch.")
	}

	definedTargets := make([]string, 0, len(targetMap))
	for _, v := range targetMap {
		definedTargets = append(definedTargets, v)
	}

	// Get the existing batch.
	resp, err := c.global.doHTTPRequestV1("/batches/"+name, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	b := api.Batch{}

	err = responseToStruct(resp, &b)
	if err != nil {
		return err
	}

	// Prompt for updates.
	origBatchName := b.Name

	b.Name, err = c.global.Asker.AskString("Batch name: ["+b.Name+"] ", b.Name, nil)
	if err != nil {
		return err
	}

	targetString, err := c.global.Asker.AskChoice(fmt.Sprintf("Target: ["+targetMap[b.TargetID]+"] (Choices: '%s') ", strings.Join(definedTargets, "', '")), definedTargets, "")
	if err != nil {
		return err
	}

	for k, v := range targetMap {
		if v == targetString {
			b.TargetID = k
			break
		}
	}

	b.TargetProject, err = c.global.Asker.AskString("Project: ["+b.TargetProject+"] ", b.TargetProject, nil)
	if err != nil {
		return err
	}

	b.StoragePool, err = c.global.Asker.AskString("Storage pool: ["+b.StoragePool+"] ", b.StoragePool, nil)
	if err != nil {
		return err
	}

	b.IncludeExpression, err = c.global.Asker.AskString("Expression to include instances: ["+b.IncludeExpression+"] ", b.IncludeExpression, func(s string) error { return nil })
	if err != nil {
		return err
	}

	windowStartValue := ""
	if !b.MigrationWindowStart.IsZero() {
		windowStartValue = b.MigrationWindowStart.Format(time.DateTime)
	}

	windowStart, err := c.global.Asker.AskString("Migration window start (YYYY-MM-DD HH:MM:SS): ["+windowStartValue+"] ", windowStartValue, func(s string) error {
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

	windowEndValue := ""
	if !b.MigrationWindowEnd.IsZero() {
		windowEndValue = b.MigrationWindowEnd.Format(time.DateTime)
	}

	windowEnd, err := c.global.Asker.AskString("Migration window end (YYYY-MM-DD HH:MM:SS): ["+windowEndValue+"] ", windowEndValue, func(s string) error {
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

	newBatchName := b.Name

	content, err := json.Marshal(b)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/batches/"+origBatchName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully updated batch '%s'.\n", newBatchName)
	return nil
}

func (c *CmdGlobal) getTargetMap() (map[int]string, error) {
	ret := make(map[int]string)

	// Get the list of all targets.
	resp, err := c.doHTTPRequestV1("/targets", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return ret, err
	}

	targets := []api.IncusTarget{}

	err = responseToStruct(resp, &targets)
	if err != nil {
		return ret, err
	}

	// Loop through returned targets.
	for _, t := range targets {
		ret[t.DatabaseID] = t.Name
	}

	return ret, nil
}
