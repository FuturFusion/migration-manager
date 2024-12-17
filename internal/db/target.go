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
	q := `INSERT INTO targets (name,endpoint,tls_client_key,tls_client_cert,oidc_tokens,insecure,incus_project) VALUES(?,?,?,?,?,?,?)`

	marshalledOIDCTokens, err := json.Marshal(incusTarget.OIDCTokens)
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, incusTarget.Name, incusTarget.Endpoint, incusTarget.TLSClientKey, incusTarget.TLSClientCert, marshalledOIDCTokens, incusTarget.Insecure, incusTarget.IncusProject)
	if err != nil {
		return mapDBError(err)
	}

	// Set the new ID assigned to the target.
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	incusTarget.DatabaseID = int(lastInsertID)

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
	// Verify no instances refer to this target.
	t, err := n.GetTarget(tx, name)
	if err != nil {
		return err
	}

	tID, err := t.GetDatabaseID()
	if err != nil {
		return err
	}

	q := `SELECT COUNT(uuid) FROM instances WHERE target_id=?`
	row := tx.QueryRow(q, tID)

	numInstances := 0
	err = row.Scan(&numInstances)
	if err != nil {
		return mapDBError(err)
	}

	if numInstances > 0 {
		return fmt.Errorf("%d instances refer to target '%s', can't delete", numInstances, name)
	}

	// Verify no batches refer to this target.
	q = `SELECT COUNT(id) FROM batches WHERE target_id=?`
	row = tx.QueryRow(q, tID)

	numBatches := 0
	err = row.Scan(&numBatches)
	if err != nil {
		return mapDBError(err)
	}

	if numBatches > 0 {
		return fmt.Errorf("%d batches refer to target '%s', can't delete", numBatches, name)
	}

	// Delete the target from the database.
	q = `DELETE FROM targets WHERE name=?`
	result, err := tx.Exec(q, name)
	if err != nil {
		return mapDBError(err)
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
	q := `UPDATE targets SET name=?,endpoint=?,tls_client_key=?,tls_client_cert=?,oidc_tokens=?,insecure=?,incus_project=? WHERE id=?`

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

	result, err := tx.Exec(q, incusTarget.Name, incusTarget.Endpoint, incusTarget.TLSClientKey, incusTarget.TLSClientCert, marshalledOIDCTokens, incusTarget.Insecure, incusTarget.IncusProject, id)
	if err != nil {
		return mapDBError(err)
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
	q := `SELECT id,name,endpoint,tls_client_key,tls_client_cert,oidc_tokens,insecure,incus_project FROM targets`
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
		newTarget := &target.InternalIncusTarget{}
		marshalledOIDCTokens := ""

		err := rows.Scan(&newTarget.DatabaseID, &newTarget.Name, &newTarget.Endpoint, &newTarget.TLSClientKey, &newTarget.TLSClientCert, &marshalledOIDCTokens, &newTarget.Insecure, &newTarget.IncusProject)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(marshalledOIDCTokens), &newTarget.OIDCTokens)
		if err != nil {
			return nil, err
		}

		ret = append(ret, newTarget)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}
