package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildAffinityRetryContext(t *testing.T, maxFallbacks int) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Set("channel_affinity_skip_retry_on_failure", true)
	ctx.Set("channel_affinity_failure_fallback_max", maxFallbacks)
	ctx.Set("channel_affinity_log_info", map[string]interface{}{})
	return ctx
}

func relayStatusError(statusCode int) *types.NewAPIError {
	return types.NewOpenAIError(
		errors.New(http.StatusText(statusCode)),
		types.ErrorCodeBadResponseStatusCode,
		statusCode,
	)
}

func TestShouldRetryCodexAffinityUsesThreeFallbacks(t *testing.T) {
	ctx := buildAffinityRetryContext(t, 3)
	err503 := relayStatusError(http.StatusServiceUnavailable)

	require.True(t, shouldRetry(ctx, err503, 4))
	require.True(t, shouldRetry(ctx, err503, 3))
	require.True(t, shouldRetry(ctx, err503, 2))
	assert.False(t, shouldRetry(ctx, err503, 1))
	assert.True(t, service.HasEscapedChannelAffinityFailure(ctx))
}

func TestShouldRetryAffinityBudgetNeverExceedsGlobalBudget(t *testing.T) {
	ctx := buildAffinityRetryContext(t, 3)
	err503 := relayStatusError(http.StatusServiceUnavailable)

	require.True(t, shouldRetry(ctx, err503, 1))
	assert.False(t, shouldRetry(ctx, err503, 0))
}

func TestShouldRetryAffinitySupportsMixedChannelErrors(t *testing.T) {
	ctx := buildAffinityRetryContext(t, 3)
	err503 := relayStatusError(http.StatusServiceUnavailable)
	channelErr := types.NewOpenAIError(
		errors.New("channel unavailable"),
		types.ErrorCodeChannelResponseTimeExceeded,
		http.StatusServiceUnavailable,
	)

	require.True(t, shouldRetry(ctx, err503, 4))
	require.True(t, shouldRetry(ctx, channelErr, 3))
	require.True(t, shouldRetry(ctx, err503, 2))
	assert.False(t, shouldRetry(ctx, channelErr, 1))
}

func TestShouldRetryTimeoutAllowsOnlyOneFallback(t *testing.T) {
	for _, statusCode := range []int{http.StatusGatewayTimeout, 524} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			err := relayStatusError(statusCode)

			require.True(t, shouldRetry(ctx, err, 4))
			assert.False(t, shouldRetry(ctx, err, 3))
		})
	}
}

func TestShouldRetryChannelTimeoutCannotBypassTimeoutGuard(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := types.NewOpenAIError(
		errors.New("channel timeout"),
		types.ErrorCodeChannelResponseTimeExceeded,
		http.StatusGatewayTimeout,
	)

	require.True(t, shouldRetry(ctx, err, 4))
	assert.False(t, shouldRetry(ctx, err, 3))
}

func TestShouldRetryStopsAfterEscapedResponseIsWritten(t *testing.T) {
	ctx := buildAffinityRetryContext(t, 3)
	err503 := relayStatusError(http.StatusServiceUnavailable)

	require.True(t, shouldRetry(ctx, err503, 4))
	ctx.Writer.WriteHeaderNow()
	assert.False(t, shouldRetry(ctx, err503, 3))
}

func TestShouldRetrySpecificChannelNeverEscapesOrRetries(t *testing.T) {
	ctx := buildAffinityRetryContext(t, 3)
	ctx.Set("specific_channel_id", 337)

	assert.False(t, shouldRetry(ctx, relayStatusError(http.StatusServiceUnavailable), 4))
	assert.False(t, service.HasEscapedChannelAffinityFailure(ctx))
}
