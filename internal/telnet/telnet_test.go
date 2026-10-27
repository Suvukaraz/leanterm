package telnet

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

func TestTelnetOptionNegotiation(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	opts := Options{
		TerminalType: "xterm-256color",
		NAWS:         true,
		Cols:         80,
		Rows:         24,
	}

	client := NewClient(clientConn, opts)
	defer client.Close()

	// Drain client.Read in background so readCh never blocks
	go func() {
		b := make([]byte, 1024)
		for {
			if _, err := client.Read(b); err != nil {
				return
			}
		}
	}()

	// 1. Server sends DO TTYPE (IAC DO 24)
	go func() {
		_, _ = serverConn.Write([]byte{CmdIAC, CmdDO, OptTTYPE})
	}()

	// Expect Client to reply WILL TTYPE (IAC WILL 24)
	buf := make([]byte, 3)
	if _, err := io.ReadFull(serverConn, buf); err != nil {
		t.Fatalf("failed to read WILL TTYPE: %v", err)
	}
	if !bytes.Equal(buf, []byte{CmdIAC, CmdWILL, OptTTYPE}) {
		t.Fatalf("expected IAC WILL TTYPE, got: %v", buf)
	}

	// 2. Server requests TTYPE (IAC SB TTYPE SEND IAC SE)
	go func() {
		_, _ = serverConn.Write([]byte{CmdIAC, CmdSB, OptTTYPE, SubSEND, CmdIAC, CmdSE})
	}()

	// Expect Client to reply: IAC SB TTYPE IS "xterm-256color" IAC SE
	expectedReply := append([]byte{CmdIAC, CmdSB, OptTTYPE, SubIS}, []byte("xterm-256color")...)
	expectedReply = append(expectedReply, CmdIAC, CmdSE)

	replyBuf := make([]byte, len(expectedReply))
	if _, err := io.ReadFull(serverConn, replyBuf); err != nil {
		t.Fatalf("failed reading TTYPE IS: %v", err)
	}
	if !bytes.Equal(replyBuf, expectedReply) {
		t.Fatalf("expected %v, got: %v", expectedReply, replyBuf)
	}

	// 3. Second TTYPE SEND -> should cycle to XTERM
	go func() {
		_, _ = serverConn.Write([]byte{CmdIAC, CmdSB, OptTTYPE, SubSEND, CmdIAC, CmdSE})
	}()

	expectedSecond := append([]byte{CmdIAC, CmdSB, OptTTYPE, SubIS}, []byte("XTERM")...)
	expectedSecond = append(expectedSecond, CmdIAC, CmdSE)
	replyBuf2 := make([]byte, len(expectedSecond))
	if _, err := io.ReadFull(serverConn, replyBuf2); err != nil {
		t.Fatalf("failed reading second TTYPE: %v", err)
	}
	if !bytes.Equal(replyBuf2, expectedSecond) {
		t.Fatalf("expected %v, got: %v", expectedSecond, replyBuf2)
	}

	// 4. Server sends DO NAWS
	go func() {
		_, _ = serverConn.Write([]byte{CmdIAC, CmdDO, OptNAWS})
	}()

	// Expect WILL NAWS (3 bytes)
	willNaws := make([]byte, 3)
	if _, err := io.ReadFull(serverConn, willNaws); err != nil {
		t.Fatalf("failed reading WILL NAWS: %v", err)
	}
	if !bytes.Equal(willNaws, []byte{CmdIAC, CmdWILL, OptNAWS}) {
		t.Fatalf("expected IAC WILL NAWS, got: %v", willNaws)
	}

	// Followed by SB NAWS (9 bytes: IAC SB NAWS 0 80 0 24 IAC SE)
	sbNaws := make([]byte, 9)
	if _, err := io.ReadFull(serverConn, sbNaws); err != nil {
		t.Fatalf("failed reading SB NAWS: %v", err)
	}
	expectedNaws := []byte{CmdIAC, CmdSB, OptNAWS, 0, 80, 0, 24, CmdIAC, CmdSE}
	if !bytes.Equal(sbNaws, expectedNaws) {
		t.Fatalf("expected SB NAWS %v, got: %v", expectedNaws, sbNaws)
	}
}

func TestTelnetDataStreamAndCR(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	client := NewClient(clientConn, Options{})
	defer client.Close()

	// Server sends "Hello\r\nWorld\r\0Test" with double IAC
	payload := []byte("Hello\r\nWorld\r\x00Test" + string([]byte{CmdIAC, CmdIAC}))
	go func() {
		_, _ = serverConn.Write(payload)
	}()

	readBuf := make([]byte, 128)
	var accumulated []byte

	done := make(chan struct{})
	go func() {
		for len(accumulated) < len("Hello\r\nWorld\rTest\xff") {
			n, err := client.Read(readBuf)
			if err != nil {
				break
			}
			accumulated = append(accumulated, readBuf[:n]...)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for read")
	}

	expected := []byte("Hello\r\nWorld\rTest\xff")
	if !bytes.Equal(accumulated, expected) {
		t.Fatalf("expected %q, got %q", expected, accumulated)
	}
}

func TestTelnetWriteEscaping(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	client := NewClient(clientConn, Options{LineEnding: "crlf"})
	defer client.Close()

	// Write data with IAC byte (0xFF) and Enter (\r)
	input := []byte{'A', CmdIAC, 'B', '\r'}
	go func() {
		_, _ = client.Write(input)
	}()

	readBuf := make([]byte, 10)
	n, err := serverConn.Read(readBuf)
	if err != nil {
		t.Fatalf("failed reading: %v", err)
	}

	// Expected: 'A', IAC, IAC, 'B', '\r', '\n'
	expected := []byte{'A', CmdIAC, CmdIAC, 'B', '\r', '\n'}
	if !bytes.Equal(readBuf[:n], expected) {
		t.Fatalf("expected %v, got %v", expected, readBuf[:n])
	}
}
