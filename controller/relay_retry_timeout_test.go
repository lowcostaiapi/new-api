package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryTaskRelayTimeoutAllowsOnlyOneFallback(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	taskErr := &dto.TaskError{
		StatusCode: http.StatusGatewayTimeout,
		Error:      errors.New("upstream timeout"),
	}

	require.True(t, shouldRetryTaskRelay(ctx, 337, taskErr, 4))
	assert.False(t, shouldRetryTaskRelay(ctx, 354, taskErr, 3))
}

func TestShouldRetryTaskRelayEscapesAffinityAndConsumesBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("channel_affinity_skip_retry_on_failure", true)
	taskErr := &dto.TaskError{
		StatusCode: http.StatusServiceUnavailable,
		Error:      errors.New("affinity upstream unavailable"),
	}

	require.True(t, shouldRetryTaskRelay(ctx, 337, taskErr, 4))
	require.True(t, service.HasEscapedChannelAffinityFailure(ctx))
	require.True(t, shouldRetryTaskRelay(ctx, 354, taskErr, 3))
	require.True(t, shouldRetryTaskRelay(ctx, 355, taskErr, 2))
	require.False(t, shouldRetryTaskRelay(ctx, 356, taskErr, 1))
}

func TestShouldRetryTaskRelayAffinityTimeoutKeepsOneAttemptGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("channel_affinity_skip_retry_on_failure", true)
	taskErr := &dto.TaskError{
		StatusCode: http.StatusGatewayTimeout,
		Error:      errors.New("affinity upstream timeout"),
	}

	require.True(t, shouldRetryTaskRelay(ctx, 337, taskErr, 4))
	assert.False(t, shouldRetryTaskRelay(ctx, 354, taskErr, 3))
}

func TestShouldRetryTaskRelaySpecificChannelDoesNotEscapeAffinity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("specific_channel_id", 337)
	ctx.Set("channel_affinity_skip_retry_on_failure", true)
	taskErr := &dto.TaskError{
		StatusCode: http.StatusServiceUnavailable,
		Error:      errors.New("pinned channel unavailable"),
	}

	assert.False(t, shouldRetryTaskRelay(ctx, 337, taskErr, 4))
	assert.False(t, service.HasEscapedChannelAffinityFailure(ctx))
}
