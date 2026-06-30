package a2autil

import (
	"context"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// FinalizeTask sends a terminal status update to complete an A2A Task.
func FinalizeTask(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool) error {
	status := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil)
	yield(status, nil)
	return nil
}
