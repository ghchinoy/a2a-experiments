package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"a2a-simple/pkg/interactions"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	"google.golang.org/genai"
)

// handleHelloWorld provides a context-aware greeting using Gemini.
func (e *agentExecutor) handleHelloWorld(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool, input string) error {
	var textResponse string
	if e.genaiClient != nil {
		prompt := fmt.Sprintf("The user said %q. Respond with a friendly, professional greeting as an A2A agent. Keep it concise.", input)
		resp, genErr := e.genaiClient.Models.GenerateContent(ctx, e.model, genai.Text(prompt), nil)
		if genErr != nil {
			log.Printf("[Task: %s] Gemini Error: %v", execCtx.TaskID, genErr)
			textResponse = "Hello! (Gemini error, fallback)"
		} else if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
			textResponse = resp.Candidates[0].Content.Parts[0].Text
		}
	}
	if textResponse == "" {
		textResponse = "Hello from the Simple A2A Server!"
	}
	
	yield(a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(textResponse)), nil)
	return nil
}

// handleEcho simple echoes the user input.
func (e *agentExecutor) handleEcho(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool, input string) error {
	yield(a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(fmt.Sprintf("You said: %s", input))), nil)
	return nil
}

// handleAdminEcho echoes input only if the user is authenticated as Admin.
func (e *agentExecutor) handleAdminEcho(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool, input string) error {
	callCtx, ok := a2asrv.CallContextFrom(ctx)
	if !ok {
		yield(a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Error: Call context missing.")), nil)
		return nil
	}
	
	yield(a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(fmt.Sprintf("Admin %s says: %s", callCtx.User.Name, input))), nil)
	return nil
}

// handleStatefulInteraction manages long-running research or chat sessions via the Interactions API.
func (e *agentExecutor) handleStatefulInteraction(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool, input string, isResearch bool) error {
	if e.interactionsClient == nil {
		yield(a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Stateful logic is unavailable: Interactions API key not configured.")), nil)
		return nil
	}

	// Session Recovery logic
	var prevInteractionID string
	if execCtx.StoredTask != nil && execCtx.StoredTask.Metadata != nil {
		if val, ok := execCtx.StoredTask.Metadata["gemini_interaction_id"].(string); ok {
			prevInteractionID = val
		}
	}
	if prevInteractionID == "" {
		for _, related := range execCtx.RelatedTasks {
			if related.Metadata != nil {
				if val, ok := related.Metadata["gemini_interaction_id"].(string); ok {
					prevInteractionID = val
					log.Printf("[Task: %s] Found Gemini session from referenced task: %s", execCtx.TaskID, related.ID)
					break
				}
			}
		}
	}

	// Always make this stateful (create a task if it doesn't exist)
	if execCtx.StoredTask == nil {
		if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
			return nil
		}
	}

	agent := ""
	if isResearch {
		agent = "deep-research-pro-preview-12-2025"
		log.Printf("[Task: %s] Starting Deep Research for: %q", execCtx.TaskID, input)
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Initializing Deep Research Agent..."))), nil)
	} else {
		log.Printf("[Task: %s] Starting Chat for: %q", execCtx.TaskID, input)
	}

	req := &interactions.InteractionRequest{
		Input:                 input,
		Background:            true,
		PreviousInteractionID: prevInteractionID,
	}
	if agent != "" {
		req.Agent = agent
	} else {
		req.Model = e.model
	}

	resp, err := e.interactionsClient.Create(ctx, req)
	if err != nil {
		log.Printf("[Task: %s] Interactions API Error: %v", execCtx.TaskID, err)
		yield(a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(fmt.Sprintf("Failed to start interaction: %v", err))), nil)
		return nil
	}

	// Update Task Metadata with new Interaction ID
	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]any)
	}
	execCtx.Metadata["gemini_interaction_id"] = resp.ID

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var finalResp *interactions.InteractionResponse
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			current, err := e.interactionsClient.Get(ctx, resp.ID)
			if err != nil {
				continue
			}

			log.Printf("[Task: %s] Interaction %s status: %s", execCtx.TaskID, resp.ID, current.Status)
			status := strings.ToLower(current.Status)
			if status != "working" && status != "in_progress" && status != "pending" {
				finalResp = current
				goto Finished
			}

			if isResearch {
				statusMsg := a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(fmt.Sprintf("Deep Research in progress (Status: %s)...", current.Status)))
				yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, statusMsg), nil)
			}
		}
	}

