package forward

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipe(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	done := make(chan bool)
	go func() {
		conn, err := l.Accept()
		if err == nil {
			buf := make([]byte, 10)
			n, _ := conn.Read(buf)
			_, _ = conn.Write(buf[:n])
			_ = conn.Close()
		}
		done <- true
	}()

	client, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Write([]byte("PING"))
	require.NoError(t, err)

	buf := make([]byte, 10)
	n, err := client.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "PING", string(buf[:n]))

	<-done
}

func TestForwarderSummary(t *testing.T) {
	f := NewForwarder()
	f.summary = []string{"-L 8080:localhost:80", "-D 1080"}
	assert.Equal(t, "-L 8080:localhost:80 -D 1080", f.Summary(false))
	_ = f.Close()
}
