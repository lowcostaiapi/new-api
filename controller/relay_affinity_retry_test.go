package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestShouldRetryEscapesChannelAffinityOnceForCapacityFailure(t *testing.T) {
	ctx := buildAffinityRetryContext(t)
	err503 := types.NewOpenAIError(
		errors.New("group capacity exhausted"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

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

func TestShouldRetryConfigured524UsesStandardRetryBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err524 := types.NewOpenAIError(
		errors.New("upstream proxy timeout"),
		types.ErrorCodeBadResponseStatusCode,
		524,
	)

	// 524 follows the same configured status-code path as 502/503: every
	// remaining RetryTimes slot is eligible, with no 524-specific allowance.
	require.True(t, shouldRetry(ctx, err524, 4))
	require.True(t, shouldRetry(ctx, err524, 1))
	require.False(t, shouldRetry(ctx, err524, 0))
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
