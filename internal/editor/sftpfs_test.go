package editor

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
)

func TestRemoteErrMapsLostConnections(t *testing.T) {
	for _, err := range []error{
		sftp.ErrSSHFxConnectionLost,
		fmt.Errorf("write: %w", io.EOF),
		io.ErrUnexpectedEOF,
		&net.OpError{Op: "read", Err: net.ErrClosed},
	} {
		assert.ErrorIs(t, remoteErr(err), ErrConnectionClosed, "%v", err)
	}
	perm := fmt.Errorf("open: %w", os.ErrPermission)
	assert.True(t, errors.Is(remoteErr(perm), os.ErrPermission), "other errors stay as they are")
	assert.NoError(t, remoteErr(nil))
}
