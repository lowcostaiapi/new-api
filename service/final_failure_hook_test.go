package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinalFailureReportsReachHTTPReceiver(t *testing.T) {
	reports := make(chan finalFailureReport, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var report finalFailureReport
		if err := common.DecodeJson(r.Body, &report); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		reports <- report
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	old := finalFailureConfig()
	finalFailureHook = finalFailureHookConfig{url: server.URL, timeout: time.Second, rateLimit: 100, skipStatus: map[int]bool{400: true}}
	t.Cleanup(func() { finalFailureHook = old })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{RequestId: "failure-test", OriginModelName: "mock-gpt", UsingGroup: "default", RetryIndex: 2}
	err := types.NewErrorWithStatusCode(assert.AnError, types.ErrorCodeModelNotFound, http.StatusServiceUnavailable)
	NotifyFinalFailure(c, info, err)
	NotifyDispatchFinalFailure(c, "default", "mock-gpt", 503, "no channel")
	seen := map[string]finalFailureReport{}
	for i := 0; i < 2; i++ {
		select {
		case report := <-reports:
			seen[report.Source] = report
		case <-time.After(3 * time.Second):
			t.Fatal("webhook not received")
		}
	}
	require.Len(t, seen, 2)
	assert.Equal(t, "failure-test", seen["relay"].RequestId)
	assert.Equal(t, 2, seen["relay"].RetryIndex)
	assert.Equal(t, 503, seen["dispatch"].StatusCode)
	assert.Equal(t, "mock-gpt", seen["dispatch"].Model)
}
