package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A listener that fails to bind must surface through both probes, naming the
// server - not vanish into a debug log while the supervisor reports Ready.
func TestProbesReportListenerFailure(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer blocker.Close()
	port := uint(blocker.Addr().(*net.TCPAddr).Port)

	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", port, "/")))
	require.NoError(t, m.Start(context.Background()))
	defer func() { _ = m.Stop(true) }()

	require.Eventually(t, func() bool {
		return m.Alive(context.Background()) != nil
	}, 2*time.Second, 10*time.Millisecond, "bind failure must reach the liveness probe")
	assert.Contains(t, m.Alive(context.Background()).Error(), "http")
	require.Error(t, m.Ready(context.Background()))
	assert.Contains(t, m.Ready(context.Background()).Error(), "http")
}

// A serving listener keeps both probes green.
func TestProbesHealthyListener(t *testing.T) {
	port := freePort(t)
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", port, "/")))
	require.NoError(t, m.Start(context.Background()))
	defer func() { _ = m.Stop(true) }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond)

	assert.NoError(t, m.Alive(context.Background()))
	assert.NoError(t, m.Ready(context.Background()))
}

// The supervisor's remedy for a dead listener is restart: Stop, Init
// (rebuilds echo), Start. A recorded failure must clear once the listener
// binds again.
func TestRestartClearsListenerFailure(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := uint(blocker.Addr().(*net.TCPAddr).Port)

	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", port, "/")))
	require.NoError(t, m.Start(context.Background()))
	require.Eventually(t, func() bool {
		return m.Alive(context.Background()) != nil
	}, 2*time.Second, 10*time.Millisecond)

	// The port frees up; the supervisor restarts the service.
	blocker.Close()
	require.NoError(t, m.Stop(true))
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	defer func() { _ = m.Stop(true) }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "rebuilt listener must serve again")
	assert.NoError(t, m.Alive(context.Background()), "a recovered listener must not keep reporting its old failure")
	assert.NoError(t, m.Ready(context.Background()))
}
