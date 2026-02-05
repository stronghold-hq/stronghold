//go:build linux

package proxy

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

const (
	// SO_ORIGINAL_DST is the socket option to get the original destination
	// of a connection redirected by iptables/nftables REDIRECT target
	SO_ORIGINAL_DST = 80
)

// GetOriginalDst retrieves the original destination of a transparently redirected connection
// This uses the SO_ORIGINAL_DST socket option which is populated by iptables/nftables
// when using the REDIRECT target
func GetOriginalDst(conn net.Conn) (string, error) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return "", fmt.Errorf("not a TCP connection")
	}

	// Get the underlying file descriptor
	file, err := tcpConn.File()
	if err != nil {
		return "", fmt.Errorf("failed to get file descriptor: %w", err)
	}
	defer file.Close()

	fd := int(file.Fd())

	// Get the original destination using getsockopt with SO_ORIGINAL_DST
	// The result is a sockaddr_in structure (for IPv4)
	var addr syscall.RawSockaddrInet4
	addrLen := uint32(syscall.SizeofSockaddrInet4)

	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		uintptr(syscall.IPPROTO_IP),
		uintptr(SO_ORIGINAL_DST),
		uintptr(unsafe.Pointer(&addr)),
		uintptr(unsafe.Pointer(&addrLen)),
		0,
	)

	if errno != 0 {
		return "", fmt.Errorf("getsockopt SO_ORIGINAL_DST failed: %v", errno)
	}

	// Parse the sockaddr_in structure
	// Port is stored in network byte order (big endian) but Go's RawSockaddrInet4
	// stores it as a uint16 in host byte order representation of the network bytes
	// So we need to swap bytes to get the actual port number
	port := uint16(addr.Port>>8) | uint16(addr.Port<<8)

	ip := net.IPv4(addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3])

	return fmt.Sprintf("%s:%d", ip.String(), port), nil
}
