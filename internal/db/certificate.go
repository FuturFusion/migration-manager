package db

import (
	"database/sql"
	"fmt"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func (n *Node) AddCertificate(tx *sql.Tx, c api.Certificate) error {
	// Add certificate to the database.
	q := `INSERT INTO certificates (name,type,certificate,description,fingerprint) VALUES(?,?,?,?,?)`

	_, err := tx.Exec(q, c.Name, c.Type, c.Certificate, c.Description, c.Fingerprint)
	if err != nil {
		return mapDBError(err)
	}

	return nil
}

func (n *Node) GetAllCertificates(tx *sql.Tx) ([]api.Certificate, error) {
	// Get all certificates from the database.
	q := "SELECT fingerprint, type, name, description, certificate FROM certificates"
	rows, err := tx.Query(q)
	if err != nil {
		return nil, mapDBError(err)
	}

	defer func() { _ = rows.Close() }()

	certs := []api.Certificate{}
	for rows.Next() {
		cert := api.Certificate{}

		err := rows.Scan(&cert.Fingerprint, &cert.Type, &cert.Name, &cert.Description, &cert.Certificate)
		if err != nil {
			return nil, mapDBError(err)
		}

		certs = append(certs, cert)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return certs, nil
}

// GetCertificateByFingerprintPrefix gets an CertBaseInfo object from the database.
// The argument fingerprint will be queried with a LIKE query, means you can
// pass a shortform and will get the full fingerprint.
// There can never be more than one certificate with a given fingerprint, as it is
// enforced by a UNIQUE constraint in the schema.
func (n *Node) GetCertificateByFingerprintPrefix(tx *sql.Tx, fingerprintPrefix string) (api.Certificate, error) {
	ret := api.Certificate{}

	q := `SELECT fingerprint, type, name, description, certificate FROM certificates WHERE fingerprint LIKE ? ORDER BY fingerprint`
	rows, err := tx.Query(q, fingerprintPrefix+"%")
	if err != nil {
		return ret, mapDBError(err)
	}

	defer func() { _ = rows.Close() }()

	numCerts := 0
	for rows.Next() {
		if numCerts > 0 {
			return ret, fmt.Errorf("More than one certificate matches fingerprint prefix '%s'", fingerprintPrefix)
		}

		err := rows.Scan(&ret.Fingerprint, &ret.Type, &ret.Name, &ret.Description, &ret.Certificate)
		if err != nil {
			return ret, mapDBError(err)
		}

		numCerts++
	}

	if rows.Err() != nil {
		return ret, rows.Err()
	}

	if numCerts == 0 {
		return ret, fmt.Errorf("No certificate exists with fingerprint prefix '%s'", fingerprintPrefix)
	}

	return ret, nil
}

func (n *Node) DeleteCertificate(tx *sql.Tx, fingerprint string) error {
	cert, err := n.GetCertificateByFingerprintPrefix(tx, fingerprint)
	if err != nil {
		return err
	}

	// Delete the certificate from the database.
	q := `DELETE FROM certificates WHERE fingerprint=?`
	result, err := tx.Exec(q, cert.Fingerprint)
	if err != nil {
		return mapDBError(err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("Certificate with fingerprint '%s' doesn't exist, can't delete", fingerprint)
	}

	return nil
}
