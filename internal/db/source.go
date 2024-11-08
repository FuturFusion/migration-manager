package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/source"
)

const ALL_SOURCES int = -1

func (n *Node) AddSource(tx *sql.Tx, s source.Source) error {
	// Add source to the database.
	q := `INSERT INTO sources (name,type,config) VALUES(?,?,?)`

	sourceType := source.SOURCETYPE_UNKNOWN
	configString := ""

	switch specificSource := s.(type) {
	case *source.CommonSource:
		sourceType = source.SOURCETYPE_COMMON
	case *source.VMwareSource:
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
	case *source.CommonSource:
		specificSource.DatabaseID = int(lastInsertId)
	case *source.VMwareSource:
		specificSource.DatabaseID = int(lastInsertId)
	}

	return nil
}

func (n *Node) GetSource(tx *sql.Tx, id int) (source.Source, error) {
	ret, err := n.getSourcesHelper(tx, id)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No source exists with ID %d", id)
	}

	return ret[0], nil
}

func (n *Node) GetAllSources(tx *sql.Tx) ([]source.Source, error) {
	return n.getSourcesHelper(tx, ALL_SOURCES)
}

func (n *Node) DeleteSource(tx *sql.Tx, id int) error {
	// Delete the source from the database.
	q := `DELETE FROM sources WHERE id=?`
	result, err := tx.Exec(q, id)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affectedRows == 0 {
		return fmt.Errorf("Source with ID %d doesn't exist, can't delete", id)
	}

	return nil
}

func (n *Node) UpdateSource(tx *sql.Tx, s source.Source) error {
	// Update source in the database.
	q := `UPDATE sources SET name=?,config=? WHERE id=?`

	configString := ""

	switch specificSource := s.(type) {
	case *source.CommonSource:
	case *source.VMwareSource:
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

func (n *Node) getSourcesHelper(tx *sql.Tx, id int) ([]source.Source, error) {
	ret := []source.Source{}

	sourceID := internal.INVALID_DATABASE_ID
	sourceName := ""
	sourceType := source.SOURCETYPE_UNKNOWN
	sourceConfig := "" 

	// Get all sources in the database.
	q := `SELECT id,name,type,config FROM sources`
	var rows *sql.Rows
	var err error
	if id != ALL_SOURCES {
		q += ` WHERE id=?`
		rows, err = tx.Query(q, id)
	} else {
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
			specificConfig := source.VMwareSourceSpecific{}
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
