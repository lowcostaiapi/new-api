package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 504/524 是超时类错误:换渠道重试有意义,但每次都要等上游超时(约 2 分钟),
// 多次重试会拖死用户。护栏:只允许额外重试 1 次。
func TestShouldRetryTimeoutStatusLimitedToOneRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, code := range []int{http.StatusGatewayTimeout, 524} {
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))

			// retryTimes=2(还有 2 次预算,含本次)→ 允许重试
			err := types.NewOpenAIError(errors.New("upstream timeout"), types.ErrorCodeBadResponseStatusCode, code)
			require.True(t, shouldRetry(ctx, err, 2))

			// retryTimes=1(只剩 1 次预算,已重试过一次)→ 不再重试
			recorder2 := httptest.NewRecorder()
			ctx2, _ := gin.CreateTestContext(recorder2)
			ctx2.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
			require.False(t, shouldRetry(ctx2, err, 1))

			// retryTimes=0(预算用尽)→ 不再重试
			recorder3 := httptest.NewRecorder()
			ctx3, _ := gin.CreateTestContext(recorder3)
			ctx3.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
			require.False(t, shouldRetry(ctx3, err, 0))
		})
	}
}

// 非超时错误(503)不受限次影响,retryTimes 大就照常重试
func TestShouldRetryNonTimeoutStatusNotLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))

	err503 := types.NewOpenAIError(errors.New("service unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable)
	require.True(t, shouldRetry(ctx, err503, 1))
	require.False(t, shouldRetry(ctx, err503, 0))
}
