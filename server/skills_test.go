package main

import (
	"context"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

func TestHandleEcho(t *testing.T) {
	e := &agentExecutor{}
	execCtx := &a2asrv.ExecutorContext{
		TaskID: "test-task-123",
		Message: &a2a.Message{
			ID:   "msg-123",
			Role: a2a.MessageRoleUser,
			Parts: []*a2a.Part{
				a2a.NewTextPart("hello"),
			},
		},
	}

	var events []a2a.Event
	yield := func(ev a2a.Event, err error) bool {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
		return true
	}

	err := e.handleEcho(context.Background(), execCtx, yield, "hello")
	if err != nil {
		t.Fatalf("handleEcho failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	msgEvent, ok := events[0].(*a2a.Message)
	if !ok {
		t.Fatalf("expected *a2a.Message, got %T", events[0])
	}

	if len(msgEvent.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(msgEvent.Parts))
	}

	textPart, ok := msgEvent.Parts[0].Content.(a2a.Text)
	if !ok {
		t.Fatalf("expected Text part, got %T", msgEvent.Parts[0].Content)
	}

	expected := "You said: hello"
	if string(textPart) != expected {
		t.Errorf("expected %q, got %q", expected, textPart)
	}
}

func TestHandleAdminEcho_MissingContext(t *testing.T) {
	e := &agentExecutor{}
	execCtx := &a2asrv.ExecutorContext{
		TaskID: "test-task-123",
	}

	var events []a2a.Event
	yield := func(ev a2a.Event, err error) bool {
		events = append(events, ev)
		return true
	}

	err := e.handleAdminEcho(context.Background(), execCtx, yield, "secret")
	if err != nil {
		t.Fatalf("handleAdminEcho failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	msgEvent, ok := events[0].(*a2a.Message)
	if !ok {
		t.Fatalf("expected *a2a.Message, got %T", events[0])
	}

	textPart, ok := msgEvent.Parts[0].Content.(a2a.Text)
	if !ok {
		t.Fatalf("expected Text part, got %T", msgEvent.Parts[0].Content)
	}

	expected := "Error: Call context missing."
	if string(textPart) != expected {
		t.Errorf("expected %q, got %q", expected, textPart)
	}
}
