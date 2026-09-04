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
	other := map[string]interface{}{
		"error_type":   err.GetErrorType(),
		"error_code":   err.GetErrorCode(),
		"status_code":  err.StatusCode,
		"channel_id":   c.GetInt("channel_id"),
		"channel_name": c.GetString("channel_name"),
		"channel_type": c.GetInt("channel_type"),
	}
	if c.Request != nil && c.Request.URL != nil {
		other["request_path"] = c.Request.URL.Path
	}
	adminInfo := map[string]interface{}{
		"use_channel": c.GetStringSlice("use_channel"),
	}
	if common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
	}
	AppendChannelAffinityAdminInfo(c, adminInfo)
	other["admin_info"] = adminInfo

	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	model.RecordErrorLog(
		c,
		c.GetInt("id"),
		c.GetInt("channel_id"),
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
