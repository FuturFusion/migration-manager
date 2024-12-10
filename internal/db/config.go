package db

import (
	"database/sql"
	"encoding/json"
	"errors"
)

func (n *Node) ReadGlobalConfig(tx *sql.Tx) (map[string]string, error) {
	ret := make(map[string]string)

	q := `SELECT global_config FROM config WHERE id=0`
	row := tx.QueryRow(q)

	marshalledConfig := ""
	err := row.Scan(&marshalledConfig)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ret, nil
		}

		return ret, err
	}

	err = json.Unmarshal([]byte(marshalledConfig), &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func (n *Node) WriteGlobalConfig(tx *sql.Tx, config map[string]string) error {
	q := `INSERT OR REPLACE INTO config (id, global_config) VALUES(0, ?)`

	marshalledConfig, err := json.Marshal(config)
	if err != nil {
		return err
	}

	_, err = tx.Exec(q, marshalledConfig)
	return err
}
