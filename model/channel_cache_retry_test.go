package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannelNeverReusesExcludedAffinityChannels(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	channelSyncLock.Lock()
	originalGroups := group2model2channels
	originalChannels := channelsIDM
	originalAdvancedConfigs := channel2advancedCustomConfig

	priorityHigh := int64(100)
	priorityLow := int64(80)
	weight := uint(10)
	group2model2channels = map[string]map[string][]int{
		"codex-team": {"gpt-test": {337, 354, 384}},
	}
	channelsIDM = map[int]*Channel{
		337: {Id: 337, Priority: &priorityHigh, Weight: &weight},
		354: {Id: 354, Priority: &priorityHigh, Weight: &weight},
		384: {Id: 384, Priority: &priorityLow, Weight: &weight},
	}
	channel2advancedCustomConfig = map[int]*kitdto.AdvancedCustomConfig{}
	channelSyncLock.Unlock()
	common.MemoryCacheEnabled = true

	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		channelSyncLock.Lock()
		group2model2channels = originalGroups
		channelsIDM = originalChannels
		channel2advancedCustomConfig = originalAdvancedConfigs
		channelSyncLock.Unlock()
	})

	responsesPathFilters := []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: "/v1/responses"}}

	channel, err := GetRandomSatisfiedChannelExcluding(
		"codex-team",
		"gpt-test",
		1,
		responsesPathFilters,
		map[int]struct{}{337: {}},
	)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 354, channel.Id)

	channel, err = GetRandomSatisfiedChannelExcluding(
		"codex-team",
		"gpt-test",
		2,
		responsesPathFilters,
		map[int]struct{}{337: {}, 354: {}},
	)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 384, channel.Id)

	channel, err = GetRandomSatisfiedChannelExcluding(
		"codex-team",
		"gpt-test",
		3,
		responsesPathFilters,
		map[int]struct{}{337: {}, 354: {}, 384: {}},
	)
	require.NoError(t, err)
	assert.Nil(t, channel)
}
