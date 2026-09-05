package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeStreamErrorRetryWindow(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	oldRetryRanges := operation_setting.AutomaticRetryStatusCodeRanges
	constant.StreamingTimeout = 30
	operation_setting.AutomaticRetryStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 500, End: 599}}
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
		operation_setting.AutomaticRetryStatusCodeRanges = oldRetryRanges
	})

	const errorEvent = "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"cpu overloaded\"}}\n\n"
	const textEvent = "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n"
	for _, format := range []types.RelayFormat{types.RelayFormatClaude, types.RelayFormatOpenAI} {
		for _, tc := range []struct {
			name    string
			prelude string
			written bool
		}{
			{name: "initial error"},
			{name: "error after output", prelude: textEvent, written: true},
		} {
			t.Run(string(format)+"/"+tc.name, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				info := &relaycommon.RelayInfo{
					RelayFormat: format,
					DisablePing: true,
					ChannelMeta: &relaycommon.ChannelMeta{},
				}
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"text/event-stream"}},
					Body:       io.NopCloser(strings.NewReader(tc.prelude + errorEvent)),
				}

				usage, relayErr := claude.ClaudeStreamHandler(c, resp, info)
				require.NotNil(t, relayErr)
				assert.Nil(t, usage)
				assert.Equal(t, http.StatusInternalServerError, relayErr.StatusCode)
				assert.Equal(t, tc.written, c.Writer.Written())
				assert.Equal(t, tc.written, recorder.Flushed)
				assert.Equal(t, !tc.written, shouldRetry(c, relayErr, 1))
				assert.False(t, shouldRetry(c, relayErr, 0))
				if tc.written {
					before := recorder.Body.String()
					writeRelayHTTPError(c, format, relayErr)
					assert.Equal(t, before, recorder.Body.String(), "committed SSE must not receive a raw JSON error tail")
					assert.Contains(t, recorder.Body.String(), "hello")
					assert.NotContains(t, recorder.Body.String(), "cpu overloaded")
					return
				}

				assert.Empty(t, recorder.Body.String())
				writeRelayHTTPError(c, format, relayErr)
				assert.Equal(t, http.StatusInternalServerError, recorder.Code)
				assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
				assert.Empty(t, recorder.Header().Get("Transfer-Encoding"))
				assert.Contains(t, recorder.Body.String(), `"message":"cpu overloaded"`)
			})
		}
	}
}
