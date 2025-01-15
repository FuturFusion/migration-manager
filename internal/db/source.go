package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) GetSourceByID(tx *sql.Tx, id int) (api.Source, error) {
	ret, err := n.getSourcesHelper(tx, "", id)
	if err != nil {
		return api.Source{}, err
	}

	if len(ret) != 1 {
		return api.Source{}, fmt.Errorf("No source exists with ID '%d'", id)
	}

	return ret[0], nil
}

func (n *Node) getSourcesHelper(tx *sql.Tx, name string, id int) ([]api.Source, error) {
	ret := []api.Source{}

	sourceID := internal.INVALID_DATABASE_ID
	sourceName := ""
	sourceType := api.SOURCETYPE_UNKNOWN
	sourceInsecure := false
	sourceConfig := ""

	// Get all sources in the database.
	q := `SELECT id,name,type,insecure,config FROM sources`
	var rows *sql.Rows
	var err error
	if name != "" {
		q += ` WHERE name=?`
		rows, err = tx.Query(q, name)
	} else if id != internal.INVALID_DATABASE_ID {
		q += ` WHERE id=?`
		rows, err = tx.Query(q, id)
	} else {
		q += ` ORDER BY name`
		rows, err = tx.Query(q)
	}

	if err != nil {
		return nil, mapDBError(err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		err := rows.Scan(&sourceID, &sourceName, &sourceType, &sourceInsecure, &sourceConfig)
		if err != nil {
			return nil, err
		}

		ret = append(ret, api.Source{
			DatabaseID: sourceID,
			Name:       sourceName,
			Insecure:   sourceInsecure,
			SourceType: sourceType,
			Properties: json.RawMessage(sourceConfig),
		})
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}
