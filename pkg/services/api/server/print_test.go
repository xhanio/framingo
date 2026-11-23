package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"go.uber.org/zap/zapcore"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// printed returns the messages the server logs for a request answered with
// status at level, read back from the log's JSON file.
func printed(t *testing.T, level zapcore.Level, status int, body *api.ErrorBody) []string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "server.log")
	s := &server{log: log.New(log.WithLevel(int(level)), log.WithFileWriter(file, 1, 1, 1))}
	s.print(&api.RequestInfo{Method: http.MethodGet, URI: "/things"}, &api.ResponseInfo{Status: status, Error: body})

	f, err := os.Open(file)
	require.NoError(t, err)
	defer f.Close()
	var msgs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var entry struct {
			Msg string `json:"msg"`
		}
		require.NoError(t, json.Unmarshal(sc.Bytes(), &entry))
		msgs = append(msgs, entry.Msg)
	}
	require.NoError(t, sc.Err())
	return msgs
}

// Outside debug, a 500 logs its error with the stack that locates it; any other
// 5xx logs the message alone, and a 4xx only the request line.
func TestPrintLogsTheStackOfA500(t *testing.T) {
	boom := errors.Newf("boom")
	tests := []struct {
		name   string
		level  zapcore.Level
		status int
		want   string // the error's own log line: "" for none, "stack" for message and frames
	}{
		{"500", zapcore.InfoLevel, http.StatusInternalServerError, "stack"},
		{"503", zapcore.InfoLevel, http.StatusServiceUnavailable, "boom"},
		{"504", zapcore.InfoLevel, http.StatusGatewayTimeout, "boom"},
		{"404", zapcore.InfoLevel, http.StatusNotFound, ""},
		{"debug 503", zapcore.DebugLevel, http.StatusServiceUnavailable, "stack"},
		{"debug 404", zapcore.DebugLevel, http.StatusNotFound, "stack"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := printed(t, tt.level, tt.status, api.WrapError(boom))
			var errLines []string
			for _, m := range msgs {
				if strings.HasPrefix(m, "boom") {
					errLines = append(errLines, m)
				}
			}
			switch tt.want {
			case "":
				assert.Empty(t, errLines)
			case "stack":
				require.Len(t, errLines, 1)
				assert.Contains(t, errLines[0], "\ngithub.com/xhanio/framingo/pkg/services/api/server.TestPrintLogsTheStackOfA500\n",
					"the frames name where boom was made")
			default:
				assert.Equal(t, []string{tt.want}, errLines)
			}
		})
	}
}

// A body a handler returned as is carries no Origin: the log shows the body.
func TestPrintLogsABodyWithoutOrigin(t *testing.T) {
	body := &api.ErrorBody{Status: http.StatusInternalServerError, Kind: "Internal", Message: "upstream failed"}
	msgs := printed(t, zapcore.InfoLevel, http.StatusInternalServerError, body)
	assert.Contains(t, msgs, "(500) Internal: upstream failed")
}
