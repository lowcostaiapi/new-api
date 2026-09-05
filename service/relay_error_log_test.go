package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRelayErrorLogOtherClearsStaleChannelForRouteFailure(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set("channel_id", 37)
	c.Set("channel_name", "stale-channel")
	c.Set("channel_type", 1)
	err := types.NewErrorWithStatusCode(errors.New("no available channel"), types.ErrorCodeGetChannelFailed, http.StatusServiceUnavailable)

	other := buildRelayErrorLogOther(c, err)

	require.NotNil(t, other)
	assert.Equal(t, true, other["route_failure"])
	assert.Equal(t, false, other["attempted_channel"])
	assert.Equal(t, 0, other["channel_id"])
	assert.Equal(t, "", other["channel_name"])
	assert.Equal(t, 0, other["channel_type"])
}

func TestBuildRelayErrorLogOtherPreservesAttemptedChannel(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set("use_channel", []string{"37"})
	c.Set("channel_id", 37)
	c.Set("channel_name", "attempted-channel")
	c.Set("channel_type", 1)
	err := types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable)

	other := buildRelayErrorLogOther(c, err)

	require.NotNil(t, other)
	assert.Equal(t, true, other["attempted_channel"])
	assert.NotContains(t, other, "route_failure")
	assert.Equal(t, 37, other["channel_id"])
	assert.Equal(t, "attempted-channel", other["channel_name"])
	assert.Equal(t, 1, other["channel_type"])
}
