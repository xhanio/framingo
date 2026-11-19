package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// An https endpoint needs no TLS option: the server is verified against the system's CAs.
func TestInitHTTPSWithoutTLS(t *testing.T) {
	require.NoError(t, New("https://localhost:8443/api/v1").Init(context.Background()))
}
