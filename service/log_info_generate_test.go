package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTextOtherInfoIncludesUpstreamHeaderTime(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	relayInfo := &relaycommon.RelayInfo{
		StartTime:          start,
		UpstreamHeaderTime: start.Add(250 * time.Millisecond),
		FirstResponseTime:  start.Add(3 * time.Second),
		ChannelMeta:        &relaycommon.ChannelMeta{},
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	other := GenerateTextOtherInfo(c, relayInfo, 1, 1, 1, 0, 0, 0, 1)

	assert.Equal(t, float64(250), other["uhrt"])
	assert.Equal(t, float64(3000), other["frt"])
}
