package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddSource(tx *sql.Tx, s api.Source) (api.Source, error) {
	// Add source to the database.
	q := `INSERT INTO sources (name,type,insecure,config) VALUES(?,?,?,?)`

	result, err := tx.Exec(q, s.Name, s.SourceType, s.Insecure, s.Properties)
	if err != nil {
		return api.Source{}, mapDBError(err)
	}

	// Set the new ID assigned to the source.
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return api.Source{}, err
	}

	s.DatabaseID = int(lastInsertID)

	return s, nil
}

func (n *Node) GetSource(tx *sql.Tx, name string) (api.Source, error) {
	ret, err := n.getSourcesHelper(tx, name, internal.INVALID_DATABASE_ID)
	if err != nil {
		return api.Source{}, err
	}

	if len(ret) != 1 {
		return api.Source{}, fmt.Errorf("No source exists with name '%s'", name)
	}

	return ret[0], nil
}

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

func (n *Node) GetAllSources(tx *sql.Tx) ([]api.Source, error) {
	return n.getSourcesHelper(tx, "", internal.INVALID_DATABASE_ID)
}

func (n *Node) DeleteSource(tx *sql.Tx, name string) error {
	// Verify no instances refer to this source and return a nicer error than 'FOREIGN KEY constraint failed' if so.
	s, err := n.GetSource(tx, name)
	if err != nil {
		return err
	}

	q := `SELECT COUNT(uuid) FROM instances WHERE source_id=?`
	row := tx.QueryRow(q, s.DatabaseID)

	numInstances := 0
	err = row.Scan(&numInstances)
	if err != nil {
		return mapDBError(err)
	}

	if numInstances > 0 {
		return fmt.Errorf("%d instances refer to source '%s', can't delete", numInstances, name)
	}

	// Delete the source from the database.
	q = `DELETE FROM sources WHERE name=?`
	result, err := tx.Exec(q, name)
	if err != nil {
		return mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Source with name '%s' doesn't exist, can't delete", name)
	}

	return nil
}

func (n *Node) UpdateSource(tx *sql.Tx, s api.Source) (api.Source, error) {
	// Update source in the database.
	q := `UPDATE sources SET name=?,insecure=?,config=? WHERE id=?`

	result, err := tx.Exec(q, s.Name, s.Insecure, s.Properties, s.DatabaseID)
	if err != nil {
		return api.Source{}, mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return api.Source{}, err
	}

	if affectedRows == 0 {
		return api.Source{}, fmt.Errorf("Source with ID %d doesn't exist, can't update", s.DatabaseID)
	}

	return s, nil
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
