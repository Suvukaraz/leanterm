package wol

import (
	"bytes"
	"testing"
)

func TestBuildMagicPacket(t *testing.T) {
	mac := "00:11:22:33:44:55"
	packet, err := BuildMagicPacket(mac)
	if err != nil {
		t.Fatalf("failed to build magic packet: %v", err)
	}

	if len(packet) != 102 {
		t.Fatalf("packet length = %d, want 102", len(packet))
	}

	// First 6 bytes must be 0xFF
	for i := 0; i < 6; i++ {
		if packet[i] != 0xFF {
			t.Errorf("packet[%d] = 0x%02X, want 0xFF", i, packet[i])
		}
	}

	// Next 16 repetitions of MAC
	expectedMAC := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	for i := 0; i < 16; i++ {
		slice := packet[6+i*6 : 6+(i+1)*6]
		if !bytes.Equal(slice, expectedMAC) {
			t.Errorf("repetition %d = %v, want %v", i, slice, expectedMAC)
		}
	}
}

func TestBuildMagicPacket_Invalid(t *testing.T) {
	_, err := BuildMagicPacket("not-a-mac")
	if err == nil {
		t.Errorf("expected error for invalid MAC")
	}
}

func TestParseProcNetArp(t *testing.T) {
	sampleArp := []byte(`IP address       HW type     Flags       HW address            Mask     Device
192.168.1.1      0x1         0x2         aa:bb:cc:dd:ee:ff     *        eth0
192.168.1.50     0x1         0x0         00:00:00:00:00:00     *        eth0
10.0.0.5         0x1         0x2         11:22:33:44:55:66     *        wlan0
`)

	mac1 := ParseProcNetArp(sampleArp, "192.168.1.1")
	if mac1 != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("got %s, want aa:bb:cc:dd:ee:ff", mac1)
	}

	mac2 := ParseProcNetArp(sampleArp, "192.168.1.50")
	if mac2 != "" {
		t.Errorf("incomplete arp entry should return empty, got %s", mac2)
	}

	mac3 := ParseProcNetArp(sampleArp, "10.0.0.5")
	if mac3 != "11:22:33:44:55:66" {
		t.Errorf("got %s, want 11:22:33:44:55:66", mac3)
	}
}
