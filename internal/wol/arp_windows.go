//go:build windows

package wol

import (
	"encoding/binary"
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSendARP = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("SendARP")

// arpLookup sends an ARP request for ip (IPv4) with SendARP.
func arpLookup(ip string) (string, error) {
	ip4 := net.ParseIP(ip).To4()
	if ip4 == nil {
		return "", fmt.Errorf("'%s' is not an IPv4 address", ip)
	}
	// IPAddr is the address in network byte order as stored in memory.
	dest := binary.LittleEndian.Uint32(ip4)
	var mac [8]byte
	size := uint32(len(mac))
	r, _, _ := procSendARP.Call(uintptr(dest), 0, uintptr(unsafe.Pointer(&mac[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return "", fmt.Errorf("no ARP answer from %s: %w", ip, windows.Errno(r))
	}
	if size < 6 {
		return "", nil
	}
	return net.HardwareAddr(mac[:6]).String(), nil
}
