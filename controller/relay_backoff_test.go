package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForRelayRetryStopsWhenRequestContextIsCanceled(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldBackoff := setting.RetryBackoffMilliseconds
	oldMaxBackoff := setting.RetryBackoffMaxMilliseconds
	setting.RetryBackoffMilliseconds = "2000"
	setting.RetryBackoffMaxMilliseconds = 2000
	t.Cleanup(func() {
		setting.RetryBackoffMilliseconds = oldBackoff
		setting.RetryBackoffMaxMilliseconds = oldMaxBackoff
	})

	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestCtx)

	assert.False(t, waitForRelayRetry(c, 0))
}

func TestParseRetryAfterCapsServerDelay(t *testing.T) {
	delay, ok := parseRetryAfter("30", time.Now(), 1500*time.Millisecond)
	require.True(t, ok)
	assert.Equal(t, 1500*time.Millisecond, delay)
}

func TestRelayRetryDelayUsesConfiguredSchedule(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldBackoff := setting.RetryBackoffMilliseconds
	oldMaxBackoff := setting.RetryBackoffMaxMilliseconds
	setting.RetryBackoffMilliseconds = "100,300,800"
	setting.RetryBackoffMaxMilliseconds = 2000
	t.Cleanup(func() {
		setting.RetryBackoffMilliseconds = oldBackoff
		setting.RetryBackoffMaxMilliseconds = oldMaxBackoff
	})

	assert.Equal(t, 100*time.Millisecond, configuredRelayRetryBackoff(0))
	assert.Equal(t, 300*time.Millisecond, configuredRelayRetryBackoff(1))
	assert.Equal(t, 800*time.Millisecond, configuredRelayRetryBackoff(2))
}

func TestConfiguredMaxRelayRetryBackoffEnforcesSafetyLimit(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldMaxBackoff := setting.RetryBackoffMaxMilliseconds
	setting.RetryBackoffMaxMilliseconds = 60_000
	t.Cleanup(func() { setting.RetryBackoffMaxMilliseconds = oldMaxBackoff })

	assert.Equal(t, absoluteMaxRelayRetryBackoff, configuredMaxRelayRetryBackoff())
}

func TestRelayRetryDelayPrefersRetryAfterHeader(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldMaxBackoff := setting.RetryBackoffMaxMilliseconds
	setting.RetryBackoffMaxMilliseconds = 2000
	t.Cleanup(func() { setting.RetryBackoffMaxMilliseconds = oldMaxBackoff })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyUpstreamRetryAfter, "1")

	assert.Equal(t, time.Second, relayRetryDelay(c, 0, time.Now()))
}

func TestWaitForRelayRetryWithRetryAfterStopsWhenTaskContextIsCanceled(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldMaxBackoff := setting.RetryBackoffMaxMilliseconds
	setting.RetryBackoffMaxMilliseconds = 2000
	t.Cleanup(func() { setting.RetryBackoffMaxMilliseconds = oldMaxBackoff })

	requestContext, cancel := context.WithCancel(context.Background())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil).WithContext(requestContext)
	common.SetContextKey(c, constant.ContextKeyUpstreamRetryAfter, "2")
	cancel()

	assert.False(t, waitForRelayRetry(c, 0))
}

func TestWriteRelayHTTPErrorRestoresStandardJSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	helper.SetEventStreamHeaders(c)
	relayErr := types.NewErrorWithStatusCode(
		context.DeadlineExceeded,
		types.ErrorCodeGetChannelFailed,
		http.StatusServiceUnavailable,
		types.ErrOptionWithSkipRetry(),
	)

	writeRelayHTTPError(c, types.RelayFormatOpenAIResponses, relayErr)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Body.String(), `"code":"get_channel_failed"`)
}

func TestWriteRelayHTTPErrorReturnsJSONForUpstreamHeaderTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	relayErr := types.NewErrorWithStatusCode(
		context.DeadlineExceeded,
		types.ErrorCodeChannelResponseTimeExceeded,
		http.StatusGatewayTimeout,
	)

	writeRelayHTTPError(c, types.RelayFormatOpenAIResponses, relayErr)

	assert.Equal(t, http.StatusGatewayTimeout, recorder.Code)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Body.String(), `"code":"channel:response_time_exceeded"`)
}
