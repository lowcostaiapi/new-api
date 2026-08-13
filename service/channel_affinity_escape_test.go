package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildAffinityEscapeContext(t *testing.T) (*gin.Context, string) {
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
		CacheKey:   cacheKeyFull,
		TTLSeconds: 60,
		RuleName:   "claude cli trace",
		SkipRetry:  true,
	})
	ctx.Set(ginKeyChannelAffinitySkipRetry, true)
	return ctx, cacheKeySuffix
}

func TestTryEscapeChannelAffinityFailureClearsCacheOnce(t *testing.T) {
	ctx, cacheKeySuffix := buildAffinityEscapeContext(t)

	require.True(t, TryEscapeChannelAffinityFailure(ctx, http.StatusServiceUnavailable, 3))
	require.True(t, HasEscapedChannelAffinityFailure(ctx))
	require.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))

	_, found, err := getChannelAffinityCache().Get(cacheKeySuffix)
	require.NoError(t, err)
	require.False(t, found)
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
