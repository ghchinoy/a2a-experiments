package main

import (
	"context"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

func TestEchoExecutor_ExecuteText(t *testing.T) {
	executor := &echoExecutor{}
	execCtx := &a2asrv.ExecutorContext{
		TaskID: "test-task-echo-123",
		Message: &a2a.Message{
			ID:   "msg-echo-123",
			Role: a2a.MessageRoleUser,
			Parts: []*a2a.Part{
				a2a.NewTextPart("ping"),
			},
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
	// 1. TaskSubmittedEvent (represented as *a2a.Task)
	// 2. StatusUpdateEvent (represented as *a2a.TaskStatusUpdateEvent with StateWorking)
	// 3. TaskArtifactUpdateEvent (representing our echoed text part)
	// 4. StatusUpdateEvent (represented as *a2a.TaskStatusUpdateEvent with StateCompleted)
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// Assert the artifact event
	artifactEv, ok := events[2].(*a2a.TaskArtifactUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskArtifactUpdateEvent, got %T", events[2])
	}

	if artifactEv.Artifact.Name != "part-0-text" {
		t.Errorf("expected artifact name 'part-0-text', got %q", artifactEv.Artifact.Name)
	}

	if len(artifactEv.Artifact.Parts) != 1 {
		t.Fatalf("expected 1 part in artifact, got %d", len(artifactEv.Artifact.Parts))
	}

	textPart, ok := artifactEv.Artifact.Parts[0].Content.(a2a.Text)
	if !ok {
		t.Fatalf("expected Text content in artifact part, got %T", artifactEv.Artifact.Parts[0].Content)
	}

	if string(textPart) != "ping" {
		t.Errorf("expected text content 'ping', got %q", textPart)
	}

	if !artifactEv.LastChunk {
		t.Error("expected LastChunk to be true on the last artifact part")
	}

	// Assert final status is Completed
	statusEv, ok := events[3].(*a2a.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected final event to be *a2a.TaskStatusUpdateEvent, got %T", events[3])
	}

	if statusEv.Status.State != a2a.TaskStateCompleted {
		t.Errorf("expected state to be Completed, got %s", statusEv.Status.State)
	}
}

func TestEchoExecutor_ExecuteEmpty(t *testing.T) {
	executor := &echoExecutor{}
	execCtx := &a2asrv.ExecutorContext{
		TaskID:  "test-task-echo-empty",
		Message: &a2a.Message{},
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

	// We expect: Submitted, Working, Artifact (with fallback "No content received"), Completed
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	artifactEv, ok := events[2].(*a2a.TaskArtifactUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskArtifactUpdateEvent, got %T", events[2])
	}

	if artifactEv.Artifact.Name != "echo" {
		t.Errorf("expected artifact name 'echo', got %q", artifactEv.Artifact.Name)
	}

	textPart, ok := artifactEv.Artifact.Parts[0].Content.(a2a.Text)
	if !ok {
		t.Fatalf("expected Text content, got %T", artifactEv.Artifact.Parts[0].Content)
	}

	if string(textPart) != "No content received" {
		t.Errorf("expected 'No content received', got %q", textPart)
	}
}
