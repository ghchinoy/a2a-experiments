package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

func TestMultimodalExecutor_AllArtifactsSuccess(t *testing.T) {
	// Create temporary assets directory for testing
	tempDir, err := os.MkdirTemp("", "multimodal-test-assets")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create expected dummy files
	requiredFiles := []string{"sample.png", "sample.wav", "sample.mp3", "sample.mp4", "sample.pdf"}
	for _, filename := range requiredFiles {
		err := os.WriteFile(filepath.Join(tempDir, filename), []byte("dummy-content"), 0644)
		if err != nil {
			t.Fatalf("failed to create dummy file %s: %v", filename, err)
		}
	}

	executor := &multimodalExecutor{assetsDir: tempDir}
	execCtx := &a2asrv.ExecutorContext{
		TaskID: "test-task-multimodal-123",
		Metadata: map[string]any{
			"skillId": "all-artifacts",
		},
		Message: &a2a.Message{
			ID:   "msg-multimodal-123",
			Role: a2a.MessageRoleUser,
		},
	}

	var events []a2a.Event
	var yieldedError error
	yield := func(ev a2a.Event, err error) bool {
		if err != nil {
			yieldedError = err
			return false
		}
		if ev != nil {
			events = append(events, ev)
		}
		return true
	}

	executor.Execute(context.Background(), execCtx)(yield)
	if yieldedError != nil {
		t.Fatalf("unexpected error: %v", yieldedError)
	}

	// We expect:
	// 1. Submitted Task (*a2a.Task)
	// 2. State Working (*a2a.TaskStatusUpdateEvent)
	// 3. Text Artifact (*a2a.TaskArtifactUpdateEvent)
	// 4. Data Artifact (*a2a.TaskArtifactUpdateEvent)
	// 5. Raw Artifact (*a2a.TaskArtifactUpdateEvent)
	// 6. FileURL Artifact (*a2a.TaskArtifactUpdateEvent)
	// 7. State Completed (*a2a.TaskStatusUpdateEvent)
	if len(events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(events))
	}

	// Assert the types and qualities of the events
	submittedTask, ok := events[0].(*a2a.Task)
	if !ok {
		t.Errorf("expected events[0] to be *a2a.Task, got %T", events[0])
	} else if submittedTask.Status.State != a2a.TaskStateSubmitted {
		t.Errorf("expected submitted state, got %s", submittedTask.Status.State)
	}

	workingEv, ok := events[1].(*a2a.TaskStatusUpdateEvent)
	if !ok {
		t.Errorf("expected events[1] to be *a2a.TaskStatusUpdateEvent, got %T", events[1])
	} else if workingEv.Status.State != a2a.TaskStateWorking {
		t.Errorf("expected working state, got %s", workingEv.Status.State)
	}

	// Verify All 4 Artifact types
	names := []string{"text-artifact", "data-artifact", "raw-artifact", "fileurl-artifact"}
	for i, name := range names {
		artEv, ok := events[2+i].(*a2a.TaskArtifactUpdateEvent)
		if !ok {
			t.Fatalf("expected events[%d] to be *a2a.TaskArtifactUpdateEvent, got %T", 2+i, events[2+i])
		}
		if artEv.Artifact.Name != name {
			t.Errorf("expected artifact name %q, got %q", name, artEv.Artifact.Name)
		}
		if name == "fileurl-artifact" && !artEv.LastChunk {
			t.Error("expected LastChunk to be true on the final URL artifact")
		}
	}

	completedEv, ok := events[6].(*a2a.TaskStatusUpdateEvent)
	if !ok {
		t.Errorf("expected events[6] to be *a2a.TaskStatusUpdateEvent, got %T", events[6])
	} else if completedEv.Status.State != a2a.TaskStateCompleted {
		t.Errorf("expected completed terminal state, got %s", completedEv.Status.State)
	}
}

func TestMultimodalExecutor_AllArtifactsMissingAssets(t *testing.T) {
	// Point to empty temp directory
	tempDir, err := os.MkdirTemp("", "multimodal-missing-assets")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	executor := &multimodalExecutor{assetsDir: tempDir}
	execCtx := &a2asrv.ExecutorContext{
		TaskID: "test-task-missing",
		Metadata: map[string]any{
			"skillId": "all-artifacts",
		},
		Message: &a2a.Message{
			ID:   "msg-missing-123",
			Role: a2a.MessageRoleUser,
		},
	}

	var events []a2a.Event
	yield := func(ev a2a.Event, err error) bool {
		if ev != nil {
			events = append(events, ev)
		}
		return true
	}

	executor.Execute(context.Background(), execCtx)(yield)

	// We expect:
	// 1. Submitted Task
	// 2. Working State
	// 3. Failed State (with description message)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	failedEv, ok := events[2].(*a2a.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", events[2])
	}

	if failedEv.Status.State != a2a.TaskStateFailed {
		t.Errorf("expected state to be Failed, got %s", failedEv.Status.State)
	}

	// Verify error message content
	if failedEv.Status.Message == nil || len(failedEv.Status.Message.Parts) != 1 {
		t.Fatal("expected status message with text description")
	}

	textPart, ok := failedEv.Status.Message.Parts[0].Content.(a2a.Text)
	if !ok {
		t.Fatalf("expected Text content, got %T", failedEv.Status.Message.Parts[0].Content)
	}

	if !strings.Contains(string(textPart), "Required test asset") {
		t.Errorf("expected descriptive missing asset error message, got %q", textPart)
	}
}

func TestMultimodalExecutor_StateTransitions(t *testing.T) {
	testCases := []struct {
		skillID   string
		wantState a2a.TaskState
	}{
		{"state-working", a2a.TaskStateWorking},
		{"state-input-required", a2a.TaskStateInputRequired},
		{"state-auth-required", a2a.TaskStateAuthRequired},
		{"state-completed", a2a.TaskStateCompleted},
		{"state-failed", a2a.TaskStateFailed},
		{"state-canceled", a2a.TaskStateCanceled},
	}

	for _, tc := range testCases {
		t.Run(tc.skillID, func(t *testing.T) {
			executor := &multimodalExecutor{}
			execCtx := &a2asrv.ExecutorContext{
				TaskID: "test-task-state",
				Metadata: map[string]any{
					"skillId": tc.skillID,
				},
				Message: &a2a.Message{
					ID:   "msg-state-123",
					Role: a2a.MessageRoleUser,
				},
			}

			var events []a2a.Event
			yield := func(ev a2a.Event, err error) bool {
				if ev != nil {
					events = append(events, ev)
				}
				return true
			}

			executor.Execute(context.Background(), execCtx)(yield)

			// Expect:
			// 1. Submitted Task
			// 2. Specific status update event for tc.wantState
			if len(events) != 2 {
				t.Fatalf("expected 2 events, got %d", len(events))
			}

			stateEv, ok := events[1].(*a2a.TaskStatusUpdateEvent)
			if !ok {
				t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", events[1])
			}

			if stateEv.Status.State != tc.wantState {
				t.Errorf("expected state %s, got %s", tc.wantState, stateEv.Status.State)
			}
		})
	}
}
