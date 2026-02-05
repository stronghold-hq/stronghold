//go:build !linux

package proxy

import (
	"fmt"
	"net"
)

// GetOriginalDst retrieves the original destination of a transparently redirected connection
// On non-Linux platforms, this returns an error as SO_ORIGINAL_DST is Linux-specific
func GetOriginalDst(conn net.Conn) (string, error) {
	// On macOS, pf uses a different mechanism (pf divert)
	// For now, return an error - macOS support would require different implementation
	return "", fmt.Errorf("SO_ORIGINAL_DST not supported on this platform")
}
