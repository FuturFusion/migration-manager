package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddSource(tx *sql.Tx, s source.Source) error {
	// Add source to the database.
	q := `INSERT INTO sources (name,type,insecure,config) VALUES(?,?,?,?)`

	var sourceType api.SourceType // defaults to SOURCETYPE_UNKNOWN
	configString := ""
	isInsecure := false

	switch specificSource := s.(type) {
	case *source.InternalCommonSource:
		sourceType = api.SOURCETYPE_COMMON
		isInsecure = specificSource.Insecure
	case *source.InternalVMwareSource:
		sourceType = api.SOURCETYPE_VMWARE
		marshalled, err := json.Marshal(specificSource.VMwareSourceSpecific)
		if err != nil {
			return err
		}

		configString = string(marshalled)
		isInsecure = specificSource.Insecure
	default:
		return fmt.Errorf("Can only add a Common or VMware source")
	}

	result, err := tx.Exec(q, s.GetName(), sourceType, isInsecure, configString)
	if err != nil {
		return err
	}

	// Set the new ID assigned to the source.
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	switch specificSource := s.(type) {
	case *source.InternalCommonSource:
		specificSource.DatabaseID = int(lastInsertID)
	case *source.InternalVMwareSource:
		specificSource.DatabaseID = int(lastInsertID)
	}

	return nil
}

func (n *Node) GetSource(tx *sql.Tx, name string) (source.Source, error) {
	ret, err := n.getSourcesHelper(tx, name, internal.INVALID_DATABASE_ID)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No source exists with name '%s'", name)
	}

	return ret[0], nil
}

func (n *Node) GetSourceByID(tx *sql.Tx, id int) (source.Source, error) {
	ret, err := n.getSourcesHelper(tx, "", id)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No source exists with ID '%d'", id)
	}

	return ret[0], nil
}

func (n *Node) GetAllSources(tx *sql.Tx) ([]source.Source, error) {
	return n.getSourcesHelper(tx, "", internal.INVALID_DATABASE_ID)
}

func (n *Node) DeleteSource(tx *sql.Tx, name string) error {
	// Verify no instances refer to this source and return a nicer error than 'FOREIGN KEY constraint failed' if so.
	s, err := n.GetSource(tx, name)
	if err != nil {
		return err
	}

	sID, err := s.GetDatabaseID()
	if err != nil {
		return err
	}

	q := `SELECT COUNT(uuid) FROM instances WHERE sourceid=?`
	row := tx.QueryRow(q, sID)

	numInstances := 0
	err = row.Scan(&numInstances)
	if err != nil {
		return err
	}

	if numInstances > 0 {
		return fmt.Errorf("%d instances refer to source '%s', can't delete", numInstances, name)
	}

	// Delete the source from the database.
	q = `DELETE FROM sources WHERE name=?`
	result, err := tx.Exec(q, name)
	if err != nil {
		return err
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

func (n *Node) UpdateSource(tx *sql.Tx, s source.Source) error {
	// Update source in the database.
	q := `UPDATE sources SET name=?,insecure=?,config=? WHERE id=?`

	configString := ""
	isInsecure := false

	switch specificSource := s.(type) {
	case *source.InternalCommonSource:
		isInsecure = specificSource.Insecure
	case *source.InternalVMwareSource:
		marshalled, err := json.Marshal(specificSource.VMwareSourceSpecific)
		if err != nil {
			return err
		}

		configString = string(marshalled)
		isInsecure = specificSource.Insecure
	default:
		return fmt.Errorf("Can only update a Common or VMware source")
	}

	id, err := s.GetDatabaseID()
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, s.GetName(), isInsecure, configString, id)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Source with ID %d doesn't exist, can't update", id)
	}

	return nil
}

func (n *Node) getSourcesHelper(tx *sql.Tx, name string, id int) ([]source.Source, error) {
	ret := []source.Source{}

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
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		err := rows.Scan(&sourceID, &sourceName, &sourceType, &sourceInsecure, &sourceConfig)
		if err != nil {
			return nil, err
		}

		switch sourceType {
		case api.SOURCETYPE_COMMON:
			newSource := source.NewCommonSource(sourceName)
			newSource.DatabaseID = sourceID
			newSource.Insecure = sourceInsecure
			ret = append(ret, newSource)
		case api.SOURCETYPE_VMWARE:
			specificConfig := source.InternalVMwareSourceSpecific{}
			err := json.Unmarshal([]byte(sourceConfig), &specificConfig)
			if err != nil {
				return nil, err
			}

			newSource := source.NewVMwareSource(sourceName, specificConfig.Endpoint, specificConfig.Username, specificConfig.Password)
			newSource.DatabaseID = sourceID
			newSource.Insecure = sourceInsecure
			ret = append(ret, newSource)
		default:
			return nil, fmt.Errorf("Unknown source type %d", sourceType)
		}
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}
