package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A single slow attempt is not evidence that a channel is broken. The upstream
// header deadline must therefore stay outside the "channel:" error namespace,
// because ShouldDisableChannel disables the channel for any channel error and
// would take every model on that channel offline after one slow response.
func TestShouldDisableChannelIgnoresUpstreamHeaderTimeout(t *testing.T) {
	originalEnabled := common.AutomaticDisableChannelEnabled
	originalRanges := operation_setting.AutomaticDisableStatusCodeRanges
	originalKeywords := operation_setting.AutomaticDisableKeywords
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = originalEnabled
		operation_setting.AutomaticDisableStatusCodeRanges = originalRanges
		operation_setting.AutomaticDisableKeywords = originalKeywords
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 401, End: 401}}
	operation_setting.AutomaticDisableKeywords = []string{"Your credit balance is too low"}

	headerTimeout := types.NewErrorWithStatusCode(
		errors.New("upstream response header timeout"),
		types.ErrorCodeUpstreamHeaderTimeout,
		http.StatusGatewayTimeout,
	)

	require.False(t, types.IsChannelError(headerTimeout))
	assert.False(t, ShouldDisableChannel(headerTimeout))

	// A genuine channel-level fault still disables the channel.
	invalidKey := types.NewErrorWithStatusCode(
		errors.New("invalid key"),
		types.ErrorCodeChannelInvalidKey,
		http.StatusUnauthorized,
	)
	assert.True(t, ShouldDisableChannel(invalidKey))
}
