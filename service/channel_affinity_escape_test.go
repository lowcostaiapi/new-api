package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildAffinityEscapeContext(t *testing.T) (*gin.Context, string) {
	return buildAffinityEscapeContextWithMax(t, 0)
}

func buildAffinityEscapeContextWithMax(t *testing.T, maxFallbacks int) (*gin.Context, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cacheKeySuffix := fmt.Sprintf("affinity-escape-test-%d", time.Now().UnixNano())
	cacheKeyFull := channelAffinityCacheNamespace + ":" + cacheKeySuffix
	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, 28, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	setChannelAffinityContext(ctx, channelAffinityMeta{
		CacheKey:                  cacheKeyFull,
		TTLSeconds:                60,
		RuleName:                  "claude cli trace",
		SkipRetry:                 true,
		FailureEscapeMaxFallbacks: maxFallbacks,
	})
	ctx.Set(ginKeyChannelAffinitySkipRetry, true)
	ctx.Set(ginKeyChannelAffinityLogInfo, map[string]interface{}{})
	return ctx, cacheKeySuffix
}

func TestChannelAffinityFailureEscapeBudgetDefaultsPerRule(t *testing.T) {
	require.Equal(t, 3, configuredChannelAffinityFailureEscapeMaxFallbacks(operation_setting.ChannelAffinityRule{Name: "codex cli trace"}))
	require.Equal(t, 1, configuredChannelAffinityFailureEscapeMaxFallbacks(operation_setting.ChannelAffinityRule{Name: "claude cli trace"}))
	require.Equal(t, 4, configuredChannelAffinityFailureEscapeMaxFallbacks(operation_setting.ChannelAffinityRule{Name: "custom", FailureEscapeMaxFallbacks: 4}))
}

func TestTryEscapeChannelAffinityFailureClearsCacheOnce(t *testing.T) {
	ctx, cacheKeySuffix := buildAffinityEscapeContext(t)

	require.True(t, TryEscapeChannelAffinityFailure(ctx, http.StatusServiceUnavailable, 3))
	require.True(t, HasEscapedChannelAffinityFailure(ctx))
	require.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))

	_, found, err := getChannelAffinityCache().Get(cacheKeySuffix)
	require.NoError(t, err)
	require.False(t, found)
	infoAny, ok := ctx.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info := infoAny.(map[string]interface{})
	require.Equal(t, 1, info["failure_escape_max_fallbacks"])
	require.Equal(t, 1, info["failure_escape_count"])
	require.False(t, TryEscapeChannelAffinityFailure(ctx, http.StatusServiceUnavailable, 2))
}

func TestTryEscapeChannelAffinityFailureRequiresUnwrittenResponse(t *testing.T) {
	ctx, cacheKeySuffix := buildAffinityEscapeContext(t)
	ctx.Writer.WriteHeaderNow()

	require.False(t, TryEscapeChannelAffinityFailure(ctx, http.StatusServiceUnavailable, 3))
	require.False(t, HasEscapedChannelAffinityFailure(ctx))

	_, found, err := getChannelAffinityCache().Get(cacheKeySuffix)
	require.NoError(t, err)
	require.True(t, found)
}

func TestTryEscapeChannelAffinityFailureStatusAllowlist(t *testing.T) {
	for _, statusCode := range []int{502, 503, 504, 524} {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			ctx, _ := buildAffinityEscapeContext(t)
			require.True(t, TryEscapeChannelAffinityFailure(ctx, statusCode, 1))
		})
	}

	ctx, _ := buildAffinityEscapeContext(t)
	require.False(t, TryEscapeChannelAffinityFailure(ctx, http.StatusTooManyRequests, 3))
}

func TestTryEscapeChannelAffinityFailureUsesConfiguredFallbackBudget(t *testing.T) {
	ctx, _ := buildAffinityEscapeContextWithMax(t, 3)

	require.True(t, TryEscapeChannelAffinityFailure(ctx, http.StatusServiceUnavailable, 4))
	require.True(t, ConsumeChannelAffinityFailureFallback(ctx, 3))
	require.True(t, ConsumeChannelAffinityFailureFallback(ctx, 2))
	require.False(t, ConsumeChannelAffinityFailureFallback(ctx, 1))
	require.False(t, ConsumeChannelAffinityFailureFallback(ctx, 0))

	infoAny, ok := ctx.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info := infoAny.(map[string]interface{})
	require.Equal(t, 3, info["failure_escape_max_fallbacks"])
	require.Equal(t, 3, info["failure_escape_count"])
}

func TestTryEscapeChannelAffinityFailureTimeoutKeepsOneFallbackCap(t *testing.T) {
	ctx, _ := buildAffinityEscapeContextWithMax(t, 3)

	require.True(t, TryEscapeChannelAffinityFailure(ctx, http.StatusGatewayTimeout, 4))
	require.False(t, ConsumeChannelAffinityFailureFallback(ctx, 3))
	infoAny, ok := ctx.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info := infoAny.(map[string]interface{})
	require.Equal(t, 1, info["failure_escape_max_fallbacks"])
}
