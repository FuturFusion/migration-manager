package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal/target"
)

const ALL_TARGETS int = -1

func (n *Node) AddTarget(t target.Target) error {
	tx, err := n.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	incusTarget, ok := t.(*target.IncusTarget)
	if !ok {
		return fmt.Errorf("Only Incus targets are supported")
	}

	// Add target to the database.
	q := `INSERT INTO targets (name,endpoint,tlsclientkey,tlsclientcert,oidctokens,insecure,incusprofile,incusproject) VALUES(?,?,?,?,?,?,?,?)`

	marshalledOIDCTokens, err := json.Marshal(incusTarget.OIDCTokens)
	if err != nil {
		return err
	}
	result, err := tx.Exec(q, incusTarget.Name, incusTarget.Endpoint, incusTarget.TLSClientKey, incusTarget.TLSClientCert, marshalledOIDCTokens, incusTarget.Insecure, incusTarget.IncusProfile, incusTarget.IncusProject)
	if err != nil {
		return err
	}

	// Set the new ID assigned to the target.
	lastInsertId, err := result.LastInsertId()
	if err != nil {
		return err
	}
	t.SetDatabaseID(int(lastInsertId))

	tx.Commit()
	return nil
}

func (n *Node) GetTarget(id int) (target.Target, error) {
	ret, err := n.getTargetsHelper(id)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No target exists with ID %d", id)
	}

	return ret[0], nil
}

func (n *Node) GetAllTargets() ([]target.Target, error) {
	return n.getTargetsHelper(ALL_TARGETS)
}

func (n *Node) DeleteTarget(id int) error {
	tx, err := n.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete the target from the database.
	q := `DELETE FROM targets WHERE id=?`
	result, err := tx.Exec(q, id)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affectedRows == 0 {
		return fmt.Errorf("Target with ID %d doesn't exist, can't delete", id)
	}

	tx.Commit()
	return nil
}

func (n *Node) UpdateTarget(t target.Target) error {
	tx, err := n.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update target in the database.
	q := `UPDATE targets SET name=?,endpoint=?,tlsclientkey=?,tlsclientcert=?,oidctokens=?,insecure=?,incusprofile=?,incusproject=? WHERE id=?`

	incusTarget, ok := t.(*target.IncusTarget)
	if !ok {
		return fmt.Errorf("Only Incus targets are supported")
	}

	marshalledOIDCTokens, err := json.Marshal(incusTarget.OIDCTokens)
	if err != nil {
		return err
	}
	result, err := tx.Exec(q, incusTarget.Name, incusTarget.Endpoint, incusTarget.TLSClientKey, incusTarget.TLSClientCert, marshalledOIDCTokens, incusTarget.Insecure, incusTarget.IncusProfile, incusTarget.IncusProject, t.GetDatabaseID())
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affectedRows == 0 {
		return fmt.Errorf("Target with ID %d doesn't exist, can't update", t.GetDatabaseID())
	}

	tx.Commit()
	return nil
}

func (n *Node) getTargetsHelper(id int) ([]target.Target, error) {
	ret := []target.Target{}

	tx, err := n.db.Begin()
	if err != nil {
		return ret, err
	}
	defer tx.Rollback()

	// Get all targets in the database.
	q := `SELECT id,name,endpoint,tlsclientkey,tlsclientcert,oidctokens,insecure,incusprofile,incusproject FROM targets`
	var rows *sql.Rows
	if id != ALL_TARGETS {
		q += ` WHERE id=?`
		rows, err = tx.Query(q, id)
	} else {
		rows, err = tx.Query(q)
	}
	if err != nil {
		return ret, err
	}

	for rows.Next() {
		newTarget := &target.IncusTarget{}
		marshalledOIDCTokens := ""

		err := rows.Scan(&newTarget.DatabaseID, &newTarget.Name, &newTarget.Endpoint, &newTarget.TLSClientKey, &newTarget.TLSClientCert, &marshalledOIDCTokens, &newTarget.Insecure, &newTarget.IncusProfile, &newTarget.IncusProject)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(marshalledOIDCTokens), &newTarget.OIDCTokens)
		if err != nil {
			return nil, err
		}

		ret = append(ret, newTarget)
	}

	tx.Commit()
	return ret, nil
}
