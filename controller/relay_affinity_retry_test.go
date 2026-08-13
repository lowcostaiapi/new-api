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
