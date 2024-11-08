package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/source"
)

func (n *Node) AddSource(tx *sql.Tx, s source.Source) error {
	// Add source to the database.
	q := `INSERT INTO sources (name,type,config) VALUES(?,?,?)`

	sourceType := source.SOURCETYPE_UNKNOWN
	configString := ""

	switch specificSource := s.(type) {
	case *source.InternalCommonSource:
		sourceType = source.SOURCETYPE_COMMON
	case *source.InternalVMwareSource:
		sourceType = source.SOURCETYPE_VMWARE
		marshalled, err := json.Marshal(specificSource.VMwareSourceSpecific)
		if err != nil {
			return err
		}
		configString = string(marshalled)
	default:
		return fmt.Errorf("Can only add a Common or VMware source")
	}

	result, err := tx.Exec(q, s.GetName(), sourceType, configString)
	if err != nil {
		return err
	}

	// Set the new ID assigned to the source.
	lastInsertId, err := result.LastInsertId()
	if err != nil {
		return err
	}
	switch specificSource := s.(type) {
	case *source.InternalCommonSource:
		specificSource.DatabaseID = int(lastInsertId)
	case *source.InternalVMwareSource:
		specificSource.DatabaseID = int(lastInsertId)
	}

	return nil
}

func (n *Node) GetSource(tx *sql.Tx, name string) (source.Source, error) {
	ret, err := n.getSourcesHelper(tx, name)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No source exists with name '%s'", name)
	}

	return ret[0], nil
}

func (n *Node) GetAllSources(tx *sql.Tx) ([]source.Source, error) {
	return n.getSourcesHelper(tx, "")
}

func (n *Node) DeleteSource(tx *sql.Tx, name string) error {
	// Delete the source from the database.
	q := `DELETE FROM sources WHERE name=?`
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
	q := `UPDATE sources SET name=?,config=? WHERE id=?`

	configString := ""

	switch specificSource := s.(type) {
	case *source.InternalCommonSource:
	case *source.InternalVMwareSource:
		marshalled, err := json.Marshal(specificSource.VMwareSourceSpecific)
		if err != nil {
			return err
		}
		configString = string(marshalled)
	default:
		return fmt.Errorf("Can only update a Common or VMware source")
	}

	id, err := s.GetDatabaseID()
	if err != nil {
		return err
	}
	result, err := tx.Exec(q, s.GetName(), configString, id)
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

func (n *Node) getSourcesHelper(tx *sql.Tx, name string) ([]source.Source, error) {
	ret := []source.Source{}

	sourceID := internal.INVALID_DATABASE_ID
	sourceName := ""
	sourceType := source.SOURCETYPE_UNKNOWN
	sourceConfig := "" 

	// Get all sources in the database.
	q := `SELECT id,name,type,config FROM sources`
	var rows *sql.Rows
	var err error
	if name != "" {
		q += ` WHERE name=?`
		rows, err = tx.Query(q, name)
	} else {
		q += ` ORDER BY name`
		rows, err = tx.Query(q)
	}
	if err != nil {
		return ret, err
	}

	for rows.Next() {
		err := rows.Scan(&sourceID, &sourceName, &sourceType, &sourceConfig)
		if err != nil {
			return nil, err
		}

		switch sourceType {
		case source.SOURCETYPE_COMMON:
			newSource := source.NewCommonSource(sourceName)
			newSource.DatabaseID = sourceID
			ret = append(ret, newSource)
		case source.SOURCETYPE_VMWARE:
			specificConfig := source.InternalVMwareSourceSpecific{}
			err := json.Unmarshal([]byte(sourceConfig), &specificConfig)
			if err != nil {
				return nil, err
			}
			newSource := source.NewVMwareSource(sourceName, specificConfig.Endpoint, specificConfig.Username, specificConfig.Password, specificConfig.Insecure)
			newSource.DatabaseID = sourceID
			ret = append(ret, newSource)
		default:
			return nil, fmt.Errorf("Unknown source type %d", sourceType)
		}
	}

	return ret, nil
}
