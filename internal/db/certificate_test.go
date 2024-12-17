package db_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbdriver "github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var (
	certificateA = api.Certificate{Name: "CertA", Type: api.CertificateTypeClient, Certificate: "<<cert data>>", Description: "CertA description", Fingerprint: "123456"}
	certificateB = api.Certificate{Name: "CertB", Type: api.CertificateTypeClient, Certificate: "<<cert data>>", Description: "CertB description", Fingerprint: "123478"}
	certificateC = api.Certificate{Name: "CertC", Type: api.CertificateTypeClient, Certificate: "<<cert data>>", Description: "CertC description", Fingerprint: "654321"}
)

func TestBatchCertificateActions(t *testing.T) {
	// Create a new temporary database.
	tmpDir := t.TempDir()
	db, err := dbdriver.OpenDatabase(tmpDir)
	require.NoError(t, err)

	// Start a transaction.
	tx, err := db.DB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Add certificateA.
	err = db.AddCertificate(tx, certificateA)
	require.NoError(t, err)

	// Add certificateB.
	err = db.AddCertificate(tx, certificateB)
	require.NoError(t, err)

	// Add certificateC.
	err = db.AddCertificate(tx, certificateC)
	require.NoError(t, err)

	// Ensure we have three certificates.
	certs, err := db.GetAllCertificates(tx)
	require.NoError(t, err)
	require.Len(t, certs, 3)

	// Should get back certificateA unchanged.
	dbCertificateA, err := db.GetCertificateByFingerprintPrefix(tx, certificateA.Fingerprint)
	require.NoError(t, err)
	require.Equal(t, certificateA, dbCertificateA)

	// Delete a certificate.
	err = db.DeleteCertificate(tx, certificateC.Fingerprint)
	require.NoError(t, err)
	_, err = db.GetBatch(tx, certificateC.Fingerprint)
	require.Error(t, err)

	// Should get an error if prefix matches more than one certificate.
	_, err = db.GetCertificateByFingerprintPrefix(tx, "1234")
	require.Error(t, err)

	// Can't get a certificate that doesn't exist.
	_, err = db.GetCertificateByFingerprintPrefix(tx, "fedcba")
	require.Error(t, err)

	// Can't delete a certificate that doesn't exist.
	err = db.DeleteCertificate(tx, "abcdef")
	require.Error(t, err)

	// Can't add a duplicate certificate.
	err = db.AddCertificate(tx, certificateA)
	require.Error(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)
}
