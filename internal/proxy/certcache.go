package proxy

import (
	"crypto/tls"
	"sort"
	"sync"
	"time"
)

const (
	defaultMaxSize = 10000
	defaultTTL     = 1 * time.Hour
	evictionPeriod = 5 * time.Minute
)

// cachedCert wraps a certificate with a last-used timestamp for eviction
type cachedCert struct {
	cert     *tls.Certificate
	lastUsed time.Time
}

// CertCache caches generated TLS certificates per domain
// to avoid regenerating certificates on every request.
// Entries are evicted after a TTL of inactivity, with a hard size cap.
type CertCache struct {
	ca      *CA
	certs   map[string]*cachedCert
	maxSize int
	ttl     time.Duration
	mu      sync.RWMutex
	stopCh  chan struct{}
}

// NewCertCache creates a new certificate cache with TTL-based eviction
func NewCertCache(ca *CA) *CertCache {
	c := &CertCache{
		ca:      ca,
		certs:   make(map[string]*cachedCert),
		maxSize: defaultMaxSize,
		ttl:     defaultTTL,
		stopCh:  make(chan struct{}),
	}
	go c.startEviction()
	return c
}

// GetCert returns a cached certificate or generates a new one for the host
func (c *CertCache) GetCert(host string) (*tls.Certificate, error) {
	// Check cache first with read lock
	c.mu.RLock()
	if entry, ok := c.certs[host]; ok {
		c.mu.RUnlock()
		// Update lastUsed with write lock
		c.mu.Lock()
		entry.lastUsed = time.Now()
		c.mu.Unlock()
		return entry.cert, nil
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
	if existing, ok := c.certs[host]; ok {
		existing.lastUsed = time.Now()
		c.mu.Unlock()
		return existing.cert, nil
	}
	c.certs[host] = &cachedCert{
		cert:     cert,
		lastUsed: time.Now(),
	}
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
	c.certs = make(map[string]*cachedCert)
	c.mu.Unlock()
}

// Size returns the number of cached certificates
func (c *CertCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.certs)
}

// Stop stops the background eviction goroutine
func (c *CertCache) Stop() {
	close(c.stopCh)
}

// startEviction runs periodic cache eviction
func (c *CertCache) startEviction() {
	ticker := time.NewTicker(evictionPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.evict()
		}
	}
}

// evict removes expired entries and enforces the max size cap
func (c *CertCache) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Phase 1: evict entries that have expired based on TTL
	for host, entry := range c.certs {
		if now.Sub(entry.lastUsed) > c.ttl {
			delete(c.certs, host)
		}
	}

	// Phase 2: if still over maxSize, evict oldest entries until at 75% capacity
	if len(c.certs) > c.maxSize {
		target := c.maxSize * 3 / 4

		type hostTime struct {
			host     string
			lastUsed time.Time
		}
		entries := make([]hostTime, 0, len(c.certs))
		for host, entry := range c.certs {
			entries = append(entries, hostTime{host, entry.lastUsed})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].lastUsed.Before(entries[j].lastUsed)
		})

		for _, e := range entries {
			if len(c.certs) <= target {
				break
			}
			delete(c.certs, e.host)
		}
	}
}
