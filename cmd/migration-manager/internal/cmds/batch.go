package cmds

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/validate"
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
	targets, err := c.global.getTargets()
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		return fmt.Errorf("No targets have been defined, cannot add a batch.")
	}

	// Add the batch.
	b := api.Batch{
		BatchPut: api.BatchPut{
			Name:        args[0],
			Constraints: []api.BatchConstraint{},
		},
	}

	if len(targets) == 1 {
		b.DefaultTarget = targets[0]
		fmt.Printf("Using target %q\n", b.DefaultTarget)
	} else {
		defaultTargetHint := "(" + strings.Join(targets, ", ") + "): "
		b.DefaultTarget, err = c.global.Asker.AskChoice("What target should this batch use? "+defaultTargetHint, targets, "")
		if err != nil {
			return err
		}
	}

	b.DefaultTargetProject, err = c.global.Asker.AskString("What Incus project should this batch use? ", "", validate.IsNotEmpty)
	if err != nil {
		return err
	}

	b.DefaultStoragePool, err = c.global.Asker.AskString("What storage pool should be used for VMs and the migration ISO images? ", "", validate.IsNotEmpty)
	if err != nil {
		return err
	}

	b.IncludeExpression, err = c.global.Asker.AskString("Expression to include instances: ", "", validate.IsAny)
	if err != nil {
		return err
	}

	retries, err := c.global.Asker.AskInt("Maximum retries if post-migration steps are not successful: ", 0, 1024, "5", nil)
	if err != nil {
		return err
	}

	b.PostMigrationRetries = int(retries)

	addWindows := true
	for addWindows {
		windowStart, err := c.global.Asker.AskString("Migration window start (YYYY-MM-DD HH:MM:SS) (empty to skip): ", "", func(s string) error {
			if s != "" {
				_, err := time.Parse(time.DateTime, s)
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		windowEnd, err := c.global.Asker.AskString("Migration window end (YYYY-MM-DD HH:MM:SS) (empty to skip): ", "", func(s string) error {
			if s != "" {
				_, err := time.Parse(time.DateTime, s)
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		if windowStart != "" || windowEnd != "" {
			if b.MigrationWindows == nil {
				b.MigrationWindows = []api.MigrationWindow{}
			}

			start, _ := time.Parse(time.DateTime, windowStart)
			end, _ := time.Parse(time.DateTime, windowEnd)
			b.MigrationWindows = append(b.MigrationWindows, api.MigrationWindow{Start: start, End: end})
		}

		addWindows, err = c.global.Asker.AskBool("Add more migration windows? (yes/no) [default=no]: ", "no")
		if err != nil {
			return err
		}
	}

	addConstraints, err := c.global.Asker.AskBool("Add constraints? (yes/no) [default=no]: ", "no")
	if err != nil {
		return err
	}

	for addConstraints {
		var constraint api.BatchConstraint
		constraint.Name, err = c.global.Asker.AskString("Constraint name: ", "", nil)
		if err != nil {
			return err
		}

		constraint.Description, err = c.global.Asker.AskString("Constraint description (empty to skip): ", "", validate.IsAny)
		if err != nil {
			return err
		}

		constraint.IncludeExpression, err = c.global.Asker.AskString("Expression to include instances: ", "", validate.IsAny)
		if err != nil {
			return err
		}

		maxConcurrent, err := c.global.Asker.AskString("Maximum concurrent instance (empty to skip): ", "0", validate.IsInt64)
		if err != nil {
			return err
		}

		constraint.MaxConcurrentInstances, err = strconv.Atoi(maxConcurrent)
		if err != nil {
			return err
		}

		constraint.MinInstanceBootTime, err = c.global.Asker.AskString("Minimum instance boot time (empty to skip): ", "", func(s string) error {
			if s != "" {
				return validate.IsMinimumDuration(0)(s)
			}

			return nil
		})
		if err != nil {
			return err
		}

		b.Constraints = append(b.Constraints, constraint)
		addConstraints, err = c.global.Asker.AskBool("Add more constraints? (yes/no) [default=no]: ", "no")
		if err != nil {
			return err
		}
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

	cmd.Printf("Successfully added new batch %q.\n", b.Name)
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
	header := []string{"Name", "Status", "Status String", "Target", "Project", "Storage Pool", "Include Expression", "Migration Windows"}
	data := [][]string{}

	for _, b := range batches {
		data = append(data, []string{b.Name, string(b.Status), b.StatusMessage, b.DefaultTarget, b.DefaultTargetProject, b.DefaultStoragePool, b.IncludeExpression, strconv.Itoa(len(b.MigrationWindows))})
	}

	sort.Sort(util.SortColumnsNaturally(data))

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

	cmd.Printf("Successfully removed batch %q.\n", name)
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

	resp, err = c.global.doHTTPRequestV1("/queue", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return err
	}

	queueEntries := []api.QueueEntry{}
	err = responseToStruct(resp, &queueEntries)
	if err != nil {
		return err
	}

	queueMap := make(map[uuid.UUID]api.QueueEntry, len(queueEntries))
	for _, q := range queueEntries {
		queueMap[q.InstanceUUID] = q
	}

	// Show the details
	cmd.Printf("Batch: %s\n", b.Name)
	cmd.Printf("  - Status:             %s\n", b.StatusMessage)
	cmd.Printf("  - Target:             %s\n", b.DefaultTarget)
	if b.DefaultTargetProject != "" {
		cmd.Printf("  - Project:            %s\n", b.DefaultTargetProject)
	}

	if b.DefaultStoragePool != "" {
		cmd.Printf("  - Storage pool:       %s\n", b.DefaultStoragePool)
	}

	if b.IncludeExpression != "" {
		cmd.Printf("  - Include expression: %s\n", b.IncludeExpression)
	}

	for i, w := range b.MigrationWindows {
		nonZero := false
		if !w.Start.IsZero() {
			nonZero = true
			cmd.Printf("  - Window start:       %s\n", w.Start)
		}

		if !w.End.IsZero() {
			nonZero = true
			cmd.Printf("  - Window end:         %s\n", w.End)
		}

		if nonZero && i != len(b.MigrationWindows)-1 {
			cmd.Println()
		}
	}

	cmd.Printf("\n  - Matched Instances:\n")
	for _, i := range instances {
		disabled := ""
		if i.Overrides.DisableMigration {
			disabled = " (Migration Disabled)"
		}

		cmd.Printf("    - %s%s\n", i.Properties.Location, disabled)
	}

	cmd.Printf("\n  - Queued Instances:\n")

	for _, i := range instances {
		q, ok := queueMap[i.Properties.UUID]
		if !ok || q.BatchName != name {
			continue
		}

		cmd.Printf("    - %s (%s)\n", i.Properties.Location, q.MigrationStatus)
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

	cmd.Printf("Successfully started batch %q.\n", name)
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

	cmd.Printf("Successfully stopped batch %q.\n", name)
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
	targets, err := c.global.getTargets()
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		return fmt.Errorf("No targets have been defined, cannot add a batch.")
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

	b.Name, err = c.global.Asker.AskString("Batch name [default="+b.Name+"]: ", b.Name, nil)
	if err != nil {
		return err
	}

	var targetChoices string
	if len(targets) > 1 {
		targetChoices = " (" + strings.Join(targets, ", ") + ")"
	}

	b.DefaultTarget, err = c.global.Asker.AskChoice("Target"+targetChoices+" [default="+b.DefaultTarget+"]: ", targets, b.DefaultTarget)
	if err != nil {
		return err
	}

	b.DefaultTargetProject, err = c.global.Asker.AskString("Project [default="+b.DefaultTargetProject+"]: ", b.DefaultTargetProject, nil)
	if err != nil {
		return err
	}

	b.DefaultStoragePool, err = c.global.Asker.AskString("Storage pool [default="+b.DefaultStoragePool+"]: ", b.DefaultStoragePool, nil)
	if err != nil {
		return err
	}

	b.IncludeExpression, err = c.global.Asker.AskString("Expression to include instances [default="+b.IncludeExpression+"]: ", b.IncludeExpression, func(s string) error { return nil })
	if err != nil {
		return err
	}

	retries, err := c.global.Asker.AskInt("Maximum retries if post-migration steps are not successful: ", 0, 1024, strconv.Itoa(b.PostMigrationRetries), nil)
	if err != nil {
		return err
	}

	b.PostMigrationRetries = int(retries)

	addWindows, err := c.global.Asker.AskBool("Replace migration windows? (yes/no) [default=no]: ", "no")
	if err != nil {
		return err
	}

	if addWindows {
		b.MigrationWindows = []api.MigrationWindow{}
	}

	for addWindows {
		windowStart, err := c.global.Asker.AskString("Migration window start (YYYY-MM-DD HH:MM:SS) (empty to skip): ", "", func(s string) error {
			if s != "" {
				_, err := time.Parse(time.DateTime, s)
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		windowEnd, err := c.global.Asker.AskString("Migration window end (YYYY-MM-DD HH:MM:SS) (empty to skip): ", "", func(s string) error {
			if s != "" {
				_, err := time.Parse(time.DateTime, s)
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		if windowStart != "" || windowEnd != "" {
			if b.MigrationWindows == nil {
				b.MigrationWindows = []api.MigrationWindow{}
			}

			start, _ := time.Parse(time.DateTime, windowStart)
			end, _ := time.Parse(time.DateTime, windowEnd)
			b.MigrationWindows = append(b.MigrationWindows, api.MigrationWindow{Start: start, End: end})
		}

		addWindows, err = c.global.Asker.AskBool("Add more migration windows? (yes/no) [default=no]: ", "no")
		if err != nil {
			return err
		}
	}

	addConstraints, err := c.global.Asker.AskBool("Replace constraints? (yes/no) [default=no]: ", "no")
	if err != nil {
		return err
	}

	if addConstraints {
		b.Constraints = []api.BatchConstraint{}
	}

	for addConstraints {
		var constraint api.BatchConstraint
		constraint.Name, err = c.global.Asker.AskString("Constraint name: ", "", nil)
		if err != nil {
			return err
		}

		constraint.Description, err = c.global.Asker.AskString("Constraint description (empty to skip): ", "", validate.IsAny)
		if err != nil {
			return err
		}

		constraint.IncludeExpression, err = c.global.Asker.AskString("Expression to include instances: ", "", validate.IsAny)
		if err != nil {
			return err
		}

		maxConcurrent, err := c.global.Asker.AskString("Maximum concurrent instance (empty to skip): ", "0", validate.IsInt64)
		if err != nil {
			return err
		}

		constraint.MaxConcurrentInstances, err = strconv.Atoi(maxConcurrent)
		if err != nil {
			return err
		}

		constraint.MinInstanceBootTime, err = c.global.Asker.AskString("Minimum instance boot time (empty to skip): ", "", func(s string) error {
			if s != "" {
				return validate.IsMinimumDuration(0)(s)
			}

			return nil
		})
		if err != nil {
			return err
		}

		b.Constraints = append(b.Constraints, constraint)
		addConstraints, err = c.global.Asker.AskBool("Add more constraints? (yes/no) [default=no]: ", "no")
		if err != nil {
			return err
		}
	}

	newBatchName := b.Name

	content, err := json.Marshal(b.BatchPut)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/batches/"+origBatchName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully updated batch %q.\n", newBatchName)
	return nil
}

func (c *CmdGlobal) getTargets() ([]string, error) {
	ret := []string{}

	// Get the list of all targets.
	resp, err := c.doHTTPRequestV1("/targets", http.MethodGet, "", nil)
	if err != nil {
		return ret, err
	}

	targets := []string{}
	err = responseToStruct(resp, &targets)
	if err != nil {
		return ret, err
	}

	for _, v := range targets {
		parts := strings.Split(v, "/")
		if len(parts) > 0 {
			ret = append(ret, parts[len(parts)-1])
		}
	}

	return ret, nil
}
