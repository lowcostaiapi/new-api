package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelAffinityFailureEscapeDefaultsRemainCompatibleWithOldRules(t *testing.T) {
	tests := []struct {
		name string
		rule ChannelAffinityRule
		want int
	}{
		{
			name: "codex old rule without field",
			rule: ChannelAffinityRule{Name: " codex CLI trace "},
			want: DefaultCodexChannelAffinityFailureEscapeMaxFallbacks,
		},
		{
			name: "claude old rule without field",
			rule: ChannelAffinityRule{Name: "claude cli trace"},
			want: DefaultChannelAffinityFailureEscapeMaxFallbacks,
		},
		{
			name: "custom old rule without field",
			rule: ChannelAffinityRule{Name: "custom"},
			want: DefaultChannelAffinityFailureEscapeMaxFallbacks,
		},
		{
			name: "explicit positive value",
			rule: ChannelAffinityRule{Name: "codex cli trace", FailureEscapeMaxFallbacks: 2},
			want: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.rule.GetFailureEscapeMaxFallbacks())
		})
	}
}

func TestChannelAffinityFailureEscapeDefaultsAfterOldOptionsJSONLoad(t *testing.T) {
	var rules []ChannelAffinityRule
	require.NoError(t, common.UnmarshalJsonStr(`[
		{"name":"codex cli trace","skip_retry_on_failure":true},
		{"name":"claude cli trace","skip_retry_on_failure":false},
		{"name":"custom","skip_retry_on_failure":true,"failure_escape_max_fallbacks":2}
	]`, &rules))
	require.Len(t, rules, 3)
	assert.Equal(t, 3, rules[0].GetFailureEscapeMaxFallbacks())
	assert.Equal(t, 1, rules[1].GetFailureEscapeMaxFallbacks())
	assert.Equal(t, 2, rules[2].GetFailureEscapeMaxFallbacks())
	assert.False(t, rules[1].SkipRetryOnFailure)
}
