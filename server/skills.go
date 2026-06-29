package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ghchinoy/cloud-interactions-go"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"google.golang.org/genai"
)

// handleHelloWorld provides a context-aware greeting using Gemini.
func (e *agentExecutor) handleHelloWorld(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool, input string) error {
	var textResponse string
	if e.genaiClient != nil {
		prompt := fmt.Sprintf("The user said %q. Respond with a friendly, professional greeting as an A2A agent. Keep it concise.", input)
		resp, genErr := e.genaiClient.Models.GenerateContent(ctx, e.model, genai.Text(prompt), nil)
		if genErr != nil {
			slog.Warn("gemini error in hello_world", "task", execCtx.TaskID, "error", genErr)
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
					slog.Debug("recovered gemini session from referenced task", "task", execCtx.TaskID, "source_task", related.ID)
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
		slog.Info("starting deep research", "task", execCtx.TaskID, "input", input)
			yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Initializing Deep Research Agent..."))), nil)
	} else {
		slog.Info("starting chat", "task", execCtx.TaskID, "input", input)
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
		slog.Error("interactions API error", "task", execCtx.TaskID, "error", err)
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

			slog.Debug("interaction poll", "task", execCtx.TaskID, "interaction", resp.ID, "status", current.Status)
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
	if len(finalResp.Outputs) == 0 && len(finalResp.Steps) > 0 {
		lastStep := finalResp.Steps[len(finalResp.Steps)-1]
		if lastStep.Text != "" {
			resultText = lastStep.Text
		} else if len(lastStep.Content) > 0 {
			resultText = lastStep.Content[0].Text
		}
	} else if len(finalResp.Outputs) > 0 {
		if finalResp.Outputs[0].Text != "" {
			resultText = finalResp.Outputs[0].Text
		} else if len(finalResp.Outputs[0].Content) > 0 { // Change .Parts to .Content
			resultText = finalResp.Outputs[0].Content[0].Text
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
	slog.Info("summarizing", "task", execCtx.TaskID, "source", sourceDescription)

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

// handleMultimodalEcho inspects all received message parts and echoes each back as a
// separate named artifact, preserving part type, mediaType, and content. It also emits
// a final summary artifact listing all part types received.
//
// This skill is the server-side target for a2acli multi-modal input testing
// (--parts / --json / --file / --data flags). Callers can assert round-trip fidelity
// by inspecting the returned artifacts.
func (e *agentExecutor) handleMultimodalEcho(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool) error {
	// Required preamble: submit the task and transition to working state.
	if execCtx.StoredTask == nil {
		if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
			return nil
		}
	}
	if !yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil), nil) {
		return nil
	}

	if execCtx.Message == nil || len(execCtx.Message.Parts) == 0 {
		yield(a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("multimodal_echo: no parts received.")), nil)
		return nil
	}

	var summary []string

	for i, part := range execCtx.Message.Parts {
		switch content := part.Content.(type) {

		case a2a.Text:
			ev := a2a.NewArtifactEvent(execCtx, a2a.NewTextPart("text: "+string(content)))
			ev.Artifact.Name = fmt.Sprintf("part-%d-text", i)
			ev.Artifact.Description = "Echoed TextPart"
			yield(ev, nil)
			summary = append(summary, fmt.Sprintf("part %d: TextPart (%d bytes)", i, len(content)))

		case a2a.Data:
			b, err := json.Marshal(content.Value)
			if err != nil {
				b = []byte(fmt.Sprintf("<marshal error: %v>", err))
			}
			mediaType := part.MediaType
			if mediaType == "" {
				mediaType = "application/json"
			}
			ev := a2a.NewArtifactEvent(execCtx, a2a.NewTextPart(string(b)))
			ev.Artifact.Name = fmt.Sprintf("part-%d-data (%s)", i, mediaType)
			ev.Artifact.Description = "Echoed DataPart"
			yield(ev, nil)
			summary = append(summary, fmt.Sprintf("part %d: DataPart mediaType=%s (%d bytes)", i, mediaType, len(b)))

		case a2a.Raw:
			rawPart := a2a.NewRawPart([]byte(content))
			rawPart.MediaType = part.MediaType
			ev := a2a.NewArtifactEvent(execCtx, rawPart)
			ev.Artifact.Name = fmt.Sprintf("part-%d-raw", i)
			ev.Artifact.Description = fmt.Sprintf("Echoed RawPart (mediaType=%s)", part.MediaType)
			yield(ev, nil)
			summary = append(summary, fmt.Sprintf("part %d: RawPart mediaType=%s (%d bytes)", i, part.MediaType, len(content)))

		case a2a.URL:
			urlPart := a2a.NewFileURLPart(content, part.MediaType)
			ev := a2a.NewArtifactEvent(execCtx, urlPart)
			ev.Artifact.Name = fmt.Sprintf("part-%d-url", i)
			ev.Artifact.Description = fmt.Sprintf("Echoed URLPart (mediaType=%s)", part.MediaType)
			yield(ev, nil)
			summary = append(summary, fmt.Sprintf("part %d: URLPart mediaType=%s url=%s", i, part.MediaType, string(content)))

		default:
			slog.Warn("multimodal_echo: unknown part type", "task", execCtx.TaskID, "index", i, "type", fmt.Sprintf("%T", part.Content))
			summary = append(summary, fmt.Sprintf("part %d: unknown type %T (skipped)", i, part.Content))
		}
	}

	// Final summary artifact listing all part types received.
	summaryText := fmt.Sprintf("Received %d part(s):\n%s", len(execCtx.Message.Parts), strings.Join(summary, "\n"))
	summaryEv := a2a.NewArtifactEvent(execCtx, a2a.NewTextPart(summaryText))
	summaryEv.Artifact.Name = "summary"
	summaryEv.Artifact.Description = "Part-type summary for round-trip validation"
	summaryEv.LastChunk = true
	yield(summaryEv, nil)

	return e.finalizeTask(ctx, execCtx, yield)
}
