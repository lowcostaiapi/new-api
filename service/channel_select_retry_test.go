package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetryParamTracksFailedChannelsWithoutDuplicates(t *testing.T) {
	param := &RetryParam{}

	param.MarkChannelFailed(33)
	param.MarkChannelFailed(15)
	param.MarkChannelFailed(33)

	require.Equal(t, map[int]struct{}{33: {}, 15: {}}, param.FailedChannels)
}
