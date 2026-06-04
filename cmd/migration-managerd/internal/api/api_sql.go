package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var sqlCmd = APIEndpoint{
	Path: "sql",

	Get:  APIEndpointAction{Handler: sqlGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit), Authenticator: UnixAuthenticate},
	Post: APIEndpointAction{Handler: sqlPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit), Authenticator: UnixAuthenticate},
}

func sqlGet(d *Daemon, r *http.Request) response.Response {
	dumpOption := api.SQLDumpOption(r.FormValue("dump"))
	if dumpOption == "" {
		dumpOption = api.SQLDumpDefault
	}

	var dumpResult string
	err := transaction.Do(r.Context(), func(ctx context.Context) error {
		var err error

		switch dumpOption {
		case api.SQLDumpDefault:
			dumpResult, err = dumpSchema(ctx, d.DBTX(), false)

		case api.SQLDumpSchema:
			dumpResult, err = dumpSchema(ctx, d.DBTX(), true)

		case api.SQLDumpOption(api.TARGETTYPE_INCUS):
			dumpResult, err = dumpTables(ctx, d.DBTX())

		default:
			return fmt.Errorf("Failed to perform dump due to missing dump option")
		}

		return err
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed dump database: %w", err))
	}

	return response.SyncResponse(true, api.SQLDump{Text: dumpResult})
}

func sqlPost(d *Daemon, r *http.Request) response.Response {
	ctx := r.Context()

	req := &api.SQLQuery{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	if req.Query == "" {
		return response.BadRequest(errors.New("No query provided"))
	}

	batch := api.SQLBatch{}
	for query := range strings.SplitSeq(req.Query, ";") {
		query = strings.TrimLeft(query, " ")

		if query == "" {
			continue
		}

		result := api.SQLResult{}

		err := transaction.Do(ctx, func(ctx context.Context) error {
			if strings.HasPrefix(strings.ToUpper(query), "SELECT") {
				err = internalSQLSelect(ctx, d.DBTX(), query, &result)
			} else {
				err = internalSQLExec(ctx, d.DBTX(), query, &result)
			}

			if err != nil {
				return err
			}

			batch.Results = append(batch.Results, result)

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}
	}

	return response.SyncResponse(true, batch)
}

// dumpTables returns a SQL text dump of all table's name, similar to
// sqlite3's dump feature.
func dumpTables(ctx context.Context, db transaction.DBTX) (string, error) {
	_, entityNames, err := getEntitiesSchemas(ctx, db)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, tableName := range entityNames {
		builder.WriteString(tableName + "\n")
	}

	return builder.String(), nil
}

// dumpSchema returns a SQL text dump of all rows across all tables, similar to
// sqlite3's dump feature.
func dumpSchema(ctx context.Context, db transaction.DBTX, schemaOnly bool) (string, error) {
	entitiesSchemas, entityNames, err := getEntitiesSchemas(ctx, db)
	if err != nil {
		return "", err
	}

	// Begin dump string.
	var builder strings.Builder
	builder.WriteString("PRAGMA foreign_keys=OFF;\n")
	builder.WriteString("BEGIN TRANSACTION;\n")

	// For each table, write the schema and optionally write the data.
	for _, tableName := range entityNames {
		builder.WriteString(entitiesSchemas[tableName][1] + "\n")

		if !schemaOnly && entitiesSchemas[tableName][0] == "table" {
			tableData, err := getTableData(ctx, db, tableName)
			if err != nil {
				return "", err
			}

			for _, stmt := range tableData {
				builder.WriteString(stmt + "\n")
			}
		}
	}

	// Sequences (unless the schemaOnly flag is true).
	if !schemaOnly {
		builder.WriteString("DELETE FROM sqlite_sequence;\n")

		tableData, err := getTableData(ctx, db, "sqlite_sequence")
		if err != nil {
			return "", fmt.Errorf("Failed to dump table sqlite_sequence: %w", err)
		}

		for _, stmt := range tableData {
			builder.WriteString(stmt + "\n")
		}
	}

	// Commit.
	builder.WriteString("COMMIT;\n")

	return builder.String(), nil
}

// getEntitiesSchemas gets all the tables, their kind, and their schema, as well as a list of entity names in their default order from
// the sqlite_master table. The returned map values are arrays of length 2 whose first element contains the entity type and the second
// contains it's schema.
func getEntitiesSchemas(ctx context.Context, db transaction.DBTX) (map[string][2]string, []string, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, type, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY rowid`)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get table names and their schema: %w", err)
	}

	defer func() { _ = rows.Close() }()

	err = rows.Err()
	if err != nil {
		return nil, nil, err
	}

	tablesSchemas := make(map[string][2]string)
	var names []string
	for rows.Next() {
		var name string
		var kind string
		var schema string
		err := rows.Scan(&name, &kind, &schema)
		if err != nil {
			return nil, nil, fmt.Errorf("Could not scan table name and schema: %w", err)
		}

		// This is based on logic from dump_callback in sqlite source for sqlite3_db_dump function.
		if strings.HasPrefix(schema, `CREATE TABLE "`) {
			schema = strings.Replace(schema, "CREATE TABLE", "CREATE TABLE IF NOT EXISTS", 1)
		}

		names = append(names, name)
		tablesSchemas[name] = [2]string{kind, schema + ";"}
	}

	return tablesSchemas, names, nil
}

// getTableData gets all the data for a single table, returning a string slice where each element is an insert statement
// for the data.
func getTableData(ctx context.Context, db transaction.DBTX, table string) ([]string, error) {
	var statements []string

	// Query all rows.
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s ORDER BY rowid", table))
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch rows for table %q: %w", table, err)
	}

	defer func() { _ = rows.Close() }()

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	// Get the column names.
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("Failed to get columns for table %q: %w", table, err)
	}

	// Generate an INSERT statement for each row.
	for i := 0; rows.Next(); i++ {
		raw := make([]any, len(columns)) // Raw column values
		row := make([]any, len(columns))
		for i := range raw {
			row[i] = &raw[i]
		}

		err := rows.Scan(row...)
		if err != nil {
			return nil, fmt.Errorf("Failed to scan row %d in table %q: %w", i, table, err)
		}

		values := make([]string, len(columns))
		for j, v := range raw {
			switch v := v.(type) {
			case int64:
				values[j] = strconv.FormatInt(v, 10)

			case string:
				// This is based on logic from dump_callback in sqlite source for sqlite3_db_dump function.
				v = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))

				if strings.Contains(v, "\r") {
					v = "replace(" + strings.ReplaceAll(v, "\r", "\\r") + ",'\\r',char(13))"
				}

				if strings.Contains(v, "\n") {
					v = "replace(" + strings.ReplaceAll(v, "\n", "\\n") + ",'\\n',char(10))"
				}

				values[j] = v

			case []byte:
				values[j] = fmt.Sprintf("'%s'", string(v))

			case time.Time:
				// Try and match the sqlite3 .dump output format.
				format := "2006-01-02 15:04:05"

				if v.Nanosecond() > 0 {
					format = format + ".000000000"
				}

				format = format + "-07:00"

				values[j] = "'" + v.Format(format) + "'"

			default:
				if v != nil {
					return nil, fmt.Errorf("Bad type in column %q of row %d in table %q", columns[j], i, table)
				}

				values[j] = "NULL"
			}
		}

		statement := fmt.Sprintf("INSERT INTO %s VALUES(%s);", table, strings.Join(values, ","))
		statements = append(statements, statement)
	}

	return statements, nil
}

