package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowWorkerPoolProcessesActiveTokensInFIFOOrder(t *testing.T) {
	pool := newWorkflowWorkerPool(2)
	queue := []*WorkflowToken{
		{ID: "token-1", NodeID: "left"},
		{ID: "token-2", NodeID: "right"},
		{ID: "token-3", NodeID: "final"},
	}

	order := pool.ScheduleOrder(queue)

	require.Equal(t, []string{"token-1", "token-2", "token-3"}, order)
	assert.Equal(t, []string{"left", "right", "final"}, []string{queue[0].NodeID, queue[1].NodeID, queue[2].NodeID})
}
