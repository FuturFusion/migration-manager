package api

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
	"github.com/lxc/incus/v6/shared/logger"
	localtls "github.com/lxc/incus/v6/shared/tls"

	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var certificatesCmd = APIEndpoint{
	Path: "certificates",

	Get:  APIEndpointAction{Handler: certificatesGet, AllowUntrusted: true},
	Post: APIEndpointAction{Handler: certificatesPost, AllowUntrusted: true},
}

var certificateCmd = APIEndpoint{
	Path: "certificates/{fingerprint}",

	Delete: APIEndpointAction{Handler: certificateDelete, AllowUntrusted: true},
	Get:    APIEndpointAction{Handler: certificateGet, AllowUntrusted: true},
}

// swagger:operation GET /1.0/certificates certificates certificates_get
//
//	Get the trusted certificates
//
//	Returns a list of trusted certificates (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API endpoints
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: List of certificates
//	          items:
//	            $ref: "#/definitions/Certificate"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func certificatesGet(d *Daemon, r *http.Request) response.Response {
	var certResponses []api.Certificate
	var err error
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		certResponses, err = d.db.GetAllCertificates(tx)

		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, certResponses)
}

// swagger:operation POST /1.0/certificates certificates certificates_post
//
//	Add a trusted certificate
//
//	Adds a certificate to the trust store.
//	In this mode, the `token` property is always ignored.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: certificate
//	    description: Certificate
//	    required: true
//	    schema:
//	      $ref: "#/definitions/CertificatesPost"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func certificatesPost(d *Daemon, r *http.Request) response.Response {
	// Parse the request.
	req := api.Certificate{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	// Extract the certificate.
	var cert *x509.Certificate
	if req.Certificate == "" {
		return response.BadRequest(fmt.Errorf("No TLS certificate provided"))
	}

	// Add supplied certificate.
	data, err := base64.StdEncoding.DecodeString(req.Certificate)
	if err != nil {
		return response.BadRequest(err)
	}

	cert, err = x509.ParseCertificate(data)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Invalid certificate material: %w", err))
	}

	// Check validity.
	err = certificateValidate(cert)
	if err != nil {
		return response.BadRequest(err)
	}

	// Calculate the fingerprint.
	fingerprint := localtls.CertFingerprint(cert)

	// Figure out a name.
	name := req.Name
	if name == "" {
		// Try to pull the CN.
		name = cert.Subject.CommonName
		if name == "" {
			// Fallback to the client's IP address.
			name, _, err = net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return response.InternalError(err)
			}
		}
	}

	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		// Check if we already have the certificate.
		_, err := d.db.GetCertificateByFingerprintPrefix(tx, fingerprint)
		if err == nil {
			return fmt.Errorf("Certificate already in trust store")
		}

		req.Name = name
		req.Certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
		req.Fingerprint = localtls.CertFingerprint(cert)

		// Store the certificate in the cluster database.
		return d.db.AddCertificate(tx, req)
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Reload the cache.
	err = updateCertificateCache(d)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/certificates/"+req.Fingerprint)
}

// swagger:operation GET /1.0/certificates/{fingerprint} certificates certificate_get
//
//	Get the trusted certificate
//
//	Gets a specific certificate entry from the trust store.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Certificate
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/Certificate"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func certificateGet(d *Daemon, r *http.Request) response.Response {
	fingerprint, err := url.PathUnescape(mux.Vars(r)["fingerprint"])
	if err != nil {
		return response.SmartError(err)
	}

	var cert api.Certificate

	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		var err error
		cert, err = d.db.GetCertificateByFingerprintPrefix(tx, fingerprint)
		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(true, cert, cert)
}

// swagger:operation DELETE /1.0/certificates/{fingerprint} certificates certificate_delete
//
//	Delete the trusted certificate
//
//	Removes the certificate from the trust store.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func certificateDelete(d *Daemon, r *http.Request) response.Response {
	fingerprint, err := url.PathUnescape(mux.Vars(r)["fingerprint"])
	if err != nil {
		return response.SmartError(err)
	}

	var cert api.Certificate

	// Get current database record.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		var err error
		cert, err = d.db.GetCertificateByFingerprintPrefix(tx, fingerprint)
		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		// Perform the delete with the expanded fingerprint.
		return d.db.DeleteCertificate(tx, cert.Fingerprint)
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Reload the cache.
	err = updateCertificateCache(d)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

func certificateValidate(cert *x509.Certificate) error {
	if time.Now().Before(cert.NotBefore) {
		return fmt.Errorf("The provided certificate isn't valid yet")
	}

	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("The provided certificate is expired")
	}

	if cert.PublicKeyAlgorithm == x509.RSA {
		pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("Unable to validate the RSA certificate")
		}

		// Check that we're dealing with at least 2048bit (Size returns a value in bytes).
		if pubKey.Size()*8 < 2048 {
			return fmt.Errorf("RSA key is too weak (minimum of 2048bit)")
		}
	}

	return nil
}

func updateCertificateCache(d *Daemon) error {
	logger.Debug("Refreshing local trusted certificate cache")

	newCerts := make(map[string]x509.Certificate)

	var dbCerts []api.Certificate
	var err error

	err = d.db.Transaction(d.ShutdownCtx, func(ctx context.Context, tx *sql.Tx) error {
		dbCerts, err = d.db.GetAllCertificates(tx)
		return err
	})
	if err != nil {
		return fmt.Errorf("Failed reading certificates from local database: %w", err)
	}

	for _, dbCert := range dbCerts {
		certBlock, _ := pem.Decode([]byte(dbCert.Certificate))
		if certBlock == nil {
			logger.Warn("Failed decoding certificate", logger.Ctx{"name": dbCert.Name, "err": err})
			continue
		}

		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			logger.Warn("Failed parsing certificate", logger.Ctx{"name": dbCert.Name, "err": err})
			continue
		}

		newCerts[localtls.CertFingerprint(cert)] = *cert
	}

	d.clientCerts.SetCertificates(newCerts)

	return nil
}
