package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
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
