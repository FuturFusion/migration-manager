package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal/source"
)

func (n *Node) AddSource(s source.Source) error {
	tx, err := n.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

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
	s.SetDatabaseID(int(lastInsertId))

	tx.Commit()
	return nil
}

func (n *Node) GetSource(id int) (source.Source, error) {
	ret, err := n.getSourcesHelper(id)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No source exists with ID %d", id)
	}

	return ret[0], nil
}

func (n *Node) GetAllSources() ([]source.Source, error) {
	return n.getSourcesHelper(-1)
}

func (n *Node) DeleteSource(id int) error {
	tx, err := n.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

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

	tx.Commit()
	return nil
}

func (n *Node) UpdateSource(s source.Source) error {
	tx, err := n.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update source in the database.
	q := `UPDATE sources SET name=?, config=? WHERE id=?`

	configString := ""

	switch specificSource := s.(type) {
	case *source.VMwareSource:
		marshalled, err := json.Marshal(specificSource.VMwareSourceSpecific)
		if err != nil {
			return err
		}
		configString = string(marshalled)
	default:
		return fmt.Errorf("Can only update a Common or VMware source")
	}

	_, err = tx.Exec(q, s.GetName(), configString, s.GetDatabaseID())
	if err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (n *Node) getSourcesHelper(id int) ([]source.Source, error) {
	ret := []source.Source{}

	tx, err := n.db.Begin()
	if err != nil {
		return ret, err
	}
	defer tx.Rollback()

	sourceID := -1
	sourceName := ""
	sourceType := source.SOURCETYPE_UNKNOWN
	sourceConfig := "" 

	// Get all sources in the database.
	q := `SELECT id,name,type,config FROM sources`
	var rows *sql.Rows
	if id != -1 {
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
			return []source.Source{}, err
		}

		switch sourceType {
		case source.SOURCETYPE_COMMON:
			newSource := &source.CommonSource{Name: sourceName, DatabaseID: sourceID}
			ret = append(ret, newSource)
		case source.SOURCETYPE_VMWARE:
			specificConfig := source.VMwareSourceSpecific{}
			err := json.Unmarshal([]byte(sourceConfig), &specificConfig)
			if err != nil {
				return []source.Source{}, err
			}
			newSource := source.NewVMwareSource(sourceName, specificConfig.Endpoint, specificConfig.Username, specificConfig.Password, specificConfig.Insecure)
			newSource.SetDatabaseID(sourceID)
			ret = append(ret, newSource)
		default:
			return []source.Source{}, fmt.Errorf("Unknown source type %d", sourceType)
		}
	}

	tx.Commit()
	return ret, nil
}
