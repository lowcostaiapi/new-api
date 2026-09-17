package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// RecordRelayErrorLog records a relay failure with a stable error code so
// operators can distinguish local routing failures from upstream responses.
func RecordRelayErrorLog(c *gin.Context, err *types.NewAPIError) {
	recordRelayErrorLog(c, err, nil, 0)
}

// recordRelayErrorLog writes the error log. attemptedChannelID, when non-zero,
// is the channel captured by the caller and wins over the gin context, which
// may already describe another channel when processed asynchronously.
func recordRelayErrorLog(c *gin.Context, err *types.NewAPIError, relayInfo *relaycommon.RelayInfo, attemptedChannelID int) {
	if c == nil || err == nil || !constant.ErrorLogEnabled || !types.IsRecordErrorLog(err) {
		return
	}
	other, channelID := buildRelayErrorLogOther(c, err, relayInfo)
	if attemptedChannelID != 0 {
		channelID = attemptedChannelID
	}

	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	model.RecordErrorLog(
		c,
		c.GetInt("id"),
		channelID,
		c.GetString("original_model"),
		c.GetString("token_name"),
		err.MaskSensitiveErrorWithStatusCode(),
		c.GetInt("token_id"),
		int(time.Since(startTime).Seconds()),
		common.GetContextKeyBool(c, constant.ContextKeyIsStream),
		c.GetString("group"),
		other,
	)
}

// buildRelayErrorLogOther returns the log metadata and the channel id the log
// row should be attributed to. Channel identity is privileged metadata, so it
// lives in the admin scope rather than the user-visible top level.
func buildRelayErrorLogOther(c *gin.Context, err *types.NewAPIError, relayInfo *relaycommon.RelayInfo) (*model.LogOther, int) {
	usedChannels := c.GetStringSlice("use_channel")
	attemptedChannel := len(usedChannels) > 0
	channelID := c.GetInt("channel_id")
	channelName := c.GetString("channel_name")
	channelType := c.GetInt("channel_type")
	if !attemptedChannel {
		channelID = 0
		channelName = ""
		channelType = 0
	}
	other := model.NewLogOther()
	other.SetPublic("error_type", err.GetErrorType())
	other.SetPublic("error_code", err.GetErrorCode())
	other.SetPublic("status_code", err.StatusCode)
	other.SetPublic("attempted_channel", attemptedChannel)
	if err.GetErrorCode() == types.ErrorCodeGetChannelFailed {
		other.SetPublic("route_failure", true)
	}
	if c.Request != nil && c.Request.URL != nil {
		other.SetPublic("request_path", c.Request.URL.Path)
	}
	other.SetAdmin("channel_id", channelID)
	other.SetAdmin("channel_name", channelName)
	other.SetAdmin("channel_type", channelType)
	AppendRelayLogAdminInfo(c, relayInfo, other)
	AppendTaskPluginContextAuditInfo(c, other)
	return other, channelID
}
