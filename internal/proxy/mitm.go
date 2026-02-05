package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// MITMHandler handles transparent HTTPS interception (Man-In-The-Middle)
type MITMHandler struct {
	certCache *CertCache
	scanner   *ScannerClient
	config    *Config
	logger    *slog.Logger
}

// NewMITMHandler creates a new MITM handler
func NewMITMHandler(certCache *CertCache, scanner *ScannerClient, config *Config, logger *slog.Logger) *MITMHandler {
	return &MITMHandler{
		certCache: certCache,
		scanner:   scanner,
		config:    config,
		logger:    logger,
	}
}

// HandleTLS intercepts a TLS connection for content inspection
func (m *MITMHandler) HandleTLS(clientConn net.Conn, originalDst string) error {
	defer clientConn.Close()

	// Parse host from original destination
	host, port, err := net.SplitHostPort(originalDst)
	if err != nil {
		host = originalDst
		port = "443"
	}

	m.logger.Debug("MITM intercepting TLS connection", "host", host, "port", port)

	// Get certificate for this domain
	cert, err := m.certCache.GetCert(host)
	if err != nil {
		m.logger.Error("failed to get certificate for host", "host", host, "error", err)
		return fmt.Errorf("failed to get certificate: %w", err)
	}

	// Create TLS config with our certificate
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Wrap client connection in TLS (we're the server to the client)
	tlsClientConn := tls.Server(clientConn, tlsConfig)
	if err := tlsClientConn.Handshake(); err != nil {
		m.logger.Debug("TLS handshake with client failed", "host", host, "error", err)
		return fmt.Errorf("TLS handshake failed: %w", err)
	}
	defer tlsClientConn.Close()

	// Connect to actual server with TLS
	serverConn, err := tls.Dial("tcp", originalDst, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		m.logger.Error("failed to connect to server", "host", host, "error", err)
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer serverConn.Close()

	// Handle HTTP requests over the TLS connection
	return m.proxyHTTPS(tlsClientConn, serverConn, host)
}

// proxyHTTPS proxies HTTP requests over established TLS connections
func (m *MITMHandler) proxyHTTPS(clientConn, serverConn net.Conn, host string) error {
	clientReader := bufio.NewReader(clientConn)

	for {
		// Set read deadline to detect closed connections
		clientConn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read HTTP request from client
		req, err := http.ReadRequest(clientReader)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "connection reset") {
				return nil // Normal connection close
			}
			return fmt.Errorf("failed to read request: %w", err)
		}

		// Fix up the request URL for proxying
		req.URL.Scheme = "https"
		req.URL.Host = host
		req.RequestURI = "" // Must be empty for client requests

		m.logger.Debug("MITM request", "method", req.Method, "url", req.URL.String())

		// Scan request body if it exists (for prompt injection in POST data)
		var requestBody []byte
		if req.Body != nil && req.ContentLength > 0 && m.config.Scanning.Content.Enabled {
			requestBody, _ = io.ReadAll(req.Body)
			req.Body.Close()

			// Scan the request content
			if len(requestBody) > 0 && len(requestBody) < 1024*1024 { // Max 1MB
				result := m.scanContent(requestBody, req.URL.String(), req.Header.Get("Content-Type"))
				if result != nil && result.Decision == DecisionBlock {
					// Block the request
					m.sendBlockResponse(clientConn, result, req)
					continue
				}
			}

			// Restore body for forwarding
			req.Body = io.NopCloser(strings.NewReader(string(requestBody)))
		}

		// Forward request to server
		if err := req.Write(serverConn); err != nil {
			return fmt.Errorf("failed to forward request: %w", err)
		}

		// Read response from server
		serverReader := bufio.NewReader(serverConn)
		resp, err := http.ReadResponse(serverReader, req)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		// Read and potentially scan response body
		responseBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		// Scan response content for threats
		var scanResult *ScanResult
		contentType := resp.Header.Get("Content-Type")
		if m.config.Scanning.Content.Enabled && len(responseBody) > 0 && len(responseBody) < 1024*1024 {
			if ShouldScanContentType(contentType) && !IsBinaryContentType(contentType) {
				scanResult = m.scanContent(responseBody, req.URL.String(), contentType)
			}
		}

		// Add Stronghold headers
		resp.Header.Set("X-Stronghold-Proxy", "mitm")
		if scanResult != nil {
			resp.Header.Set("X-Stronghold-Decision", string(scanResult.Decision))
			resp.Header.Set("X-Stronghold-Reason", scanResult.Reason)

			// Block if needed
			action := getAction(scanResult.Decision, m.config.Scanning.Content)
			if action == "block" {
				m.sendBlockResponse(clientConn, scanResult, req)
				continue
			}
		}

		// Forward response to client with the read body
		resp.Body = io.NopCloser(strings.NewReader(string(responseBody)))
		resp.ContentLength = int64(len(responseBody))

		if err := resp.Write(clientConn); err != nil {
			return fmt.Errorf("failed to forward response: %w", err)
		}
	}
}

// scanContent scans content for threats
func (m *MITMHandler) scanContent(body []byte, sourceURL, contentType string) *ScanResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := m.scanner.ScanContent(ctx, body, sourceURL, contentType)
	if err != nil {
		m.logger.Error("scan error", "error", err)
		if m.config.Scanning.FailOpen {
			return nil
		}
		return &ScanResult{
			Decision: DecisionBlock,
			Reason:   "Scan failed - blocking for safety",
		}
	}

	return result
}

// sendBlockResponse sends a block response to the client
func (m *MITMHandler) sendBlockResponse(conn net.Conn, result *ScanResult, req *http.Request) {
	m.logger.Warn("content blocked", "url", req.URL.String(), "reason", result.Reason)

	body := fmt.Sprintf(`{
	"error": "Content blocked by Stronghold security scan",
	"reason": "%s",
	"url": "%s"
}`, result.Reason, req.URL.String())

	resp := &http.Response{
		StatusCode:    http.StatusForbidden,
		Status:        "403 Forbidden",
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}

	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("X-Stronghold-Decision", string(result.Decision))
	resp.Header.Set("X-Stronghold-Reason", result.Reason)

	resp.Write(conn)
}

// prefixConn wraps a connection with a prefix that was already read
type prefixConn struct {
	net.Conn
	prefix []byte
	reader io.Reader
}

func newPrefixConn(conn net.Conn, prefix []byte) *prefixConn {
	return &prefixConn{
		Conn:   conn,
		prefix: prefix,
		reader: io.MultiReader(strings.NewReader(string(prefix)), conn),
	}
}

func (c *prefixConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}
