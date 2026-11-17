package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMixedWorkloadDistribution(t *testing.T) {
	counts := make(map[operation]int)
	for i := 0; i < 100; i++ {
		counts[chooseOperation("mixed", i)]++
	}
	require.Equal(t, 20, counts[opHotPost])
	require.Equal(t, 20, counts[opPostList])
	require.Equal(t, 20, counts[opComments])
	require.Equal(t, 15, counts[opConversations])
	require.Equal(t, 10, counts[opFollow])
	require.Equal(t, 5, counts[opMessageHistory])
	require.Equal(t, 5, counts[opUnread])
	require.Equal(t, 5, counts[opSendMessage])
}

func TestDMModeOnlySendsMessages(t *testing.T) {
	for i := 0; i < 100; i++ {
		require.Equal(t, opSendMessage, chooseOperation("dm", i))
	}
}
