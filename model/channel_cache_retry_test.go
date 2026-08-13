package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testChannel(id int, priority int64, weight uint) *Channel {
	return &Channel{Id: id, Priority: &priority, Weight: &weight}
}

func TestFilterRetryCandidatesPrefersUntriedChannelInSamePriority(t *testing.T) {
	channels := []*Channel{
		testChannel(33, 100, 50),
		testChannel(15, 100, 30),
		testChannel(35, 100, 20),
	}

	candidates := filterRetryCandidates(channels, map[int]struct{}{33: {}, 15: {}})

	require.Equal(t, []int{35}, channelIDs(candidates))
}

func TestFilterRetryCandidatesDoesNotReturnPreviouslyFailedChannel(t *testing.T) {
	channels := []*Channel{
		testChannel(30, 100, 50),
		testChannel(33, 100, 30),
		testChannel(35, 100, 20),
	}

	candidates := filterRetryCandidates(channels, map[int]struct{}{30: {}, 33: {}})

	require.Equal(t, []int{35}, channelIDs(candidates))
}

func TestFilterRetryCandidatesExhaustsHealthyPriorityBeforeDescending(t *testing.T) {
	channels := []*Channel{
		testChannel(30, 100, 50),
		testChannel(33, 100, 30),
		testChannel(35, 80, 20),
	}

	candidates := filterRetryCandidates(channels, map[int]struct{}{30: {}})
	require.Equal(t, []int{33}, channelIDs(candidates))

	candidates = filterRetryCandidates(channels, map[int]struct{}{30: {}, 33: {}})
	require.Equal(t, []int{35}, channelIDs(candidates))
}

func TestFilterRetryCandidatesAllowsRepeatOnlyAfterAllCandidatesTried(t *testing.T) {
	channels := []*Channel{
		testChannel(30, 100, 50),
		testChannel(33, 100, 30),
		testChannel(35, 80, 20),
	}

	candidates := filterRetryCandidates(channels, map[int]struct{}{30: {}, 33: {}, 35: {}})

	require.ElementsMatch(t, []int{30, 33}, channelIDs(candidates))
}

func TestFilterRetryCandidatesReturnsEmptyForAutoGroupAfterExhaustion(t *testing.T) {
	channels := []*Channel{
		testChannel(30, 100, 50),
		testChannel(33, 80, 30),
	}

	candidates := filterRetryCandidates(channels, map[int]struct{}{30: {}, 33: {}}, false)

	require.Empty(t, candidates)
}

func TestFilterRetryAbilitiesMatchesCacheRetryPolicy(t *testing.T) {
	p100 := int64(100)
	p80 := int64(80)
	abilities := []Ability{
		{ChannelId: 30, Priority: &p100, Weight: 50},
		{ChannelId: 33, Priority: &p100, Weight: 30},
		{ChannelId: 35, Priority: &p80, Weight: 20},
	}

	filtered := filterRetryAbilities(abilities, map[int]struct{}{30: {}}, true)
	require.Equal(t, []int{33}, abilityChannelIDs(filtered))

	filtered = filterRetryAbilities(abilities, map[int]struct{}{30: {}, 33: {}}, true)
	require.Equal(t, []int{35}, abilityChannelIDs(filtered))

	filtered = filterRetryAbilities(abilities, map[int]struct{}{30: {}, 33: {}, 35: {}}, false)
	require.Empty(t, filtered)
}

func TestFilterRetryCandidatesKeepsWeightsOnRemainingCandidates(t *testing.T) {
	channels := []*Channel{
		testChannel(30, 100, 70),
		testChannel(33, 100, 20),
		testChannel(35, 100, 10),
	}

	candidates := filterRetryCandidates(channels, map[int]struct{}{30: {}})

	require.Equal(t, []int{33, 35}, channelIDs(candidates))
	require.Equal(t, 20, candidates[0].GetWeight())
	require.Equal(t, 10, candidates[1].GetWeight())
}

func abilityChannelIDs(abilities []Ability) []int {
	ids := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		ids = append(ids, ability.ChannelId)
	}
	return ids
}

func channelIDs(channels []*Channel) []int {
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.Id)
	}
	return ids
}
