package cmds

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/lxc/incus-os/incus-osd/cli"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdAdmin struct {
	Global *CmdGlobal
}

func (c *CmdAdmin) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "admin"
	cmd.Short = "Manage IncusOS"
	cmd.Long = `Description:
  Manage IncusOS
`

	// os
	adminOSCmd := cmdAdminOS{global: c.Global}

	cmd.AddCommand(adminOSCmd.Command())

	// sql

	adminSQLCmd := cmdAdminSQL{global: c.Global}
	cmd.AddCommand(adminSQLCmd.Command())

	return cmd
}

type cmdAdminOS struct {
	global *CmdGlobal
}

func (c *cmdAdminOS) Command() *cobra.Command {
	args := &cli.Args{
		SupportsTarget:    false,
		SupportsRemote:    false,
		DefaultListFormat: "table",
		DoHTTP: func(_ string, req *http.Request) (*http.Response, error) {
			client, url, err := c.global.buildClient(c.global.GetDefaultRemote().Addr + req.URL.String())
			if err != nil {
				return nil, err
			}

			req.URL = url
			return c.global.requestFunc(client)(req)
		},
	}

	cmd := cli.NewCommand(args)
	preFunc := cmd.PersistentPreRun
	preFuncErr := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if preFuncErr != nil {
			err := preFuncErr(cmd, args)
			if err != nil {
				return err
			}
		} else if preFunc != nil {
			preFunc(cmd, args)
		}

		return c.global.PreRun(cmd, args)
	}

	return cmd
}

type cmdAdminSQL struct {
	global *CmdGlobal

	flagFormat string
}

func (c *cmdAdminSQL) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "sql <query>"
	cmd.Short = "Execute a SQL query against the local database"
	cmd.Long = `Description:
  Execute a SQL query against the local database

  If <query> is the special value "-", then the query is read from
  standard input.

  If <query> is the special value ".dump", the command returns a SQL text
  dump of the given database.

  If <query> is the special value ".schema", the command returns the SQL
  text schema of the given database.

  If <query> is the special value ".tables", the command returns the SQL
  text tables of the given database.

  This internal command is mostly useful for debugging and disaster
  recovery. The development team will occasionally provide hotfixes to users as a
  set of database queries to fix some data inconsistency.
`

	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)

	cmd.PreRunE = c.validateArgsAndFlags
	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminSQL) validateArgsAndFlags(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	return nil
}

func (c *cmdAdminSQL) run(cmd *cobra.Command, args []string) error {
	query := args[0]

	if query == "-" {
		// Read from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("Failed to read from stdin: %w", err)
		}

		query = string(bytes)
	}

	if query == ".dump" || query == ".schema" || query == ".tables" {
		queryParams := url.Values{}
		switch query {
		case ".schema":
			queryParams.Add("dump", string(api.SQLDumpSchema))

		case ".tables":
			queryParams.Add("dump", string(api.SQLDumpTables))
		}

		response, _, err := c.global.doHTTPRequestV1("/internal/sql", http.MethodGet, queryParams.Encode(), nil)
		if err != nil {
			return fmt.Errorf("Failed to request dump: %w", err)
		}

		dumpResult := api.SQLDump{}
		err = json.Unmarshal(response.Metadata, &dumpResult)
		if err != nil {
			return fmt.Errorf("Failed to parse dump response: %w", err)
		}

		fmt.Print(dumpResult.Text)
		return nil
	}

	data := api.SQLQuery{
		Query: query,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	response, _, err := c.global.doHTTPRequestV1("/internal/sql", http.MethodPost, "", b)
	if err != nil {
		return err
	}

	batch := api.SQLBatch{}
	err = json.Unmarshal(response.Metadata, &batch)
	if err != nil {
		return err
	}

	for i, result := range batch.Results {
		if len(batch.Results) > 1 {
			fmt.Printf("=> Query %d:"+"\n\n", i)
		}

		if result.Type == "select" {
			err := c.sqlPrintSelectResult(cmd, result)
			if err != nil {
				return err
			}
		} else {
			fmt.Printf("Rows affected: %d"+"\n", result.RowsAffected)
		}

		if len(batch.Results) > 1 {
			fmt.Println("")
		}
	}

	return nil
}

func (c *cmdAdminSQL) sqlPrintSelectResult(cmd *cobra.Command, result api.SQLResult) error {
	data := make([][]string, 0, len(result.Rows))

	for _, row := range result.Rows {
		rowData := make([]string, 0, len(row))

		for _, col := range row {
			rowData = append(rowData, fmt.Sprintf("%v", col))
		}

		data = append(data, rowData)
	}

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, result.Columns, data, result)
}
