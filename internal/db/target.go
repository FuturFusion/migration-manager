package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/target"
)

func (n *Node) AddTarget(tx *sql.Tx, t target.Target) error {
	incusTarget, ok := t.(*target.InternalIncusTarget)
	if !ok {
		return fmt.Errorf("Only Incus targets are supported")
	}

	// Add target to the database.
	q := `INSERT INTO targets (name,endpoint,tlsclientkey,tlsclientcert,oidctokens,insecure,incusproject,storagepool,bootisoimage,driversisoimage) VALUES(?,?,?,?,?,?,?,?,?,?)`

	marshalledOIDCTokens, err := json.Marshal(incusTarget.OIDCTokens)
	if err != nil {
		return err
	}
	result, err := tx.Exec(q, incusTarget.Name, incusTarget.Endpoint, incusTarget.TLSClientKey, incusTarget.TLSClientCert, marshalledOIDCTokens, incusTarget.Insecure, incusTarget.IncusProject, incusTarget.StoragePool, incusTarget.BootISOImage, incusTarget.DriversISOImage)
	if err != nil {
		return err
	}

	// Set the new ID assigned to the target.
	lastInsertId, err := result.LastInsertId()
	if err != nil {
		return err
	}
	incusTarget.DatabaseID = int(lastInsertId)

	return nil
}

func (n *Node) GetTarget(tx *sql.Tx, name string) (target.Target, error) {
	ret, err := n.getTargetsHelper(tx, name, internal.INVALID_DATABASE_ID)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No target exists with name '%s'", name)
	}

	return ret[0], nil
}

func (n *Node) GetTargetByID(tx *sql.Tx, id int) (target.Target, error) {
	ret, err := n.getTargetsHelper(tx, "", id)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No target exists with ID '%d'", id)
	}

	return ret[0], nil
}

func (n *Node) GetAllTargets(tx *sql.Tx) ([]target.Target, error) {
	return n.getTargetsHelper(tx, "", internal.INVALID_DATABASE_ID)
}

func (n *Node) DeleteTarget(tx *sql.Tx, name string) error {
	// Verify no instances refer to this target and return a nicer error than 'FOREIGN KEY constraint failed' if so.
	t, err := n.GetTarget(tx, name)
	if err != nil {
		return err
	}
	tID, err := t.GetDatabaseID()
	if err != nil {
		return err
	}
	q := `SELECT COUNT(uuid) FROM instances WHERE targetid=?`
	row := tx.QueryRow(q, tID)

	numInstances := 0
	err = row.Scan(&numInstances)
	if err != nil {
		return err
	}
	if numInstances > 0 {
		return fmt.Errorf("%d instances refer to target '%s', can't delete", numInstances, name)
	}

	// Delete the target from the database.
	q = `DELETE FROM targets WHERE name=?`
	result, err := tx.Exec(q, name)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affectedRows == 0 {
		return fmt.Errorf("Target with name '%s' doesn't exist, can't delete", name)
	}

	return nil
}

func (n *Node) UpdateTarget(tx *sql.Tx, t target.Target) error {
	// Update target in the database.
	q := `UPDATE targets SET name=?,endpoint=?,tlsclientkey=?,tlsclientcert=?,oidctokens=?,insecure=?,incusproject=?,storagepool=?,bootisoimage=?,driversisoimage=? WHERE id=?`

	incusTarget, ok := t.(*target.InternalIncusTarget)
	if !ok {
		return fmt.Errorf("Only Incus targets are supported")
	}

	id, err := t.GetDatabaseID()
	if err != nil {
		return err
	}
	marshalledOIDCTokens, err := json.Marshal(incusTarget.OIDCTokens)
	if err != nil {
		return err
	}
	result, err := tx.Exec(q, incusTarget.Name, incusTarget.Endpoint, incusTarget.TLSClientKey, incusTarget.TLSClientCert, marshalledOIDCTokens, incusTarget.Insecure, incusTarget.IncusProject, incusTarget.StoragePool, incusTarget.BootISOImage, incusTarget.DriversISOImage, id)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affectedRows == 0 {
		return fmt.Errorf("Target with ID %d doesn't exist, can't update", id)
	}

	return nil
}

func (n *Node) getTargetsHelper(tx *sql.Tx, name string, id int) ([]target.Target, error) {
	ret := []target.Target{}

	// Get all targets in the database.
	q := `SELECT id,name,endpoint,tlsclientkey,tlsclientcert,oidctokens,insecure,incusproject,storagepool,bootisoimage,driversisoimage FROM targets`
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
		return ret, err
	}

	for rows.Next() {
		newTarget := &target.InternalIncusTarget{}
		marshalledOIDCTokens := ""

		err := rows.Scan(&newTarget.DatabaseID, &newTarget.Name, &newTarget.Endpoint, &newTarget.TLSClientKey, &newTarget.TLSClientCert, &marshalledOIDCTokens, &newTarget.Insecure, &newTarget.IncusProject, &newTarget.StoragePool, &newTarget.BootISOImage, &newTarget.DriversISOImage)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(marshalledOIDCTokens), &newTarget.OIDCTokens)
		if err != nil {
			return nil, err
		}

		ret = append(ret, newTarget)
	}

	return ret, nil
}
