package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newTestCtx() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func TestExtractUpstreamRequestId(t *testing.T) {
	assert.Equal(t, "", ExtractUpstreamRequestId(nil))
	assert.Equal(t, "", ExtractUpstreamRequestId(http.Header{}))

	h := http.Header{}
	h.Set("X-Cpa-Trace-Id", "20260909230408-59530628f5afdb22-35a66a6c")
	assert.Equal(t, "20260909230408-59530628f5afdb22-35a66a6c", ExtractUpstreamRequestId(h))

	h.Set(common.RequestIdKey, "oneapi-123")
	assert.Equal(t, "oneapi-123", ExtractUpstreamRequestId(h), "X-Oneapi-Request-Id wins over the CPA trace id")
}

func TestShouldCopyUpstreamHeader_CpaTraceId(t *testing.T) {
	c := newTestCtx()
	assert.False(t, ShouldCopyUpstreamHeader(c, "X-Cpa-Trace-Id", []string{"trace-1"}), "CPA trace id must not be forwarded downstream")
	assert.Equal(t, "trace-1", c.GetString(common.UpstreamRequestIdKey))

	// An explicit X-Oneapi-Request-Id overrides a previously captured trace id.
	assert.False(t, ShouldCopyUpstreamHeader(c, common.RequestIdKey, []string{"oneapi-9"}))
	assert.Equal(t, "oneapi-9", c.GetString(common.UpstreamRequestIdKey))
	// ...and a later CPA trace id does not clobber it.
	assert.False(t, ShouldCopyUpstreamHeader(c, "x-cpa-trace-id", []string{"trace-2"}))
	assert.Equal(t, "oneapi-9", c.GetString(common.UpstreamRequestIdKey))

	assert.True(t, ShouldCopyUpstreamHeader(c, "Content-Type", []string{"application/json"}))
	assert.False(t, ShouldCopyUpstreamHeader(c, "Content-Length", []string{"1"}))
}
