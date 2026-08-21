package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestRetryParamKeepsGlobalAttemptWhenSelectionRetryResets(t *testing.T) {
	param := RetryParam{Retry: common.GetPointer(3)}
	param.MarkChannelFailed(337)
	param.MarkChannelFailed(354)
	param.MarkChannelFailed(337)

	param.SetRetry(0)
	param.ResetRetryNextTry()
	param.IncreaseAttempt()

	assert.Equal(t, 1, param.GetAttempt())
	assert.Equal(t, 0, param.GetRetry())
	assert.Equal(t, map[int]struct{}{337: {}, 354: {}}, param.FailedChannels)
}