Finished:
	if finalResp == nil || finalResp.Error != nil {
		errMsg := "Interaction failed."
		if finalResp != nil && finalResp.Error != nil {
			errMsg = fmt.Sprintf("Interaction failed: %s", finalResp.Error.Message)
		}
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateFailed, a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(errMsg))), nil)
		return nil
	}

	var resultText string
	if len(finalResp.Outputs) > 0 {
		if finalResp.Outputs[0].Text != "" {
			resultText = finalResp.Outputs[0].Text
		} else if len(finalResp.Outputs[0].Parts) > 0 {
			resultText = finalResp.Outputs[0].Parts[0].Text
		}
	}

	if isResearch {
		artifactEvent := a2a.NewArtifactEvent(execCtx, a2a.NewTextPart(resultText))
		artifactEvent.Artifact.Name = "Deep Research Report"
		artifactEvent.Artifact.Description = fmt.Sprintf("Research result for: %s", input)
		yield(artifactEvent, nil)
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil), nil)
	} else {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(resultText))), nil)
	}

	return nil
}

// handleSummarize handles the summarization of input or referenced task artifacts.
func (e *agentExecutor) handleSummarize(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool, input string) error {
	var contentToSummarize string
	sourceDescription := "direct input"

	findReport := func(task *a2a.Task) string {
		if task == nil {
			return ""
		}
		for _, artifact := range task.Artifacts {
			if artifact.Name == "Deep Research Report" {
				for _, part := range artifact.Parts {
					if text, ok := part.Content.(a2a.Text); ok {
						return string(text)
					}
				}
			}
		}
		return ""
	}

	if report := findReport(execCtx.StoredTask); report != "" {
		contentToSummarize = report
		sourceDescription = "Task history"
	} else if len(execCtx.RelatedTasks) > 0 {
		for _, related := range execCtx.RelatedTasks {
			if report := findReport(related); report != "" {
				contentToSummarize = report
				sourceDescription = fmt.Sprintf("Referenced Task (%s)", related.ID)
				break
			}
		}
	}

	if contentToSummarize == "" && len(execCtx.Message.ReferenceTasks) > 0 {
		msg := a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(
			fmt.Sprintf("Warning: I couldn't find a 'Deep Research Report' in the referenced task(s). Please check the Task ID: %v", execCtx.Message.ReferenceTasks),
		))
		yield(msg, nil)
		return e.finalizeTask(ctx, execCtx, yield)
	}

	if contentToSummarize == "" {
		contentToSummarize = input
	}
	log.Printf("[Task: %s] Summarizing from %s", execCtx.TaskID, sourceDescription)

	var summary string
	if e.genaiClient != nil {
		prompt := fmt.Sprintf("Summarize the following content into a single paragraph of 3 concise sentences:\n\n%s", contentToSummarize)
		resp, err := e.genaiClient.Models.GenerateContent(ctx, e.model, genai.Text(prompt), nil)
		if err == nil && len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
			summary = resp.Candidates[0].Content.Parts[0].Text
		}
	}

	if summary == "" {
		summary = "Summary unavailable."
	}

	yield(a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(summary)), nil)
	return nil
}

// finalizeTask sends a terminal status update to complete an A2A Task.
func (e *agentExecutor) finalizeTask(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool) error {
	status := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil)
	yield(status, nil)
	return nil
}
