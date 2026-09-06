package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// RecordRelayErrorLog records a relay failure with a stable error code so
// operators can distinguish local routing failures from upstream responses.
func RecordRelayErrorLog(c *gin.Context, err *types.NewAPIError) {
	if c == nil || err == nil || !constant.ErrorLogEnabled || !types.IsRecordErrorLog(err) {
		return
	}
	other := buildRelayErrorLogOther(c, err)

	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	model.RecordErrorLog(
		c,
		c.GetInt("id"),
		other["channel_id"].(int),
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

func buildRelayErrorLogOther(c *gin.Context, err *types.NewAPIError) map[string]interface{} {
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
	other := map[string]interface{}{
		"error_type":        err.GetErrorType(),
		"error_code":        err.GetErrorCode(),
		"status_code":       err.StatusCode,
		"attempted_channel": attemptedChannel,
		"channel_id":        channelID,
		"channel_name":      channelName,
		"channel_type":      channelType,
	}
	if err.GetErrorCode() == types.ErrorCodeGetChannelFailed {
		other["route_failure"] = true
	}
	if c.Request != nil && c.Request.URL != nil {
		other["request_path"] = c.Request.URL.Path
	}
	adminInfo := map[string]interface{}{
		"use_channel": usedChannels,
	}
	if common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
	}
	AppendChannelAffinityAdminInfo(c, adminInfo)
	other["admin_info"] = adminInfo
	return other
}
