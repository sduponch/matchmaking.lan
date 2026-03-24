package server

import (
	"net"
	"syscall"
	"time"
)

// A2S_INFO request packet
var a2sQuery = append(
	[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x54},
	[]byte("Source Engine Query\x00")...,
)

// discoverLAN sends a UDP broadcast on port 27015 and collects responding server addresses.
func discoverLAN(timeout time.Duration) []string {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil
	}
	defer conn.Close()

	// Enable broadcast
	if raw, err := conn.SyscallConn(); err == nil {
		raw.Control(func(fd uintptr) {
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
		})
	}

	broadcast := &net.UDPAddr{IP: net.IPv4(255, 255, 255, 255), Port: 27015}
	conn.WriteToUDP(a2sQuery, broadcast)
	conn.SetReadDeadline(time.Now().Add(timeout))

	seen := map[string]bool{}
	buf := make([]byte, 1400)
	for {
		_, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		seen[addr.String()] = true
	}

	result := make([]string, 0, len(seen))
	for addr := range seen {
		result = append(result, addr)
	}
	return result
}
