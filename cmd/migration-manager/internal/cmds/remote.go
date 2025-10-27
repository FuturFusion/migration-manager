package cmds

import (
	"fmt"
	"net/url"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/internal/config"
	"github.com/FuturFusion/migration-manager/internal/util"
)

type CmdRemote struct {
	Global *CmdGlobal
}

func (c *CmdRemote) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remote"
	cmd.Short = "Manage the list of remote Migration Managers"
	cmd.Long = `Description:
  Manage the list of remote Migration Managers
`
	// Allow this sub-command to function even with pre-run checks failing.
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		err := c.Global.PreRun(cmd, args)
		if err != nil {
			cmd.PrintErrf("Warning: %v\n", err.Error())
		}

		return nil
	}

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	// Add
	remoteAddCmd := cmdRemoteAdd{global: c.Global}

	cmd.AddCommand(remoteAddCmd.Command())

	// List
	remoteListCmd := cmdRemoteList{global: c.Global}

	cmd.AddCommand(remoteListCmd.Command())

	// Remove
	remoteRemoveCmd := cmdRemoteRemove{global: c.Global}

	cmd.AddCommand(remoteRemoveCmd.Command())

	// Switch
	remoteSwitchCmd := cmdRemoteSwitch{global: c.Global}

	cmd.AddCommand(remoteSwitchCmd.Command())

	return cmd
}

// Add remote.
type cmdRemoteAdd struct {
	global *CmdGlobal

	authType string
}

func (c *cmdRemoteAdd) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "add <name> <URL>"
	cmd.Short = "Add a new remote"
	cmd.Long = `Description:
  Add a new remote

  Adds a new remote Migration Manager.
`

	cmd.Flags().StringVar(&c.authType, "auth-type", "tls", "Server authentication type (tls or oidc)")

	cmd.PreRunE = c.validateArgsAndFlags
	cmd.RunE = c.run

	return cmd
}

func (c *cmdRemoteAdd) validateArgsAndFlags(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	name := args[0]
	addr := args[1]

	if name == "" {
		return fmt.Errorf(`Name of remote can not be empty`)
	}

	if name == "local" {
		return fmt.Errorf(`Name of remote can not be "local", because it is a reserved name for local access through the unix socket`)
	}

	addrURL, err := url.Parse(addr)
	if err != nil {
		return fmt.Errorf(`Provided URL %q is not valid: %v`, addr, err)
	}

	if addrURL.Scheme != "https" {
		return fmt.Errorf(`Provided URL %q is not valid: protocol scheme needs to be https`, addr)
	}

	if config.AuthType(c.authType) != config.AuthTypeTLS && config.AuthType(c.authType) != config.AuthTypeOIDC {
		return fmt.Errorf(`Value for flag "--auth-type" needs to be %q or %q`, config.AuthTypeTLS, config.AuthTypeOIDC)
	}

	return nil
}

func (c *cmdRemoteAdd) run(cmd *cobra.Command, args []string) error {
	name := args[0]
	remote := config.Remote{
		Addr:     args[1],
		AuthType: config.AuthType(c.authType),
	}

	cfg := c.global.config
	_, ok := cfg.Remotes[name]
	if ok {
		return fmt.Errorf(`Remote with name %q already exists`, name)
	}

	if cfg.Remotes == nil {
		cfg.Remotes = map[string]config.Remote{}
	}

	err := c.global.CheckRemoteConnectivity(name, &remote)
	if err != nil {
		return err
	}

	cfg.Remotes[name] = remote
	err = cfg.SaveConfig()
	if err != nil {
		return fmt.Errorf(`Failed to update client config: %v`, err)
	}

	return nil
}

// List remotes.
type cmdRemoteList struct {
	global *CmdGlobal

	flagFormat string
}

func (c *cmdRemoteList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Aliases = []string{"ls"}
	cmd.Short = "List available remotes"
	cmd.Long = `Description:
  List the available remotes
`

	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)

	cmd.PreRunE = c.validateArgsAndFlags
	cmd.RunE = c.run

	return cmd
}

func (c *cmdRemoteList) validateArgsAndFlags(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	return validateFlagFormat(cmd.Flag("format").Value.String())
}

func (c *cmdRemoteList) run(cmd *cobra.Command, args []string) error {
	cfg := c.global.config

	// Render the table.
	header := []string{"Name", "Address", "Auth Type"}
	data := [][]string{}
	localName := "local"
	if cfg.DefaultRemote == "" {
		localName = "local (current)"
	}

	data = append(data, []string{localName, "unix://", "file access"})

	for name, remote := range cfg.Remotes {
		if name == cfg.DefaultRemote {
			name += " (current)"
		}

		data = append(data, []string{name, remote.Addr, string(remote.AuthType)})
	}

	sort.Sort(util.SortColumnsNaturally(data))

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, cfg.Remotes)
}

// Remove remote.
type cmdRemoteRemove struct {
	global *CmdGlobal
}

func (c *cmdRemoteRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remove <name>"
	cmd.Aliases = []string{"rm"}
	cmd.Short = "Remove a remote"
	cmd.Long = `Description:
  Remove a remote

  Removes a remote Migration Manager.
`

	cmd.PreRunE = c.validateArgsAndFlags
	cmd.RunE = c.run

	return cmd
}

func (c *cmdRemoteRemove) validateArgsAndFlags(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	name := args[0]

	if name == "local" {
		return fmt.Errorf(`Remote "local" can not be removed`)
	}

	return nil
}

func (c *cmdRemoteRemove) run(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg := c.global.config
	remote, ok := cfg.Remotes[name]
	if !ok {
		return fmt.Errorf(`Remote with name %q does not exist`, name)
	}

	delete(cfg.Remotes, name)

	if cfg.DefaultRemote == name {
		cfg.DefaultRemote = ""
	}

	if remote.AuthType == config.AuthTypeOIDC {
		err := os.Remove(cfg.OIDCTokenPath(name))
		if err != nil {
			cmd.PrintErrf("Warning: Failed to remove oidc tokens file: %v\n", err)
		}
	}

	err := cfg.SaveConfig()
	if err != nil {
		return fmt.Errorf(`Failed to update client config: %v`, err)
	}

	return nil
}

// Switch remote.
type cmdRemoteSwitch struct {
	global *CmdGlobal
}

func (c *cmdRemoteSwitch) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "switch <name>"
	cmd.Short = "Switch remote"
	cmd.Long = `Description:
  Switch remote

  Switches the default remote Migration Manager that is interacted with.
`

	cmd.PreRunE = c.validateArgsAndFlags
	cmd.RunE = c.run

	return cmd
}

func (c *cmdRemoteSwitch) validateArgsAndFlags(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	return nil
}

func (c *cmdRemoteSwitch) run(cmd *cobra.Command, args []string) error {
	name := args[0]

	if name == "local" {
		name = ""
	}

	cfg := c.global.config
	_, ok := cfg.Remotes[name]
	if !ok && name != "" {
		return fmt.Errorf(`Remote with name %q does not exist`, name)
	}

	cfg.DefaultRemote = name

	err := cfg.SaveConfig()
	if err != nil {
		return fmt.Errorf(`Failed to update client config: %v`, err)
	}

	return nil
}
