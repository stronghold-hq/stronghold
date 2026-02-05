package proxy

import (
	"crypto/tls"
	"sync"
)

// CertCache caches generated TLS certificates per domain
// to avoid regenerating certificates on every request
type CertCache struct {
	ca    *CA
	certs map[string]*tls.Certificate
	mu    sync.RWMutex
}

// NewCertCache creates a new certificate cache
func NewCertCache(ca *CA) *CertCache {
	return &CertCache{
		ca:    ca,
		certs: make(map[string]*tls.Certificate),
	}
}

// GetCert returns a cached certificate or generates a new one for the host
func (c *CertCache) GetCert(host string) (*tls.Certificate, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if cert, ok := c.certs[host]; ok {
		c.mu.RUnlock()
		return cert, nil
	}
	c.mu.RUnlock()

	// Generate new certificate
	cert, err := c.ca.GenerateCert(host)
	if err != nil {
		return nil, err
	}

	// Cache the certificate with write lock
	c.mu.Lock()
	// Double-check in case another goroutine generated it
	if existingCert, ok := c.certs[host]; ok {
		c.mu.Unlock()
		return existingCert, nil
	}
	c.certs[host] = cert
	c.mu.Unlock()

	return cert, nil
}

// GetCertificate returns a function suitable for tls.Config.GetCertificate
func (c *CertCache) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return c.GetCert(hello.ServerName)
}

// Clear removes all cached certificates
func (c *CertCache) Clear() {
	c.mu.Lock()
	c.certs = make(map[string]*tls.Certificate)
	c.mu.Unlock()
}

// Size returns the number of cached certificates
func (c *CertCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.certs)
}
