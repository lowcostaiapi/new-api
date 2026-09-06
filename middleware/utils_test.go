package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAbortWithOpenAIMessageIncludesChannelSelectionErrorCode(t *testing.T) {
	oldErrorLogEnabled := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = false
	t.Cleanup(func() { constant.ErrorLogEnabled = oldErrorLogEnabled })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	abortWithOpenAiMessage(
		c,
		http.StatusServiceUnavailable,
		"no available channel",
		types.ErrorCodeGetChannelFailed,
	)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"get_channel_failed"`)
}
