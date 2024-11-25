package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type cmdTarget struct {
	global *cmdGlobal
}

func (c *cmdTarget) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "target"
	cmd.Short = "Interact with migration targets"
	cmd.Long = `Description:
  Interact with migration targets

  Configure migration targets for use by the migration manager.
`

	// Add
	targetAddCmd := cmdTargetAdd{global: c.global}
	cmd.AddCommand(targetAddCmd.Command())

	// List
	targetListCmd := cmdTargetList{global: c.global}
	cmd.AddCommand(targetListCmd.Command())

	// Remove
	targetRemoveCmd := cmdTargetRemove{global: c.global}
	cmd.AddCommand(targetRemoveCmd.Command())

	// Update
	targetUpdateCmd := cmdTargetUpdate{global: c.global}
	cmd.AddCommand(targetUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// Add
type cmdTargetAdd struct {
	global *cmdGlobal

	flagInsecure bool
	flagNoTestConnection bool
}

func (c *cmdTargetAdd) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "add <name> <IP|FQDN|URL>"
	cmd.Short = "Add a new target"
	cmd.Long = `Description:
  Add a new target

  Adds a new target for the migration manager to use.

  Depending on the target type, you may be prompted for additional information required
  to connect to the target.
`

	cmd.RunE = c.Run
	cmd.Flags().BoolVar(&c.flagInsecure, "insecure", false, "Allow insecure TLS connections to the target")
	cmd.Flags().BoolVar(&c.flagNoTestConnection, "no-test-connection", false, "Don't test connection to the new target")

	return cmd
}

