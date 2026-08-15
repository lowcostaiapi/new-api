package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

func TestFinalFailureReportJSONPreservesTimestampAndAttemptOrder(t *testing.T) {
	report := finalFailureReport{
		Ts:         1723687200123,
		RequestId:  "req-1",
		StatusCode: 503,
		UseChannel: []string{"30", "33", "37"},
	}
	body, err := common.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	want := `{"ts":1723687200123,"source":"","request_id":"req-1","user_id":0,"username":"","group":"","model":"","status_code":503,"error_code":"","error_type":"","message":"","channel_id":0,"channel_name":"","use_channel":["30","33","37"],"retry_index":0,"path":""}`
	if got != want {
		t.Fatalf("unexpected report JSON:\n got %s\nwant %s", got, want)
	}
}

func TestFinalFailureStatusSkipIsExplicitOnly(t *testing.T) {
	finalFailureConfigOnce = sync.Once{}
	finalFailureHook = finalFailureHookConfig{}
	t.Setenv("FINAL_FAILURE_SKIP_STATUS", "401,413")
	cfg := finalFailureConfig()
	for _, code := range []int{401, 413} {
		if !cfg.skipStatus[code] {
			t.Fatalf("status %d should be explicitly skipped", code)
		}
	}
	for _, code := range []int{400, 403, 429, 500, 502, 503, 504, 524} {
		if cfg.skipStatus[code] {
			t.Fatalf("status %d should be reported unless the error has skip_retry", code)
		}
	}
}

func TestFinalFailureStructuredSkipRetry(t *testing.T) {
	err := types.NewErrorWithStatusCode(errors.New("invalid request"), types.ErrorCodeInvalidRequest, 400, types.ErrOptionWithSkipRetry())
	if !types.IsSkipRetryError(err) {
		t.Fatal("test error must carry skip_retry")
	}
}

func TestSendFinalFailureReportPostsAuthenticatedJSON(t *testing.T) {
	received := make(chan finalFailureReport, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("unexpected authorization header: %q", got)
		}
		var report finalFailureReport
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			t.Errorf("decode report: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- report
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	finalFailureRate = struct {
		sync.Mutex
		windowSec int64
		sent      int
		dropped   int
	}{}
	sendFinalFailureReport(finalFailureHookConfig{
		url: server.URL, token: "test-token", timeout: time.Second, rateLimit: 30,
	}, &finalFailureReport{
		Ts: 1723687200123, RequestId: "req-send", StatusCode: 503,
		UseChannel: []string{"30", "33"},
	})

	select {
	case got := <-received:
		if got.Ts != 1723687200123 || got.RequestId != "req-send" {
			t.Fatalf("unexpected report: %+v", got)
		}
		if len(got.UseChannel) != 2 || got.UseChannel[0] != "30" || got.UseChannel[1] != "33" {
			t.Fatalf("attempt chain changed: %#v", got.UseChannel)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for webhook")
	}
}
