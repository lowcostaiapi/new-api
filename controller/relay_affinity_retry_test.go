package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildAffinityRetryContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	ctx.Set("channel_affinity_skip_retry_on_failure", true)
	require.True(t, service.ShouldSkipRetryAfterChannelAffinityFailure(ctx))
	return ctx
}

func setRetryTimesForTest(t *testing.T, retryTimes int) {
	t.Helper()
	previous := common.RetryTimes
	common.RetryTimes = retryTimes
	t.Cleanup(func() {
		common.RetryTimes = previous
	})
}

func TestShouldRetryEscapesChannelAffinityThenUsesNormalRetryBudget(t *testing.T) {
	ctx := buildAffinityRetryContext(t)
	err503 := types.NewOpenAIError(
		errors.New("group capacity exhausted"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

	// The first failure releases affinity and consumes the default one
	// fallback. Missing rule config keeps the legacy behavior.
	require.True(t, shouldRetry(ctx, err503, 3))
	require.True(t, service.HasEscapedChannelAffinityFailure(ctx))
	require.False(t, shouldRetry(ctx, err503, 2))
}

func TestShouldRetryKeepsAffinityForNonEscapeStatus(t *testing.T) {
	ctx := buildAffinityRetryContext(t)
	err429 := types.NewOpenAIError(
		errors.New("rate limited"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)

	require.False(t, shouldRetry(ctx, err429, 3))
	require.False(t, service.HasEscapedChannelAffinityFailure(ctx))
}

func TestShouldRetryWithoutAffinityKeepsNormalRetryBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("channel_affinity_skip_retry_on_failure", false)
	err503 := types.NewOpenAIError(
		errors.New("service unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

	require.True(t, shouldRetry(ctx, err503, 3))
	require.True(t, shouldRetry(ctx, err503, 2))
	require.True(t, shouldRetry(ctx, err503, 1))
	require.False(t, shouldRetry(ctx, err503, 0))
}

func TestShouldRetryDoesNotEscapeAffinityWithoutRetryBudget(t *testing.T) {
	ctx := buildAffinityRetryContext(t)
	err503 := types.NewOpenAIError(
		errors.New("group capacity exhausted"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

	require.False(t, shouldRetry(ctx, err503, 0))
	require.False(t, service.HasEscapedChannelAffinityFailure(ctx))
}

func TestShouldRetryDoesNotEscapeSpecificChannel(t *testing.T) {
	ctx := buildAffinityRetryContext(t)
	ctx.Set("specific_channel_id", 28)
	err503 := types.NewOpenAIError(
		errors.New("group capacity exhausted"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

	require.False(t, shouldRetry(ctx, err503, 3))
	require.False(t, service.HasEscapedChannelAffinityFailure(ctx))
}

func TestShouldRetryConfigured524StillUsesOneRetryGuard(t *testing.T) {
	setRetryTimesForTest(t, 4)
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err524 := types.NewOpenAIError(
		errors.New("upstream proxy timeout"),
		types.ErrorCodeBadResponseStatusCode,
		524,
	)

	// With RetryTimes=4, the first failure may trigger one timeout fallback;
	// after one retry, retryTimes=3 and the timeout guard stops further tries.
	require.True(t, shouldRetry(ctx, err524, 4))
	require.False(t, shouldRetry(ctx, err524, 3))
	require.False(t, shouldRetry(ctx, err524, 0))
}

func TestShouldRetryEscapedChannelErrorTimeoutStillUsesOneRetryGuard(t *testing.T) {
	setRetryTimesForTest(t, 4)
	gin.SetMode(gin.TestMode)
	for _, code := range []int{http.StatusGatewayTimeout, 524} {
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			ctx := buildAffinityRetryContext(t)
			err := types.NewOpenAIError(
				errors.New("adapter timeout"),
				types.ErrorCodeChannelResponseTimeExceeded,
				code,
			)

			// The first failure escapes affinity; the next timeout must still be
			// blocked even though its error code is channel:*.
			require.True(t, shouldRetry(ctx, err, 4))
			require.True(t, service.HasEscapedChannelAffinityFailure(ctx))
			require.False(t, shouldRetry(ctx, err, 3))
		})
	}
}

func TestShouldRetryConfigured524StillHonorsSpecificChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("specific_channel_id", 30)
	err524 := types.NewOpenAIError(
		errors.New("upstream proxy timeout"),
		types.ErrorCodeBadResponseStatusCode,
		524,
	)

	require.False(t, shouldRetry(ctx, err524, 4))
}

func TestShouldRetryConfiguredStatusCodeRegression(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	newStatusError := func(status int) *types.NewAPIError {
		return types.NewOpenAIError(
			errors.New(http.StatusText(status)),
			types.ErrorCodeBadResponseStatusCode,
			status,
		)
	}

	require.True(t, shouldRetry(ctx, newStatusError(http.StatusBadGateway), 1))
	require.True(t, shouldRetry(ctx, newStatusError(http.StatusServiceUnavailable), 1))
	require.False(t, shouldRetry(ctx, newStatusError(http.StatusBadRequest), 1))
}