func (c *cmdTargetAdd) Run(cmd *cobra.Command, args []string) error {

	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	// Add the target.
	t := api.IncusTarget{
		Name: args[0],
		Endpoint: args[1],
		Insecure: c.flagInsecure,
	}

	authType, err := c.global.asker.AskChoice("Use OIDC or TLS certificates to authenticate to target? [oidc] ", []string{"oidc", "tls"}, "oidc")
	if err != nil {
		return err
	}

	// Only TLS certs require additional prompting at the moment; we'll grab OIDC tokens below when we verify the target.
	if authType == "tls" {
		tlsCertPath, err := c.global.asker.AskString("Please enter path to client TLS certificate: ", "", nil)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(tlsCertPath)
		if err != nil {
			return err
		}
		t.TLSClientCert = string(contents)

		tlsKeyPath, err := c.global.asker.AskString("Please enter path to client TLS key: ", "", nil)
		if err != nil {
			return err
		}
		contents, err = os.ReadFile(tlsKeyPath)
		if err != nil {
			return err
		}
		t.TLSClientKey = string(contents)
	}

	t.IncusProject, err = c.global.asker.AskString("What Incus project should this target use? [default] ", "default", nil)
	if err != nil {
		return err
	}

	t.StoragePool, err = c.global.asker.AskString("What storage pool should be used for VMs and the migration ISO images? [local] ", "local", nil)
	if err != nil {
		return err
	}

	t.BootISOImage, err = c.global.asker.AskString("What is the migration environment boot ISO image name? ", "", nil)
	if err != nil {
		return err
	}

	t.DriversISOImage, err = c.global.asker.AskString("What is the virtio drivers ISO image name? ", "", nil)
	if err != nil {
		return err
	}

	content, err := json.Marshal(t)
	if err != nil {
		return err
	}

	// Verify we can connect to the target, and if using OIDC grab the tokens.
	ctx := context.TODO()

	internalTarget := target.InternalIncusTarget{}
	err = json.Unmarshal(content, &internalTarget)
	if err != nil {
		return err
	}

	if !c.flagNoTestConnection {
		err = internalTarget.Connect(ctx)
		if err != nil {
			return err
		}
	}

	// Insert into database.
	content, err = json.Marshal(internalTarget)
	if err != nil {
		return err
	}

	_, err = c.global.DoHttpRequest("/1.0/targets", http.MethodPost, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully added new target '%s'.\n", t.Name)
	return nil
}

// List
type cmdTargetList struct {
	global *cmdGlobal

	flagFormat string
}

func (c *cmdTargetList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Short = "List available targets"
	cmd.Long = `Description:
  List the available targets
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", "Format (csv|json|table|yaml|compact)")

	return cmd
}

func (c *cmdTargetList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the list of all targets.
	resp, err := c.global.DoHttpRequest("/1.0/targets", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	targets := []api.IncusTarget{}

	// Loop through returned targets.
	for _, anyTarget := range resp.Metadata.([]any) {
		newTarget, err := parseReturnedTarget(anyTarget)
		if err != nil {
			return err
		}
		targets = append(targets, newTarget.(api.IncusTarget))
	}

	// Render the table.
	header := []string{"Name", "Endpoint", "Auth Type", "Project", "Storage Pool", "Migration ISO", "Drivers ISO", "Insecure"}
	data := [][]string{}

	for _, t := range targets {
		authType := "OIDC"
		if t.TLSClientKey != "" {
			authType = "TLS"
		}
		data = append(data, []string{t.Name, t.Endpoint, authType, t.IncusProject, t.StoragePool, t.BootISOImage, t.DriversISOImage, strconv.FormatBool(t.Insecure)})
	}

	return util.RenderTable(c.flagFormat, header, data, targets)
}

func parseReturnedTarget(t any) (any, error) {
	reJsonified, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	var ret = api.IncusTarget{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Remove
type cmdTargetRemove struct {
	global *cmdGlobal
}

func (c *cmdTargetRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remove <name>"
	cmd.Short = "Remove target"
	cmd.Long = `Description:
  Remove target
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdTargetRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Remove the target.
	_, err = c.global.DoHttpRequest("/1.0/targets/" + name, http.MethodDelete, "", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully removed target '%s'.\n", name)
	return nil
}

// Update
type cmdTargetUpdate struct {
	global *cmdGlobal
}

func (c *cmdTargetUpdate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "update <name>"
	cmd.Short = "Update target"
	cmd.Long = `Description:
  Update target
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdTargetUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	// Get the existing target.
	resp, err := c.global.DoHttpRequest("/1.0/targets/" + name, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	t, err := parseReturnedTarget(resp.Metadata)
	if err != nil {
		return err
	}

	// Prompt for updates.
	origTargetName := ""
	newTargetName := ""
	switch incusTarget := t.(type) {
	case api.IncusTarget:
		origTargetName = incusTarget.Name

		incusTarget.Name, err = c.global.asker.AskString("Target name: [" + incusTarget.Name + "] ", incusTarget.Name, nil)
		if err != nil {
			return err
		}

		incusTarget.Endpoint, err = c.global.asker.AskString("Endpoint: [" + incusTarget.Endpoint + "] ", incusTarget.Endpoint, nil)
		if err != nil {
			return err
		}

		updateAuth, err := c.global.asker.AskBool("Update configured authentication? [no] ", "no")
		if err != nil {
			return err
		}
		if updateAuth {
			// Clear out existing auth.
			incusTarget.TLSClientKey = ""
			incusTarget.TLSClientCert = ""
			incusTarget.OIDCTokens = nil

			authType, err := c.global.asker.AskChoice("Use OIDC or TLS certificates to authenticate to target? [oidc] ", []string{"oidc", "tls"}, "oidc")
			if err != nil {
				return err
			}

			// Only TLS certs require additional prompting at the moment; we'll grab OIDC tokens below when we verify the target.
			if authType == "tls" {
				tlsCertPath, err := c.global.asker.AskString("Please enter path to client TLS certificate: ", "", nil)
				if err != nil {
					return err
				}
				contents, err := os.ReadFile(tlsCertPath)
				if err != nil {
					return err
				}
				incusTarget.TLSClientCert = string(contents)

				tlsKeyPath, err := c.global.asker.AskString("Please enter path to client TLS key: ", "", nil)
				if err != nil {
					return err
				}
				contents, err = os.ReadFile(tlsKeyPath)
				if err != nil {
					return err
				}
				incusTarget.TLSClientKey = string(contents)
			}
		}

		isInsecure := "no"
		if incusTarget.Insecure {
			isInsecure = "yes"
		}
		incusTarget.Insecure, err = c.global.asker.AskBool("Allow insecure TLS? [" + isInsecure + "] ", isInsecure)
		if err != nil {
			return err
		}

		incusTarget.IncusProject, err = c.global.asker.AskString("Project: [" + incusTarget.IncusProject + "] ", incusTarget.IncusProject, nil)
		if err != nil {
			return err
		}

		incusTarget.StoragePool, err = c.global.asker.AskString("Storage pool: [" + incusTarget.StoragePool + "] ", incusTarget.StoragePool, nil)
		if err != nil {
			return err
		}

		incusTarget.BootISOImage, err = c.global.asker.AskString("Boot ISO image: [" + incusTarget.BootISOImage + "] ", incusTarget.BootISOImage, nil)
		if err != nil {
			return err
		}

		incusTarget.DriversISOImage, err = c.global.asker.AskString("Drivers ISO image: [" + incusTarget.DriversISOImage + "] ", incusTarget.DriversISOImage, nil)
		if err != nil {
			return err
		}

		newTargetName = incusTarget.Name
		t = incusTarget
	}

	content, err := json.Marshal(t)
	if err != nil {
		return err
	}

	// Verify we can connect to the updated target, and if needed grab new OIDC tokens.
	ctx := context.TODO()

	internalTarget := target.InternalIncusTarget{}
	err = json.Unmarshal(content, &internalTarget)
	if err != nil {
		return err
	}

	err = internalTarget.Connect(ctx)
	if err != nil {
		return err
	}

	// Update the target.
	content, err = json.Marshal(internalTarget)
	if err != nil {
		return err
	}

	_, err = c.global.DoHttpRequest("/1.0/targets/" + origTargetName, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully updated target '%s'.\n", newTargetName)
	return nil
}
