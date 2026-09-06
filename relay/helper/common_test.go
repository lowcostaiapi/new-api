package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlushingEventStreamHeadersMarksGinWriterWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	SetEventStreamHeaders(c)
	require.False(t, c.Writer.Written())
	require.NoError(t, FlushWriter(c))
	assert.True(t, c.Writer.Written())
}

func TestResetEventStreamHeadersPreservesJSONErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	SetEventStreamHeaders(c)
	ResetEventStreamHeaders(c)
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"error":"unavailable"}`, recorder.Body.String())
}
