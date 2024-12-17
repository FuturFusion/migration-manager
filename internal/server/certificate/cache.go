package certificate

import (
	"crypto/x509"
	"sync"
)

// Cache represents an thread-safe in-memory cache of the certificates in the database.
type Cache struct {
	// certificates is a map of certificate Type to map of certificate fingerprint to x509.Certificate.
	certificates map[string]x509.Certificate

	mu sync.RWMutex
}

// SetCertificates sets the certificates on the Cache.
func (c *Cache) SetCertificates(certificates map[string]x509.Certificate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.certificates = certificates
}

// GetCertificates returns a read-only copy of the certificate map.
func (c *Cache) GetCertificates() map[string]x509.Certificate {
	c.mu.RLock()
	defer c.mu.RUnlock()

	certificates := make(map[string]x509.Certificate, len(c.certificates))
	for f, cert := range c.certificates {
		certificates[f] = cert
	}

	return certificates
}
