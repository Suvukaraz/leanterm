//go:build !windows

package wol

import (
	"fmt"
	"net"
	"os"
	"time"
)

// arpLookup triggers ARP resolution with a quick TCP connection attempt on port
// 7 (echo) and reads the result from /proc/net/arp.
func arpLookup(ip string) (string, error) {
	d := net.Dialer{Timeout: 400 * time.Millisecond}
	if conn, _ := d.Dial("tcp", net.JoinHostPort(ip, "7")); conn != nil {
		_ = conn.Close()
	}
	content, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return "", fmt.Errorf("cannot open /proc/net/arp: %w", err)
	}
	return ParseProcNetArp(content, ip), nil
}
