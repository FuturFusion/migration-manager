package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddNetwork(tx *sql.Tx, net *api.Network) error {
	// Add network to the database.
	q := `INSERT INTO networks (name, config) VALUES (?,?)`

	marshalledconfig, err := json.Marshal(net.Config)
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, net.Name, marshalledconfig)
	if err != nil {
		return err
	}

	// Set the new ID assigned to the network.
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	net.DatabaseID = int(lastInsertID)

	return err
}

func (n *Node) GetNetwork(tx *sql.Tx, name string) (api.Network, error) {
	ret, err := n.getNetworksHelper(tx, name)
	if err != nil {
		return api.Network{}, err
	}

	if len(ret) != 1 {
		return api.Network{}, fmt.Errorf("No network '%s'", name)
	}

	return ret[0], nil
}

func (n *Node) GetAllNetworks(tx *sql.Tx) ([]api.Network, error) {
	return n.getNetworksHelper(tx, "")
}

func (n *Node) DeleteNetwork(tx *sql.Tx, name string) error {
	// Delete the network from the database.
	q := `DELETE FROM networks WHERE name=?`
	result, err := tx.Exec(q, name)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("No network '%s' exists, can't delete", name)
	}

	return nil
}

func (n *Node) UpdateNetwork(tx *sql.Tx, net api.Network) error {
	// Update network in the database.
	q := `UPDATE networks SET name=?,config=? WHERE id=?`

	marshalledconfig, err := json.Marshal(net.Config)
	if err != nil {
		return err
	}

	result, err := tx.Exec(q, net.Name, marshalledconfig, net.DatabaseID)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("No network '%s' exists, can't update", net.Name)
	}

	return nil
}

func (n *Node) getNetworksHelper(tx *sql.Tx, name string) ([]api.Network, error) {
	ret := []api.Network{}

	// Get all networks in the database.
	q := `SELECT id,name,config FROM networks`
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
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		netNetwork := api.Network{}
		marshalledConfig := ""

		err := rows.Scan(&netNetwork.DatabaseID, &netNetwork.Name, &marshalledConfig)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(marshalledConfig), &netNetwork.Config)
		if err != nil {
			return nil, err
		}

		ret = append(ret, netNetwork)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ret, nil
}
