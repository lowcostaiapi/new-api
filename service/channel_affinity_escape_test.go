package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildAffinityEscapeContext(t *testing.T, maxFallbacks int) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	setChannelAffinityContext(ctx, channelAffinityMeta{
		CacheKey:                  "channel-affinity-escape-test",
		TTLSeconds:                60,
		RuleName:                  "codex cli trace",
		SkipRetry:                 true,
		FailureEscapeMaxFallbacks: maxFallbacks,
	})
	ctx.Set(ginKeyChannelAffinitySkipRetry, true)
	ctx.Set(ginKeyChannelAffinityLogInfo, map[string]interface{}{})
	return ctx
}

func TestChannelAffinityFailureEscapeConsumesConfiguredBudget(t *testing.T) {
	ctx := buildAffinityEscapeContext(t, 3)

	require.True(t, ChannelAffinityPinnedFirstAttempt(ctx))
	require.True(t, TryEscapeChannelAffinityFailure(ctx, http.StatusBadGateway, 4))
	assert.True(t, HasEscapedChannelAffinityFailure(ctx))
	assert.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))
	assert.True(t, ChannelAffinityPinnedFirstAttempt(ctx))
	require.True(t, ConsumeChannelAffinityFailureFallback(ctx, http.StatusServiceUnavailable, 3))
	require.True(t, ConsumeChannelAffinityFailureFallback(ctx, http.StatusServiceUnavailable, 2))
	assert.False(t, ConsumeChannelAffinityFailureFallback(ctx, http.StatusServiceUnavailable, 1))

	infoAny, ok := ctx.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info := infoAny.(map[string]interface{})
	assert.Equal(t, 3, info["failure_escape_count"])
	assert.Equal(t, 3, info["failure_escape_max_fallbacks"])
}

func TestChannelAffinityFailureEscapeTimeoutOnlyLowersBudget(t *testing.T) {
	ctx := buildAffinityEscapeContext(t, 3)

	require.True(t, TryEscapeChannelAffinityFailure(ctx, http.StatusBadGateway, 4))
	require.True(t, ConsumeChannelAffinityFailureFallback(ctx, http.StatusServiceUnavailable, 3))
	assert.False(t, ConsumeChannelAffinityFailureFallback(ctx, http.StatusGatewayTimeout, 2))

	infoAny, ok := ctx.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info := infoAny.(map[string]interface{})
	assert.Equal(t, 2, info["failure_escape_count"])
	assert.Equal(t, 1, info["failure_escape_max_fallbacks"])
}

func TestChannelAffinityFailureEscapeRequiresUnwrittenResponse(t *testing.T) {
	ctx := buildAffinityEscapeContext(t, 3)
	ctx.Writer.WriteHeaderNow()

	assert.False(t, TryEscapeChannelAffinityFailure(ctx, http.StatusServiceUnavailable, 4))
	assert.False(t, HasEscapedChannelAffinityFailure(ctx))
}
