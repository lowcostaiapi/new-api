package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTaskRelayDoesNotRetryCommittedResponse(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			taskErr := &dto.TaskError{StatusCode: status}
			assert.True(t, shouldRetryTaskRelay(c, 1, taskErr, 1))
			assert.False(t, shouldRetryTaskRelay(c, 1, taskErr, 0))
			c.Writer.WriteHeaderNow()
			assert.False(t, shouldRetryTaskRelay(c, 1, taskErr, 1))
		})
	}
}