func internalSQLSelect(ctx context.Context, db transaction.DBTX, query string, result *api.SQLResult) error {
	result.Type = "select"

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("Failed to execute query: %w", err)
	}

	defer func() { _ = rows.Close() }()

	result.Columns, err = rows.Columns()
	if err != nil {
		return fmt.Errorf("Failed to fetch column names: %w", err)
	}

	for rows.Next() {
		row := make([]any, len(result.Columns))
		rowPointers := make([]any, len(result.Columns))
		for i := range row {
			rowPointers[i] = &row[i]
		}

		err := rows.Scan(rowPointers...)
		if err != nil {
			return fmt.Errorf("Failed to scan row: %w", err)
		}

		for i, column := range row {
			// Convert bytes to string. This is safe as
			// long as we don't have any BLOB column type.
			data, ok := column.([]byte)
			if ok {
				row[i] = string(data)
			}
		}

		result.Rows = append(result.Rows, row)
	}

	err = rows.Err()
	if err != nil {
		return fmt.Errorf("Got a row error: %w", err)
	}

	return nil
}

func internalSQLExec(ctx context.Context, db transaction.DBTX, query string, result *api.SQLResult) error {
	result.Type = "exec"
	r, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("Failed to exec query: %w", err)
	}

	result.RowsAffected, err = r.RowsAffected()
	if err != nil {
		return fmt.Errorf("Failed to fetch affected rows: %w", err)
	}

	return nil
}
