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
	"github.com/a2aproject/a2a-go/a2asrv/eventqueue"
	"google.golang.org/genai"
)

// handleHelloWorld provides a context-aware greeting using Gemini.
func (e *agentExecutor) handleHelloWorld(ctx context.Context, reqCtx *a2asrv.RequestContext, q eventqueue.Queue, input string) error {
	var textResponse string
	if e.genaiClient != nil {
		prompt := fmt.Sprintf("The user said %q. Respond with a friendly, professional greeting as an A2A agent. Keep it concise.", input)
		resp, genErr := e.genaiClient.Models.GenerateContent(ctx, e.model, genai.Text(prompt), nil)
		if genErr != nil {
			log.Printf("[Task: %s] Gemini Error: %v", reqCtx.TaskID, genErr)
			textResponse = "Hello! (Gemini error, fallback)"
		} else if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
			textResponse = resp.Candidates[0].Content.Parts[0].Text
		}
	}
	if textResponse == "" {
		textResponse = "Hello from the Simple A2A Server!"
	}
	if err := q.Write(ctx, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: textResponse})); err != nil {
		return err
	}
	// Finalize Task to ensure ID propagation
	return e.finalizeTask(ctx, reqCtx, q)
}

// handleEcho simple echoes the user input.
func (e *agentExecutor) handleEcho(ctx context.Context, reqCtx *a2asrv.RequestContext, q eventqueue.Queue, input string) error {
	if err := q.Write(ctx, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: fmt.Sprintf("You said: %s", input)})); err != nil {
		return err
	}
	return e.finalizeTask(ctx, reqCtx, q)
}

// handleAdminEcho echoes input only if the user is authenticated as Admin.
func (e *agentExecutor) handleAdminEcho(ctx context.Context, reqCtx *a2asrv.RequestContext, q eventqueue.Queue, input string) error {
	callCtx, ok := a2asrv.CallContextFrom(ctx)
	if !ok {
		return q.Write(ctx, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: "Error: Call context missing."}))
	}
	if err := q.Write(ctx, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: fmt.Sprintf("Admin %s says: %s", callCtx.User.Name(), input)})); err != nil {
		return err
	}
	return e.finalizeTask(ctx, reqCtx, q)
}

// handleStatefulInteraction manages long-running research or chat sessions via the Interactions API.
func (e *agentExecutor) handleStatefulInteraction(ctx context.Context, reqCtx *a2asrv.RequestContext, q eventqueue.Queue, input string, isResearch bool) error {
	if e.interactionsClient == nil {
		return q.Write(ctx, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: "Stateful logic is unavailable: Interactions API key not configured."}))
	}

	// Session Recovery logic
	var prevInteractionID string
	if reqCtx.StoredTask != nil && reqCtx.StoredTask.Metadata != nil {
		if val, ok := reqCtx.StoredTask.Metadata["gemini_interaction_id"].(string); ok {
			prevInteractionID = val
		}
	}
	if prevInteractionID == "" {
		for _, related := range reqCtx.RelatedTasks {
			if related.Metadata != nil {
				if val, ok := related.Metadata["gemini_interaction_id"].(string); ok {
					prevInteractionID = val
					log.Printf("[Task: %s] Found Gemini session from referenced task: %s", reqCtx.TaskID, related.ID)
					break
				}
			}
		}
	}

	agent := ""
	if isResearch {
		agent = "deep-research-pro-preview-12-2025"
		log.Printf("[Task: %s] Starting Deep Research for: %q", reqCtx.TaskID, input)
		q.Write(ctx, a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateWorking, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: "Initializing Deep Research Agent..."})))
	} else {
		log.Printf("[Task: %s] Starting Chat for: %q", reqCtx.TaskID, input)
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
		log.Printf("[Task: %s] Interactions API Error: %v", reqCtx.TaskID, err)
		return q.Write(ctx, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: fmt.Sprintf("Failed to start interaction: %v", err)}))
	}

	// Update Task Metadata with new Interaction ID
	if reqCtx.Metadata == nil {
		reqCtx.Metadata = make(map[string]any)
	}
	reqCtx.Metadata["gemini_interaction_id"] = resp.ID

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

			log.Printf("[Task: %s] Interaction %s status: %s", reqCtx.TaskID, resp.ID, current.Status)
			status := strings.ToLower(current.Status)
			if status != "working" && status != "in_progress" && status != "pending" {
				finalResp = current
				goto Finished
			}

			if isResearch {
				statusMsg := a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: fmt.Sprintf("Deep Research in progress (Status: %s)...", current.Status)})
				q.Write(ctx, a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateWorking, statusMsg))
			}
		}
	}

Finished:
	if finalResp == nil || finalResp.Error != nil {
		errMsg := "Interaction failed."
		if finalResp != nil && finalResp.Error != nil {
			errMsg = fmt.Sprintf("Interaction failed: %s", finalResp.Error.Message)
		}
		return q.Write(ctx, a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateFailed, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: errMsg})))
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
		artifactEvent := a2a.NewArtifactEvent(reqCtx, a2a.TextPart{Text: resultText})
		artifactEvent.Artifact.Name = "Deep Research Report"
		artifactEvent.Artifact.Description = fmt.Sprintf("Research result for: %s", input)
		q.Write(ctx, artifactEvent)
	} else {
		q.Write(ctx, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: resultText}))
	}

	return e.finalizeTask(ctx, reqCtx, q)
}

// handleSummarize handles the summarization of input or referenced task artifacts.
func (e *agentExecutor) handleSummarize(ctx context.Context, reqCtx *a2asrv.RequestContext, q eventqueue.Queue, input string) error {
	var contentToSummarize string
	sourceDescription := "direct input"

	findReport := func(task *a2a.Task) string {
		if task == nil {
			return ""
		}
		for _, artifact := range task.Artifacts {
			if artifact.Name == "Deep Research Report" {
				for _, part := range artifact.Parts {
					if tp, ok := part.(a2a.TextPart); ok {
						return tp.Text
					}
				}
			}
		}
		return ""
	}

	if report := findReport(reqCtx.StoredTask); report != "" {
		contentToSummarize = report
		sourceDescription = "Task history"
	} else if len(reqCtx.RelatedTasks) > 0 {
		for _, related := range reqCtx.RelatedTasks {
			if report := findReport(related); report != "" {
				contentToSummarize = report
				sourceDescription = fmt.Sprintf("Referenced Task (%s)", related.ID)
				break
			}
		}
	}

	if contentToSummarize == "" && len(reqCtx.Message.ReferenceTasks) > 0 {
		msg := a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{
			Text: fmt.Sprintf("Warning: I couldn't find a 'Deep Research Report' in the referenced task(s). Please check the Task ID: %v", reqCtx.Message.ReferenceTasks),
		})
		q.Write(ctx, msg)
		return e.finalizeTask(ctx, reqCtx, q)
	}

	if contentToSummarize == "" {
		contentToSummarize = input
	}
	log.Printf("[Task: %s] Summarizing from %s", reqCtx.TaskID, sourceDescription)

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

	if err := q.Write(ctx, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: summary})); err != nil {
		return err
	}
	return e.finalizeTask(ctx, reqCtx, q)
}

// finalizeTask sends a terminal status update to complete an A2A Task.
func (e *agentExecutor) finalizeTask(ctx context.Context, reqCtx *a2asrv.RequestContext, q eventqueue.Queue) error {
	status := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateCompleted, nil)
	status.Final = true
	return q.Write(ctx, status)
}
