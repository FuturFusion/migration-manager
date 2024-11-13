package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/instance"
)

func (n *Node) AddInstance(tx *sql.Tx, i instance.Instance) error {
	internalInstance, ok := i.(*instance.InternalInstance)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalInstance?")
	}

	// Add instance to the database.
	q := `INSERT INTO instances (uuid,migrationstatus,lastupdatefromsource,lastmanualupdate,sourceid,targetid,batchid,name,architecture,os,osversion,disks,nics,numbercpus,memoryinmib,uselegacybios,securebootenabled,tpmpresent) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	marshalledLastUpdateFromSource, err := internalInstance.LastUpdateFromSource.MarshalText()
	if err != nil {
		return err
	}
	marshalledLastManualUpdate, err := internalInstance.LastManualUpdate.MarshalText()
	if err != nil {
		return err
	}
	marshalledDisks, err := json.Marshal(internalInstance.Disks)
	if err != nil {
		return err
	}
	marshalledNICs, err := json.Marshal(internalInstance.NICs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(q, internalInstance.UUID, internalInstance.MigrationStatus, marshalledLastUpdateFromSource, marshalledLastManualUpdate, internalInstance.SourceID, internalInstance.TargetID, internalInstance.BatchID, internalInstance.Name, internalInstance.Architecture, internalInstance.OS, internalInstance.OSVersion, marshalledDisks, marshalledNICs, internalInstance.NumberCPUs, internalInstance.MemoryInMiB, internalInstance.UseLegacyBios, internalInstance.SecureBootEnabled, internalInstance.TPMPresent)

	return err
}

func (n *Node) GetInstance(tx *sql.Tx, UUID uuid.UUID) (instance.Instance, error) {
	ret, err := n.getInstancesHelper(tx, UUID)
	if err != nil {
		return nil, err
	}

	if len(ret) != 1 {
		return nil, fmt.Errorf("No instance exists with UUID '%s'", UUID)
	}

	return ret[0], nil
}

func (n *Node) GetAllInstances(tx *sql.Tx) ([]instance.Instance, error) {
	return n.getInstancesHelper(tx, [16]byte{})
}

func (n *Node) DeleteInstance(tx *sql.Tx, UUID uuid.UUID) error {
	// Delete the instance from the database.
	q := `DELETE FROM instances WHERE uuid=?`
	result, err := tx.Exec(q, UUID)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affectedRows == 0 {
		return fmt.Errorf("Instance with UUID '%s' doesn't exist, can't delete", UUID)
	}

	return nil
}

func (n *Node) UpdateInstance(tx *sql.Tx, i instance.Instance) error {
	// Update instance in the database.
	q := `UPDATE instances SET migrationstatus=?,lastupdatefromsource=?,lastmanualupdate=?,sourceid=?,targetid=?,batchid=?,name=?,architecture=?,os=?,osversion=?,disks=?,nics=?,numbercpus=?,memoryinmib=?,uselegacybios=?,securebootenabled=?,tpmpresent=? WHERE uuid=?`

	internalInstance, ok := i.(*instance.InternalInstance)
	if !ok {
		return fmt.Errorf("Wasn't given an InternalInstance?")
	}

	marshalledLastUpdateFromSource, err := internalInstance.LastUpdateFromSource.MarshalText()
	if err != nil {
		return err
	}
	marshalledLastManualUpdate, err := internalInstance.LastManualUpdate.MarshalText()
	if err != nil {
		return err
	}
	marshalledDisks, err := json.Marshal(internalInstance.Disks)
	if err != nil {
		return err
	}
	marshalledNICs, err := json.Marshal(internalInstance.NICs)
	if err != nil {
		return err
	}
	result, err := tx.Exec(q, internalInstance.MigrationStatus, marshalledLastUpdateFromSource, marshalledLastManualUpdate, internalInstance.SourceID, internalInstance.TargetID, internalInstance.BatchID, internalInstance.Name, internalInstance.Architecture, internalInstance.OS, internalInstance.OSVersion, marshalledDisks, marshalledNICs, internalInstance.NumberCPUs, internalInstance.MemoryInMiB, internalInstance.UseLegacyBios, internalInstance.SecureBootEnabled, internalInstance.TPMPresent, internalInstance.UUID)
	if err != nil {
		return err
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affectedRows == 0 {
		return fmt.Errorf("Instance with ID %d doesn't exist, can't update", internalInstance.UUID)
	}

	return nil
}

func (n *Node) getInstancesHelper(tx *sql.Tx, UUID uuid.UUID) ([]instance.Instance, error) {
	ret := []instance.Instance{}

	// Get all instances in the database.
	q := `SELECT uuid,migrationstatus,lastupdatefromsource,lastmanualupdate,sourceid,targetid,batchid,name,architecture,os,osversion,disks,nics,numbercpus,memoryinmib,uselegacybios,securebootenabled,tpmpresent FROM instances`
	var rows *sql.Rows
	var err error
	if UUID != [16]byte{} {
		q += ` WHERE uuid=?`
		rows, err = tx.Query(q, UUID)
	} else {
		q += ` ORDER BY name`
		rows, err = tx.Query(q)
	}
	if err != nil {
		return ret, err
	}

	for rows.Next() {
		newInstance := &instance.InternalInstance{}
		marshalledLastUpdateFromSource := ""
		marshalledLastManualUpdate := ""
		marshalledDisks := ""
		marshalledNICs := ""

		err := rows.Scan(&newInstance.UUID, &newInstance.MigrationStatus, &marshalledLastUpdateFromSource, &marshalledLastManualUpdate, &newInstance.SourceID, &newInstance.TargetID, &newInstance.BatchID, &newInstance.Name, &newInstance.Architecture, &newInstance.OS, &newInstance.OSVersion, &marshalledDisks, &marshalledNICs, &newInstance.NumberCPUs, &newInstance.MemoryInMiB, &newInstance.UseLegacyBios, &newInstance.SecureBootEnabled, &newInstance.TPMPresent)
		if err != nil {
			return nil, err
		}
		err = newInstance.LastUpdateFromSource.UnmarshalText([]byte(marshalledLastUpdateFromSource))
		if err != nil {
			return nil, err
		}
		err = newInstance.LastManualUpdate.UnmarshalText([]byte(marshalledLastManualUpdate))
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(marshalledDisks), &newInstance.Disks)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(marshalledNICs), &newInstance.NICs)
		if err != nil {
			return nil, err
		}

		ret = append(ret, newInstance)
	}

	return ret, nil
}
