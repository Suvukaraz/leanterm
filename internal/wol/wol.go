package wol

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// BuildMagicPacket constructs a 102-byte Wake-on-LAN magic packet:
// 6 bytes of 0xFF followed by 16 repetitions of the 6-byte target MAC address.
func BuildMagicPacket(macStr string) ([]byte, error) {
	hw, err := net.ParseMAC(strings.TrimSpace(macStr))
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address '%s': %w", macStr, err)
	}
	if len(hw) != 6 {
		return nil, fmt.Errorf("MAC address must be 6 bytes, got %d", len(hw))
	}

	packet := make([]byte, 102)
	// 6 bytes of 0xFF
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	// 16 repetitions of the MAC address
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:6+(i+1)*6], hw)
	}

	return packet, nil
}

// SendMagicPacket sends a Wake-on-LAN UDP magic packet to the target address.
func SendMagicPacket(macStr, host string, port int, broadcast bool) error {
	packet, err := BuildMagicPacket(macStr)
	if err != nil {
		return err
	}

	if port <= 0 {
		port = 9
	}

	var targetAddr string
	if broadcast || strings.TrimSpace(host) == "" {
		targetAddr = net.JoinHostPort("255.255.255.255", strconv.Itoa(port))
	} else {
		targetAddr = net.JoinHostPort(host, strconv.Itoa(port))
	}

	raddr, err := net.ResolveUDPAddr("udp", targetAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP target '%s': %w", targetAddr, err)
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return fmt.Errorf("failed to dial UDP '%s': %w", targetAddr, err)
	}
	defer conn.Close()

	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("failed to send magic packet: %w", err)
	}

	return nil
}

// LookupArpMAC finds the MAC address of ip on the local network: from the ARP
// cache after a quick TCP ping (Linux), or with an ARP request (Windows).
func LookupArpMAC(ip string) (string, error) {
	cleanIP := strings.TrimSpace(ip)
	if cleanIP == "" {
		return "", fmt.Errorf("ip address cannot be empty")
	}
	if mac, err := arpLookup(cleanIP); err == nil && mac != "" {
		return mac, nil
	} else if err != nil {
		return "", err
	}
	return "", fmt.Errorf("MAC address not found in ARP cache for IP '%s'", cleanIP)
}

// ParseProcNetArp parses /proc/net/arp content (exported for testing).
func ParseProcNetArp(content []byte, ip string) string {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	if scanner.Scan() {
		_ = scanner.Text() // skip header
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 4 && fields[0] == ip && fields[3] != "00:00:00:00:00:00" {
			return fields[3]
		}
	}
	return ""
}
